//go:build linux

package kwin

import (
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"
)

// testKWinOwner is a bus name KWin could hold; only whether it is empty matters.
const testKWinOwner = ":1.42"

// TestGeometry_KWinLeavingTheBusReportsWhyRatherThanEmptying covers the
// staleness a restart causes, and what has to be said about it.
//
// The script lives inside KWin, so `kwin --replace` or a Plasma crash takes it
// with it. Nothing would arrive again, and an installed bridge never retries —
// so without this the cache would stay frozen at the last rectangle and serve
// it as the truth for the rest of the daemon's life, which is the same
// confidently wrong answer reached by a different route.
//
// Emptying it is only half the answer. An empty cache with no reason means the
// bridge works and nothing is focused, which would send a caller to the active
// screen believing it had asked and been told — the silent widening this bridge
// exists to end. A compositor that is not on the bus has to say so.
func TestGeometry_KWinLeavingTheBusReportsWhyRatherThanEmptying(t *testing.T) {
	geometry := newGeometry(zap.NewNop())

	geometry.beginStart()
	geometry.endStart(nil)
	_ = geometry.UpdateActiveWindow("100,50,800,600,konsole,konsole,Konsole")

	geometry.kwinOwnerChanged("")

	rect, ok, err := geometry.Bounds()
	if ok {
		t.Fatalf("Bounds() after KWin left = (%v, %v), want no window", rect, ok)
	}

	if !errors.Is(err, errKWinAbsent) {
		t.Errorf("Bounds() error = %v, want it to name the absent compositor — "+
			"an empty cache with no reason reads as an unfocused desktop", err)
	}

	if !geometry.beginStart() {
		t.Error("a bridge whose compositor restarted still counted itself installed, " +
			"so nothing would ever reinstall the script")
	}
}

// TestGeometry_KWinReturningToTheBusClearsTheReason keeps the departure from
// outliving itself: a compositor that came back is not an absent one, and the
// reinstall that follows is what says so.
func TestGeometry_KWinReturningToTheBusClearsTheReason(t *testing.T) {
	geometry := newGeometry(zap.NewNop())

	geometry.kwinOwnerChanged("")

	geometry.beginStart()
	geometry.endStart(nil)

	_, ok, err := geometry.Bounds()
	if ok || err != nil {
		t.Fatalf("Bounds() after a reinstall = (%v, %v), want (false, nil) — "+
			"installed, with nothing reported yet", ok, err)
	}
}

// TestKWinOwnerFrom_ReadsOnlyKWinOwnerChanges keeps the restart watch from
// acting on somebody else's traffic.
//
// The match rule is a request to the bus, not a guarantee about what arrives:
// this is the process-wide session connection, so every other subscription on
// it is delivered here too. A stray signal read as "KWin restarted" would empty
// a good cache and reinstall the script for no reason.
func TestKWinOwnerFrom_ReadsOnlyKWinOwnerChanges(t *testing.T) {
	cases := []struct {
		name      string
		signal    *dbus.Signal
		wantOwner string
		wantOK    bool
	}{
		{
			name: "kwin acquired the name",
			signal: &dbus.Signal{
				Name: dbusNameOwnerSignal,
				Body: []any{scriptingDest, "", testKWinOwner},
			},
			wantOwner: testKWinOwner,
			wantOK:    true,
		},
		{
			name: "kwin released the name",
			signal: &dbus.Signal{
				Name: dbusNameOwnerSignal,
				Body: []any{scriptingDest, testKWinOwner, ""},
			},
			wantOK: true,
		},
		{
			name: "another name changed hands",
			signal: &dbus.Signal{
				Name: dbusNameOwnerSignal,
				Body: []any{"org.kde.plasmashell", "", ":1.9"},
			},
		},
		{
			name: "another signal entirely",
			signal: &dbus.Signal{
				Name: "org.freedesktop.portal.Request.Response",
				Body: []any{scriptingDest, "", testKWinOwner},
			},
		},
		{
			name:   "truncated body",
			signal: &dbus.Signal{Name: dbusNameOwnerSignal, Body: []any{scriptingDest, ""}},
		},
		{
			name: "body of the wrong type",
			signal: &dbus.Signal{
				Name: dbusNameOwnerSignal,
				Body: []any{scriptingDest, "", 42},
			},
		},
		{name: "no signal at all"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			owner, ok := kwinOwnerFrom(testCase.signal)
			if ok != testCase.wantOK || owner != testCase.wantOwner {
				t.Errorf("kwinOwnerFrom() = (%q, %v), want (%q, %v)",
					owner, ok, testCase.wantOwner, testCase.wantOK)
			}
		})
	}
}

// TestClaim_TakeAdmitsOneHolderAtATime pins what keeps the installer's retries
// and every later reinstall from each adding a match rule and a goroutine that
// live as long as the daemon does — while still letting an arming that failed
// be tried again.
func TestClaim_TakeAdmitsOneHolderAtATime(t *testing.T) {
	var watching claim

	if !watching.take() {
		t.Fatal("the first caller was not allowed to arm the restart watch")
	}

	if watching.take() {
		t.Error("a second caller armed the restart watch again")
	}

	watching.release()

	if !watching.take() {
		t.Error("a watch that could not be armed was never retried")
	}
}
