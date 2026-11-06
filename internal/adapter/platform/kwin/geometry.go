//go:build linux

package kwin

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

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

	dbusIface        = "org.freedesktop.DBus"
	dbusNameHasOwner = dbusIface + ".NameHasOwner"

	scriptFileMode = 0o600

	// scriptDirShared are the bits that would expose the script's directory to
	// anyone but its owner. KWin executes the script, so the directory must be
	// private before the file's own mode means anything.
	scriptDirShared = 0o077

	// installRetries further attempts follow a failed install, backing off
	// from installRetryBackoff and doubling (~1s, 2s, 4s). They cover a daemon
	// that started before the session bus or KWin, which resolves in seconds
	// or not at all.
	installRetries      = 3
	installRetryBackoff = time.Second
)

// scriptFileName is the KWin script's name under $XDG_RUNTIME_DIR. It is stable
// so a restarted daemon unloads its predecessor's copy instead of stacking a
// second one.
const scriptFileName = "neru-kwin-geometry.js"

// errKWinAbsent is returned to callers, not just logged, because "KWin never
// answered" and "no window is focused" are different answers and only the
// second should send a caller to the active screen.
var errKWinAbsent = errors.New("org.kde.KWin is not on the session bus")

// errBusClosed is the reason recorded when the bridge's connection went away
// under it, until the reinstall it schedules says otherwise.
var errBusClosed = errors.New("the session bus connection carrying the KWin bridge was closed")

// errNoRuntimeDir and errRuntimeDirNotPrivate are separate because a user
// fixes them differently. The first is a session started outside logind, the
// second a runtime directory pointed somewhere shared.
var (
	errNoRuntimeDir = errors.New(
		"XDG_RUNTIME_DIR is not set, so there is no private directory to load " +
			"the KWin geometry script from",
	)
	errRuntimeDirNotPrivate = errors.New(
		"XDG_RUNTIME_DIR is not a directory private to this user, so it is not " +
			"somewhere the KWin geometry script may be loaded from",
	)
)

// Window is what KWin last said about the focused window: its rectangle and
// identity. The identity lets a caller check that the rectangle and its own
// data, such as an AT-SPI frame, describe the same window.
//
// Class, Name and Title are KWin's resourceClass, resourceName and caption.
// Any can be empty, as they are a correlation key, never a requirement. Both
// application identifiers are kept because they disagree for some windows (an
// XWayland Firefox is class "firefox" and name "navigator").
type Window struct {
	Rect  image.Rectangle
	Class string
	Name  string
	Title string
}

// Geometry caches the focused window, fed by the KWin script through the
// exported D-Bus methods. It answers two questions from that one cache: the
// window's origin (AT-SPI reports element coordinates relative to it on
// Wayland) and the window's rectangle (SystemPort.FocusedWindowBounds).
type Geometry struct {
	// logger is set once by the first caller with one and read from any
	// goroutine.
	logger atomic.Pointer[zap.Logger]

	// watching is the restart watch's per-connection claim (restart_watch.go).
	watching claim

	// conn is the bridge's own session-bus connection, never the process-wide
	// dbus.SessionBus(). The tray closes that one when its loop ends, which
	// would silently end the restart watch and leave the receiver on a dead
	// connection with a stale window cached. Guarded by startMu.
	conn *dbus.Conn

	startMu   sync.Mutex
	starting  bool
	installed bool
	warned    bool

	// installer is what one attempt runs. It is a field so tests can drive the
	// claim and generation logic without a session bus.
	installer func() error

	// generation bumps whenever KWin changes hands. An attempt publishes
	// nothing unless the generation it began in is still current, so a success
	// against a departed compositor cannot mark the bridge installed or erase
	// the record of its going. Guarded by startMu.
	generation uint64

	mu       sync.RWMutex
	window   Window
	valid    bool
	startErr error
}

var (
	sharedOnce     sync.Once
	sharedGeometry *Geometry

	// nopLogger is what an unadopted cache logs through.
	nopLogger = zap.NewNop()
)

// Shared returns the process-wide KWin geometry cache. A second cache would
// mean a second owner of the D-Bus name and a second script inside KWin. The
// first non-nil logger handed in is adopted.
func Shared(logger *zap.Logger) *Geometry {
	sharedOnce.Do(func() { sharedGeometry = newGeometry(nil) })

	sharedGeometry.adoptLogger(logger)

	return sharedGeometry
}

func newGeometry(logger *zap.Logger) *Geometry {
	geometry := &Geometry{}
	geometry.installer = geometry.install
	geometry.adoptLogger(logger)

	return geometry
}

// EnsureStarted installs the KWin script off the calling goroutine and never
// blocks, because callers include the mode handler, which holds the keyboard
// grab. A call during an in-flight attempt gets what the last completed one
// learned. Waiting would only stall activation, since a freshly loaded script
// still has to call back before there is a rectangle.
func (g *Geometry) EnsureStarted() {
	if generation, ok := g.beginStart(); ok {
		go g.start(generation)
	}
}

// UpdateActiveWindow is the exported D-Bus method the KWin script calls when
// there is a focused window to report. The payload is
// "x,y,w,h,resourceClass,resourceName,caption" (a single string, to avoid KWin
// number marshaling quirks).
//
// A push that does not describe a window on screen is dropped, because the
// previous geometry is a better answer than an empty rectangle a caller cannot
// tell from a real one. ClearActiveWindow covers the case where the previous
// geometry is not an answer at all.
func (g *Geometry) UpdateActiveWindow(payload string) *dbus.Error {
	window, ok := parseWindowPayload(payload)
	if !ok {
		return nil
	}

	g.mu.Lock()
	g.window, g.valid = window, true
	g.mu.Unlock()

	// The title is window contents, so it is never logged. The class is an
	// application identity, which the app watcher already logs.
	g.log().Debug("KWin active window geometry",
		zap.Int("x", window.Rect.Min.X), zap.Int("y", window.Rect.Min.Y),
		zap.Int("w", window.Rect.Dx()), zap.Int("h", window.Rect.Dy()),
		zap.String("class", window.Class))

	return nil
}

// ClearActiveWindow is the exported D-Bus method the KWin script calls when
// activation has left every window it can report on: the desktop is focused,
// the focused window was minimized, or the last one closed.
//
// It is separate from UpdateActiveWindow because the two say opposite things
// about the cache. A malformed push failed, so the previous rectangle survives
// it. A clear says the previous rectangle describes nothing on screen, and
// keeping it would be the confidently wrong answer this bridge exists to end.
// Emptying the cache is not a failure, so callers widen to the active screen
// knowingly.
//
// The reason is a fixed word from the script. The argument exists because a
// no-argument callDBus is marshaling nothing has exercised, and a silently
// failed clear would restore the stale rectangle.
func (g *Geometry) ClearActiveWindow(reason string) *dbus.Error {
	g.invalidate()

	g.log().Debug("KWin reports no focused window", zap.String("reason", reason))

	return nil
}

// Focused returns the focused window as KWin last reported it.
//
// The three answers are distinct on purpose. A window with ok is what KWin
// says. ok=false with no error means the bridge is installed (or installing
// for the first time) and has nothing to report. A non-nil error means the
// bridge could not be installed, which a caller must not read as "there is no
// focused window".
//
// A retry in flight does not soften the last failure into the middle answer,
// or a session with no KWin would hide behind a permanent "installing". The
// reason is cleared the moment an attempt supersedes it.
func (g *Geometry) Focused() (Window, bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.valid {
		return g.window, true, nil
	}

	return Window{}, false, g.startErr
}

// Bounds returns just the focused window's on-screen client rectangle, for
// SystemPort.FocusedWindowBounds. It carries Focused's three answers unchanged.
func (g *Geometry) Bounds() (image.Rectangle, bool, error) {
	window, ok, err := g.Focused()

	return window.Rect, ok, err
}

// invalidate empties the cache, for both a clear and a compositor restart.
func (g *Geometry) invalidate() {
	g.mu.Lock()
	g.window, g.valid = Window{}, false
	g.mu.Unlock()
}

// adoptLogger keeps the first non-nil logger offered, so a caller without one
// cannot decide that the cache logs nothing.
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

// start runs an install attempt and retries failures on this goroutine before
// releasing the claim. Retrying on the next request instead would make that
// request read the failure, and for the AT-SPI origin place a screenful of
// hints wrongly. The claim is held across the sequence so no second installer
// stacks, and each failure is published as it happens so Bounds keeps naming
// the current reason during the backoff.
func (g *Geometry) start(generation uint64) {
	g.runInstall(generation, g.installer, installRetryBackoff)
}

// runInstall is start's loop with its two dependencies passed in, so the retry
// policy can be tested without a session bus or a wall-clock wait.
func (g *Geometry) runInstall(
	generation uint64,
	install func() error,
	backoff time.Duration,
) {
	for attempt := range installRetries + 1 {
		err := install()
		if err == nil || attempt == installRetries {
			g.endStart(generation, err)

			return
		}

		g.recordAttempt(generation, err)
		time.Sleep(backoff << attempt)
	}
}

// beginStart claims the right to run an install attempt and returns its
// generation. It is false when one is in flight or the script is installed, so
// callers can ask on every request.
func (g *Geometry) beginStart() (uint64, bool) {
	g.startMu.Lock()
	defer g.startMu.Unlock()

	if g.starting || g.installed {
		return 0, false
	}

	g.starting = true

	return g.generation, true
}

// endStart records the last attempt of a sequence and releases the claim, so
// the next caller can retry a failure and none can repeat a success.
//
// An attempt from a superseded generation publishes nothing, or a stale
// success would leave an empty cache with no reason, which reads as an
// unfocused desktop. It schedules a fresh attempt instead, because a
// compositor that arrived while it held the claim could not start an
// installer of its own.
func (g *Geometry) endStart(generation uint64, err error) {
	g.startMu.Lock()
	g.starting = false
	current := generation == g.generation

	if current {
		g.installed = err == nil
	}

	g.startMu.Unlock()

	if !current {
		g.log().Debug("KWin geometry install finished for a compositor that has since gone")
		g.EnsureStarted()

		return
	}

	g.recordAttempt(generation, err)
}

// forgetInstall marks the bridge uninstalled and retires every attempt in
// flight, returning the new generation so the caller can speak for it.
func (g *Geometry) forgetInstall() uint64 {
	g.startMu.Lock()
	defer g.startMu.Unlock()

	g.installed = false
	g.generation++

	return g.generation
}

// recordAttempt publishes an attempt's outcome without releasing the claim. A
// failure becomes the reason Bounds cannot answer and warns once, or a session
// with no KWin would warn on every hint activation. A success clears it. A
// superseded generation publishes nothing, for the reason endStart gives.
func (g *Geometry) recordAttempt(generation uint64, err error) {
	g.startMu.Lock()

	if generation != g.generation {
		g.startMu.Unlock()

		return
	}

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

// install arms the restart watch, then exports the D-Bus receiver and loads the
// KWin script. KWin's presence is checked before anything is exported, or a
// KDE-labeled session with no running KWin would own a bus name and write a
// script for nobody (#1430). The watch is armed ahead of that check on
// purpose, see watchKWin.
func (g *Geometry) install() error {
	conn, err := g.connection()
	if err != nil {
		return fmt.Errorf("session bus: %w", err)
	}

	g.watchKWin(conn)

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
		g.log().Warn("KWin bridge could not own its bus name",
			zap.Error(nameErr), zap.Int("reply", int(reply)))
		// Continue, as the export still receives calls if the name frees up.
	}

	return g.installScript(conn)
}

// connection returns the bridge's own bus connection, dialing one when there
// is none or the last was closed. The old watcher releases its claim
// (connectionClosed), and install re-arms everything on the new one.
func (g *Geometry) connection() (*dbus.Conn, error) {
	g.startMu.Lock()
	defer g.startMu.Unlock()

	if g.conn != nil && g.conn.Connected() {
		return g.conn, nil
	}

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}

	g.conn = conn

	return conn, nil
}

// connectionClosed is how a watcher reports that its channel closed. It is
// true only for the connection the bridge still holds, so a watcher whose
// connection was already replaced cannot empty the successor's cache or
// schedule a third install.
func (g *Geometry) connectionClosed(conn *dbus.Conn) bool {
	g.startMu.Lock()
	defer g.startMu.Unlock()

	if g.conn != conn {
		return false
	}

	g.conn = nil

	return true
}

// installScript writes the KWin script to disk, then loads and starts it.
func (g *Geometry) installScript(conn *dbus.Conn) error {
	dir, dirErr := scriptDir()
	if dirErr != nil {
		return dirErr
	}

	path := filepath.Join(dir, scriptFileName)

	writeErr := writeScript(path)
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

// scriptDir resolves the directory the geometry script is loaded from, which
// is $XDG_RUNTIME_DIR and nowhere else. Never fall back to os.TempDir(), as a
// fixed-name file in world-writable /tmp that the compositor executes is not
// safe. A logind session always has a runtime directory.
//
// The mode is checked because the XDG spec requires the directory to be
// private (logind creates it 0700), and a runtime directory pointed somewhere
// shared is the same exposure. Ownership is not checked, because a 0700
// directory owned by someone else fails the write anyway.
func scriptDir() (string, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return "", errNoRuntimeDir
	}

	// Stat, not Lstat, because a symlinked runtime directory is judged by what
	// it resolves to, which is the directory the script would really land in.
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("XDG_RUNTIME_DIR: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("%w: %s is not a directory at all", errRuntimeDirNotPrivate, dir)
	}

	if info.Mode().Perm()&scriptDirShared != 0 {
		return "", fmt.Errorf(
			"%w: %s is mode %04o, wanted 0700",
			errRuntimeDirNotPrivate, dir, info.Mode().Perm(),
		)
	}

	return dir, nil
}

// writeScript writes a fresh file beside path and renames it over it, as the
// portal restore token is written
// (internal/adapter/platform/linux/portal_restore_token.go). The file gets
// this package's mode rather than inheriting a stray copy's, and the path
// only ever names a complete script, so KWin never executes half of one.
func writeScript(path string) error {
	// A unique temp name keeps leftovers recognizable and racing daemons off
	// each other's half-written copy.
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}

	defer func() { _ = os.Remove(temp.Name()) }()

	err = temp.Chmod(scriptFileMode)
	if err != nil {
		_ = temp.Close()

		return err
	}

	_, err = temp.WriteString(geometryScript)
	if err != nil {
		_ = temp.Close()

		return err
	}

	err = temp.Close()
	if err != nil {
		return err
	}

	return os.Rename(temp.Name(), path)
}
