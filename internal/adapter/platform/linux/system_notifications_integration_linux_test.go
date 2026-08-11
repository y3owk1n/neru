//go:build integration && linux

package linux_test

import (
	"context"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/derrors"
)

// The freedesktop Desktop Notification Specification, spelled out here rather
// than imported from the adapter: this test is the independent statement of
// what has to go on the wire, so sharing the adapter's constants would let a
// wrong name pass on both sides.
const (
	notificationsName   = "org.freedesktop.Notifications"
	notificationsPath   = dbus.ObjectPath("/org/freedesktop/Notifications")
	criticalUrgency     = byte(2)
	normalUrgency       = byte(1)
	neverExpires        = int32(0)
	daemonChoosesExpiry = int32(-1)
)

// notifyCall is one delivered Notify, in the argument order the specification
// fixes.
type notifyCall struct {
	appName  string
	replaces uint32
	icon     string
	summary  string
	body     string
	actions  []string
	hints    map[string]dbus.Variant
	expire   int32
}

// fakeDaemon stands in for mako, dunst or a desktop's own daemon: it owns the
// notification name for the length of the test and records what arrives.
type fakeDaemon struct {
	calls chan notifyCall
}

// Notify is the spec's single method. godbus exports it by name and derives the
// signature from these parameters, so a wrong order or type here fails the call
// exactly as a real daemon would.
func (d *fakeDaemon) Notify(
	appName string,
	replaces uint32,
	icon, summary, body string,
	actions []string,
	hints map[string]dbus.Variant,
	expire int32,
) (uint32, *dbus.Error) {
	d.calls <- notifyCall{
		appName:  appName,
		replaces: replaces,
		icon:     icon,
		summary:  summary,
		body:     body,
		actions:  actions,
		hints:    hints,
		expire:   expire,
	}

	return 1, nil
}

// startFakeDaemon owns the notification name on the session bus for this test.
// It skips rather than fails when there is no bus (a container or an ssh
// session) or when a real daemon already owns the name — neither says anything
// about the code under test.
//
// That makes the delivery tests and TestShowNotification_ReportsASessionWithNoDaemon
// mutually exclusive by construction, which is the point: a session either has
// something able to show a notification or it does not, and each test covers
// the state its machine is actually in. Between a developer desktop and CI both
// halves get exercised.
func startFakeDaemon(t *testing.T) *fakeDaemon {
	t.Helper()

	conn, err := dbus.SessionBus()
	if err != nil {
		t.Skipf("no D-Bus session bus on this machine: %v", err)
	}

	daemon := &fakeDaemon{calls: make(chan notifyCall, 1)}

	err = conn.Export(daemon, notificationsPath, notificationsName)
	if err != nil {
		t.Fatalf("exporting the fake notification daemon failed: %v", err)
	}

	reply, err := conn.RequestName(notificationsName, dbus.NameFlagDoNotQueue)
	if err != nil {
		t.Fatalf("requesting %s failed: %v", notificationsName, err)
	}

	if reply != dbus.RequestNameReplyPrimaryOwner {
		t.Skipf("%s is already owned by a real notification daemon", notificationsName)
	}

	t.Cleanup(func() {
		_, _ = conn.ReleaseName(notificationsName)
		_ = conn.Export(nil, notificationsPath, notificationsName)
	})

	return daemon
}

// awaitNotification returns the delivered call, failing rather than hanging if
// nothing arrives.
func (d *fakeDaemon) awaitNotification(t *testing.T) notifyCall {
	t.Helper()

	select {
	case call := <-d.calls:
		return call
	case <-time.After(5 * time.Second):
		t.Fatal("no Notify reached the daemon within 5s")

		return notifyCall{}
	}
}

// TestShowNotification_ReachesTheFreedesktopDaemon is the wire-level check the
// unit tests cannot make: that the arguments Neru sends are the ones a real
// notification daemon accepts, in the order and types the specification fixes.
// A signature mistake here is invisible to a compiler and silent at runtime —
// the message simply never appears.
func TestShowNotification_ReachesTheFreedesktopDaemon(t *testing.T) {
	daemon := startFakeDaemon(t)

	err := linux.ShowNotification(context.Background(), "Neru", "Scroll invert: on")
	if err != nil {
		t.Fatalf("ShowNotification failed against a live daemon: %v", err)
	}

	call := daemon.awaitNotification(t)

	if call.summary != "Neru" {
		t.Errorf("summary = %q, want %q", call.summary, "Neru")
	}

	if call.body != "Scroll invert: on" {
		t.Errorf("body = %q, want %q", call.body, "Scroll invert: on")
	}

	if urgency, ok := call.hints["urgency"].Value().(byte); !ok || urgency != normalUrgency {
		t.Errorf("urgency hint = %v, want %d", call.hints["urgency"].Value(), normalUrgency)
	}

	if call.expire != daemonChoosesExpiry {
		t.Errorf("expire_timeout = %d, want %d (daemon's choice)",
			call.expire, daemonChoosesExpiry)
	}
}

// TestShowNotification_ReportsASessionWithNoDaemon is the other half, and the
// state this change exists for: a session bus that works and nothing owning the
// notification name — an ordinary minimal wlroots setup. The bus answers
// ServiceUnknown, and that has to reach the caller as CodeNotSupported instead
// of being swallowed the way the empty body swallowed it.
func TestShowNotification_ReportsASessionWithNoDaemon(t *testing.T) {
	conn, err := dbus.SessionBus()
	if err != nil {
		t.Skipf("no D-Bus session bus on this machine: %v", err)
	}

	var owned bool

	err = conn.BusObject().
		Call("org.freedesktop.DBus.NameHasOwner", 0, notificationsName).
		Store(&owned)
	if err != nil {
		t.Fatalf("asking the bus who owns %s failed: %v", notificationsName, err)
	}

	if owned {
		t.Skipf("%s is owned here, so the no-daemon path cannot be exercised", notificationsName)
	}

	err = linux.ShowNotification(context.Background(), "Neru", "nobody is listening")
	if err == nil {
		t.Fatal("ShowNotification with no daemon returned nil; the message was dropped silently")
	}

	if !derrors.IsNotSupported(err) {
		t.Errorf("ShowNotification returned %v (code %q), want CodeNotSupported",
			err, derrors.GetCode(err))
	}
}

// TestShowAlert_ReachesTheDaemonAndStaysUp pins the decision Linux made about
// alerts: macOS blocks the user with a modal NSAlert, and the stand-in is a
// critical-urgency notification with no expiry, so a config error cannot scroll
// away before it is read.
func TestShowAlert_ReachesTheDaemonAndStaysUp(t *testing.T) {
	daemon := startFakeDaemon(t)

	err := linux.ShowAlert(context.Background(), "Neru could not load the config", "line 3")
	if err != nil {
		t.Fatalf("ShowAlert failed against a live daemon: %v", err)
	}

	call := daemon.awaitNotification(t)

	if urgency, ok := call.hints["urgency"].Value().(byte); !ok || urgency != criticalUrgency {
		t.Errorf(
			"urgency hint = %v, want %d (critical)",
			call.hints["urgency"].Value(),
			criticalUrgency,
		)
	}

	if call.expire != neverExpires {
		t.Errorf("expire_timeout = %d, want %d (never)", call.expire, neverExpires)
	}
}
