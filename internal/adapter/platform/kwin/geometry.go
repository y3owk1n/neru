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

	// The geometry script is only readable/writable by the owner.
	scriptFileMode = 0o600

	// scriptDirShared are the permission bits that would make the directory
	// holding the script reachable by anyone but its owner. The script is code
	// KWin executes, so the directory it is written into has to be private
	// before the file's own mode means anything.
	scriptDirShared = 0o077

	// installRetries is how many further attempts the installer makes on its
	// own after a failed one, and installRetryBackoff the wait before the
	// first of them, doubling each time (~1s, 2s, 4s). The window being
	// covered is a daemon that started before the session bus or KWin did,
	// which resolves in seconds or not at all.
	installRetries      = 3
	installRetryBackoff = time.Second
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

// errNoRuntimeDir and errRuntimeDirNotPrivate are the two ways a session can
// fail to offer somewhere the geometry script may live. They are separate
// because a user fixes them differently: the first is a session started outside
// logind, the second a runtime directory pointed somewhere it should not be.
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

// Window is what KWin last said about the focused window: where it is, and
// which window it is.
//
// The identity travels with the rectangle because a caller pairing the
// rectangle with something of its own — the AT-SPI frame it is about to walk —
// has no other way to tell that the two describe the same window.
//
// Class, Name and Title are KWin's resourceClass, resourceName and caption, and
// any of them can be empty: they are a correlation key, never a requirement.
// Both application identifiers are here because they disagree for some windows
// (an XWayland Firefox is class "firefox" and name "navigator") and a reader
// comparing against a third spelling of the same application should be able to
// match either.
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
	// logger is whatever the first caller with one handed in. Both callers
	// reach this through Shared and only one of them carries a logger, so it
	// is set once and read from any goroutine.
	logger atomic.Pointer[zap.Logger]

	// watching is the restart watch's one-shot claim (restart_watch.go).
	watching claim

	startMu   sync.Mutex
	starting  bool
	installed bool
	warned    bool

	// installer is what one attempt runs. It is a field rather than a method
	// call so a test can drive the whole claim-and-generation machinery —
	// including the reinstall that a retired attempt has to schedule — without
	// a session bus to install into.
	installer func() error

	// generation counts how many times what an install attempt is *about* has
	// stopped being true — KWin changing hands underneath one in flight. An
	// attempt carries the generation it began in and publishes nothing unless
	// that is still the current one, so a success against a compositor that has
	// since gone cannot mark the bridge installed or erase the record of its
	// going. Guarded by startMu.
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
	geometry.installer = geometry.install
	geometry.adoptLogger(logger)

	return geometry
}

// EnsureStarted installs the KWin script, off the calling goroutine. It never
// blocks: the install is a handful of D-Bus round trips to KWin, and the
// callers reaching it include the mode handler, where a slow native call holds
// the keyboard grab.
//
// Until the install lands, Bounds reports nothing rather than a rectangle, and
// a call that arrives while an attempt is in flight still gets whatever the
// last completed attempt learned. Waiting for the in-flight one instead would
// not answer it either — a script that loads this instant still has to call
// back before there is a rectangle to report — so the wait would buy a stall on
// the activation path and nothing else. The install converging without a caller
// is what start covers.
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
// A push that does not describe a window on screen is dropped rather than
// cached: the previous geometry is a better answer than an empty rectangle,
// which a caller cannot tell from a real one. That is why there is a second
// method — ClearActiveWindow — for the case where the previous geometry is not
// an answer at all.
func (g *Geometry) UpdateActiveWindow(payload string) *dbus.Error {
	window, ok := parseWindowPayload(payload)
	if !ok {
		return nil
	}

	g.mu.Lock()
	g.window, g.valid = window, true
	g.mu.Unlock()

	// The title is a window's contents by any reasonable reading — a document
	// name, a chat, a page — so it is correlated with and never logged. The
	// class is an application identity, which the app watcher already logs.
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
// It is a separate method rather than a shape of UpdateActiveWindow's payload
// because the two say opposite things about the cache. A malformed or
// degenerate push is a push that failed, and the previous rectangle survives it
// as the best available answer. This is KWin saying the previous rectangle
// describes nothing on screen, and keeping it would answer "the focused window
// is here" with a rectangle no window is in — the confidently wrong answer this
// bridge exists to end.
//
// Emptying the cache is not a failure: the bridge is installed and working, and
// what it reports is that nothing is focused. Callers widen to the active
// screen knowingly, which is what they should do.
//
// The reason is a fixed word from the script naming which of those happened. It
// takes an argument at all so the call has the same shape as UpdateActiveWindow
// — a KWin script's callDBus with no arguments would be the one call here whose
// marshaling nothing has ever exercised, and a clear that silently failed would
// restore exactly the stale rectangle this method exists to remove.
func (g *Geometry) ClearActiveWindow(reason string) *dbus.Error {
	g.invalidate()

	g.log().Debug("KWin reports no focused window", zap.String("reason", reason))

	return nil
}

// Focused returns the focused window as KWin last reported it.
//
// The three answers are distinct on purpose. A window with ok is what KWin
// says. ok=false with no error means the bridge is installed (or installing for
// the first time) and has nothing to report — the desktop is focused, the
// focused window was minimized or closed, or activation has not moved since the
// daemon started. A non-nil error means the bridge could not be installed,
// which is the one case a caller must not read as "there is no focused window".
//
// A retry already in flight does not soften the last failure into that middle
// answer. It is the failure that is true right now, and reporting "installing"
// on every call would hide a session with no KWin behind a permanent maybe —
// the silent widening to the active screen this bridge exists to end. The
// reason is cleared the moment an attempt supersedes it.
func (g *Geometry) Focused() (Window, bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.valid {
		return g.window, true, nil
	}

	return Window{}, false, g.startErr
}

// Bounds returns just the focused window's on-screen client rectangle, for the
// caller that has nothing to correlate it against — SystemPort.FocusedWindowBounds
// answers about "the focused window" as such, so KWin's own answer to that is
// the whole of it. It carries Focused's three answers unchanged.
func (g *Geometry) Bounds() (image.Rectangle, bool, error) {
	window, ok, err := g.Focused()

	return window.Rect, ok, err
}

// invalidate empties the cache without saying anything about why. It is what a
// clear and a compositor restart have in common.
func (g *Geometry) invalidate() {
	g.mu.Lock()
	g.window, g.valid = Window{}, false
	g.mu.Unlock()
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

// start runs an install attempt and, when it fails, retries it a few times on
// this goroutine before releasing the claim.
//
// Retrying only when the next caller arrives heals the session but not that
// caller: the request that triggers the retry is the one that reads the
// previous failure, and for the AT-SPI origin it is the one that places a
// screenful of hints at the wrong position. Retrying here costs the request
// path nothing, because no request ever waits for this goroutine.
//
// The claim is held across the whole sequence, so a request arriving mid-retry
// does not stack a second installer; each failure is still published as it
// happens, so Bounds keeps naming the current reason rather than going quiet
// for the length of the backoff.
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

// beginStart claims the right to run an install attempt and hands back the
// generation that attempt is about. It is false when one is already in flight or
// the script is installed, so callers can ask on every request without stacking
// attempts or reinstalling.
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
// An attempt from a superseded generation publishes nothing. It was about a
// compositor that has since changed hands, so its success is not evidence that
// this one is installed, and its outcome must not overwrite what the departure
// recorded — a stale success would otherwise leave an empty cache with no
// reason, which reads as an unfocused desktop and sends callers silently back to
// the active screen.
//
// It schedules a fresh attempt on its way out instead, because it was holding
// the claim for the whole time it was stale: a compositor that arrived in that
// window found the claim taken, could not start an installer of its own, and has
// nobody else coming. Leaving that to the next caller would mean the request
// that discovers it is the one that pays, which for the AT-SPI origin is a
// screenful of hints placed window-relative.
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

// forgetInstall puts the bridge back where it was before it ever installed
// anything and retires every attempt in flight, returning the generation that
// replaces them so the caller can speak for the new state itself.
func (g *Geometry) forgetInstall() uint64 {
	g.startMu.Lock()
	defer g.startMu.Unlock()

	g.installed = false
	g.generation++

	return g.generation
}

// recordAttempt publishes an attempt's outcome without releasing the claim, for
// the failures the installer is about to retry itself.
//
// A failure is remembered as the reason Bounds cannot answer — and said out
// loud once, since the error reaches every later caller anyway and a session
// with no KWin would otherwise warn on every hint activation. A success clears
// the reason: it described a condition that has passed.
//
// A superseded generation publishes nothing, for the reason endStart gives.
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
// KWin script.
//
// KWin's presence is checked before anything is exported. The bridge is reached
// from a backend decision, not from an environment guess, but a KDE-labeled
// session with no running KWin would otherwise still own a bus name and write a
// script into $XDG_RUNTIME_DIR for nobody to read (#1430). The watch is armed
// ahead of that check on purpose — watchKWin says why it is not the same kind
// of act.
func (g *Geometry) install() error {
	conn, err := dbus.SessionBus()
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
		g.log().Warn("KWin bridge: could not own name",
			zap.Error(nameErr), zap.Int("reply", int(reply)))
		// Continue anyway: the export may still receive calls if another
		// instance relinquishes the name.
	}

	return g.installScript(conn)
}

// installScript writes the KWin script to disk and loads + starts it.
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
// is $XDG_RUNTIME_DIR and nowhere else.
//
// It used to fall back to os.TempDir() when the variable was unset, which is
// /tmp: a directory every user on the machine can write into, holding a file
// with a fixed name, whose contents the compositor then executes. There is no
// version of that which is safe to keep, and the answer to a session with no
// runtime directory is to say so — a KDE session started by logind always has
// one, so nothing that works today stops working, and a session that does not
// have one gets the same loud, recorded reason as a session with no KWin.
//
// The mode is checked rather than assumed. The XDG base directory
// specification requires $XDG_RUNTIME_DIR to be owned by the user and
// accessible to nobody else, and logind creates it 0700, so the check is a
// formality on every real session; what it catches is a runtime directory
// pointed somewhere shared, which is the same exposure the fallback was.
//
// The mode is the whole of the check because ownership adds nothing a caller
// could act on: a 0700 directory belonging to somebody else is one this
// process cannot write into either, so the write below fails and says so. What
// matters is that nobody else can write into the directory the compositor
// loads from, and that is what the mode says.
func scriptDir() (string, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return "", errNoRuntimeDir
	}

	// Stat, not Lstat: a symlinked runtime directory is judged by what it
	// resolves to, which is the directory the script would really land in.
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

// writeScript puts the script at path by writing a fresh file beside it and
// renaming it over whatever was there, the way the portal restore token is
// written (internal/adapter/platform/linux/portal_restore_token.go).
//
// Two things follow from that which truncating in place does not give. The new
// file carries this package's mode whatever an earlier version or a stray copy
// left behind, instead of inheriting it; and the path only ever names a
// complete script, so the loadScript that follows cannot hand KWin half of one
// — which matters more here than for a token, because KWin executes what it
// reads.
func writeScript(path string) error {
	// The temporary file is named after the script it will become, so a
	// leftover is recognizable and two daemons racing to install cannot write
	// over each other's half-written copy.
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}

	// From here on the temporary file must not be left behind on any path.
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
