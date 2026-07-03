//go:build linux

// internal/core/infra/accessibility/kwin_geometry_linux.go
// KWin geometry bridge: under Wayland a client cannot know its own absolute
// screen position, and AT-SPI therefore reports window-relative coordinates.
// This installs a small KWin script that pushes the focused window's on-screen
// client geometry to a neru D-Bus service whenever focus changes, so hints can
// be offset into true screen coordinates.
// It does NOT move the cursor or read the accessibility tree; it only supplies
// the active window's origin.

package accessibility

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"
)

const (
	kwinBridgeName  = "org.neru.KWinBridge"
	kwinBridgePath  = "/org/neru/KWinBridge"
	kwinBridgeIface = "org.neru.KWinBridge"

	kwinScriptingDest  = "org.kde.KWin"
	kwinScriptingPath  = "/Scripting"
	kwinScriptingIface = "org.kde.kwin.Scripting"

	// UpdateActiveWindow payload is "x,y,w,h,resourceClass": up to 5 fields,
	// with the geometry minimum being the first 4 (resourceClass is optional).
	kwinGeometryPayloadParts = 5
	kwinGeometryMinParts     = 4

	// KWin geometry script is only readable/writable by the owner.
	kwinScriptFileMode = 0o600

	// kwinOriginSizeTolerance is the max per-dimension difference (px) allowed
	// between the cached KWin client geometry and the AT-SPI frame extents
	// before the cached origin is considered stale. Qt apps match exactly;
	// GTK client-side decorations can differ by a titlebar's worth of pixels.
	kwinOriginSizeTolerance = 32
)

// kwinGeometryScript is loaded into KWin. It reports the focused window's
// client geometry (content rect, excludes the titlebar so it aligns 1:1 with
// the AT-SPI content origin). It ignores neru's own overlay so that activating
// the hint overlay does not overwrite the real window's geometry.
//
// neruIgnore filters out non-application surfaces that briefly take activation
// but are never hint targets: panels/docks/OSD/popups/tooltips/utility windows
// (caught by KWin's window-type flags), plus a few known transient classes
// (the XWayland video bridge, plasmashell, and the portal consent dialog).
// Without this, focus flicking to e.g. plasmashell or the RemoteDesktop consent
// dialog would clobber the real window origin and mis-offset hint clicks.
// Accessing an absent KWin property yields undefined (falsy), so listing extra
// type flags is safe across KWin versions.
const kwinGeometryScript = `
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

// kwinBridge caches the focused window's on-screen origin, fed by the KWin
// script via the exported D-Bus method.
type kwinBridge struct {
	logger *zap.Logger

	mu    sync.RWMutex
	x     int
	y     int
	w     int
	h     int
	cls   string
	valid bool
}

func newKWinBridge(logger *zap.Logger) *kwinBridge {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &kwinBridge{logger: logger.Named("accessibility.kwin")}
}

// UpdateActiveWindow is the exported D-Bus method the KWin script calls. The
// payload is "x,y,w,h,resourceClass" (single string to avoid KWin number
// marshaling quirks).
func (b *kwinBridge) UpdateActiveWindow(payload string) *dbus.Error {
	parts := strings.SplitN(payload, ",", kwinGeometryPayloadParts)
	if len(parts) < kwinGeometryMinParts {
		return nil
	}

	originX, errX := strconv.Atoi(strings.TrimSpace(parts[0]))
	originY, errY := strconv.Atoi(strings.TrimSpace(parts[1]))
	width, errW := strconv.Atoi(strings.TrimSpace(parts[2]))
	height, errH := strconv.Atoi(strings.TrimSpace(parts[3]))

	if errX != nil || errY != nil || errW != nil || errH != nil {
		return nil //nolint:nilerr // best-effort: malformed payloads are ignored, not surfaced to KWin.
	}

	cls := ""
	if len(parts) == kwinGeometryPayloadParts {
		cls = parts[4]
	}

	b.mu.Lock()
	b.x, b.y, b.w, b.h, b.cls, b.valid = originX, originY, width, height, cls, true
	b.mu.Unlock()

	b.logger.Debug("KWin active window geometry",
		zap.Int("x", originX), zap.Int("y", originY),
		zap.Int("w", width), zap.Int("h", height),
		zap.String("cls", cls))

	return nil
}

// origin returns the cached focused-window screen origin.
func (b *kwinBridge) origin() (int, int, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.x, b.y, b.valid
}

// originFor returns the cached origin only when the cached client size matches
// the given AT-SPI frame size within kwinOriginSizeTolerance. The cache is fed
// by KWin focus events; if KWin missed a transition (or deliberately ignored a
// transient surface that became the active AT-SPI frame) the cached origin
// belongs to a different window, and offsetting by it would land every hint at
// the previous window's screen position. A size mismatch is the cheapest
// reliable staleness signal, so callers then fall back to unoffset
// (window-relative) coordinates.
func (b *kwinBridge) originFor(frameW, frameH int) (int, int, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.valid {
		return 0, 0, false
	}

	if absInt(b.w-frameW) > kwinOriginSizeTolerance ||
		absInt(b.h-frameH) > kwinOriginSizeTolerance {
		b.logger.Debug("KWin origin rejected: cached size does not match AT-SPI frame",
			zap.Int("cachedW", b.w), zap.Int("cachedH", b.h),
			zap.Int("frameW", frameW), zap.Int("frameH", frameH),
			zap.String("cachedClass", b.cls))

		return 0, 0, false
	}

	return b.x, b.y, true
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}

	return v
}

// start exports the D-Bus receiver and installs the KWin script. It is
// best-effort: failures are logged and hints simply fall back to unoffset
// (window-relative) coordinates.
func (b *kwinBridge) start() {
	conn, err := dbus.SessionBus()
	if err != nil {
		b.logger.Warn("KWin bridge: no session bus", zap.Error(err))

		return
	}

	exportErr := conn.Export(b, dbus.ObjectPath(kwinBridgePath), kwinBridgeIface)
	if exportErr != nil {
		b.logger.Warn("KWin bridge: export failed", zap.Error(exportErr))

		return
	}

	reply, nameErr := conn.RequestName(kwinBridgeName, dbus.NameFlagReplaceExisting)
	if nameErr != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		b.logger.Warn("KWin bridge: could not own name",
			zap.Error(nameErr), zap.Int("reply", int(reply)))
		// Continue anyway: the export may still receive calls if another
		// instance relinquishes the name.
	}

	installErr := b.installScript(conn)
	if installErr != nil {
		b.logger.Warn("KWin bridge: script install failed", zap.Error(installErr))

		return
	}

	b.logger.Debug("KWin geometry bridge installed")
}

// installScript writes the KWin script to disk and loads + starts it.
func (b *kwinBridge) installScript(conn *dbus.Conn) error {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}

	path := filepath.Join(dir, "neru-kwin-geometry.js")

	writeErr := os.WriteFile(path, []byte(kwinGeometryScript), kwinScriptFileMode)
	if writeErr != nil {
		return writeErr
	}

	obj := conn.Object(kwinScriptingDest, kwinScriptingPath)

	// Best-effort unload of a stale copy from a previous run.
	_ = obj.Call(kwinScriptingIface+".unloadScript", 0, path).Err

	var id int32

	loadErr := obj.Call(kwinScriptingIface+".loadScript", 0, path).Store(&id)
	if loadErr != nil {
		return fmt.Errorf("loadScript: %w", loadErr)
	}

	startErr := obj.Call(kwinScriptingIface+".start", 0).Err
	if startErr != nil {
		return fmt.Errorf("start: %w", startErr)
	}

	return nil
}
