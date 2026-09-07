//go:build linux

package gnomeshell

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

const (
	busName       = "org.neru.Shell"
	objectPath    = "/org/neru/Shell"
	iface         = "org.neru.Shell"
	focusedMethod = iface + ".FocusedWindow"
	changedMember = "FocusedWindowChanged"
	changedSignal = iface + "." + changedMember

	shellName = "org.gnome.Shell"

	dbusIface           = "org.freedesktop.DBus"
	dbusNameHasOwner    = dbusIface + ".NameHasOwner"
	dbusNameOwnerMember = "NameOwnerChanged"
	dbusNameOwnerSignal = dbusIface + "." + dbusNameOwnerMember
	nameOwnerArgs       = 3
	nameOwnerNewIndex   = 2

	// stateArgs is the arity of both FocusedWindow's reply and the
	// FocusedWindowChanged signal: found, app id, title, x, y, width, height.
	stateArgs    = 7
	signalBuffer = 32
	// queryTimeout bounds the one synchronous round trip the bridge makes,
	// which runs off the request path but still should not wedge a start.
	queryTimeout = 2 * time.Second
)

// errExtensionAbsent is the reason recorded on a session where the extension
// is not on the bus: not installed, not enabled, or installed into a session
// that has not been logged into since. It is returned to callers rather than
// logged and forgotten, because "the extension never answered" and "no window
// is focused" are different answers and only one of them should send a caller
// to the active screen.
var errExtensionAbsent = errors.New(
	"the Neru GNOME Shell extension is not running (org.neru.Shell is not on the session bus)",
)

// errNotConnected is what Focused reports before the first connection attempt
// has finished, so a caller arriving ahead of the warm-up reads "not yet"
// rather than "nothing is focused".
var errNotConnected = errors.New("the GNOME Shell bridge has not connected yet")

// Window is what the extension last said about the focused window: where it
// is, and which window it is.
//
// AppID is Mutter's WM_CLASS for the window, which for a native Wayland client
// is its app_id and for an Xwayland client its X class, so it is the same
// vocabulary the foreign-toplevel protocol speaks on every other Wayland
// desktop. Title is the window title. Either can be empty: they are a
// correlation key, never a requirement.
type Window struct {
	Rect  image.Rectangle
	AppID string
	Title string
}

// Bridge caches the focused window as the extension reports it and answers
// three questions from that one cache: the window's origin, its rectangle and
// its app id.
type Bridge struct {
	logger atomic.Pointer[zap.Logger]

	startMu   sync.Mutex
	starting  bool
	connected bool
	warned    bool
	watching  bool
	installed bool
	// installHint is what the one install attempt did, kept so every later
	// "absent" answer carries the next step, not only the attempt's own.
	installHint string
	// dataDir is where the extension files go, captured on the caller's
	// goroutine when a start is claimed so the environment it reads is the
	// one the caller had.
	dataDir string

	// conn is the bridge's own session-bus connection. Not the process-wide
	// dbus.SessionBus(): closing that closes every channel subscribed on it,
	// and the tray closes it when its loop ends, which would end the signal
	// stream here with the last answer still cached and nothing to say so.
	conn *dbus.Conn

	mu       sync.RWMutex
	window   Window
	valid    bool
	startErr error

	fdOnce sync.Once
	events *os.File
	notify *os.File
}

var (
	sharedBridge *Bridge
	sharedOnce   sync.Once
	nopLogger    = zap.NewNop()
)

// Shared returns the process-wide bridge. There is one because a cache per
// caller is a fact per caller, and because the notification pipe the app
// watcher polls has to be the one the signal handler writes to.
//
// The first non-nil logger handed in is adopted, so the caller that has one
// names the log lines and the caller that does not silences nothing.
func Shared(logger *zap.Logger) *Bridge {
	sharedOnce.Do(func() { sharedBridge = newBridge(nil) })

	sharedBridge.adoptLogger(logger)

	return sharedBridge
}

func newBridge(logger *zap.Logger) *Bridge {
	bridge := &Bridge{startErr: errNotConnected}
	bridge.adoptLogger(logger)

	return bridge
}

// EnsureStarted connects to the extension, off the calling goroutine. It never
// blocks: the callers reaching it include the mode handler, where a slow call
// holds the keyboard grab. Until the connection lands, Focused reports
// errNotConnected, and a caller that arrives while an attempt is in flight
// gets whatever the last completed attempt learned.
func (b *Bridge) EnsureStarted() {
	if !b.beginStart() {
		return
	}

	go b.start()
}

// Focused returns the focused window as the extension last reported it.
//
// The three answers are distinct on purpose. A window with ok is what the
// shell says. ok=false with no error means the extension is connected and has
// nothing to report: the desktop or the shell itself is focused. A non-nil
// error means the extension could not be reached, which is the one case a
// caller must not read as "there is no focused window".
func (b *Bridge) Focused() (Window, bool, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.valid {
		return b.window, true, nil
	}

	return Window{}, false, b.startErr
}

// Bounds returns just the focused window's rectangle, carrying Focused's three
// answers unchanged.
func (b *Bridge) Bounds() (image.Rectangle, bool, error) {
	window, ok, err := b.Focused()

	return window.Rect, ok, err
}

// FocusEventFD returns a file descriptor that becomes readable whenever the
// cache changes, for the app watcher to poll instead of asking on a timer. The
// descriptor is owned by the bridge and lives as long as the process; a caller
// dups it and drains what it reads. It exists before the extension answers, so
// a watcher that subscribes early is woken by the first answer too.
func (b *Bridge) FocusEventFD() (int, bool) {
	b.fdOnce.Do(func() {
		reader, writer, err := os.Pipe()
		if err != nil {
			b.log().Debug("GNOME Shell focus pipe unavailable", zap.Error(err))

			return
		}

		for _, file := range []*os.File{reader, writer} {
			err := unix.SetNonblock(int(file.Fd()), true)
			if err != nil {
				b.log().
					Debug("GNOME Shell focus pipe could not be made non-blocking", zap.Error(err))
			}
		}

		b.events, b.notify = reader, writer
	})

	if b.events == nil {
		return -1, false
	}

	return int(b.events.Fd()), true
}

func (b *Bridge) beginStart() bool {
	b.startMu.Lock()
	defer b.startMu.Unlock()

	if b.starting || b.connected {
		return false
	}

	b.starting = true
	b.dataDir = extensionDir()

	return true
}

func (b *Bridge) start() {
	err := b.connect()

	b.startMu.Lock()
	b.starting = false
	b.connected = err == nil
	b.startMu.Unlock()

	b.record(err)
}

// record publishes an attempt's outcome. A failure is remembered as the reason
// Focused cannot answer, and said out loud once, since the error reaches every
// later caller anyway and a session without the extension would otherwise warn
// on every hint activation. A success clears the reason.
//
// The one warning waits for a logger: the first caller is the system adapter's
// warm-up, which has none, and a warning said into the nop logger is a warning
// nobody reads. adoptLogger replays it.
func (b *Bridge) record(err error) {
	b.mu.Lock()
	b.startErr = err
	b.mu.Unlock()

	if err == nil {
		b.log().Debug("GNOME Shell extension connected")

		return
	}

	b.warnOnce(err)
}

func (b *Bridge) warnOnce(err error) {
	logger := b.logger.Load()
	if logger == nil {
		return
	}

	b.startMu.Lock()
	first := !b.warned
	b.warned = true
	b.startMu.Unlock()

	if first {
		logger.Warn("GNOME Shell extension unavailable", zap.Error(err))

		return
	}

	logger.Debug("GNOME Shell extension still unavailable", zap.Error(err))
}

// connect arms the bus watch, asks whether the extension is on the bus, and
// reads the current state once. Everything after that arrives as signals.
func (b *Bridge) connect() error {
	conn, err := b.connection()
	if err != nil {
		return fmt.Errorf("session bus: %w", err)
	}

	b.watch(conn)

	present, err := nameHasOwner(conn, busName)
	if err != nil {
		return err
	}

	if !present {
		return b.absent(b.installIfShellIsUp(conn))
	}

	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	var (
		found                  bool
		appID, title           string
		posX, posY, width, hgt int32
	)

	err = conn.Object(busName, objectPath).CallWithContext(ctx, focusedMethod, 0).
		Store(&found, &appID, &title, &posX, &posY, &width, &hgt)
	if err != nil {
		return fmt.Errorf("%s: %w", focusedMethod, err)
	}

	b.update(windowFrom(appID, title, posX, posY, width, hgt), found)

	return nil
}

// absent is the extension-not-running answer, with the install attempt's
// outcome attached once one has been made.
func (b *Bridge) absent(hint string) error {
	b.startMu.Lock()
	if hint != "" {
		b.installHint = hint
	}

	hint = b.installHint
	b.startMu.Unlock()

	if hint == "" {
		return errExtensionAbsent
	}

	return fmt.Errorf("%w; %s", errExtensionAbsent, hint)
}

// connection returns the bridge's own bus connection, dialing one when there
// is none or the last one was closed under it.
func (b *Bridge) connection() (*dbus.Conn, error) {
	b.startMu.Lock()
	defer b.startMu.Unlock()

	if b.conn != nil && b.conn.Connected() {
		return b.conn, nil
	}

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}

	b.conn = conn
	b.watching = false

	return conn, nil
}

func nameHasOwner(conn *dbus.Conn, name string) (bool, error) {
	var hasOwner bool

	err := conn.BusObject().Call(dbusNameHasOwner, 0, name).Store(&hasOwner)
	if err != nil {
		return false, fmt.Errorf("NameHasOwner(%s): %w", name, err)
	}

	return hasOwner, nil
}

// watch subscribes once to the extension's state signal and to its name
// changing hands, so a shell that reloads the extension is followed without a
// caller having to notice. A match rule is a subscription, not a claim:
// nothing is exported and nothing is written by it.
func (b *Bridge) watch(conn *dbus.Conn) {
	b.startMu.Lock()
	already := b.watching
	b.watching = true
	b.startMu.Unlock()

	if already {
		return
	}

	for _, options := range [][]dbus.MatchOption{
		{
			dbus.WithMatchInterface(dbusIface),
			dbus.WithMatchMember(dbusNameOwnerMember),
			dbus.WithMatchArg(0, busName),
		},
		{
			dbus.WithMatchInterface(iface),
			dbus.WithMatchMember(changedMember),
		},
	} {
		err := conn.AddMatchSignal(options...)
		if err != nil {
			b.log().Debug("GNOME Shell signal watch unavailable", zap.Error(err))
		}
	}

	signals := make(chan *dbus.Signal, signalBuffer)
	conn.Signal(signals)

	go b.serve(signals)
}

// serve runs as long as the connection does. Every signal is checked by shape
// rather than trusted to the match rules. The channel closing means the
// connection went away, and a cache with no stream behind it is the stale
// answer this bridge exists to end, so the bridge reconnects.
func (b *Bridge) serve(signals <-chan *dbus.Signal) {
	for signal := range signals {
		switch signal.Name {
		case changedSignal:
			if window, found, ok := windowFromBody(signal.Body); ok {
				b.update(window, found)
			}
		case dbusNameOwnerSignal:
			if owner, ok := ownerFrom(signal); ok {
				b.ownerChanged(owner)
			}
		}
	}

	b.log().Debug("GNOME Shell bridge connection closed; reconnecting")

	b.mu.Lock()
	b.window, b.valid = Window{}, false
	b.startErr = errNotConnected
	b.mu.Unlock()

	b.startMu.Lock()
	b.connected = false
	b.watching = false
	b.startMu.Unlock()

	b.EnsureStarted()
}

func (b *Bridge) ownerChanged(owner string) {
	b.mu.Lock()
	b.window, b.valid = Window{}, false
	b.mu.Unlock()

	b.startMu.Lock()
	b.connected = false
	b.startMu.Unlock()

	if owner == "" {
		b.record(b.absent(""))
		b.wake()

		return
	}

	b.log().Debug("GNOME Shell extension is on the session bus")
	b.EnsureStarted()
}

func (b *Bridge) update(window Window, found bool) {
	b.mu.Lock()
	b.window, b.valid = window, found
	b.startErr = nil
	b.mu.Unlock()

	// The title is a window's contents by any reasonable reading, so it is
	// never logged. The app id is an application identity, which the app
	// watcher already logs.
	b.log().Debug("GNOME Shell focused window",
		zap.Bool("found", found),
		zap.Int("x", window.Rect.Min.X), zap.Int("y", window.Rect.Min.Y),
		zap.Int("w", window.Rect.Dx()), zap.Int("h", window.Rect.Dy()),
		zap.String("app_id", window.AppID))

	b.wake()
}

// wake makes the focus pipe readable. A full pipe means the reader is behind
// and will drain everything at once, so a refused write is the same wake-up.
func (b *Bridge) wake() {
	if b.notify == nil {
		return
	}

	_, _ = b.notify.Write([]byte{1})
}

// windowFromBody reads the FocusedWindowChanged body, which is the same tuple
// FocusedWindow replies with.
func windowFromBody(body []any) (Window, bool, bool) {
	if len(body) != stateArgs {
		return Window{}, false, false
	}

	found, okFound := body[0].(bool)
	appID, okApp := body[1].(string)
	title, okTitle := body[2].(string)
	posX, okX := body[3].(int32)
	posY, okY := body[4].(int32)
	width, okW := body[5].(int32)
	height, okH := body[6].(int32)

	if !okFound || !okApp || !okTitle || !okX || !okY || !okW || !okH {
		return Window{}, false, false
	}

	return windowFrom(appID, title, posX, posY, width, height), found, true
}

func windowFrom(appID, title string, posX, posY, width, height int32) Window {
	return Window{
		Rect:  image.Rect(int(posX), int(posY), int(posX)+int(width), int(posY)+int(height)),
		AppID: appID,
		Title: title,
	}
}

// ownerFrom reads a NameOwnerChanged for org.neru.Shell and returns its new
// owner, empty when the name was released.
func ownerFrom(signal *dbus.Signal) (string, bool) {
	if signal == nil || len(signal.Body) < nameOwnerArgs {
		return "", false
	}

	name, nameOK := signal.Body[0].(string)
	if !nameOK || name != busName {
		return "", false
	}

	owner, ownerOK := signal.Body[nameOwnerNewIndex].(string)
	if !ownerOK {
		return "", false
	}

	return owner, true
}

func (b *Bridge) adoptLogger(logger *zap.Logger) {
	if logger == nil || !b.logger.CompareAndSwap(nil, logger.Named("gnomeshell")) {
		return
	}

	b.mu.RLock()
	pending := b.startErr
	b.mu.RUnlock()

	if pending != nil && !errors.Is(pending, errNotConnected) {
		b.warnOnce(pending)
	}
}

func (b *Bridge) log() *zap.Logger {
	if adopted := b.logger.Load(); adopted != nil {
		return adopted
	}

	return nopLogger
}
