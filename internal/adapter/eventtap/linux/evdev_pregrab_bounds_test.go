//go:build linux && cgo

package linux

import (
	"testing"
	"time"
)

// TestPreGrabBoundsStayBoundedAndOrdered pins the two pre-grab waits as bounds
// rather than conditions, and pins which of them is the long one.
//
// waitForEvdevKeysReleased cannot be driven from a test — it reads EVIOCGKEY off
// real /dev/input devices — so what is asserted here is the policy those
// constants carry, which is the half that regressed twice and both times
// silently:
//
//   - The modifier wait had no deadline at all. A modifier the kernel reported
//     as held left the mode active, the overlay drawn and the keyboard never
//     grabbed, with every key going to the focused application and nothing
//     logged. Nothing failed; the daemon just stopped taking input.
//   - The hold wait had the same five seconds as the modifier one, which is
//     what a user typing through an activation actually waits out: they hold a
//     key at almost every poll, so the wait runs to its deadline and the mode
//     accepts nothing until it does.
//
// The ordering is the design and not an accident of the numbers: the modifier
// wait is allowed to be the patient one because swallowing a modifier's release
// leaves it stuck across every application the user touches next, while the
// hold wait runs only once modifiers are clear, so the worst it can cost is one
// suppressed press on a plain key — which initialKeys already handles.
//
// Both bounds must also outlast the poll that watches them, or the loop reaches
// its deadline before it has looked at the keyboard even once and the wait
// becomes a sleep.
func TestPreGrabBoundsStayBoundedAndOrdered(t *testing.T) {
	t.Parallel()

	if waylandEvdevModifierReleaseTimeout <= 0 {
		t.Errorf(
			"waylandEvdevModifierReleaseTimeout = %v; the modifier wait must carry a "+
				"deadline — without one a held modifier means the keyboard is never "+
				"grabbed and every key reaches the focused application instead "+
				"(internal/adapter/platform/linux/AGENTS.md: never block on the "+
				"eventtap goroutine)",
			waylandEvdevModifierReleaseTimeout,
		)
	}

	if waylandEvdevPreGrabHoldTimeout <= 0 {
		t.Errorf(
			"waylandEvdevPreGrabHoldTimeout = %v; the hold wait must carry a deadline",
			waylandEvdevPreGrabHoldTimeout,
		)
	}

	if waylandEvdevPreGrabHoldTimeout >= waylandEvdevModifierReleaseTimeout {
		t.Errorf(
			"hold wait %v is not shorter than the modifier wait %v; a plain key held "+
				"at grab time costs one suppressed press, a modifier costs every "+
				"application the user touches next, so the patient one is the modifier "+
				"wait — a hold wait grown to match it is the mashing-keys bug coming back",
			waylandEvdevPreGrabHoldTimeout, waylandEvdevModifierReleaseTimeout,
		)
	}

	for _, bound := range []struct {
		name    string
		timeout time.Duration
		poll    time.Duration
	}{
		{"modifier", waylandEvdevModifierReleaseTimeout, waylandEvdevModifierReleasePollPeriod},
		{"hold", waylandEvdevPreGrabHoldTimeout, waylandEvdevPreGrabHoldPollPeriod},
	} {
		if bound.poll >= bound.timeout {
			t.Errorf(
				"the %s wait polls every %v against a %v deadline; a wait that expires "+
					"before it has read the keyboard twice is a sleep, not a wait",
				bound.name, bound.poll, bound.timeout,
			)
		}
	}
}
