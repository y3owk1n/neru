//go:build linux

package linux

import (
	"context"
	"image"
	"math"
	"sync"
	"time"

	"github.com/y3owk1n/neru/internal/domain/motion"
)

// relativeCursorAnimator animates relative moves on backends whose only
// trustworthy motion is a native delta (wlroots, which has no
// authoritative cursor query, so read-then-warp would compound the client
// cache error). It never reads a position. The remaining delta drains in
// integer chunks that sum exactly to the total.
//
// A delta arriving mid-drain is added to the remainder and the schedule
// restarts, so held-key repeat composes losslessly. Completion is per session,
// as in smoothCursorAnimator.
//
// A move reports success before its motion completes, so a failed injection
// loses at most the rest of that gesture. It is not replayed later, which
// would jump the cursor by stale motion. The failure is flagged, and moves stay
// on the loud direct path until a success proves the backend recovered.
type relativeCursorAnimator struct {
	moveBy func(image.Point) error

	mu        sync.Mutex
	remaining image.Point // un-posted delta of the drain in flight
	stepsLeft int
	stepDelay time.Duration
	running   bool
	stopCh    chan struct{}
	done      *cursorAnimationDone // session completion; nil until the first session

	// injectionFailed is set when a native motion errors, which the drain
	// cannot return because the move already reported success. The caller reads
	// it and routes the next move through the loud direct path.
	injectionFailed bool

	// injectSem serializes injection and fences stop()/settle(), exactly as in
	// smoothCursorAnimator.
	injectSem chan struct{}
}

func newRelativeCursorAnimator(moveBy func(image.Point) error) *relativeCursorAnimator {
	return &relativeCursorAnimator{
		moveBy:    moveBy,
		injectSem: make(chan struct{}, 1),
	}
}

// addDelta folds delta into the drain in flight, starting one when idle, and
// restarts the schedule over durationMs in at most steps chunks. Integer
// chunks sum exactly to the accumulated deltas.
func (a *relativeCursorAnimator) addDelta(delta image.Point, steps, durationMs int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.remaining = a.remaining.Add(delta)
	if a.remaining == (image.Point{}) && !a.running {
		return
	}

	duration := float64(durationMs)
	if duration < motion.MinCursorAnimationDuration {
		duration = motion.MinCursorAnimationDuration
	}

	actualSteps := steps
	if actualSteps <= 0 {
		actualSteps = defaultCursorSteps
	}

	maxSteps := max(int(duration/float64(minCursorStepDelay)), 1)
	if actualSteps > maxSteps {
		actualSteps = maxSteps
	}

	a.stepsLeft = actualSteps
	stepDelayMs := max(int(math.Round(duration/float64(actualSteps))), minCursorStepDelay)
	a.stepDelay = time.Duration(stepDelayMs) * time.Millisecond

	if !a.running {
		a.running = true
		a.done = newCursorAnimationDone()
		a.stopCh = make(chan struct{})

		go a.run(a.stopCh)
	}
}

// run drains the remaining delta one chunk per tick, then ends the session.
// State is re-read under a.mu each tick, so mid-drain deltas join the next one.
func (a *relativeCursorAnimator) run(stopCh chan struct{}) {
	timer := time.NewTimer(time.Hour)

	stopAndDrainTimer(timer)
	defer timer.Stop()

	for {
		chunk, delay, ok := a.nextChunk(stopCh)
		if !ok {
			return
		}

		if chunk != (image.Point{}) && !a.stepChunk(chunk, stopCh) {
			return
		}

		timer.Reset(delay)

		select {
		case <-stopCh:
			stopAndDrainTimer(timer)

			return
		case <-timer.C:
		}
	}
}

// nextChunk carves the next integer chunk off the remainder under a.mu. It
// reports false when this worker was superseded (stopCh identity) or the drain
// is complete, in which case it ends the session. a.done keeps the closed
// completion so a late WaitForCursorIdle sees idle.
func (a *relativeCursorAnimator) nextChunk(
	stopCh chan struct{},
) (image.Point, time.Duration, bool) {
	a.mu.Lock()

	if a.stopCh != stopCh {
		a.mu.Unlock()

		return image.Point{}, 0, false
	}

	if a.remaining == (image.Point{}) {
		done := a.done
		a.running = false
		a.stopCh = nil
		a.mu.Unlock()

		done.close()

		return image.Point{}, 0, false
	}

	chunk := image.Point{
		X: a.remaining.X / a.stepsLeft,
		Y: a.remaining.Y / a.stepsLeft,
	}
	if a.stepsLeft <= 1 {
		chunk = a.remaining
	}

	if a.stepsLeft > 1 {
		a.stepsLeft--
	}

	delay := a.stepDelay
	a.mu.Unlock()

	return chunk, delay, true
}

// stepChunk posts one native relative motion under injectSem, skipping it once
// stopCh is closed. The chunk is deducted only after posting and re-clamped
// against remaining at post time, which keeps posted + remaining equal to the
// accumulated deltas through every cancellation. A chunk canceled before
// injection was never deducted, so settle() flushes it. A chunk a new session
// picked while settle() zeroed remaining clamps to zero and is skipped. Taking
// mu under injectSem is safe because nothing takes injectSem while holding mu.
func (a *relativeCursorAnimator) stepChunk(chunk image.Point, stopCh <-chan struct{}) bool {
	a.injectSem <- struct{}{}
	defer func() { <-a.injectSem }()

	select {
	case <-stopCh:
		return false
	default:
	}

	a.mu.Lock()
	chunk = image.Point{
		X: clampTowardZero(chunk.X, a.remaining.X),
		Y: clampTowardZero(chunk.Y, a.remaining.Y),
	}
	a.mu.Unlock()

	if chunk == (image.Point{}) {
		return true
	}

	err := a.moveBy(chunk)
	if err != nil {
		// A failed chunk is not counted as posted. Retrying a broken backend
		// would livelock the drain, so drop the gesture and flag the failure
		// for the caller's loud direct path.
		a.mu.Lock()
		a.remaining = image.Point{}
		a.injectionFailed = true
		a.mu.Unlock()

		return true
	}

	a.mu.Lock()
	a.remaining = a.remaining.Sub(chunk)
	a.mu.Unlock()

	return true
}

// injectionFailurePending reports whether a native motion failed with no
// recovery since. While true, callers use the loud direct path, so a broken
// backend keeps surfacing errors instead of losing animated moves silently.
func (a *relativeCursorAnimator) injectionFailurePending() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.injectionFailed
}

// clearInjectionFailure re-arms animation. Callers invoke it only after a
// direct native motion succeeded, which proves the backend recovered.
func (a *relativeCursorAnimator) clearInjectionFailure() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.injectionFailed = false
}

// clampTowardZero limits step to the portion of remaining that shares its
// sign, so a posted chunk can never exceed what is still owed on that axis,
// including after opposite-sign deltas shrank or flipped the remainder.
func clampTowardZero(step, remaining int) int {
	if remaining >= 0 {
		return min(max(step, 0), remaining)
	}

	return max(min(step, 0), remaining)
}

// settle cancels the drain and posts the whole remainder as one motion, so an
// action firing mid-drain acts where the user aimed. The remainder is read
// only after acquiring injectSem, so a mid-injection chunk has been deducted
// and a selected but uninjected one is still in remaining. Every delta posts
// exactly once.
func (a *relativeCursorAnimator) settle() {
	done := a.detachWorker()
	done.close()

	a.injectSem <- struct{}{}

	a.mu.Lock()
	remainder := a.remaining
	a.remaining = image.Point{}
	a.mu.Unlock()

	if remainder != (image.Point{}) {
		err := a.moveBy(remainder)
		if err != nil {
			a.mu.Lock()
			a.injectionFailed = true
			a.mu.Unlock()
		}
	}

	<-a.injectSem
}

// stop cancels the drain and discards the remainder, for an absolute move that
// supersedes it, where flushing would fight the warp. Like settle it zeroes the
// remainder only after the injectSem fence, so an in-flight chunk's deduction
// cannot drive it negative.
func (a *relativeCursorAnimator) stop() {
	done := a.detachWorker()
	done.close()

	a.injectSem <- struct{}{}

	a.mu.Lock()
	a.remaining = image.Point{}
	a.mu.Unlock()

	<-a.injectSem
}

// detachWorker cancels the current worker and returns the session completion
// to close, nil (safe to close) when no session ever ran.
func (a *relativeCursorAnimator) detachWorker() *cursorAnimationDone {
	a.mu.Lock()
	defer a.mu.Unlock()

	done := a.done
	stopCh := a.stopCh
	a.stopCh = nil
	a.running = false

	if stopCh != nil {
		close(stopCh)
	}

	return done
}

// wait blocks until the drain in flight settles or ctx is canceled, returning
// immediately when no drain has ever run.
func (a *relativeCursorAnimator) wait(ctx context.Context) error {
	a.mu.Lock()
	done := a.done
	a.mu.Unlock()

	if done == nil {
		return nil
	}

	select {
	case <-done.ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
