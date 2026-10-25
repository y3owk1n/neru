package state_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails the package if any goroutine outlives the tests. Every state
// type here notifies subscribers from goroutines, and a blocked callback
// goroutine is invisible to the test that spawned it, so the leak surfaces later
// as an unrelated test hanging. TestModifierState_OnChange once burned a
// 10-minute package timeout that way and never reproduced (~250 stress runs).
// Bounded waits catch a stall, and this catches the pass that leaves a
// goroutine stuck behind it.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
