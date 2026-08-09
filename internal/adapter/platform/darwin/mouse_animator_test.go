//go:build darwin

package darwin

import (
	"context"
	"image"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/domain/action"
)

// cursorRecorder stands in for the window server: it captures every move the
// animator posts, and answers pos() with the last posted point so a glide
// composes across requests the way a real cursor does.
type cursorRecorder struct {
	mu     sync.Mutex
	posted []image.Point
	events [][2]uint32 // {eventType, button} per posted move
	start  image.Point
}

func (r *cursorRecorder) pos() image.Point {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.posted) > 0 {
		return r.posted[len(r.posted)-1]
	}

	return r.start
}

func (r *cursorRecorder) post(point image.Point, eventType, button uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.posted = append(r.posted, point)
	r.events = append(r.events, [2]uint32{eventType, button})
}

func (r *cursorRecorder) last() (image.Point, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.posted) == 0 {
		return image.Point{}, false
	}

	return r.posted[len(r.posted)-1], true
}

func (r *cursorRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.posted)
}

func (r *cursorRecorder) postedEvents() [][2]uint32 {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([][2]uint32(nil), r.events...)
}

// workerAnimator returns an animator backed by a real worker goroutine, with
// the cursor position and the move post routed into rec instead of cgo.
func workerAnimator(rec *cursorRecorder) *smoothCursorAnimator {
	return newSmoothCursorAnimator(rec.pos, rec.post)
}

// testAnimator returns an animator whose request channel exists but has no
// worker goroutine behind it. submitLocked then only queues (and drops) —
// nothing consumes the requests, so no mouse events are posted during tests.
func testAnimator() *smoothCursorAnimator {
	animator := newSmoothCursorAnimator(
		func() image.Point { return image.Point{} },
		func(image.Point, uint32, uint32) {},
	)
	animator.reqCh = make(chan cursorRequest, 1)

	return animator
}

// submitForTest queues one target and hands back the completion of the session
// it joined, so a test can assert on when waiters are released.
func (a *smoothCursorAnimator) submitForTest(end image.Point) *cursorAnimationDone {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.submitLocked(cursorRequest{end: end})

	return a.done
}

// animateForTest submits an animation with explicit timing. It stands in for
// animateTo, which reads max_duration / duration_per_pixel from the process
// config; a test needs to choose them so the glide reliably outlives the
// requests that supersede it.
func (a *smoothCursorAnimator) animateForTest(
	end image.Point,
	steps, maxDuration int,
	durationPerPixel float64,
	eventType, button uint32,
) {
	a.submit(cursorRequest{
		end:              end,
		steps:            steps,
		eventType:        eventType,
		button:           button,
		maxDuration:      maxDuration,
		durationPerPixel: durationPerPixel,
	})
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

// TestCursorAnimator_FinishOrNext guards the completion path. Reaching a
// target ends the session only when nothing newer is queued — a target that
// superseded this one continues the same session, keeping its waiters
// attached — and a worker left over from a canceled session must not close a
// newer session's completion.
func TestCursorAnimator_FinishOrNext(t *testing.T) {
	animator := testAnimator()
	animator.stopCh = make(chan struct{})

	animator.submitForTest(image.Point{X: 10, Y: 10})

	newerEnd := image.Point{X: 20, Y: 20}
	done := animator.submitForTest(newerEnd)

	// A worker whose session was already canceled must leave this one alone.
	if _, ok := animator.finishOrNext(animator.reqCh, make(chan struct{})); ok {
		t.Fatal("a superseded worker was handed the current session's next target")
	}

	if got, pending := animator.pendingTarget(); !pending || got != newerEnd {
		t.Fatalf(
			"pendingTarget() = %v, %v after a stale finish, want %v, true",
			got,
			pending,
			newerEnd,
		)
	}

	select {
	case <-done.ch:
		t.Fatal("a superseded worker closed the current session's completion")
	default:
	}

	// The queued newer target continues the session rather than ending it.
	next, continued := animator.finishOrNext(animator.reqCh, animator.stopCh)
	if !continued || next.end != newerEnd {
		t.Fatalf(
			"finishOrNext() = %v, %v, want the queued target %v, true",
			next.end,
			continued,
			newerEnd,
		)
	}

	select {
	case <-done.ch:
		t.Fatal("the session completed while a newer target was still queued")
	default:
	}

	// Nothing left to animate: the session ends and its waiters are released.
	if _, ok := animator.finishOrNext(animator.reqCh, animator.stopCh); ok {
		t.Fatal("finishOrNext() handed back a target on an empty queue")
	}

	select {
	case <-done.ch:
	default:
		t.Fatal("the session completion was not closed once the last target was reached")
	}

	if _, ok := animator.pendingTarget(); ok {
		t.Fatal("pendingTarget() still reports an animation after the session completed")
	}
}

// TestCursorAnimator_AnimateRelativeBy_NoopKeepsAnimation pins the no-op
// guard: a move whose endpoint equals the pending one — zero delta, or a
// delta the screen-edge clamp ate — must not cancel and resubmit the
// animation already heading there.
func TestCursorAnimator_AnimateRelativeBy_NoopKeepsAnimation(t *testing.T) {
	animator := testAnimator()

	end := image.Point{X: 10, Y: 10}
	animator.submitForTest(end)

	identity := func(p image.Point) image.Point { return p }
	animator.animateRelativeBy(image.Point{}, identity, 1, 50, 0, 0)

	clampToEnd := func(image.Point) image.Point { return end }
	animator.animateRelativeBy(image.Point{X: 5, Y: 0}, clampToEnd, 1, 50, 0, 0)

	select {
	case req := <-animator.reqCh:
		if req.end != end {
			t.Fatalf(
				"queued request end = %v, want the original %v (no-op moves must not resubmit)",
				req.end,
				end,
			)
		}
	default:
		t.Fatal("original request missing from queue after no-op moves")
	}

	select {
	case req := <-animator.reqCh:
		t.Fatalf("no-op move enqueued an extra request with end %v", req.end)
	default:
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

// TestCursorAnimator_TakePendingForSettle pins the state transition behind
// settle(): it hands back the in-flight endpoint exactly once, releases
// waiters, invalidates queued steps, and reports ok == false on an idle
// animator so no settle warp is posted when there is nothing to finish.
func TestCursorAnimator_TakePendingForSettle(t *testing.T) {
	animator := testAnimator()

	if _, ok := animator.takePendingForSettle(); ok {
		t.Fatal("takePendingForSettle() reported an animation on an idle animator")
	}

	end := image.Point{X: 30, Y: 40}
	generation := animator.currentGeneration()
	done := animator.submitForTest(end)

	got, ok := animator.takePendingForSettle()
	if !ok || got != end {
		t.Fatalf("takePendingForSettle() = %v, %v, want %v, true", got, ok, end)
	}

	if _, ok := animator.pendingTarget(); ok {
		t.Fatal("pendingTarget() still reports an animation after settle")
	}

	select {
	case <-done.ch:
	default:
		t.Fatal("settle did not release the animation's waiters")
	}

	if animator.postIfCurrent(generation, end, 0, 0) {
		t.Fatal("a step from the settled animation was still posted after settle")
	}
}

// TestCursorAnimator_WaiterTracksSupersedingTarget guards the coalescing race,
// mirroring Linux's TestSmoothCursorAnimatorWaiterTracksSupersedingTarget: a
// waiter that attaches
// while an earlier target animates must not be released until the cursor
// reaches the *latest* target, even when the queued target is superseded
// mid-flight. With per-request completion this waiter returned early at the
// intermediate/previous position, and ActionService — which moves, waits, then
// acts — performed its action at a point the cursor had already left.
func TestCursorAnimator_WaiterTracksSupersedingTarget(t *testing.T) {
	rec := &cursorRecorder{}

	// Slow the posts so the animation spans the follow-up submissions.
	slowPost := func(point image.Point, eventType, button uint32) {
		time.Sleep(2 * time.Millisecond)
		rec.post(point, eventType, button)
	}

	animator := newSmoothCursorAnimator(rec.pos, slowPost)
	defer animator.stop()

	final := image.Point{X: 900, Y: 900}

	// Start a long first animation, then attach a waiter mid-flight.
	animator.animateForTest(image.Point{X: 120, Y: 120}, 20, 400, 1.0, 0, 0)

	waitResult := make(chan error, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		waitResult <- animator.wait(ctx)
	}()

	// Supersede the target twice while the session is still animating.
	time.Sleep(6 * time.Millisecond)
	animator.animateForTest(image.Point{X: 500, Y: 500}, 20, 400, 1.0, 0, 0)
	time.Sleep(6 * time.Millisecond)
	animator.animateForTest(final, 20, 400, 1.0, 0, 0)

	err := <-waitResult
	if err != nil {
		t.Fatalf("wait returned error: %v", err)
	}

	last, ok := rec.last()
	if !ok {
		t.Fatal("expected at least one move to be posted")
	}

	if last != final {
		t.Fatalf(
			"waiter released before reaching the superseding target: got %v want %v",
			last,
			final,
		)
	}
}

// TestCursorAnimator_WaitAfterSettledReturnsImmediately pins that a waiter
// attaching after the cursor has come to rest is not left blocking on the
// completed session.
func TestCursorAnimator_WaitAfterSettledReturnsImmediately(t *testing.T) {
	rec := &cursorRecorder{}
	animator := workerAnimator(rec)

	defer animator.stop()

	target := image.Point{X: 60, Y: 40}
	animator.animateForTest(target, 6, 60, 0.1, 0, 0)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := animator.wait(ctx)
	if err != nil {
		t.Fatalf("wait returned error: %v", err)
	}

	if last, ok := rec.last(); !ok || last != target {
		t.Fatalf("animation landed on %v, %v, want %v, true", last, ok, target)
	}

	// The session is over; a late waiter must not block on it.
	lateCtx, lateCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer lateCancel()

	lateErr := animator.wait(lateCtx)
	if lateErr != nil {
		t.Fatalf("wait on a settled animator returned error: %v", lateErr)
	}
}

// TestCursorAnimator_StopHaltsPostsAndReleasesWaiter pins the cancellation
// window: a waiter already blocked on the running session is released by
// stop() rather than left hanging, and once stop() returns no further step
// from the canceled animation may reach the window server.
func TestCursorAnimator_StopHaltsPostsAndReleasesWaiter(t *testing.T) {
	rec := &cursorRecorder{}
	animator := workerAnimator(rec)

	// Long glide so the waiter attaches, and stop() lands, mid-animation.
	animator.animateForTest(image.Point{X: 5000, Y: 5000}, 200, 5000, 5.0, 0, 0)

	waitResult := make(chan error, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		waitResult <- animator.wait(ctx)
	}()

	// The session is still running, so the waiter must still be blocked —
	// otherwise stop() releasing it below would prove nothing.
	select {
	case <-waitResult:
		t.Fatal("wait returned while the animation was still in flight")
	case <-time.After(20 * time.Millisecond):
	}

	animator.stop()

	select {
	case err := <-waitResult:
		if err != nil {
			t.Fatalf("wait after stop returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stop() did not release the blocked waiter")
	}

	settled := rec.count()

	time.Sleep(50 * time.Millisecond)

	if got := rec.count(); got != settled {
		t.Fatalf("%d step(s) posted after stop() returned, want none", got-settled)
	}
}

// TestCursorAnimator_DragEventTypeReachesEveryStep pins the darwin-only drag
// path: a move issued while a mouse button is held must post every step as a
// drag of that button, or the application never sees the drag.
func TestCursorAnimator_DragEventTypeReachesEveryStep(t *testing.T) {
	rec := &cursorRecorder{}
	animator := workerAnimator(rec)

	defer animator.stop()

	// The pair MoveMouse threads through while a button is held.
	drag := eventsForButton(action.ButtonRight)
	want := [2]uint32{uint32(drag.dragged), uint32(drag.button)}

	animator.animateForTest(image.Point{X: 90, Y: 90}, 6, 60, 0.5, want[0], want[1])

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := animator.wait(ctx)
	if err != nil {
		t.Fatalf("wait returned error: %v", err)
	}

	events := rec.postedEvents()
	if len(events) == 0 {
		t.Fatal("expected at least one move to be posted")
	}

	for index, event := range events {
		if event != want {
			t.Fatalf(
				"step %d posted {eventType, button} = %v, want the drag pair %v",
				index,
				event,
				want,
			)
		}
	}
}
