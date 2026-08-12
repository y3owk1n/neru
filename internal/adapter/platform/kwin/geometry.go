//go:build linux

package kwin

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"
)

const (
	bridgeName  = "org.neru.KWinBridge"
	bridgePath  = "/org/neru/KWinBridge"
	bridgeIface = "org.neru.KWinBridge"

	scriptingDest  = "org.kde.KWin"
	scriptingPath  = "/Scripting"
	scriptingIface = "org.kde.kwin.Scripting"

	dbusNameHasOwner = "org.freedesktop.DBus.NameHasOwner"

	// UpdateActiveWindow payload is "x,y,w,h,resourceClass": up to 5 fields,
	// with the geometry minimum being the first 4 (resourceClass is optional).
	payloadParts    = 5
	payloadMinParts = 4

	// The geometry script is only readable/writable by the owner.
	scriptFileMode = 0o600
)

// scriptFileName is the KWin script's name under $XDG_RUNTIME_DIR. It is stable
// so a restarted daemon unloads its predecessor's copy instead of stacking a
// second one.
const scriptFileName = "neru-kwin-geometry.js"

// errKWinAbsent is the reason recorded on a session where KWin is not on the
// bus at all. It is returned to callers rather than logged and forgotten,
// because "KWin never answered" and "no window is focused" are different
// answers and only one of them should send a caller to the active screen.
var errKWinAbsent = errors.New("org.kde.KWin is not on the session bus")

// geometryScript is loaded into KWin. It reports the focused window's client
// geometry (content rect, excludes the titlebar so it aligns 1:1 with the
// AT-SPI content origin). It ignores Neru's own overlay so that activating the
// hint overlay does not overwrite the real window's geometry.
//
// neruIgnore filters out non-application surfaces that briefly take activation
// but are never hint targets: panels/docks/OSD/popups/tooltips/utility windows
// (caught by KWin's window-type flags), plus a few known transient classes
// (the XWayland video bridge, plasmashell, and the portal consent dialog).
// Without this, focus flicking to e.g. plasmashell or the RemoteDesktop consent
// dialog would clobber the real window origin and mis-offset hint clicks.
// Accessing an absent KWin property yields undefined (falsy), so listing extra
// type flags is safe across KWin versions.
const geometryScript = `
function neruIgnore(c) {
    if (!c) return true;
    if (c.resourceClass == "neru") return true;
    if (c.specialWindow || c.dock || c.desktopWindow || c.splash ||
        c.utility || c.toolbar || c.menu || c.dropdownMenu || c.popupMenu ||
        c.tooltip || c.notification || c.criticalNotification ||
        c.onScreenDisplay || c.comboBox || c.dndIcon) return true;
    var cls = ("" + c.resourceClass).toLowerCase();
    if (cls == "xwaylandvideobridge" || cls == "plasmashell" ||
        cls == "org.kde.plasmashell" ||
        cls == "org.freedesktop.impl.portal.desktop.kde") return true;
    return false;
}
function neruPush(c) {
    if (neruIgnore(c)) return;
    var g = c.clientGeometry ? c.clientGeometry : c.frameGeometry;
    if (!g) return;
    callDBus("org.neru.KWinBridge", "/org/neru/KWinBridge", "org.neru.KWinBridge",
             "UpdateActiveWindow",
             "" + Math.round(g.x) + "," + Math.round(g.y) + "," +
             Math.round(g.width) + "," + Math.round(g.height) + "," + c.resourceClass);
}
workspace.windowActivated.connect(neruPush);
neruPush(workspace.activeWindow);
`

// Geometry caches the focused window's on-screen client rectangle, fed by the
// KWin script through the exported D-Bus method. It answers two questions from
// that one cache: the window's origin (AT-SPI reports element coordinates
// relative to it on Wayland) and the window's rectangle
// (SystemPort.FocusedWindowBounds).
type Geometry struct {
	// logger is whatever the first caller with one handed in. Both callers
	// reach this through Shared and only one of them carries a logger, so it
	// is set once and read from any goroutine.
	logger atomic.Pointer[zap.Logger]

	startMu   sync.Mutex
	starting  bool
	installed bool
	warned    bool

	mu       sync.RWMutex
	rect     image.Rectangle
	valid    bool
	startErr error
}

var (
	sharedOnce     sync.Once
	sharedGeometry *Geometry

	// nopLogger is what an unadopted cache logs through.
	nopLogger = zap.NewNop()
)

// Shared returns the process-wide KWin geometry cache. There is one because a
// second means a second owner of the D-Bus name and a second copy of the script
// inside KWin, and because a cache per caller is a fact per caller.
//
// The first non-nil logger handed in is adopted, so the caller that has one
// names the log lines and the caller that does not silences nothing.
func Shared(logger *zap.Logger) *Geometry {
	sharedOnce.Do(func() { sharedGeometry = newGeometry(nil) })

	sharedGeometry.adoptLogger(logger)

	return sharedGeometry
}

func newGeometry(logger *zap.Logger) *Geometry {
	geometry := &Geometry{}
	geometry.adoptLogger(logger)

	return geometry
}

// EnsureStarted installs the KWin script, off the calling goroutine. It never
// blocks: the install is a handful of D-Bus round trips to KWin, and the
// callers reaching it include the mode handler, where a slow native call holds
// the keyboard grab.
//
// Until the install lands, Bounds reports nothing rather than a rectangle. A
// failed attempt is retried by the next caller — the daemon starts when the
// session does, and spending its only attempt on a bus that was not up yet
// would leave the whole run with no geometry.
func (g *Geometry) EnsureStarted() {
	if g.beginStart() {
		go g.start()
	}
}

// UpdateActiveWindow is the exported D-Bus method the KWin script calls. The
// payload is "x,y,w,h,resourceClass" (a single string, to avoid KWin number
// marshaling quirks).
//
// A push that does not describe a window on screen is dropped rather than
// cached: the previous geometry is a better answer than an empty rectangle,
// which a caller cannot tell from a real one.
func (g *Geometry) UpdateActiveWindow(payload string) *dbus.Error {
	parts := strings.SplitN(payload, ",", payloadParts)
	if len(parts) < payloadMinParts {
		return nil
	}

	originX, errX := strconv.Atoi(strings.TrimSpace(parts[0]))
	originY, errY := strconv.Atoi(strings.TrimSpace(parts[1]))
	width, errW := strconv.Atoi(strings.TrimSpace(parts[2]))
	height, errH := strconv.Atoi(strings.TrimSpace(parts[3]))

	if errX != nil || errY != nil || errW != nil || errH != nil {
		return nil //nolint:nilerr // best-effort: malformed payloads are ignored, not surfaced to KWin.
	}

	if width <= 0 || height <= 0 {
		return nil
	}

	class := ""
	if len(parts) == payloadParts {
		class = parts[4]
	}

	g.mu.Lock()
	g.rect, g.valid = image.Rect(originX, originY, originX+width, originY+height), true
	g.mu.Unlock()

	g.log().Debug("KWin active window geometry",
		zap.Int("x", originX), zap.Int("y", originY),
		zap.Int("w", width), zap.Int("h", height),
		zap.String("class", class))

	return nil
}

// Bounds returns the focused window's on-screen client rectangle as KWin last
// reported it.
//
// The three answers are distinct on purpose. A rectangle with ok is what KWin
// says. ok=false with no error means the bridge is installed (or still
// installing) and has been told about no window — the desktop is focused, or
// activation has not moved since the daemon started. A non-nil error means the
// bridge could not be installed at all, which is the one case a caller must not
// read as "there is no focused window".
func (g *Geometry) Bounds() (image.Rectangle, bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.valid {
		return g.rect, true, nil
	}

	return image.Rectangle{}, false, g.startErr
}

// adoptLogger takes the first real logger offered and keeps it. Callers reach
// this cache from wiring that may or may not carry one, and whichever gets
// there first should not decide that it logs nothing.
func (g *Geometry) adoptLogger(logger *zap.Logger) {
	if logger == nil {
		return
	}

	g.logger.CompareAndSwap(nil, logger.Named("kwin"))
}

func (g *Geometry) log() *zap.Logger {
	if adopted := g.logger.Load(); adopted != nil {
		return adopted
	}

	return nopLogger
}

func (g *Geometry) start() {
	g.endStart(g.install())
}

// beginStart claims the right to run an install attempt. It is false when one
// is already in flight or the script is installed, so callers can ask on every
// request without stacking attempts or reinstalling.
func (g *Geometry) beginStart() bool {
	g.startMu.Lock()
	defer g.startMu.Unlock()

	if g.starting || g.installed {
		return false
	}

	g.starting = true

	return true
}

// endStart records an attempt's outcome. A failure is remembered as the reason
// Bounds cannot answer — and said out loud once, since the error reaches every
// later caller anyway and a session with no KWin would otherwise warn on every
// hint activation. A success clears the reason: it described a condition that
// has passed.
func (g *Geometry) endStart(err error) {
	g.startMu.Lock()
	g.starting = false
	g.installed = err == nil
	firstFailure := err != nil && !g.warned
	g.warned = g.warned || err != nil
	g.startMu.Unlock()

	g.mu.Lock()
	g.startErr = err
	g.mu.Unlock()

	switch {
	case err == nil:
		g.log().Debug("KWin geometry script installed")
	case firstFailure:
		g.log().Warn("KWin geometry script unavailable", zap.Error(err))
	default:
		g.log().Debug("KWin geometry script still unavailable", zap.Error(err))
	}
}

// install exports the D-Bus receiver and loads the KWin script.
//
// KWin's presence is checked before anything leaves this process. The bridge is
// reached from a backend decision, not from an environment guess, but a
// KDE-labeled session with no running KWin would otherwise still own a bus
// name and write a script into $XDG_RUNTIME_DIR for nobody to read (#1430).
func (g *Geometry) install() error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return fmt.Errorf("session bus: %w", err)
	}

	var hasOwner bool

	ownerErr := conn.BusObject().Call(dbusNameHasOwner, 0, scriptingDest).Store(&hasOwner)
	if ownerErr != nil {
		return fmt.Errorf("NameHasOwner(%s): %w", scriptingDest, ownerErr)
	}

	if !hasOwner {
		return errKWinAbsent
	}

	exportErr := conn.Export(g, dbus.ObjectPath(bridgePath), bridgeIface)
	if exportErr != nil {
		return fmt.Errorf("export %s: %w", bridgePath, exportErr)
	}

	reply, nameErr := conn.RequestName(bridgeName, dbus.NameFlagReplaceExisting)
	if nameErr != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		g.log().Warn("KWin bridge: could not own name",
			zap.Error(nameErr), zap.Int("reply", int(reply)))
		// Continue anyway: the export may still receive calls if another
		// instance relinquishes the name.
	}

	return g.installScript(conn)
}

// installScript writes the KWin script to disk and loads + starts it.
func (g *Geometry) installScript(conn *dbus.Conn) error {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}

	path := filepath.Join(dir, scriptFileName)

	writeErr := os.WriteFile(path, []byte(geometryScript), scriptFileMode)
	if writeErr != nil {
		return writeErr
	}

	obj := conn.Object(scriptingDest, scriptingPath)

	// Best-effort unload of a stale copy from a previous run.
	_ = obj.Call(scriptingIface+".unloadScript", 0, path).Err

	var id int32

	loadErr := obj.Call(scriptingIface+".loadScript", 0, path).Store(&id)
	if loadErr != nil {
		return fmt.Errorf("loadScript: %w", loadErr)
	}

	startErr := obj.Call(scriptingIface+".start", 0).Err
	if startErr != nil {
		return fmt.Errorf("start: %w", startErr)
	}

	return nil
}
