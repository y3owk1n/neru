//go:build linux

package linux

import (
	"context"
	"errors"
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The one bound over the session-bus dial.
//
// godbus's dial takes no context: the connect, the auth handshake and the Hello
// round trip are all uncancellable, so a bus that accepts a connection and then
// stops answering blocks its caller until it answers. The dial therefore runs on
// a goroutine the caller can abandon, which is what every caller here needs and
// none of them can express on its own.

// sessionBusDialPolicy decides what a caller that arrives while a dial is
// already outstanding gets.
type sessionBusDialPolicy int

const (
	// refuseWhileDialing turns the arriving caller away immediately. It is what
	// the portal path needs: every mid-action input operation reaches it while
	// no session is established, on the goroutine holding the keyboard grab, so
	// a wedged bus would otherwise buy one more orphaned goroutine and half-open
	// connection per keypress.
	refuseWhileDialing sessionBusDialPolicy = iota
	// awaitTheDialInFlight has the arriving caller wait on the outstanding
	// attempt and take its outcome. It is what callers that can afford to wait
	// need, and it is what keeps a notification and a `neru doctor` probe from
	// spoiling each other.
	awaitTheDialInFlight
)

// The ways a bounded dial ends without handing back a connection.
//
// The dialer never phrases any of them for a user: the sentence a user reads
// names what could not be done — a portal session asked for, a notification
// shown, a theme change followed — which the dial has no way to know, and the
// wording plus the derrors code is part of each caller's own contract.
// portalBusConnection and privateBusConnection below are two such callers that
// happen to live in this file; the notifier is a third, in
// system_notifications.go.
var (
	// errSessionBusDialInFlight means a dial was already outstanding and this
	// dialer refuses rather than adding to it.
	errSessionBusDialInFlight = errors.New("a session-bus dial is already outstanding")
	// errSessionBusDialAbandoned means the caller's context ran out before the
	// dial answered. The dial itself carries on — it cannot be canceled — and
	// whatever it eventually produces is disposed of rather than leaked.
	errSessionBusDialAbandoned = errors.New("the session-bus dial outlasted the caller")
	// errSessionBusDialEmpty means the dial reported neither a connection nor a
	// reason. No godbus dial does that, but a caller handed a nil connection
	// would panic on first use, so it is answered as a failure rather than
	// passed on.
	errSessionBusDialEmpty = errors.New("the session-bus dial produced no connection")
)

// privateBusDialer bounds the dial ConnectSessionBus performs. It is separate
// from the portal's because the two answer to different callers: refusing is
// right where the keyboard grab is held, and there is no grab behind this one.
var privateBusDialer = &sessionBusDialer{
	connect: connectPrivateSessionBus,
	policy:  awaitTheDialInFlight,
}

// ConnectSessionBus opens a session-bus connection of the caller's own under
// the same bound the portal and the notifier dial under, and hands it over for
// the caller to close.
//
// It exists for the app's Linux theme observer, which runs this on the
// goroutine that starts the daemon: an unbounded dial there does not merely
// delay a notification, it holds startup itself until a wedged bus answers. A
// caller that gives up should fall back to whatever it does when there is no
// bus at all — the failure is the same one, arriving later.
//
// A caller arriving while a dial is outstanding waits on it rather than being
// refused, so a bus that is merely slow does not send a second caller straight
// to its fallback. The connection it then gets is the first caller's as well:
// concurrent callers share one, and whichever closes it first closes it for
// both. A Neru process starts one theme observer, so there is no second caller
// today — a second use of this needs a connection of its own, which means a
// dialer of its own rather than a second caller on this one.
func ConnectSessionBus(ctx context.Context) (*dbus.Conn, error) {
	return privateBusConnection(ctx, privateBusDialer)
}

// privateBusConnection phrases one bounded dial for a caller that owns what it
// gets.
//
// The refusal is answered even though this dialer waits rather than refusing:
// a bus that is merely slow reported as one that is not there sends a user
// looking for a bus they have, and that mistake is one policy field away from
// being made.
func privateBusConnection(ctx context.Context, dialer *sessionBusDialer) (*dbus.Conn, error) {
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
	case errors.Is(err, errSessionBusDialInFlight):
		return nil, derrors.New(
			derrors.CodeTimeout,
			"an earlier connection to the D-Bus session bus is still unanswered",
		)
	default:
		return nil, derrors.Wrap(err, derrors.CodeNotSupported, "no reachable D-Bus session bus")
	}
}

// connectPrivateSessionBus opens a session-bus connection of the caller's own,
// rather than the process-wide one dbus.SessionBus hands back. A connection a
// caller may close has to be one nothing else is speaking over.
//
// It exists because godbus spells that dial as a variadic function and a dialer
// takes a plain one; none of the options it accepts apply here.
func connectPrivateSessionBus() (*dbus.Conn, error) {
	return dbus.ConnectSessionBus()
}

// sessionBusDialer holds the one session-bus dial that may be outstanding at a
// time, and bounds it for callers that cannot wait on it forever.
type sessionBusDialer struct {
	// connect performs the uncancellable dial. It is a field rather than a
	// direct godbus call because the callers reach different connections — the
	// portal a private dbus.ConnectSessionBus, the notifier the process-wide
	// dbus.SessionBus — and because a wedged bus is otherwise untestable on a
	// machine whose own bus answers.
	connect func() (*dbus.Conn, error)

	// policy decides what a caller arriving while a dial is stuck gets.
	policy sessionBusDialPolicy

	// processWide marks connect as handing back the connection the whole process
	// shares rather than one this dialer's callers own. It decides the fate of a
	// dial every caller walked away from: a private connection is closed,
	// because a wedged bus would otherwise leak one per abandoned dial, and the
	// process-wide one is left alone, because closing it would take the tray's
	// connection with it.
	processWide bool

	mu sync.Mutex
	// inFlight is the dial running right now, and nil when none is.
	inFlight *sessionBusDial
}

// sessionBusDial is one attempt to reach the session bus and the outcome every
// caller waiting on it reads. conn and err are written once, before done is
// closed, and read only after it closes — that close is what publishes them, so
// a caller sees the result of the dial it actually waited on.
type sessionBusDial struct {
	done chan struct{}
	conn *dbus.Conn
	err  error

	// holders counts the callers with an interest in this dial, and is guarded
	// by the dialer's mutex. Everyone still waiting counts, and so does anyone
	// who left holding the connection — a caller that takes the connection keeps
	// its count for good, which is precisely how the connection stays owned.
	// Reaching zero is what says nobody is left to close it.
	holders int
}

// dial returns a session-bus connection, or the reason there is none, without
// letting an unresponsive bus hold the caller past ctx.
//
// A caller either leaves holding the connection or releases its interest in the
// dial, never both and never neither. That is the whole of how the dialer tells
// a connection somebody is using from one nobody is left to close.
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

// run performs the uncancellable dial and publishes its outcome.
//
// The in-flight marker clears when connect returns rather than when the caller
// is done with what it produced, so a consent dialog left open for two minutes
// blocks nothing: what is being capped is the dial, not the session behind it.
func (d *sessionBusDialer) run(attempt *sessionBusDial) {
	conn, err := d.connect()

	d.mu.Lock()

	attempt.conn, attempt.err = conn, err
	orphaned := attempt.holders == 0

	d.inFlight = nil

	d.mu.Unlock()

	// Outside the lock, because the caller waiting on it may be the one holding
	// the keyboard grab: it takes this same mutex to start its own dial, and a
	// teardown is not something to make it wait behind.
	if orphaned {
		d.discard(conn)
	}

	close(attempt.done)
}

// release drops one caller's interest in a dial it took nothing from — because
// the dial outlasted it, or because there was nothing to take — and disposes of
// the connection when that caller was the last one interested.
//
// A dial still running has no connection to dispose of; the caller that walked
// away is simply no longer counted, and run disposes of what it produces when
// it finds nobody left.
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

// discard closes a connection nobody is left to own.
//
// The process-wide connection is never closed here: it is the one the tray's
// StatusNotifierItem speaks over too, and closing it because a notification
// timed out would take the tray down with it.
func (d *sessionBusDialer) discard(conn *dbus.Conn) {
	if conn == nil || d.processWide {
		return
	}

	_ = conn.Close()
}
