//go:build linux

package linux

import (
	"context"
	"errors"
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The one bound over the session-bus dial. godbus's dial takes no context, and
// the connect, auth handshake and Hello are all uncancellable, so a bus that
// accepts and then stops answering blocks its caller. The dial therefore runs
// on a goroutine the caller can abandon.

// sessionBusDialPolicy decides what a caller that arrives while a dial is
// already outstanding gets.
type sessionBusDialPolicy int

const (
	// refuseWhileDialing turns the arriving caller away immediately. The portal
	// path needs it, because input operations reach it on the goroutine holding
	// the keyboard grab, and a wedged bus would otherwise buy one orphaned
	// goroutine and half-open connection per keypress.
	refuseWhileDialing sessionBusDialPolicy = iota
	// awaitTheDialInFlight has the arriving caller wait on the outstanding
	// attempt and take its outcome, so a notification and a `neru doctor` probe
	// do not spoil each other.
	awaitTheDialInFlight
)

// The ways a bounded dial ends without a connection. The dialer never phrases
// them for a user. Each caller owns its wording and derrors code, because only
// it knows what could not be done. The callers are portalBusConnection,
// privateBusConnection and the notifier in system_notifications.go.
var (
	// errSessionBusDialInFlight means a dial was already outstanding and this
	// dialer refuses to add to it. Only a shared dialer can produce it.
	errSessionBusDialInFlight = errors.New("a session-bus dial is already outstanding")
	// errSessionBusDialAbandoned means the caller's context ran out first. The
	// dial carries on, and whatever it produces is disposed of, not leaked.
	errSessionBusDialAbandoned = errors.New("the session-bus dial outlasted the caller")
	// errSessionBusDialEmpty means the dial reported neither a connection nor a
	// reason. A nil connection would panic on first use, so it is a failure.
	errSessionBusDialEmpty = errors.New("the session-bus dial produced no connection")
)

// ConnectSessionBus opens a bounded session-bus connection of the caller's own
// for the caller to close.
//
// It exists for the app's Linux theme observer, which dials on the goroutine
// that starts the daemon, so an unbounded dial would hold startup on a wedged
// bus. A caller that gives up should fall back to its no-bus behavior.
//
// It provides the bound only. There is deliberately no process-wide cap here.
// The cap that must exist is the portal's, on its own dialer.
func ConnectSessionBus(ctx context.Context) (*dbus.Conn, error) {
	return privateBusConnection(ctx, connectPrivateSessionBus)
}

// privateBusConnection dials one bounded connection for a caller that owns what
// it gets, and phrases the ways that ends without one.
//
// The dialer is built per call, not shared. A waiting dialer hands every waiter
// the same connection, and two owners would each close it. The bound survives
// whole, only the cross-caller cap goes. A single-caller dialer never refuses,
// so errSessionBusDialInFlight is not answered here.
func privateBusConnection(
	ctx context.Context,
	connect func() (*dbus.Conn, error),
) (*dbus.Conn, error) {
	dialer := &sessionBusDialer{
		connect: connect,
		// Inert with one caller. Written so the zero value, refuseWhileDialing,
		// does not read as a refusal this path cannot make.
		policy: awaitTheDialInFlight,
	}

	conn, err := dialer.dial(ctx)

	switch {
	case err == nil:
		return conn, nil
	case errors.Is(err, errSessionBusDialAbandoned):
		return nil, derrors.Wrap(
			ctx.Err(),
			derrors.CodeTimeout,
			"the D-Bus session bus did not accept a connection in time",
		)
	default:
		return nil, derrors.Wrap(err, derrors.CodeNotSupported, "no reachable D-Bus session bus")
	}
}

// connectPrivateSessionBus opens a session-bus connection of the caller's own,
// not the process-wide dbus.SessionBus one, since a connection a caller may
// close must be one nothing else speaks over. It adapts godbus's variadic dial
// to the plain func a dialer takes.
func connectPrivateSessionBus() (*dbus.Conn, error) {
	return dbus.ConnectSessionBus()
}

// sessionBusDialer holds the one session-bus dial that may be outstanding at a
// time, and bounds it for callers that cannot wait on it forever.
type sessionBusDialer struct {
	// connect performs the uncancellable dial. It is a field because callers
	// reach different connections (the portal a private one, the notifier the
	// process-wide dbus.SessionBus), and so a wedged bus is testable.
	connect func() (*dbus.Conn, error)

	// policy decides what a caller arriving while a dial is stuck gets.
	policy sessionBusDialPolicy

	// processWide marks connect as handing back the connection the whole process
	// shares. It decides the fate of a dial every caller walked away from. A
	// private connection is closed so abandoned dials do not leak, and the
	// process-wide one is left alone because the tray speaks over it.
	processWide bool

	mu sync.Mutex
	// inFlight is the dial running right now, and nil when none is.
	inFlight *sessionBusDial
}

// sessionBusDial is one attempt to reach the session bus and the outcome every
// caller waiting on it reads. conn and err are written once before done is
// closed and read only after, so the close publishes them.
type sessionBusDial struct {
	done chan struct{}
	conn *dbus.Conn
	err  error

	// holders counts the callers with an interest in this dial, guarded by the
	// dialer's mutex. Waiters count, and a caller that takes the connection
	// keeps its count for good. Zero means nobody is left to close it.
	holders int
}

// dial returns a session-bus connection, or the reason there is none, without
// letting an unresponsive bus hold the caller past ctx. A caller either leaves
// holding the connection or releases its interest, never both and never neither.
func (d *sessionBusDialer) dial(ctx context.Context) (*dbus.Conn, error) {
	attempt, err := d.begin()
	if err != nil {
		return nil, err
	}

	select {
	case <-attempt.done:
	case <-ctx.Done():
		d.release(attempt)

		return nil, errSessionBusDialAbandoned
	}

	if attempt.err == nil && attempt.conn != nil {
		return attempt.conn, nil
	}

	d.release(attempt)

	if attempt.err != nil {
		return nil, attempt.err
	}

	return nil, errSessionBusDialEmpty
}

// begin joins the outstanding dial or starts one, and reports the refusal when
// there is an outstanding one this dialer will not add to.
func (d *sessionBusDialer) begin() (*sessionBusDial, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.inFlight != nil {
		if d.policy == refuseWhileDialing {
			return nil, errSessionBusDialInFlight
		}

		d.inFlight.holders++

		return d.inFlight, nil
	}

	attempt := &sessionBusDial{done: make(chan struct{}), holders: 1}
	d.inFlight = attempt

	go d.run(attempt)

	return attempt, nil
}

// run performs the uncancellable dial and publishes its outcome. The in-flight
// marker clears when connect returns, not when the caller is done, so a consent
// dialog left open blocks nothing. The cap is on the dial, not the session.
func (d *sessionBusDialer) run(attempt *sessionBusDial) {
	conn, err := d.connect()

	d.mu.Lock()

	attempt.conn, attempt.err = conn, err
	orphaned := attempt.holders == 0

	d.inFlight = nil

	d.mu.Unlock()

	// Outside the lock, because a caller holding the keyboard grab may need this
	// mutex to start its own dial.
	if orphaned {
		d.discard(conn)
	}

	close(attempt.done)
}

// release drops one caller's interest in a dial it took nothing from, and
// disposes of the connection when that caller was the last one. A dial still
// running has nothing to dispose of yet, and run handles it on finding nobody.
func (d *sessionBusDialer) release(attempt *sessionBusDial) {
	d.mu.Lock()

	attempt.holders--
	orphaned := attempt.holders == 0
	conn := attempt.conn

	d.mu.Unlock()

	if orphaned {
		d.discard(conn)
	}
}

// discard closes a connection nobody is left to own. The process-wide one is
// never closed, because the tray's StatusNotifierItem speaks over it too.
func (d *sessionBusDialer) discard(conn *dbus.Conn) {
	if conn == nil || d.processWide {
		return
	}

	_ = conn.Close()
}
