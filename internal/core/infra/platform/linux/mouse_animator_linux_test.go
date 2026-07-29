//go:build linux

//nolint:testpackage // Exercises the unexported smooth cursor animator directly.
package linux

import (
	"context"
	"image"
	"sync"
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
