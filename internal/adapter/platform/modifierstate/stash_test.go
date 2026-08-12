package modifierstate_test

import (
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/platform/modifierstate"
)

// button numbers are arbitrary; a stash only ever passes them back.
const (
	leftButton  = 1
	rightButton = 3
)

// TestStash_Take_ReportsThePlanThePressStashed is what the stash exists for: a
// press and its release are two separate calls, so the edits that undo the
// press have to outlive the call that made them.
func TestStash_Take_ReportsThePlanThePressStashed(t *testing.T) {
	var stash modifierstate.Stash

	stash.Put(leftButton, modifierstate.Plan{
		Suppress: []uint32{controlLeft},
		Press:    []uint32{shiftLeft},
	})

	plan, held := stash.Take(leftButton)
	if !held {
		t.Fatal("Take reported no plan for a button a press stashed one for")
	}

	if !equalKeycodes(plan.Suppress, []uint32{controlLeft}) {
		t.Fatalf("Take returned suppress %v, want %v", plan.Suppress, []uint32{controlLeft})
	}

	if !equalKeycodes(plan.Press, []uint32{shiftLeft}) {
		t.Fatalf("Take returned press %v, want %v", plan.Press, []uint32{shiftLeft})
	}
}

// TestStash_Take_AnswersNothingForAReleaseWithNoPress covers a release this
// process never made the press for — a bare mouse_up action, or a daemon
// restarted mid-drag. There is nothing to undo, and reporting so is how the
// caller knows to present the release's own modifiers instead.
func TestStash_Take_AnswersNothingForAReleaseWithNoPress(t *testing.T) {
	var stash modifierstate.Stash

	plan, held := stash.Take(leftButton)
	if held {
		t.Fatalf("Take reported plan %v for a button nothing pressed, want none", plan)
	}
}

// TestStash_Take_ForgetsThePlanItHandedBack keeps a second release from undoing
// the same press twice: pressing a suppressed key back a second time leaves a
// hold nothing answers.
func TestStash_Take_ForgetsThePlanItHandedBack(t *testing.T) {
	var stash modifierstate.Stash

	stash.Put(leftButton, modifierstate.Plan{Suppress: []uint32{controlLeft}})

	if _, held := stash.Take(leftButton); !held {
		t.Fatal("the first Take reported no plan, want the one that was put")
	}

	if plan, held := stash.Take(leftButton); held {
		t.Fatalf("the second Take reported plan %v, want none", plan)
	}
}

// TestStash_Take_KeepsEachButtonApart covers two buttons held at once: the
// release of one must not undo the hold the other is still relying on.
func TestStash_Take_KeepsEachButtonApart(t *testing.T) {
	var stash modifierstate.Stash

	stash.Put(leftButton, modifierstate.Plan{Suppress: []uint32{controlLeft}})
	stash.Put(rightButton, modifierstate.Plan{Suppress: []uint32{altLeft}})

	plan, held := stash.Take(rightButton)
	if !held || !equalKeycodes(plan.Suppress, []uint32{altLeft}) {
		t.Fatalf("Take(right) = %v, %t, want the right button's own plan", plan, held)
	}

	plan, held = stash.Take(leftButton)
	if !held || !equalKeycodes(plan.Suppress, []uint32{controlLeft}) {
		t.Fatalf("Take(left) = %v, %t, want the left button's own plan", plan, held)
	}
}

// TestStash_Put_CarriesBothPressesForwardForOneButton covers a second press of a
// button already down. Dropping the first plan would strand both of its halves:
// nothing would release the key it pressed, and nothing would press back the key
// it suppressed — the second press cannot see that key held, because the first
// one released it.
func TestStash_Put_CarriesBothPressesForwardForOneButton(t *testing.T) {
	var stash modifierstate.Stash

	stash.Put(leftButton, modifierstate.Plan{
		Suppress: []uint32{controlLeft},
		Press:    []uint32{shiftLeft},
	})
	stash.Put(leftButton, modifierstate.Plan{
		Suppress: []uint32{altLeft},
		Press:    []uint32{superLeft},
	})

	plan, held := stash.Take(leftButton)
	if !held {
		t.Fatal("Take reported no plan after two presses")
	}

	wantSuppress := []uint32{controlLeft, altLeft}
	if !equalKeycodes(plan.Suppress, wantSuppress) {
		t.Fatalf("Take returned suppress %v, want %v", plan.Suppress, wantSuppress)
	}

	wantPress := []uint32{shiftLeft, superLeft}
	if !equalKeycodes(plan.Press, wantPress) {
		t.Fatalf("Take returned press %v, want %v", plan.Press, wantPress)
	}
}

// TestStash_Put_DropsAKeyTheEarlierPressAlreadyAnswers is the crossing case:
// the second press reads a key the first one pressed as held, so a naive union
// would both release it and press it back. Whichever half of the pair the later
// plan repeats, the earlier one already describes what the key was before Neru
// touched it, and that is the state the release has to land on.
func TestStash_Put_DropsAKeyTheEarlierPressAlreadyAnswers(t *testing.T) {
	var stash modifierstate.Stash

	// The first press pressed shift itself and released the ctrl the user was
	// holding; the second sees exactly the opposite of both.
	stash.Put(leftButton, modifierstate.Plan{
		Suppress: []uint32{controlLeft},
		Press:    []uint32{shiftLeft},
	})
	stash.Put(leftButton, modifierstate.Plan{
		Suppress: []uint32{shiftLeft},
		Press:    []uint32{controlLeft},
	})

	plan, _ := stash.Take(leftButton)

	if !equalKeycodes(plan.Suppress, []uint32{controlLeft}) {
		t.Fatalf(
			"Take returned suppress %v, want only the ctrl the user is holding",
			plan.Suppress,
		)
	}

	if !equalKeycodes(plan.Press, []uint32{shiftLeft}) {
		t.Fatalf("Take returned press %v, want only the shift Neru pressed", plan.Press)
	}
}

// TestStash_Put_SerializesConcurrentPresses pins the lock. Presses and releases
// arrive on whichever goroutine ran the action, so the map behind the stash is
// reachable from more than one at a time.
func TestStash_Put_SerializesConcurrentPresses(t *testing.T) {
	var (
		stash modifierstate.Stash
		wait  sync.WaitGroup
	)

	const buttons = 8

	// One slot per goroutine, so the answers say which button lost its plan.
	restored := make([]bool, buttons)

	for button := range uint32(buttons) {
		wait.Go(func() {
			stash.Put(button, modifierstate.Plan{Suppress: []uint32{controlLeft}})

			plan, held := stash.Take(button)
			restored[button] = held && equalKeycodes(plan.Suppress, []uint32{controlLeft})
		})
	}

	wait.Wait()

	for button, ok := range restored {
		if !ok {
			t.Fatalf("button %d did not get its own plan back", button)
		}
	}
}
