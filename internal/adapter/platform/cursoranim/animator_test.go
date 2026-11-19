package cursoranim

import (
	"context"
	"image"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/domain/motion"
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

func TestAnimatorLandsOnTarget(t *testing.T) {
	t.Parallel()

	rec := &recorder{start: image.Point{X: 0, Y: 0}}
	animator := New(rec.pos, rec.move)

	target := image.Point{X: 300, Y: 200}
	animator.AnimateTo(target, 8, 60, 0.1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := animator.Wait(ctx)
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

// TestAnimatorFloorsShortAnimationsAtConfigMinimum pins the floor
// this animator applies to motion.MinCursorAnimationDuration, the same
// constant ValidateSmoothCursor rejects a shorter relative_movement_duration
// against, so the value the validator promises to honor is the value the
// animator honors.
//
// It reads the floor back out of behavior rather than comparing constants. A
// below-floor duration asked for in far more steps than the floor can schedule
// collapses to one step per MinStepDelay of the floor, so the number of
// injected moves *is* the floor. Move the config constant and this expectation
// moves with it. Re-introduce a local copy that disagrees and this fails.
func TestAnimatorFloorsShortAnimationsAtConfigMinimum(t *testing.T) {
	t.Parallel()

	wantSteps := motion.MinCursorAnimationDuration / MinStepDelay

	rec := &recorder{}
	animator := New(rec.pos, rec.move)

	animator.AnimateRelativeBy(
		image.Point{X: 4 * wantSteps, Y: 0},
		func(point image.Point) image.Point { return point },
		wantSteps*100,
		1,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := animator.Wait(ctx)
	if err != nil {
		t.Fatalf("wait returned error: %v", err)
	}

	if got := rec.count(); got != wantSteps {
		t.Fatalf(
			"injected %d steps for a below-floor animation, want %d "+
				"(motion.MinCursorAnimationDuration / MinStepDelay)",
			got,
			wantSteps,
		)
	}
}

func TestAnimatorCoalescesToLatestTarget(t *testing.T) {
	t.Parallel()

	rec := &recorder{start: image.Point{X: 0, Y: 0}}
	animator := New(rec.pos, rec.move)

	// Fire several requests back-to-back. Only the last target must win.
	animator.AnimateTo(image.Point{X: 100, Y: 100}, 10, 80, 0.5)
	animator.AnimateTo(image.Point{X: 500, Y: 50}, 10, 80, 0.5)
	final := image.Point{X: 640, Y: 480}
	animator.AnimateTo(final, 10, 80, 0.5)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := animator.Wait(ctx)
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

func TestAnimatorWaitReturnsWhenIdle(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	animator := New(rec.pos, rec.move)

	// No animation started, so wait must return immediately, preserving the
	// historical no-op WaitForCursorIdle behavior on the direct move path.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := animator.Wait(ctx)
	if err != nil {
		t.Fatalf("wait on idle animator returned error: %v", err)
	}

	if rec.count() != 0 {
		t.Fatalf("idle animator injected %d moves, want 0", rec.count())
	}
}

// TestAnimatorWaiterTracksSupersedingTarget guards the coalescing
// race. A waiter that attaches while an earlier target animates must not be
// released until the cursor reaches the *latest* target, even when the queued
// target is superseded mid-flight. A per-request completion would release
// this waiter early at the intermediate or previous position.
func TestAnimatorWaiterTracksSupersedingTarget(t *testing.T) {
	t.Parallel()

	// Slow the moves so the animation spans the follow-up enqueues.
	rec := &recorder{start: image.Point{X: 0, Y: 0}}
	slowMove := func(p image.Point) error {
		time.Sleep(2 * time.Millisecond)

		return rec.move(p)
	}

	animator := New(rec.pos, slowMove)

	final := image.Point{X: 900, Y: 900}

	// Start a long first animation, then attach a waiter mid-flight.
	animator.AnimateTo(image.Point{X: 120, Y: 120}, 20, 400, 1.0)

	waitResult := make(chan error, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		waitResult <- animator.Wait(ctx)
	}()

	// Supersede the target twice while the session is still animating.
	time.Sleep(6 * time.Millisecond)
	animator.AnimateTo(image.Point{X: 500, Y: 500}, 20, 400, 1.0)
	time.Sleep(6 * time.Millisecond)
	animator.AnimateTo(final, 20, 400, 1.0)

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

// TestAnimatorBestEffortOnMoveFailure documents that a failing
// backend warp during the animation is best-effort (matching darwin). The
// session still settles and WaitForCursorIdle returns without error.
func TestAnimatorBestEffortOnMoveFailure(t *testing.T) {
	t.Parallel()

	failingMove := func(_ image.Point) error {
		return context.DeadlineExceeded // any non-nil error
	}
	pos := func() image.Point {
		return image.Point{}
	}

	animator := New(pos, failingMove)
	animator.AnimateTo(image.Point{X: 400, Y: 400}, 8, 60, 0.1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := animator.Wait(ctx)
	if err != nil {
		t.Fatalf("smooth-move failure must not surface through wait: got %v", err)
	}
}

// TestAnimatorStopFencesInflightStep verifies Stop does not
// return while a backend injection step is in flight, and blocks any further
// step from injecting. This is the ordering guarantee that lets a bypassed
// direct warp (issued right after stop) land last instead of racing a stale
// animation step.
func TestAnimatorStopFencesInflightStep(t *testing.T) {
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

	animator := New(pos, blockingMove)
	animator.AnimateTo(image.Point{X: 100, Y: 100}, 10, 200, 1.0)

	<-entered // a backend step is now in flight, blocked in blockingMove

	stopReturned := make(chan struct{})

	go func() {
		animator.Stop()
		close(stopReturned)
	}()

	// Stop must not return while the step is mid-injection.
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

// TestAnimatorConcurrentStopAndAnimate guards the animateTo/stop
// race. A bypassed move (stop) must never strand a smooth request on an
// orphaned channel with an unclosed done. Run with -race. The test passes if it
// completes without deadlocking on a wait that never returns.
func TestAnimatorConcurrentStopAndAnimate(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	animator := New(rec.pos, rec.move)

	var waitGroup sync.WaitGroup

	for index := range 200 {
		waitGroup.Add(2)

		go func() {
			defer waitGroup.Done()

			animator.AnimateTo(image.Point{X: index, Y: index}, 6, 40, 0.2)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := animator.Wait(ctx)
			if err != nil {
				t.Errorf("wait blocked or errored during concurrent stop: %v", err)
			}
		}()

		go func() {
			defer waitGroup.Done()

			animator.Stop()
		}()
	}

	waitGroup.Wait()
}

// TestAnimatorStopReleasesWaiter pins the cancellation window. A waiter
// already blocked on the running session is released by Stop rather than left
// hanging, and once Stop returns no further step from the canceled animation
// lands.
func TestAnimatorStopReleasesWaiter(t *testing.T) {
	t.Parallel()

	rec := &recorder{start: image.Point{X: 0, Y: 0}}
	animator := New(rec.pos, rec.move)

	// Long animation so the waiter attaches, and Stop lands, mid-flight.
	animator.AnimateTo(image.Point{X: 5000, Y: 5000}, 200, 5000, 5.0)

	waitResult := make(chan error, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		waitResult <- animator.Wait(ctx)
	}()

	// The session is still running, so the waiter must still be blocked.
	// Otherwise Stop releasing it below would prove nothing.
	select {
	case err := <-waitResult:
		t.Fatalf("wait returned %v while the animation was still in flight", err)
	case <-time.After(20 * time.Millisecond):
	}

	animator.Stop()

	select {
	case err := <-waitResult:
		if err != nil {
			t.Fatalf("wait after stop returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not release the blocked waiter")
	}

	settled := rec.count()

	time.Sleep(50 * time.Millisecond)

	if got := rec.count(); got != settled {
		t.Fatalf("%d step(s) landed after Stop returned, want none", got-settled)
	}
}

// identityClamp passes points through unchanged, for tests that exercise
// relative animation away from screen edges.
func identityClamp(p image.Point) image.Point { return p }

// TestAnimatorRelativeLandsExactly pins that a relative move from
// idle animates as a stepped glide and lands exactly on start+delta.
func TestAnimatorRelativeLandsExactly(t *testing.T) {
	t.Parallel()

	start := image.Point{X: 7, Y: 9}
	rec := &recorder{start: start}
	animator := New(rec.pos, rec.move)

	delta := image.Point{X: 35, Y: -20}
	animator.AnimateRelativeBy(delta, identityClamp, 5, 30)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := animator.Wait(ctx)
	if err != nil {
		t.Fatalf("wait returned error: %v", err)
	}

	last, ok := rec.last()
	if !ok {
		t.Fatal("expected at least one move to be injected")
	}

	if want := start.Add(delta); last != want {
		t.Fatalf("relative move landed on %v, want %v", last, want)
	}

	if rec.count() < 2 {
		t.Fatalf("expected a stepped glide (>=2 moves), got %d", rec.count())
	}
}

// TestAnimatorRelativeExtendsPendingEndpoint pins the held-repeat
// contract. A delta arriving mid-animation extends the pending endpoint
// instead of restarting from the mid-glide position, and Settle warps
// straight to the extended endpoint. Once Stop cancels a glide, the next
// delta starts from the cursor again, not from the abandoned endpoint.
func TestAnimatorRelativeExtendsPendingEndpoint(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	animator := New(rec.pos, rec.move)

	// Long fixed durations so both deltas land in one pending session.
	animator.AnimateRelativeBy(image.Point{X: 10, Y: 0}, identityClamp, 10, 5000)
	animator.AnimateRelativeBy(image.Point{X: 15, Y: 5}, identityClamp, 10, 5000)

	want := image.Point{X: 25, Y: 5}

	if got, pending := animator.pendingTarget(); !pending || got != want {
		t.Fatalf("pendingTarget() = %v, %v after two deltas, want %v, true", got, pending, want)
	}

	animator.Settle()

	if last, moved := rec.last(); !moved || last != want {
		t.Fatalf("Settle warped to %v, %v, want %v, true", last, moved, want)
	}

	if _, pending := animator.pendingTarget(); pending {
		t.Fatal("pendingTarget() still reports an animation after Settle")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := animator.Wait(ctx)
	if err != nil {
		t.Fatalf("wait after settle returned error: %v", err)
	}

	animator.AnimateRelativeBy(image.Point{X: 100, Y: 0}, identityClamp, 10, 5000)
	animator.Stop()

	base := rec.pos()

	animator.AnimateRelativeBy(image.Point{X: 1, Y: 0}, identityClamp, 10, 5000)

	defer animator.Stop()

	if got, _ := animator.pendingTarget(); got != base.Add(image.Point{X: 1, Y: 0}) {
		t.Fatalf(
			"after Stop, a delta of 1 extended to %v, want it based on the cursor at %v",
			got,
			base,
		)
	}
}

// TestAnimatorRelativeNoopKeepsAnimation pins the screen-edge
// guard. A delta the clamp fully absorbs must not cancel and restart the
// animation already heading to the pending endpoint. A restart is visible as
// the worker sampling the cursor again for its new start.
func TestAnimatorRelativeNoopKeepsAnimation(t *testing.T) {
	t.Parallel()

	rec := &recorder{}

	var samples atomic.Int32

	pos := func() image.Point {
		samples.Add(1)

		return rec.pos()
	}
	animator := New(pos, rec.move)

	defer animator.Stop()

	end := image.Point{X: 50, Y: 0}
	animator.AnimateRelativeBy(end, identityClamp, 10, 5000)

	// One sample is the relative base, the second is the worker's start.
	deadline := time.Now().Add(2 * time.Second)
	for samples.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	if got := samples.Load(); got != 2 {
		t.Fatalf("cursor sampled %d times before the no-op, want 2", got)
	}

	// Clamp everything back to the pending endpoint, as a screen edge would.
	animator.AnimateRelativeBy(image.Point{X: 100, Y: 0}, func(image.Point) image.Point {
		return end
	}, 10, 5000)

	time.Sleep(50 * time.Millisecond)

	if got := samples.Load(); got != 2 {
		t.Fatalf("clamped no-op restarted the animation (%d cursor samples, want 2)", got)
	}

	got, ok := animator.pendingTarget()
	if !ok || got != end {
		t.Fatalf("pendingTarget() = %v, %v after clamped no-op, want %v, true", got, ok, end)
	}
}

// TestAnimatorRelativeConcurrentDeltasCompose pins the
// single-lock read-modify-write of animateRelativeBy. Deltas from concurrent
// movers must all end up in the final position, none silently overwritten.
// The recorder's pos() returns the last landed point, so composition must
// stay exact even across session boundaries.
func TestAnimatorRelativeConcurrentDeltasCompose(t *testing.T) {
	t.Parallel()

	rec := &recorder{}
	animator := New(rec.pos, rec.move)

	const movers = 8

	const deltasPerMover = 25

	var waitGroup sync.WaitGroup
	for range movers {
		waitGroup.Go(func() {
			for range deltasPerMover {
				animator.AnimateRelativeBy(image.Point{X: 1, Y: 0}, identityClamp, 2, 1)
			}
		})
	}

	waitGroup.Wait()

	animator.Settle()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := animator.Wait(ctx)
	if err != nil {
		t.Fatalf("wait returned error: %v", err)
	}

	last, ok := rec.last()
	if !ok {
		t.Fatal("expected moves to be injected")
	}

	if want := movers * deltasPerMover; last.X != want {
		t.Fatalf("final X = %d after %d concurrent unit deltas, want %d (lost deltas)",
			last.X, want, want)
	}
}
