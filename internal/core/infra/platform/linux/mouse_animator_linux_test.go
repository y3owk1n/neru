//go:build linux

//nolint:testpackage // Exercises the unexported smooth cursor animator directly.
package linux

import (
	"context"
	"errors"
	"image"
	"sync"
	"testing"
	"time"
)

// errMoveFailed is a static sentinel for the backend-move-failure test, so the
// propagated error can be matched with errors.Is.
var errMoveFailed = errors.New("virtual pointer disconnected")

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

func TestSmoothCursorAnimatorPropagatesMoveError(t *testing.T) {
	t.Parallel()

	failingMove := func(_ image.Point) error {
		return errMoveFailed
	}
	pos := func() image.Point {
		return image.Point{}
	}

	animator := newSmoothCursorAnimator(pos, failingMove)
	animator.animateTo(image.Point{X: 400, Y: 400}, 8, 60, 0.1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := animator.wait(ctx)
	if !errors.Is(err, errMoveFailed) {
		t.Fatalf("wait did not surface the backend move error: got %v want %v", err, errMoveFailed)
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
