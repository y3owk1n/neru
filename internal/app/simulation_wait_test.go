package app_test

import (
	"testing"
	"time"
)

// TestSimHarness_WaitForWithinAsksTheConditionAfterTheBudgetIsSpent pins the
// property that keeps the journeys off the flake list (#1324): a wait fails
// only on work that has not happened.
//
// It is written as the shape a loaded runner produces rather than by loading
// one. A poll sleeps, the runner takes the CPU away for longer than the whole
// budget, and the poll wakes to a deadline already in the past — with the
// awaited work long since done. Handing waitForWithin a budget that is already
// spent is exactly that moment, minus the waiting: a wait that consults the
// clock before the condition never asks it at all and fails the journey, and
// one that asks first sees the work and returns.
func TestSimHarness_WaitForWithinAsksTheConditionAfterTheBudgetIsSpent(t *testing.T) {
	sim := &simHarness{t: t}

	const spent = -time.Second

	asked := 0

	sim.waitForWithin(spent, "work that finished while the process was starved", func() bool {
		asked++

		return true
	})

	if asked == 0 {
		t.Fatal(
			"waitForWithin returned without asking its condition once the budget was spent; " +
				"a journey descheduled past its deadline then fails on work that did happen",
		)
	}
}
