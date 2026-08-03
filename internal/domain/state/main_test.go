package state_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails the package if any goroutine outlives the tests. Every state
// type here notifies subscribers from goroutines, and a blocked callback
// goroutine is invisible to the test that spawned it — the leak surfaces later
// as an unrelated test hanging. TestModifierState_OnChange once burned a
// 10-minute package timeout that way and never reproduced (~250 stress runs);
// bounded waits catch a stall, this catches the pass that leaves a goroutine
// wedged behind it.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
