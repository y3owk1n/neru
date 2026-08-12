//go:build linux

package linux

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The two budgets the tests here run on: how long a caller is willing to wait
// on a dial that will never answer, and how long a test waits for something
// that should be immediate before calling it a hang.
const (
	dialTestBudget = 50 * time.Millisecond
	dialTestLimit  = 5 * time.Second
)

// wedgedConnect stands in for a session bus that accepts the connection and
// then stops answering: it announces each dial on dials and parks until release
// is closed, which is the state every bound in this file exists to survive.
func wedgedConnect(dials chan<- struct{}, release <-chan struct{}) func() (*dbus.Conn, error) {
	return func() (*dbus.Conn, error) {
		dials <- struct{}{}

		<-release

		return nil, errNoBus
	}
}

// connectNothing gives the one answer no godbus dial gives: neither a
// connection nor a reason. Handing that on would have a caller panic on first
// use, so the dialer answers it as a failure of its own.
//
//nolint:nilnil // returning exactly that pair is what this stands for.
func connectNothing() (*dbus.Conn, error) {
	return nil, nil
}

// awaitDial waits for one dial to have been started, failing rather than
// hanging when none ever is.
func awaitDial(t *testing.T, dials <-chan struct{}) {
	t.Helper()

	select {
	case <-dials:
	case <-time.After(dialTestLimit):
		t.Fatal("no dial was ever started")
	}
}

// TestSessionBusDialer_Dial_RefusesASecondCallerWhileOneIsStuck pins the policy
// the mid-action input path depends on: an uncancellable dial that has wedged
// gets one outstanding attempt, not one per caller. Every mid-action input
// operation reaches the portal while no session is up, so restarting would buy
// an orphaned goroutine and a half-open connection per keypress.
func TestSessionBusDialer_Dial_RefusesASecondCallerWhileOneIsStuck(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	dials := make(chan struct{}, 2)

	dialer := &sessionBusDialer{
		connect: wedgedConnect(dials, release),
		policy:  refuseWhileDialing,
	}

	stuck, cancelStuck := context.WithTimeout(context.Background(), dialTestBudget)
	defer cancelStuck()

	first := make(chan error, 1)

	go func() {
		_, err := dialer.dial(stuck)
		first <- err
	}()

	awaitDial(t, dials)

	second, cancelSecond := context.WithTimeout(context.Background(), dialTestBudget)
	defer cancelSecond()

	_, err := dialer.dial(second)
	if !errors.Is(err, errSessionBusDialInFlight) {
		t.Fatalf("a second dial while one is outstanding = %v, want errSessionBusDialInFlight", err)
	}

	select {
	case <-dials:
		t.Error("a refused caller started its own dial rather than being turned away")
	default:
	}

	err = <-first
	if !errors.Is(err, errSessionBusDialAbandoned) {
		t.Errorf(
			"the first dial = %v, want errSessionBusDialAbandoned once its context ran out",
			err,
		)
	}
}

// TestSessionBusDialer_Dial_SharesTheOutstandingDialWithLaterCallers pins the
// other policy. The notifier and the theme observer can afford to wait, and
// turning them away would have a `neru doctor` probe spoil a tray notification
// that was already waiting on the same bus.
func TestSessionBusDialer_Dial_SharesTheOutstandingDialWithLaterCallers(t *testing.T) {
	release := make(chan struct{})
	dials := make(chan struct{}, 2)
	shared := openConn(t)

	dialer := &sessionBusDialer{
		policy: awaitTheDialInFlight,
		connect: func() (*dbus.Conn, error) {
			dials <- struct{}{}

			<-release

			return shared, nil
		},
	}

	results := make(chan *dbus.Conn, 2)

	share := func() {
		conn, err := dialer.dial(context.Background())
		if err != nil {
			results <- nil

			return
		}

		results <- conn
	}

	go share()

	// The first caller has the dial parked before the second arrives, so the
	// second can only ever be the one that joins rather than the one that
	// starts.
	awaitDial(t, dials)

	go share()

	awaitWaiters(t, dialer, 2)

	// Both callers are now waiting on the one attempt. Letting it answer has to
	// answer both of them, with the connection it actually produced.
	close(release)

	for range 2 {
		select {
		case conn := <-results:
			if conn != shared {
				t.Errorf("a waiting caller got %v, want the connection the dial produced", conn)
			}
		case <-time.After(dialTestLimit):
			t.Fatal("a caller waiting on the outstanding dial was never answered")
		}
	}

	select {
	case <-dials:
		t.Error("a later caller started its own dial rather than sharing the outstanding one")
	default:
	}
}

// TestSessionBusDialer_Dial_ClosesAPrivateConnectionEveryCallerWalkedAwayFrom is
// the half of abandoning that is easy to lose. The dial cannot be canceled, so
// it eventually produces a connection whether or not anyone is still waiting;
// one nobody is left to close is a leaked file descriptor and a live bus
// connection per wedged dial.
func TestSessionBusDialer_Dial_ClosesAPrivateConnectionEveryCallerWalkedAwayFrom(t *testing.T) {
	release := make(chan struct{})
	dials := make(chan struct{}, 1)
	abandoned := openConn(t)

	dialer := &sessionBusDialer{
		policy: refuseWhileDialing,
		connect: func() (*dbus.Conn, error) {
			dials <- struct{}{}

			<-release

			return abandoned, nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), dialTestBudget)
	defer cancel()

	_, err := dialer.dial(ctx)
	if !errors.Is(err, errSessionBusDialAbandoned) {
		t.Fatalf("dial against a wedged bus = %v, want errSessionBusDialAbandoned", err)
	}

	awaitDial(t, dials)
	close(release)

	awaitClosed(t, abandoned)
}

// TestSessionBusDialer_Dial_LeavesTheProcessWideConnectionOpenWhenAbandoned is
// the exception, and it is not a detail: dbus.SessionBus hands back one
// connection per process, which the tray's StatusNotifierItem speaks over too.
// Closing it because a notification timed out would take the tray down with it.
func TestSessionBusDialer_Dial_LeavesTheProcessWideConnectionOpenWhenAbandoned(t *testing.T) {
	release := make(chan struct{})
	dials := make(chan struct{}, 1)
	process := openConn(t)

	dialer := &sessionBusDialer{
		policy:      awaitTheDialInFlight,
		processWide: true,
		connect: func() (*dbus.Conn, error) {
			dials <- struct{}{}

			<-release

			return process, nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), dialTestBudget)
	defer cancel()

	_, err := dialer.dial(ctx)
	if !errors.Is(err, errSessionBusDialAbandoned) {
		t.Fatalf("dial against a wedged bus = %v, want errSessionBusDialAbandoned", err)
	}

	awaitDial(t, dials)
	close(release)

	// The dialer has to be given the chance to close it and not take it, so this
	// waits for the abandoned dial to finish rather than dialing again — a
	// second caller would join the outstanding attempt and own it, which is the
	// one thing that would stop the disposal being reached at all.
	awaitIdle(t, dialer)
	stayOpen(t, process)
}

// TestSessionBusDialer_Dial_ReportsADialThatProducedNeitherConnectionNorReason
// guards the one answer a caller cannot use. Nothing hands back (nil, nil)
// today, but a caller given a nil connection panics on first use, so it is
// answered as a failure rather than passed on.
func TestSessionBusDialer_Dial_ReportsADialThatProducedNeitherConnectionNorReason(t *testing.T) {
	dialer := &sessionBusDialer{
		policy:  refuseWhileDialing,
		connect: connectNothing,
	}

	conn, err := dialer.dial(context.Background())
	if conn != nil {
		t.Error("dial handed back a nil connection as a success")
	}

	if !errors.Is(err, errSessionBusDialEmpty) {
		t.Errorf("dial = %v, want errSessionBusDialEmpty", err)
	}
}

// TestPrivateBusConnection_GivesUpOnAWedgedBusRatherThanHoldingStartup is what
// ConnectSessionBus exists for. The app's theme observer dials on the goroutine
// that starts the daemon, so an uncancellable dial into a bus that accepts the
// connection and then stops answering would hold startup itself — the caller
// has to be told to fall back instead.
func TestPrivateBusConnection_GivesUpOnAWedgedBusRatherThanHoldingStartup(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	dials := make(chan struct{}, 1)

	dialer := &sessionBusDialer{
		policy:  awaitTheDialInFlight,
		connect: wedgedConnect(dials, release),
	}

	ctx, cancel := context.WithTimeout(context.Background(), dialTestBudget)
	defer cancel()

	conn, err := privateBusConnection(ctx, dialer)
	if conn != nil {
		t.Error("privateBusConnection handed back a connection the dial never produced")
	}

	if !derrors.IsCode(err, derrors.CodeTimeout) {
		t.Errorf("privateBusConnection = %v (code %q), want %q",
			err, derrors.GetCode(err), derrors.CodeTimeout)
	}

	awaitDial(t, dials)
}

// TestPrivateBusConnection_ReportsAnAbsentBusAsUnsupported keeps the caller's
// two fallbacks the same one. A session with no bus is an ordinary state on a
// minimal setup, and the theme observer polls either way.
func TestPrivateBusConnection_ReportsAnAbsentBusAsUnsupported(t *testing.T) {
	dialer := &sessionBusDialer{
		policy:  awaitTheDialInFlight,
		connect: func() (*dbus.Conn, error) { return nil, errNoBus },
	}

	_, err := privateBusConnection(context.Background(), dialer)
	if !derrors.IsNotSupported(err) {
		t.Errorf("privateBusConnection = %v (code %q), want %q",
			err, derrors.GetCode(err), derrors.CodeNotSupported)
	}
}

// TestPrivateBusConnection_DoesNotCallAStuckBusAnAbsentOne separates the two
// answers a caller would act on differently. Nothing dials this with the
// refusing policy today, and the point is that switching it to one could not
// start telling a user their session bus is missing when it is merely slow.
func TestPrivateBusConnection_DoesNotCallAStuckBusAnAbsentOne(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	dials := make(chan struct{}, 1)

	dialer := &sessionBusDialer{
		policy:  refuseWhileDialing,
		connect: wedgedConnect(dials, release),
	}

	stuck, cancelStuck := context.WithTimeout(context.Background(), dialTestBudget)
	defer cancelStuck()

	_, _ = privateBusConnection(stuck, dialer)

	awaitDial(t, dials)

	refused, cancelRefused := context.WithTimeout(context.Background(), dialTestBudget)
	defer cancelRefused()

	_, err := privateBusConnection(refused, dialer)
	if derrors.IsNotSupported(err) {
		t.Errorf("a refused caller was told %q, which reads as a session with no bus", err)
	}

	if !derrors.IsCode(err, derrors.CodeTimeout) {
		t.Errorf("a refused caller got code %q, want %q", derrors.GetCode(err), derrors.CodeTimeout)
	}
}

// awaitWaiters waits until want callers are waiting on the outstanding dial,
// so a test can release that dial knowing everyone meant to share it has
// arrived rather than racing the release.
func awaitWaiters(t *testing.T, dialer *sessionBusDialer, want int) {
	t.Helper()

	deadline := time.Now().Add(dialTestLimit)

	for time.Now().Before(deadline) {
		dialer.mu.Lock()

		holders := 0
		if dialer.inFlight != nil {
			holders = dialer.inFlight.holders
		}

		dialer.mu.Unlock()

		if holders == want {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatalf("fewer than %d callers ever waited on the outstanding dial", want)
}

// awaitClosed waits for the dialer to close a connection it was left holding
// alone. The disposal happens on the dial's own goroutine, after the caller
// that abandoned it has already been answered.
func awaitClosed(t *testing.T, conn *dbus.Conn) {
	t.Helper()

	deadline := time.Now().Add(dialTestLimit)

	for time.Now().Before(deadline) {
		if !conn.Connected() {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Error("the connection nobody was left waiting for was never closed")
}

// stayOpen fails when conn is closed within the window a wrongly-closed one
// would be: closing is the first thing the dial's goroutine does with an
// outcome it decides nobody owns.
func stayOpen(t *testing.T, conn *dbus.Conn) {
	t.Helper()

	deadline := time.Now().Add(dialTestBudget)

	for time.Now().Before(deadline) {
		if !conn.Connected() {
			t.Fatal("the process-wide connection was closed under the tray")
		}

		time.Sleep(time.Millisecond)
	}
}

// awaitIdle waits until no dial is outstanding, which is how a test observes
// that the dial it abandoned has run to the point of deciding what to do with
// what it produced.
func awaitIdle(t *testing.T, dialer *sessionBusDialer) {
	t.Helper()

	deadline := time.Now().Add(dialTestLimit)

	for time.Now().Before(deadline) {
		dialer.mu.Lock()
		outstanding := dialer.inFlight != nil
		dialer.mu.Unlock()

		if !outstanding {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatal("the abandoned dial never finished")
}
