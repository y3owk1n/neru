package state_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails the package if any goroutine outlives the tests.
//
// Every state type in this package notifies subscribers from goroutines
// (AppState uses `go callback(...)` for all three of its flags, ModifierState
// does the same for its initial OnChange delivery). A callback goroutine that
// blocks — on a mutex a test still holds, on a channel nobody drains — is
// invisible to an ordinary assertion: the test that spawned it has already
// moved on, and the leak only surfaces later as an unrelated test appearing to
// hang.
//
// That is not hypothetical here. TestModifierState_OnChange once ran out a
// 10-minute package timeout and took the whole binary down with an unreadable
// goroutine dump. It has not reproduced since — roughly 250 stress iterations
// including -race did not trigger it — so the mechanism is still unknown, and a
// stuck notification goroutine is the most plausible candidate. The waits in
// these tests are now bounded so a stall fails fast and names the callback that
// did not fire; this catches the complementary case where the test passes but
// leaves a goroutine wedged behind it.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
