//go:build linux

package linux

import (
	"context"
	"image"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// deltaRecorder captures the native relative motions a drain injects so tests
// can assert on exactness without a compositor.
type deltaRecorder struct {
	mu     sync.Mutex
	deltas []image.Point
}

func (r *deltaRecorder) moveBy(d image.Point) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.deltas = append(r.deltas, d)

	return nil
}

func (r *deltaRecorder) sum() image.Point {
	r.mu.Lock()
	defer r.mu.Unlock()

	var total image.Point
	for _, d := range r.deltas {
		total = total.Add(d)
	}

	return total
}

func (r *deltaRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.deltas)
}

// TestRelativeCursorAnimatorDrainsExactly pins the core invariant: the posted
// chunks sum exactly to the requested delta — nothing lost to rounding — and
// the motion is chunked, not a single warp-sized jump.
func TestRelativeCursorAnimatorDrainsExactly(t *testing.T) {
	t.Parallel()

	rec := &deltaRecorder{}
	animator := newRelativeCursorAnimator(rec.moveBy)

	delta := image.Point{X: 37, Y: -23}
	animator.addDelta(delta, 5, 30)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := animator.wait(ctx)
	if err != nil {
		t.Fatalf("wait returned error: %v", err)
	}

	if got := rec.sum(); got != delta {
		t.Fatalf("drained %v, want exactly %v", got, delta)
	}

	if rec.count() < 2 {
		t.Fatalf("expected a chunked drain (>=2 motions), got %d", rec.count())
	}
}

// TestRelativeCursorAnimatorFloorsShortDrainsAtConfigMinimum is the drain twin
// of TestSmoothCursorAnimatorFloorsShortAnimationsAtConfigMinimum: this
// animator floors a below-floor drain at the same
// config.MinSmoothCursorAnimationDuration, so a gesture asked for in more
// chunks than the floor can schedule posts exactly one chunk per
// minCursorStepDelay of the floor. Exactness of the drained total is
// TestRelativeCursorAnimatorDrainsExactly's question, not this one's.
func TestRelativeCursorAnimatorFloorsShortDrainsAtConfigMinimum(t *testing.T) {
	t.Parallel()

	wantChunks := config.MinSmoothCursorAnimationDuration / minCursorStepDelay

	rec := &deltaRecorder{}
	animator := newRelativeCursorAnimator(rec.moveBy)

	// Sized so every scheduled chunk carries at least one pixel; a chunk that
	// rounded to zero would post nothing and undercount the schedule.
	delta := image.Point{X: 10 * wantChunks, Y: 0}
	animator.addDelta(delta, wantChunks*100, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := animator.wait(ctx)
	if err != nil {
		t.Fatalf("wait returned error: %v", err)
	}

	if got := rec.count(); got != wantChunks {
		t.Fatalf(
			"posted %d chunks for a below-floor drain, want %d "+
				"(config.MinSmoothCursorAnimationDuration / minCursorStepDelay)",
			got,
			wantChunks,
		)
	}
}

// TestRelativeCursorAnimatorComposesConcurrentDeltas pins that deltas folded
// in from concurrent movers all reach the compositor — the drain equivalent
// of the absolute animator's pending-endpoint extension.
func TestRelativeCursorAnimatorComposesConcurrentDeltas(t *testing.T) {
	t.Parallel()

	rec := &deltaRecorder{}
	animator := newRelativeCursorAnimator(rec.moveBy)

	const movers = 8

	const deltasPerMover = 25

	var waitGroup sync.WaitGroup
	for range movers {
		waitGroup.Go(func() {
			for range deltasPerMover {
				animator.addDelta(image.Point{X: 1, Y: 0}, 2, 1)
			}
		})
	}

	waitGroup.Wait()

	// Flush whatever is still draining, then everything must have landed.
	animator.settle()

	if want := (image.Point{X: movers * deltasPerMover}); rec.sum() != want {
		t.Fatalf("drained %v after concurrent deltas, want %v (lost deltas)", rec.sum(), want)
	}
}

// TestRelativeCursorAnimatorSettleFlushesRemainder pins settle(): an action
// firing mid-drain must receive the full accumulated motion immediately, as
// one final native flush, and waiters must be released.
func TestRelativeCursorAnimatorSettleFlushesRemainder(t *testing.T) {
	t.Parallel()

	rec := &deltaRecorder{}
	animator := newRelativeCursorAnimator(rec.moveBy)

	// A very slow drain so most of the delta is still pending at settle time.
	delta := image.Point{X: 100, Y: 60}
	animator.addDelta(delta, 10, 10000)

	animator.settle()

	if got := rec.sum(); got != delta {
		t.Fatalf("settle() flushed to %v, want exactly %v", got, delta)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := animator.wait(ctx)
	if err != nil {
		t.Fatalf("wait after settle returned error: %v", err)
	}
}

// TestRelativeCursorAnimatorStopDiscardsRemainder pins stop(): an absolute
// move superseding the drain discards what has not been posted yet, and no
// canceled chunk lands after the stop fence.
func TestRelativeCursorAnimatorStopDiscardsRemainder(t *testing.T) {
	t.Parallel()

	rec := &deltaRecorder{}
	animator := newRelativeCursorAnimator(rec.moveBy)

	total := image.Point{X: 100, Y: 0}
	animator.addDelta(total, 10, 10000)

	animator.stop()

	if got := rec.sum(); got.X >= total.X {
		t.Fatalf("stop() flushed the full delta (%v); the remainder must be discarded", got)
	}

	posted := rec.count()

	time.Sleep(50 * time.Millisecond)

	if rec.count() != posted {
		t.Fatalf("chunks kept landing after stop(): %d -> %d", posted, rec.count())
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := animator.wait(ctx)
	if err != nil {
		t.Fatalf("wait after stop returned error: %v", err)
	}
}

// TestRelativeCursorAnimatorWaitIdle preserves the WaitForCursorIdle no-op
// contract when no drain has ever run.
func TestRelativeCursorAnimatorWaitIdle(t *testing.T) {
	t.Parallel()

	animator := newRelativeCursorAnimator((&deltaRecorder{}).moveBy)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := animator.wait(ctx)
	if err != nil {
		t.Fatalf("wait with no drain returned error: %v", err)
	}
}

// smoothOnProvider serves a config with smooth cursor enabled, for tests that
// exercise the animated MoveCursorBy paths.
type smoothOnProvider struct{}

func (smoothOnProvider) Get() *config.Config {
	cfg := config.DefaultConfig()
	cfg.SmoothCursor.MoveMouseEnabled = true

	return cfg
}

// TestMoveCursorBySmoothKeepsStubErrorLoud pins the CGO-off contract: with
// smooth cursor enabled on a wlroots backend, MoveCursorBy must keep
// surfacing the nocgo stub's CodeNotSupported instead of laundering it
// through the best-effort drain, which drops injection errors by design.
func TestMoveCursorBySmoothKeepsStubErrorLoud(t *testing.T) {
	if nativeBackendsCompiledIn {
		t.Skip("only meaningful on CGO-off builds, where the wlroots injector is the loud stub")
	}

	// No t.Parallel: this mutates the process-global config provider.
	SetConfigProvider(smoothOnProvider{})
	defer SetConfigProvider(nil)

	adapter := NewSystemAdapter(backendWaylandWlroots)

	handled, err := adapter.MoveCursorBy(context.Background(), image.Point{X: 5, Y: 0})
	if !handled {
		t.Fatal("MoveCursorBy on wlroots reported handled == false")
	}

	if !derrors.IsNotSupported(err) {
		t.Errorf("MoveCursorBy returned %v (code %q), want CodeNotSupported",
			err, derrors.GetCode(err))
	}
}

// failingMover errors on every native motion, simulating a lost virtual
// pointer or disconnected compositor client.
func failingMover(image.Point) error {
	return derrors.New(derrors.CodeActionFailed, "virtual pointer lost")
}

// TestRelativeCursorAnimatorInjectionFailureEndsDrainAndFlags pins the
// failure contract: a failed native motion is not counted as posted, the
// drain ends instead of livelocking on a broken backend, waiters are
// released, and the failure stays flagged — sending every subsequent move
// down the loud direct path — until recovery is explicitly proven.
func TestRelativeCursorAnimatorInjectionFailureEndsDrainAndFlags(t *testing.T) {
	t.Parallel()

	animator := newRelativeCursorAnimator(failingMover)

	animator.addDelta(image.Point{X: 40, Y: 0}, 4, 20)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := animator.wait(ctx)
	if err != nil {
		t.Fatalf("wait returned error: %v (drain must end, not livelock, on a broken backend)", err)
	}

	if !animator.injectionFailurePending() {
		t.Fatal("injectionFailurePending() = false after a failed native motion")
	}

	if !animator.injectionFailurePending() {
		t.Fatal("injectionFailurePending() must stay set until recovery is proven")
	}

	animator.clearInjectionFailure()

	if animator.injectionFailurePending() {
		t.Fatal("injectionFailurePending() still set after clearInjectionFailure()")
	}
}

// TestRelativeCursorAnimatorSettleFlushFailureFlags pins that a failed
// settle flush is also flagged, so the loss is surfaced on the next move.
func TestRelativeCursorAnimatorSettleFlushFailureFlags(t *testing.T) {
	t.Parallel()

	animator := newRelativeCursorAnimator(failingMover)

	// A very slow drain so the remainder is still pending at settle time.
	animator.addDelta(image.Point{X: 100, Y: 0}, 10, 10000)

	animator.settle()

	if !animator.injectionFailurePending() {
		t.Fatal("injectionFailurePending() = false after a failed settle flush")
	}
}
