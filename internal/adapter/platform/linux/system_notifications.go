//go:build linux

package linux

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Desktop notifications and alerts for every Linux backend.
//
// Notifications are a session-bus service rather than a display-server
// capability, so one implementation serves X11, wlroots and KDE alike — over
// the same session bus the tray's StatusNotifierItem already speaks. None of
// it is cgo, so the CGO_ENABLED=0 build shows notifications too and there is
// no _nocgo twin to keep in step.
//
// A session with no notification daemon is an ordinary state on a minimal
// wlroots setup, so every path here says why it could not deliver rather than
// returning nil: a caller that cannot tell "shown" from "dropped" has nothing
// to fall back to.
const (
	// notifyBusName is the well-known name a notification daemon owns.
	notifyBusName = "org.freedesktop.Notifications"
	// notifyObjectPath is the object every such daemon exports.
	notifyObjectPath = dbus.ObjectPath("/org/freedesktop/Notifications")
	// notifyMethod is the spec's single delivery method.
	notifyMethod = "org.freedesktop.Notifications.Notify"
	// notifyAppName is the application name daemons group our messages under.
	notifyAppName = "Neru"
	// nameHasOwnerMethod answers whether anything owns a well-known name.
	nameHasOwnerMethod = "org.freedesktop.DBus.NameHasOwner"
	// listActivatableNamesMethod lists the names the bus can start on demand.
	// A daemon installed as a D-Bus service owns nothing until the first call
	// reaches it, so ownership alone answers the wrong question.
	listActivatableNamesMethod = "org.freedesktop.DBus.ListActivatableNames"
	// serviceUnknownError and nameHasNoOwnerError are the two bus errors that
	// mean "nobody is listening" rather than "the daemon refused this".
	serviceUnknownError = "org.freedesktop.DBus.Error.ServiceUnknown"
	nameHasNoOwnerError = "org.freedesktop.DBus.Error.NameHasNoOwner"
	// urgencyHint is the hint key carrying the urgency level.
	urgencyHint = "urgency"
	// noNotificationIcon leaves app_icon empty: Neru installs no icon under a
	// name every icon theme has, and daemons substitute their own.
	noNotificationIcon = ""
	// noCallFlags sends a plain method call — a reply is what makes the
	// failure observable, so NO_REPLY_EXPECTED is exactly what must not be set.
	noCallFlags = 0
)

// Urgency levels from the freedesktop Desktop Notification Specification.
const (
	urgencyNormal   byte = 1
	urgencyCritical byte = 2
)

// Expiry timeouts in milliseconds, from the same specification: -1 leaves the
// choice to the daemon, 0 keeps the notification up until it is dismissed.
const (
	expireDaemonChoice int32 = -1
	expireNever        int32 = 0
)

// replacesNothing is the replaces_id that asks for a new notification instead
// of an update to an earlier one. Neru keeps no notification ids, so nothing
// it sends supersedes anything it sent before.
const replacesNothing uint32 = 0

// notifyTimeout caps one round trip when the caller's context carries no
// deadline of its own. A daemon being D-Bus activated may take a moment; one
// that will never answer must not hold the caller, which is the tray's menu
// loop or the startup path.
const notifyTimeout = 2 * time.Second

// The two reasons delivery fails that a user can act on, phrased for them:
// nothing to talk to, or nobody listening.
const (
	noSessionBusDetail = "no reachable D-Bus session bus on this linux session, so " +
		notifyBusName + " cannot be asked to show anything"
	noDaemonDetail = "no notification daemon on this linux session; nothing owns " +
		notifyBusName + ", and the session bus has no service registered to start one " +
		"on demand — install or start one (mako, dunst, or the desktop's own)"
	dialTimedOutDetail = "the D-Bus session bus on this linux session did not accept a " +
		"connection in time"
)

// notification is one desktop notification as this adapter sends it: what the
// user reads, how insistently, and for how long.
type notification struct {
	summary string
	body    string
	urgency byte
	expire  int32
}

// toastNotification is the lightweight shape ShowNotification sends: normal
// urgency, and the daemon's own idea of how long to leave it up.
func toastNotification(title, message string) notification {
	return notification{
		summary: title,
		body:    message,
		urgency: urgencyNormal,
		expire:  expireDaemonChoice,
	}
}

// alertNotification is the shape ShowAlert sends. macOS takes the session over
// with a modal NSAlert; no ordinary Wayland or X11 client can do that, so the
// nearest honest equivalent is a critical-urgency notification that stays on
// screen until it is dismissed — the one urgency the specification requires a
// daemon not to expire on its own.
func alertNotification(title, message string) notification {
	return notification{
		summary: title,
		body:    message,
		urgency: urgencyCritical,
		expire:  expireNever,
	}
}

// notifier delivers notifications over the session bus. The connector is a
// field rather than a direct dbus.SessionBus call so the no-bus and no-daemon
// paths stay testable on a machine that has both.
type notifier struct {
	connect func() (*dbus.Conn, error)

	mu sync.Mutex
	// conn is the connection a completed dial produced, kept so the ordinary
	// path after the first notification dials nothing.
	conn *dbus.Conn
	// dialErr is what the last completed dial reported.
	dialErr error
	// dialDone is non-nil while a dial is in flight, and is closed when it
	// finishes. Callers wait on it rather than starting a second dial.
	dialDone chan struct{}
}

// sessionNotifier is the process-wide notifier. dbus.SessionBus hands back one
// shared connection per process, so this reuses the session bus the tray also
// dials rather than opening a second one.
var sessionNotifier = &notifier{connect: dbus.SessionBus}

// ShowNotification shows a lightweight desktop notification through the
// session's notification daemon, reporting CodeNotSupported when there is no
// session bus or no daemon to show it.
func ShowNotification(ctx context.Context, title, message string) error {
	return sessionNotifier.send(ctx, toastNotification(title, message))
}

// ShowAlert shows a message that stays on screen until the user dismisses it,
// reporting CodeNotSupported when there is no session bus or no daemon to show
// it. See alertNotification for why this is not a modal dialog.
func ShowAlert(ctx context.Context, title, message string) error {
	return sessionNotifier.send(ctx, alertNotification(title, message))
}

// send delivers one notification, mapping every failure to a code a caller can
// branch on. It never reports success it did not observe: the call waits for
// the daemon's reply rather than firing it off with NO_REPLY_EXPECTED, which
// is the whole difference between this and the empty body it replaces.
func (n *notifier) send(ctx context.Context, note notification) error {
	callCtx, cancel := withNotifyDeadline(ctx)
	defer cancel()

	conn, err := n.session(callCtx)
	if err != nil {
		return err
	}

	call := conn.Object(notifyBusName, notifyObjectPath).CallWithContext(
		callCtx,
		notifyMethod,
		noCallFlags,
		notifyAppName,
		replacesNothing,
		noNotificationIcon,
		note.summary,
		note.body,
		[]string{},
		map[string]dbus.Variant{urgencyHint: dbus.MakeVariant(note.urgency)},
		note.expire,
	)
	if call.Err != nil {
		return notifyError(call.Err)
	}

	return nil
}

// daemonReachable reports nil when a notification the user sent now would
// reach a daemon, and why not otherwise. It asks the bus daemon rather than
// sending a notification, so `neru doctor` can answer "will I see
// notifications?" without putting one on screen.
//
// Two questions, because ownership alone answers a narrower one than the user
// asked. A daemon shipped as a D-Bus service — which is how most desktops ship
// theirs — owns nothing until something calls it, and the bus starts it on the
// first Notify. Reporting that session as having no notifications would send a
// user off to install what they already have.
func (n *notifier) daemonReachable(ctx context.Context) error {
	callCtx, cancel := withNotifyDeadline(ctx)
	defer cancel()

	conn, err := n.session(callCtx)
	if err != nil {
		return err
	}

	var owned bool

	err = conn.BusObject().
		CallWithContext(callCtx, nameHasOwnerMethod, noCallFlags, notifyBusName).
		Store(&owned)
	if err != nil {
		return notifyError(err)
	}

	if owned {
		return nil
	}

	var activatable []string

	err = conn.BusObject().
		CallWithContext(callCtx, listActivatableNamesMethod, noCallFlags).
		Store(&activatable)
	if err != nil {
		// The bus could not say what it can start, and nothing owns the name:
		// the last thing known is that nobody is listening, so that is what is
		// reported rather than a guess in either direction.
		return derrors.Wrap(err, derrors.CodeNotSupported, noDaemonDetail)
	}

	if !daemonReachableFrom(owned, activatable) {
		return derrors.New(derrors.CodeNotSupported, noDaemonDetail)
	}

	return nil
}

// daemonReachableFrom turns the bus's two answers into the one the user asked
// for: a notification sent now would reach a daemon if one holds the name, and
// equally if the bus would start one on being asked.
func daemonReachableFrom(owned bool, activatable []string) bool {
	return owned || slices.Contains(activatable, notifyBusName)
}

// session returns a live session-bus connection, bounded by ctx.
//
// The dial has to be bounded separately from the call because dbus.SessionBus
// takes no context: the connect, the auth handshake and the Hello round trip
// are all uncancellable, so a bus that accepts a connection and then stops
// answering blocks the caller forever. It therefore runs on a goroutine the
// caller can abandon, bounded by each caller's own deadline rather than by the
// dial. One dial exists at a time — godbus serializes them on its own lock, so
// a second could do nothing but wait behind the first — and later callers
// share its outcome instead of being turned away, which is what keeps a tray
// notification and a `neru doctor` probe from spoiling each other. A
// connection that succeeds is kept, so the ordinary path after the first
// notification dials nothing at all.
func (n *notifier) session(ctx context.Context) (*dbus.Conn, error) {
	n.mu.Lock()

	if n.conn != nil && n.conn.Connected() {
		conn := n.conn
		n.mu.Unlock()

		return conn, nil
	}

	if n.dialDone == nil {
		n.dialDone = make(chan struct{})
		go n.dial(n.dialDone)
	}

	dialed := n.dialDone
	n.mu.Unlock()

	select {
	case <-dialed:
	case <-ctx.Done():
		return nil, derrors.Wrap(ctx.Err(), derrors.CodeTimeout, dialTimedOutDetail)
	}

	n.mu.Lock()
	conn, err := n.conn, n.dialErr
	n.mu.Unlock()

	if conn != nil {
		return conn, nil
	}

	if err != nil {
		return nil, derrors.Wrap(err, derrors.CodeNotSupported, noSessionBusDetail)
	}

	return nil, derrors.New(derrors.CodeNotSupported, noSessionBusDetail)
}

// dial performs the one uncancellable connect and publishes its outcome by
// closing done. It clears the in-flight marker first, so the next caller after
// a failed dial retries rather than reading a stale answer forever.
func (n *notifier) dial(done chan struct{}) {
	conn, err := n.connect()

	n.mu.Lock()

	n.dialErr = err
	n.dialDone = nil

	if err == nil {
		n.conn = conn
	}

	n.mu.Unlock()

	close(done)
}

// withNotifyDeadline bounds the round trip. A caller's own deadline wins — it
// knows what it can afford to wait — and one is imposed otherwise so a wedged
// daemon cannot hold the caller open.
func withNotifyDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	_, ok := ctx.Deadline()
	if ok {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(ctx, notifyTimeout)
}

// notifyError classifies a failed call so callers can tell a session that
// cannot show notifications at all from a daemon that refused this one.
// CodeNotSupported is reserved for the former, because that is what callers
// degrade on; a rejected message is a live failure and stays one.
func notifyError(err error) error {
	if isMissingDaemon(err) {
		return derrors.Wrap(err, derrors.CodeNotSupported, noDaemonDetail)
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return derrors.Wrap(
			err,
			derrors.CodeTimeout,
			"the linux notification daemon did not answer in time",
		)
	}

	return derrors.Wrap(
		err,
		derrors.CodeActionFailed,
		"the linux notification daemon rejected the message",
	)
}

// isMissingDaemon reports whether a bus error means nothing owns the
// notification name. godbus yields dbus.Error by value on a client call and by
// pointer from NewError, so both are matched.
func isMissingDaemon(err error) bool {
	var name string

	var busError dbus.Error

	var busErrorPtr *dbus.Error

	switch {
	case errors.As(err, &busError):
		name = busError.Name
	case errors.As(err, &busErrorPtr):
		name = busErrorPtr.Name
	default:
		return false
	}

	return name == serviceUnknownError || name == nameHasNoOwnerError
}
