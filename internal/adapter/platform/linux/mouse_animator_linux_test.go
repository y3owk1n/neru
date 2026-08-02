//go:build linux

//nolint:testpackage // Exercises the unexported smooth cursor animator directly.
package linux

import (
	"context"
	"image"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recorder captures the moves an animator injects so tests can assert on the
// glide path and final landing point without touching a real display server.
type recorder struct {
	mu    sync.Mutex
	moves []image.Point
	start image.Point
}

func (r *recorder) pos() image.Point {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.moves) > 0 {
		return r.moves[len(r.moves)-1]
	}

	return r.start
}

func (r *recorder) move(p image.Point) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.moves = append(r.moves, p)

	return nil
}

func (r *recorder) last() (image.Point, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.moves) == 0 {
		return image.Point{}, false
	}

	return r.moves[len(r.moves)-1], true
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.moves)
}

func TestSmoothCursorAnimatorLandsOnTarget(t *testing.T) {
	t.Parallel()

	rec := &recorder{start: image.Point{X: 0, Y: 0}}
	animator := newSmoothCursorAnimator(rec.pos, rec.move)

	target := image.Point{X: 300, Y: 200}
	animator.animateTo(target, 8, 60, 0.1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := animator.wait(ctx)
	if err != nil {
		t.Fatalf("wait returned error: %v", err)
	}

	last, ok := rec.last()
	if !ok {
		t.Fatal("expected at least one move to be injected")
	}

	if last != target {
		t.Fatalf("cursor did not land on target: got %v want %v", last, target)
	}

	if rec.count() < 2 {
		t.Fatalf("expected a stepped glide (>=2 moves), got %d", rec.count())
	}
}

func TestSmoothCursorAnimatorCoalescesToLatestTarget(t *testing.T) {
	t.Parallel()

	rec := &recorder{start: image.Point{X: 0, Y: 0}}
	animator := newSmoothCursorAnimator(rec.pos, rec.move)

	// Fire several requests back-to-back; only the last target must win.
	animator.animateTo(image.Point{X: 100, Y: 100}, 10, 80, 0.5)
	animator.animateTo(image.Point{X: 500, Y: 50}, 10, 80, 0.5)
	final := image.Point{X: 640, Y: 480}
	animator.animateTo(final, 10, 80, 0.5)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := animator.wait(ctx)
	if err != nil {
		t.Fatalf("wait returned error: %v", err)
	}

	last, ok := rec.last()
	if !ok {
		t.Fatal("expected at least one move to be injected")
	}

	if last != final {
		t.Fatalf("coalesced animation landed on %v, want latest target %v", last, final)
	}
}

func TestSmoothCursorAnimatorWaitReturnsWhenIdle(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	animator := newSmoothCursorAnimator(rec.pos, rec.move)

	// No animation started: wait must return immediately, preserving the
	// historical no-op WaitForCursorIdle behavior on the direct move path.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := animator.wait(ctx)
	if err != nil {
		t.Fatalf("wait on idle animator returned error: %v", err)
	}

	if rec.count() != 0 {
		t.Fatalf("idle animator injected %d moves, want 0", rec.count())
	}
}

// TestSmoothCursorAnimatorWaiterTracksSupersedingTarget guards the coalescing
// race: a waiter that attaches while an earlier target animates must not be
// released until the cursor reaches the *latest* target, even when the queued
// target is superseded mid-flight. With per-request completion this waiter
// returned early at the intermediate/previous position.
func TestSmoothCursorAnimatorWaiterTracksSupersedingTarget(t *testing.T) {
	t.Parallel()

	// Slow the moves so the animation spans the follow-up enqueues.
	rec := &recorder{start: image.Point{X: 0, Y: 0}}
	slowMove := func(p image.Point) error {
		time.Sleep(2 * time.Millisecond)

		return rec.move(p)
	}

	animator := newSmoothCursorAnimator(rec.pos, slowMove)

	final := image.Point{X: 900, Y: 900}

	// Start a long first animation, then attach a waiter mid-flight.
	animator.animateTo(image.Point{X: 120, Y: 120}, 20, 400, 1.0)

	waitResult := make(chan error, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		waitResult <- animator.wait(ctx)
	}()

	// Supersede the target twice while the session is still animating.
	time.Sleep(6 * time.Millisecond)
	animator.animateTo(image.Point{X: 500, Y: 500}, 20, 400, 1.0)
	time.Sleep(6 * time.Millisecond)
	animator.animateTo(final, 20, 400, 1.0)

	err := <-waitResult
	if err != nil {
		t.Fatalf("wait returned error: %v", err)
	}

	last, ok := rec.last()
	if !ok {
		t.Fatal("expected at least one move to be injected")
	}

	if last != final {
		t.Fatalf(
			"waiter released before reaching the superseding target: got %v want %v",
			last,
			final,
		)
	}
}

// TestSmoothCursorAnimatorBestEffortOnMoveFailure documents that a failing
// backend warp during the animation is best-effort (matching darwin): the
// session still settles and WaitForCursorIdle returns without error.
func TestSmoothCursorAnimatorBestEffortOnMoveFailure(t *testing.T) {
	t.Parallel()

	failingMove := func(_ image.Point) error {
		return context.DeadlineExceeded // any non-nil error
	}
	pos := func() image.Point {
		return image.Point{}
	}

	animator := newSmoothCursorAnimator(pos, failingMove)
	animator.animateTo(image.Point{X: 400, Y: 400}, 8, 60, 0.1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := animator.wait(ctx)
	if err != nil {
		t.Fatalf("smooth-move failure must not surface through wait: got %v", err)
	}
}

// TestSmoothCursorAnimatorStopFencesInflightStep verifies stop() does not
// return while a backend injection step is in flight, and blocks any further
// step from injecting. This is the ordering guarantee that lets a bypassed
// direct warp (issued right after stop) land last instead of racing a stale
// animation step.
func TestSmoothCursorAnimatorStopFencesInflightStep(t *testing.T) {
	t.Parallel()

	entered := make(chan struct{}, 1)
	release := make(chan struct{})

	var moveActive atomic.Bool

	blockingMove := func(_ image.Point) error {
		moveActive.Store(true)

		select {
		case entered <- struct{}{}:
		default:
		}

		<-release
		moveActive.Store(false)

		return nil
	}
	pos := func() image.Point {
		return image.Point{}
	}

	animator := newSmoothCursorAnimator(pos, blockingMove)
	animator.animateTo(image.Point{X: 100, Y: 100}, 10, 200, 1.0)

	<-entered // a backend step is now in flight, blocked in blockingMove

	stopReturned := make(chan struct{})

	go func() {
		animator.stop()
		close(stopReturned)
	}()

	// stop() must not return while the step is mid-injection.
	select {
	case <-stopReturned:
		t.Fatal("stop returned while a backend move was in flight")
	case <-time.After(50 * time.Millisecond):
	}

	if !moveActive.Load() {
		t.Fatal("expected the in-flight step to still be active")
	}

	close(release) // let the in-flight step finish

	select {
	case <-stopReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("stop did not return after the in-flight step completed")
	}
}

// TestSmoothCursorAnimatorConcurrentStopAndAnimate guards the animateTo/stop
// race: a bypassed move (stop) must never strand a smooth request on an
// orphaned channel with an unclosed done. Run with -race. The test passes if it
// completes without deadlocking on a wait that never returns.
func TestSmoothCursorAnimatorConcurrentStopAndAnimate(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	animator := newSmoothCursorAnimator(rec.pos, rec.move)

	var waitGroup sync.WaitGroup

	for index := range 200 {
		waitGroup.Add(2)

		go func() {
			defer waitGroup.Done()

			animator.animateTo(image.Point{X: index, Y: index}, 6, 40, 0.2)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := animator.wait(ctx)
			if err != nil {
				t.Errorf("wait blocked or errored during concurrent stop: %v", err)
			}
		}()

		go func() {
			defer waitGroup.Done()

			animator.stop()
		}()
	}

	waitGroup.Wait()
}

func TestSmoothCursorAnimatorStopReleasesWaiter(t *testing.T) {
	t.Parallel()

	rec := &recorder{start: image.Point{X: 0, Y: 0}}
	animator := newSmoothCursorAnimator(rec.pos, rec.move)

	// Long animation so stop() interrupts it mid-flight.
	animator.animateTo(image.Point{X: 5000, Y: 5000}, 200, 5000, 5.0)

	go func() {
		time.Sleep(20 * time.Millisecond)
		animator.stop()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := animator.wait(ctx)
	if err != nil {
		t.Fatalf("wait after stop returned error: %v", err)
	}
}
