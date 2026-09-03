//go:build linux

package linux

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/derrors"
)

// errNoBus stands in for whatever godbus reports when there is no session bus
// to reach — the message never matters, only that connecting failed.
var errNoBus = errors.New("dbus: couldn't determine address of session bus")

// unreachableNotifier is a notifier whose session bus cannot be reached, which
// is what a daemon started outside a desktop session sees.
func unreachableNotifier() *notifier {
	return newNotifier(func() (*dbus.Conn, error) { return nil, errNoBus })
}

// idleTransport is enough of a connection for godbus to build a *dbus.Conn
// around without a session bus. Nothing is ever sent over it: the tests using
// it need a connection they can close, to stand for one the bus dropped.
type idleTransport struct{}

func (idleTransport) Read([]byte) (int, error)    { return 0, io.EOF }
func (idleTransport) Write(p []byte) (int, error) { return len(p), nil }
func (idleTransport) Close() error                { return nil }

// openConn is a connection that reports itself connected until it is closed.
func openConn(t *testing.T) *dbus.Conn {
	t.Helper()

	conn, err := dbus.NewConn(idleTransport{})
	if err != nil {
		t.Fatalf("building a test connection failed: %v", err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	return conn
}

// TestNotifier_SendReportsNotSupportedWithoutASessionBus is the loud-stub pin
// for the path this change exists to close: ShowNotification used to have an
// empty body, so a dropped message was indistinguishable from a delivered one.
// Anything that cannot be shown must come back as CodeNotSupported, never nil.
func TestNotifier_SendReportsNotSupportedWithoutASessionBus(t *testing.T) {
	sends := map[string]notification{
		"toast": toastNotification("Neru", "something happened"),
		"alert": alertNotification("Neru", "something went wrong"),
	}

	for name, note := range sends {
		t.Run(name, func(t *testing.T) {
			err := unreachableNotifier().send(context.Background(), note)
			if err == nil {
				t.Fatal("send with no session bus returned nil; the message was dropped silently")
			}

			if !derrors.IsNotSupported(err) {
				t.Errorf("send returned %v (code %q), want CodeNotSupported so callers can degrade",
					err, derrors.GetCode(err))
			}

			if !strings.Contains(err.Error(), notifyBusName) {
				t.Errorf("error %q does not name %q, so it does not say what is missing",
					err.Error(), notifyBusName)
			}
		})
	}
}

// TestNotifier_SendGivesUpOnADialThatNeverReturns pins why the dial runs on
// its own goroutine rather than inline: dbus.SessionBus takes no context, so a
// bus that accepts the connection and then stops answering would park the
// caller forever. The tray calls this from the loop that also carries Quit, and
// the config alerts call it before the daemon is up.
func TestNotifier_SendGivesUpOnADialThatNeverReturns(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	// Each dial announces itself, so a second one is observable.
	dials := make(chan struct{}, 2)

	wedged := newNotifier(wedgedConnect(dials, release))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	sent := make(chan error, 1)

	go func() {
		sent <- wedged.send(ctx, toastNotification("Neru", "anyone there"))
	}()

	select {
	case err := <-sent:
		if err == nil {
			t.Fatal("send against a wedged session bus returned nil")
		}

		if !derrors.IsCode(err, derrors.CodeTimeout) {
			t.Errorf("send returned %v (code %q), want CodeTimeout", err, derrors.GetCode(err))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("send waited on a dial that never returns; a wedged bus would hang the caller")
	}

	select {
	case <-dials:
	case <-time.After(5 * time.Second):
		t.Fatal("no dial was ever started")
	}

	// The dial is still outstanding. The next caller shares it — bounded by its
	// own deadline rather than by the dial — instead of starting a second one,
	// which godbus would only queue behind the first anyway.
	secondCtx, cancelSecond := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelSecond()

	second := wedged.send(secondCtx, toastNotification("Neru", "still there"))
	if !derrors.IsCode(second, derrors.CodeTimeout) {
		t.Errorf("a second send while the first dial is outstanding returned %v (code %q), "+
			"want CodeTimeout", second, derrors.GetCode(second))
	}

	select {
	case <-dials:
		t.Error("a second send started its own dial rather than sharing the outstanding one")
	default:
	}
}

// TestNotifier_SessionReportsAFailedRedialRatherThanTheDroppedConnection pins
// what the cached connection is worth once the session bus goes away. A logind
// session ending, a bus restart or a suspend all drop it, and the notifier then
// holds a connection nothing can be sent over. Handing that back would have the
// caller talk into a dead transport and report the daemon rejecting a message
// that never left — sending a user to look at a daemon that is fine.
func TestNotifier_SessionReportsAFailedRedialRatherThanTheDroppedConnection(t *testing.T) {
	dropped := openConn(t)
	redialed := openConn(t)

	dials := 0

	reconnecting := newNotifier(func() (*dbus.Conn, error) {
		dials++

		switch dials {
		case 1:
			return dropped, nil
		case 2:
			return nil, errNoBus
		default:
			return redialed, nil
		}
	})

	first, err := reconnecting.session(context.Background())
	if err != nil {
		t.Fatalf("the first session dial failed: %v", err)
	}

	if first != dropped {
		t.Fatal("session returned a connection the dial did not produce")
	}

	// The bus goes away under the cached connection, and the redial fails.
	_ = dropped.Close()

	_, err = reconnecting.session(context.Background())
	if err == nil {
		t.Fatal("session handed back the dropped connection instead of reporting the failed redial")
	}

	if !derrors.IsNotSupported(err) {
		t.Errorf("session returned %v (code %q), want CodeNotSupported naming the session bus",
			err, derrors.GetCode(err))
	}

	if !strings.Contains(err.Error(), notifyBusName) {
		t.Errorf("error %q does not say what could not be reached", err.Error())
	}

	// A failed dial leaves nothing cached, so the session coming back is picked
	// up rather than the failure sticking for the rest of the daemon's life.
	third, err := reconnecting.session(context.Background())
	if err != nil {
		t.Fatalf("session did not recover once the bus came back: %v", err)
	}

	if third != redialed {
		t.Error("session returned a stale connection after a successful redial")
	}
}

// TestNotifier_DaemonReachableReportsNotSupportedWithoutASessionBus keeps the
// `neru doctor` probe honest in the same session: a capability that cannot be
// confirmed must not answer "reachable".
func TestNotifier_DaemonReachableReportsNotSupportedWithoutASessionBus(t *testing.T) {
	err := unreachableNotifier().daemonReachable(context.Background())
	if err == nil {
		t.Fatal("daemonReachable with no session bus returned nil")
	}

	if !derrors.IsNotSupported(err) {
		t.Errorf("daemonReachable returned %v (code %q), want CodeNotSupported",
			err, derrors.GetCode(err))
	}
}

// TestNotifyError_ClassifiesWhatTheCallerCanActOn separates the two cases that
// look alike on the wire and mean different things to a caller: nobody is
// listening (degrade), versus the daemon refused this message (a live
// failure). Misclassifying the second as CodeNotSupported would have callers
// stop trying to notify for the rest of the session.
func TestNotifyError_ClassifiesWhatTheCallerCanActOn(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want derrors.Code
	}{
		{
			name: "no daemon owns the name",
			err:  dbus.Error{Name: serviceUnknownError, Body: []any{"no such service"}},
			want: derrors.CodeNotSupported,
		},
		{
			name: "the name has no owner",
			err:  dbus.Error{Name: nameHasNoOwnerError, Body: []any{"no owner"}},
			want: derrors.CodeNotSupported,
		},
		{
			name: "a daemon replied with an error",
			err: dbus.Error{
				Name: "org.freedesktop.DBus.Error.InvalidArgs",
				Body: []any{"bad args"},
			},
			want: derrors.CodeActionFailed,
		},
		{
			name: "the daemon never answered",
			err:  context.DeadlineExceeded,
			want: derrors.CodeTimeout,
		},
		{
			name: "the connection went away mid-call",
			err:  dbus.ErrClosed,
			want: derrors.CodeNotSupported,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := notifyError(testCase.err)
			if err == nil {
				t.Fatal("notifyError returned nil for a failed call")
			}

			if got := derrors.GetCode(err); got != testCase.want {
				t.Errorf("notifyError(%v) has code %q, want %q", testCase.err, got, testCase.want)
			}
		})
	}
}

// TestDaemonReachableFrom_CountsAServiceTheBusWouldStart pins the question the
// probe is actually asking. Most desktops ship their notification daemon as a
// D-Bus service, which owns nothing until the first Notify starts it, so
// ownership alone would have `neru doctor` send a user to install what they
// already have — and the message they would see says to install a daemon.
func TestDaemonReachableFrom_CountsAServiceTheBusWouldStart(t *testing.T) {
	tests := []struct {
		name        string
		owned       bool
		activatable []string
		want        bool
	}{
		{
			name:  "a running daemon owns the name",
			owned: true,
			want:  true,
		},
		{
			name:        "nothing owns it, but the bus would start one",
			activatable: []string{"org.freedesktop.portal.Desktop", notifyBusName},
			want:        true,
		},
		{
			name:        "nothing owns it and nothing would be started",
			activatable: []string{"org.freedesktop.portal.Desktop"},
			want:        false,
		},
		{
			name: "the bus can start nothing at all",
			want: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := daemonReachableFrom(testCase.owned, testCase.activatable)
			if got != testCase.want {
				t.Errorf("daemonReachableFrom(%t, %v) = %t, want %t",
					testCase.owned, testCase.activatable, got, testCase.want)
			}
		})
	}
}

// TestIsMissingDaemon_MatchesBothGodbusErrorShapes guards the reason the check
// is written twice: godbus yields dbus.Error by value from a client call and
// *dbus.Error from NewError, and matching only one shape would classify a
// missing daemon as a live failure.
func TestIsMissingDaemon_MatchesBothGodbusErrorShapes(t *testing.T) {
	byValue := dbus.Error{Name: serviceUnknownError}
	byPointer := dbus.NewError(serviceUnknownError, nil)

	if !isMissingDaemon(byValue) {
		t.Error("a dbus.Error value naming ServiceUnknown was not recognized as a missing daemon")
	}

	if !isMissingDaemon(byPointer) {
		t.Error("a *dbus.Error naming ServiceUnknown was not recognized as a missing daemon")
	}

	if isMissingDaemon(errNoBus) {
		t.Error("a plain error was mistaken for a missing daemon")
	}
}

// TestAlertNotification_StaysUpUntilDismissed pins the decision this change
// made about what an alert is on Linux: macOS blocks the user with a modal
// NSAlert, and the Linux stand-in is a critical-urgency notification with no
// expiry. A daemon may expire a normal-urgency message on its own, so a config
// error delivered as a toast is one the user can miss entirely.
func TestAlertNotification_StaysUpUntilDismissed(t *testing.T) {
	alert := alertNotification("title", "body")

	if alert.urgency != urgencyCritical {
		t.Errorf("alert urgency = %d, want %d (critical)", alert.urgency, urgencyCritical)
	}

	if alert.expire != expireNever {
		t.Errorf("alert expiry = %d, want %d (never)", alert.expire, expireNever)
	}

	toast := toastNotification("title", "body")

	if toast.urgency != urgencyNormal {
		t.Errorf("toast urgency = %d, want %d (normal)", toast.urgency, urgencyNormal)
	}

	if toast.expire != expireDaemonChoice {
		t.Errorf("toast expiry = %d, want %d (daemon's choice)", toast.expire, expireDaemonChoice)
	}
}

// TestWithNotifyDeadline_KeepsTheCallersDeadline checks the bound: a caller
// that stated what it can afford keeps it, and one that did not gets a cap, so
// no path can wait on a wedged daemon forever.
func TestWithNotifyDeadline_KeepsTheCallersDeadline(t *testing.T) {
	callerDeadline := time.Now().Add(time.Hour)

	ctx, cancel := context.WithDeadline(context.Background(), callerDeadline)
	defer cancel()

	bounded, cancelBounded := withNotifyDeadline(ctx)
	defer cancelBounded()

	kept, hasDeadline := bounded.Deadline()
	if !hasDeadline {
		t.Fatal("withNotifyDeadline dropped the caller's deadline")
	}

	if !kept.Equal(callerDeadline) {
		t.Errorf("deadline = %v, want the caller's %v", kept, callerDeadline)
	}

	capped, cancelCapped := withNotifyDeadline(context.Background())
	defer cancelCapped()

	deadline, hasDeadline := capped.Deadline()
	if !hasDeadline {
		t.Fatal("withNotifyDeadline left a deadline-less context unbounded")
	}

	if remaining := time.Until(deadline); remaining > notifyTimeout {
		t.Errorf("imposed deadline is %v away, want at most %v", remaining, notifyTimeout)
	}
}
