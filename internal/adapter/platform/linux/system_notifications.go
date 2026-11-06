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

// Desktop notifications and alerts for every Linux backend, over the session
// bus the tray also speaks. None of it is cgo, so there is no _nocgo twin.
//
// No notification daemon is an ordinary state on a minimal wlroots setup, so
// every path says why it could not deliver rather than returning nil.
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
	listActivatableNamesMethod = "org.freedesktop.DBus.ListActivatableNames"
	// serviceUnknownError and nameHasNoOwnerError are the two bus errors that
	// mean "nobody is listening" rather than "the daemon refused this".
	serviceUnknownError = "org.freedesktop.DBus.Error.ServiceUnknown"
	nameHasNoOwnerError = "org.freedesktop.DBus.Error.NameHasNoOwner"
	// urgencyHint is the hint key carrying the urgency level.
	urgencyHint = "urgency"
	// noNotificationIcon leaves app_icon empty, so daemons use their own.
	noNotificationIcon = ""
	// noCallFlags must never include NO_REPLY_EXPECTED, since the reply is
	// what makes a failure observable.
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

// replacesNothing is the replaces_id for a new notification. Neru keeps no ids.
const replacesNothing uint32 = 0

// notifyTimeout caps one round trip when the caller's context has no deadline,
// so a daemon that never answers cannot hold the tray menu loop or startup.
const notifyTimeout = 2 * time.Second

// Delivery failures a user can act on, phrased for them.
const (
	noSessionBusDetail = "no reachable D-Bus session bus on this linux session, so " +
		notifyBusName + " cannot be asked to show anything"
	noDaemonDetail = "no notification daemon on this linux session; nothing owns " +
		notifyBusName + ", and the session bus has no service registered to start one " +
		"on demand — install or start one (mako, dunst, or the desktop's own)"
	dialTimedOutDetail = "the D-Bus session bus on this linux session did not accept a " +
		"connection in time"
)

// notification is one desktop notification as this adapter sends it.
type notification struct {
	summary string
	body    string
	urgency byte
	expire  int32
}

// toastNotification is the shape ShowNotification sends, normal urgency with
// the daemon's own expiry.
func toastNotification(title, message string) notification {
	return notification{
		summary: title,
		body:    message,
		urgency: urgencyNormal,
		expire:  expireDaemonChoice,
	}
}

// alertNotification is the shape ShowAlert sends. No ordinary Wayland or X11
// client can show a modal like macOS's NSAlert, so it is a critical-urgency
// notification, the one urgency the spec forbids a daemon to expire.
func alertNotification(title, message string) notification {
	return notification{
		summary: title,
		body:    message,
		urgency: urgencyCritical,
		expire:  expireNever,
	}
}

// notifier delivers notifications over the session bus.
type notifier struct {
	// dialer bounds the uncancellable dial (session_bus_dial.go).
	dialer *sessionBusDialer

	mu sync.Mutex
	// conn is the last dial's connection, so later notifications dial nothing.
	// A failed dial clears it.
	conn *dbus.Conn
}

// newNotifier builds a notifier over connect that waits on an outstanding dial,
// since unlike the input path both its callers can afford to wait. The
// connection is processWide, because the tray speaks over it too.
func newNotifier(connect func() (*dbus.Conn, error)) *notifier {
	return &notifier{
		dialer: &sessionBusDialer{
			connect:     connect,
			policy:      awaitTheDialInFlight,
			processWide: true,
		},
	}
}

// sessionNotifier is the process-wide notifier, sharing the tray's connection.
var sessionNotifier = newNotifier(dbus.SessionBus)

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
// branch on. It waits for the reply, so it never reports unobserved success.
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

// daemonReachable reports nil when a notification sent now would reach a
// daemon, and why not otherwise, without putting one on screen. It also asks
// for activatable names, because a daemon shipped as a D-Bus service (most
// desktops) owns nothing until the bus starts it on the first Notify.
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
		// Nothing owns the name, the last thing known, so report that.
		return derrors.Wrap(err, derrors.CodeNotSupported, noDaemonDetail)
	}

	if !daemonReachableFrom(owned, activatable) {
		return derrors.New(derrors.CodeNotSupported, noDaemonDetail)
	}

	return nil
}

// daemonReachableFrom reports whether a daemon holds the name or the bus would
// start one.
func daemonReachableFrom(owned bool, activatable []string) bool {
	return owned || slices.Contains(activatable, notifyBusName)
}

// session returns a live session-bus connection, with the dial bounded by ctx
// through session_bus_dial.go. One dial exists at a time, since godbus
// serializes them anyway, and later callers share its outcome.
//
// A failed dial is a failure even when a connection was cached. The cached one
// had disconnected, which is why the dial ran, and reusing it would report the
// daemon rejecting a message that never left.
func (n *notifier) session(ctx context.Context) (*dbus.Conn, error) {
	cached := n.cachedSession()
	if cached != nil {
		return cached, nil
	}

	conn, err := n.dialer.dial(ctx)
	if err == nil {
		n.remember(conn)

		return conn, nil
	}

	if errors.Is(err, errSessionBusDialAbandoned) {
		return nil, derrors.Wrap(ctx.Err(), derrors.CodeTimeout, dialTimedOutDetail)
	}

	// Drop the dead cached connection.
	n.remember(nil)

	if errors.Is(err, errSessionBusDialEmpty) {
		return nil, derrors.New(derrors.CodeNotSupported, noSessionBusDetail)
	}

	return nil, derrors.Wrap(err, derrors.CodeNotSupported, noSessionBusDetail)
}

// cachedSession returns the connection the last dial produced when it is still
// live, and nil when there is nothing worth handing back.
func (n *notifier) cachedSession() *dbus.Conn {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.conn == nil || !n.conn.Connected() {
		return nil
	}

	return n.conn
}

// remember replaces the cached connection with what the last dial produced,
// which is nothing when it failed.
func (n *notifier) remember(conn *dbus.Conn) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.conn = conn
}

// withNotifyDeadline bounds the round trip. A caller's own deadline wins, and
// notifyTimeout applies otherwise.
func withNotifyDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	_, ok := ctx.Deadline()
	if ok {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(ctx, notifyTimeout)
}

// notifyError classifies a failed call. CodeNotSupported, which callers
// degrade on, is reserved for a session that cannot show notifications at all.
// A rejected message is a live failure.
func notifyError(err error) error {
	if isMissingDaemon(err) {
		return derrors.Wrap(err, derrors.CodeNotSupported, noDaemonDetail)
	}

	// The bus went away after the liveness check, not a daemon refusal. The
	// next call redials.
	if errors.Is(err, dbus.ErrClosed) {
		return derrors.Wrap(err, derrors.CodeNotSupported, noSessionBusDetail)
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
