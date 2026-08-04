//go:build darwin

package darwin

import (
	"image"
	"sync"
	"testing"
)

// testAnimator returns an animator whose request channel exists but has no
// worker goroutine behind it. submitLocked then only queues (and drops) —
// nothing consumes the requests, so no mouse events are posted during tests.
func testAnimator() *smoothCursorAnimator {
	return &smoothCursorAnimator{reqCh: make(chan cursorRequest, 1)}
}

func (a *smoothCursorAnimator) submitForTest(end image.Point) *cursorAnimationDone {
	done := newCursorAnimationDone()

	a.mu.Lock()
	a.submitLocked(cursorRequest{end: end, done: done})
	a.mu.Unlock()

	return done
}

// TestCursorAnimator_PendingTarget pins the pending-target contract relative
// moves build on: the endpoint of the animation in flight is visible while it
// is pending and gone once the animation is stopped. Without it, each held-key
// repeat would restart from the mid-animation cursor position and silently
// drop the un-animated remainder of every delta.
func TestCursorAnimator_PendingTarget(t *testing.T) {
	animator := testAnimator()

	if _, ok := animator.pendingTarget(); ok {
		t.Fatal("pendingTarget() reported an animation before any was submitted")
	}

	end := image.Point{X: 120, Y: 80}
	animator.submitForTest(end)

	got, ok := animator.pendingTarget()
	if !ok {
		t.Fatal("pendingTarget() reported no animation while one is pending")
	}

	if got != end {
		t.Fatalf("pendingTarget() = %v, want %v", got, end)
	}

	animator.stop()

	if _, ok := animator.pendingTarget(); ok {
		t.Fatal("pendingTarget() still reports an animation after stop()")
	}
}

// TestCursorAnimator_ClearDoneIfCurrent_KeepsNewerPending guards the
// completion path: finishing an older animation must not clear the pending
// endpoint of a newer one that superseded it.
func TestCursorAnimator_ClearDoneIfCurrent_KeepsNewerPending(t *testing.T) {
	animator := testAnimator()

	older := animator.submitForTest(image.Point{X: 10, Y: 10})

	newerEnd := image.Point{X: 20, Y: 20}
	newer := animator.submitForTest(newerEnd)

	animator.clearDoneIfCurrent(older)

	got, ok := animator.pendingTarget()
	if !ok || got != newerEnd {
		t.Fatalf("pendingTarget() = %v, %v after stale clear, want %v, true", got, ok, newerEnd)
	}

	animator.clearDoneIfCurrent(newer)

	if _, ok := animator.pendingTarget(); ok {
		t.Fatal("pendingTarget() still reports an animation after the current one completed")
	}
}

// TestCursorAnimator_AnimateRelativeBy_ConcurrentDeltasCompose pins the
// single-lock read-modify-write of animateRelativeBy: deltas from concurrent
// movers (mode held-repeat and hotkey held-repeat are independent goroutines)
// must all end up in the pending endpoint, none silently overwritten. Run
// under -race, this also covers the submit/pendingTarget interleavings.
func TestCursorAnimator_AnimateRelativeBy_ConcurrentDeltasCompose(t *testing.T) {
	animator := testAnimator()

	// Seed a pending animation so every animateRelativeBy extends the endpoint
	// rather than reading the live cursor position.
	animator.submitForTest(image.Point{})

	const movers = 8

	const deltasPerMover = 50

	identity := func(p image.Point) image.Point { return p }

	var waitGroup sync.WaitGroup
	for range movers {
		waitGroup.Go(func() {
			for range deltasPerMover {
				animator.animateRelativeBy(image.Point{X: 1, Y: 0}, identity, 1, 1, 0, 0)
			}
		})
	}

	waitGroup.Wait()

	got, ok := animator.pendingTarget()
	if !ok {
		t.Fatal("pendingTarget() reported no animation after concurrent relative moves")
	}

	if want := movers * deltasPerMover; got.X != want {
		t.Fatalf("pending endpoint X = %d after %d concurrent unit deltas, want %d (lost deltas)",
			got.X, want, want)
	}
}
