//go:build linux

package linux

import (
	"context"
	"image"
	"math"
	"sync"
	"time"

	"github.com/y3owk1n/neru/internal/config"
)

// relativeCursorAnimator animates relative cursor moves on backends whose
// only trustworthy motion primitive is a native delta (Wayland wlroots, where
// there is no authoritative cursor query and read-then-warp would compound
// the client cache error). It never reads a position: the remaining delta is
// drained in integer chunks that sum exactly to the requested total, each
// posted as one native relative motion.
//
// A delta arriving while a drain is in flight is added to the remaining
// total and the drain schedule restarts, so held-key repeat composes
// losslessly — the wlroots twin of the absolute animator's pending-endpoint
// extension. Completion is per session, mirroring smoothCursorAnimator, so
// WaitForCursorIdle tracks the latest delta rather than an intermediate one.
//
// Failure bound: animation reports success before motion completes (inherent
// to any animated move; the absolute animator behaves the same), so when a
// native injection fails mid-drain, at most the remainder of that one
// gesture is lost — deliberately not stored and replayed later, which would
// jump the cursor by stale motion after recovery. The failure is flagged on
// the first failed chunk and every subsequent move stays on the loud direct
// path until a success proves the backend recovered.
type relativeCursorAnimator struct {
	moveBy func(image.Point) error

	mu        sync.Mutex
	remaining image.Point // un-posted delta of the drain in flight
	stepsLeft int
	stepDelay time.Duration
	running   bool
	stopCh    chan struct{}
	done      *cursorAnimationDone // session completion; nil until the first session

	// injectionFailed is set when a native motion errors. The drain cannot
	// return that error to anyone — the move call already reported handled —
	// so the caller consumes this flag (takeInjectionFailure) and routes the
	// next move through the direct native path, which fails loudly.
	injectionFailed bool

	// injectSem serializes backend injection and is the ordering handoff to
	// stop()/settle(), exactly as in smoothCursorAnimator: the worker holds it
	// across each chunk and re-checks stopCh under it, so once a canceler
	// acquires the token no canceled chunk is mid-flight and none will start.
	injectSem chan struct{}
}

func newRelativeCursorAnimator(moveBy func(image.Point) error) *relativeCursorAnimator {
	return &relativeCursorAnimator{
		moveBy:    moveBy,
		injectSem: make(chan struct{}, 1),
	}
}

// addDelta folds delta into the drain in flight (starting one when idle) and
// restarts the schedule: the combined remainder drains over durationMs in at
// most steps chunks. Chunks are integer subdivisions, so the posted motions
// always sum exactly to the accumulated deltas — nothing is lost to rounding.
func (a *relativeCursorAnimator) addDelta(delta image.Point, steps, durationMs int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.remaining = a.remaining.Add(delta)
	if a.remaining == (image.Point{}) && !a.running {
		return
	}

	duration := float64(durationMs)
	if duration < config.MinSmoothCursorAnimationDuration {
		duration = config.MinSmoothCursorAnimationDuration
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

// run drains the remaining delta one chunk per tick until it reaches zero,
// then ends the session. All shared state is re-read under a.mu each tick, so
// deltas folded in mid-drain are picked up on the next chunk.
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

// nextChunk carves the next integer chunk off the remaining delta under a.mu.
// It reports ok == false when this worker has been superseded (stopCh
// identity, as in smoothCursorAnimator.finishOrNext) or when the drain is
// complete, in which case it ends the session — keeping a.done referencing
// the closed completion so a late WaitForCursorIdle still sees "idle".
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

// stepChunk posts one native relative motion while holding injectSem,
// re-checking stopCh so a chunk cannot inject once the session is canceled.
//
// The chunk is deducted from the remaining delta only after it is posted, and
// re-clamped against remaining at post time. Together these keep the drain's
// core invariant — posted + remaining == accumulated deltas — through every
// cancellation interleaving: a chunk canceled between selection and injection
// was never deducted, so settle() flushes it; a chunk selected by a new
// session while settle() was zeroing remaining clamps to zero and is skipped
// rather than double-posted. Taking mu while holding injectSem is safe
// because no caller acquires injectSem while holding mu. Best-effort like the
// absolute animator's stepMove: a failed backend motion still counts as
// posted.
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
		// A failed chunk is NOT counted as posted. The backend is broken
		// (client disconnected, virtual pointer lost), so retrying would
		// livelock the drain: end it instead, discard the rest of the gesture,
		// and flag the failure so the caller's next move takes the direct
		// native path — which surfaces the backend error loudly.
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

// injectionFailurePending reports whether a native motion has failed and no
// recovery has been proven since. While it holds, callers route every
// relative move through the direct (loud) native path, so a broken backend
// keeps surfacing errors instead of alternating between loud probes and
// silent animated losses.
func (a *relativeCursorAnimator) injectionFailurePending() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.injectionFailed
}

// clearInjectionFailure re-arms animation. Callers invoke it only after a
// direct native motion succeeded — the proof the backend recovered.
func (a *relativeCursorAnimator) clearInjectionFailure() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.injectionFailed = false
}

// clampTowardZero limits step to the portion of remaining that shares its
// sign, so a posted chunk can never exceed what is still owed on that axis —
// including after opposite-sign deltas shrank or flipped the remainder.
func clampTowardZero(step, remaining int) int {
	if remaining >= 0 {
		return min(max(step, 0), remaining)
	}

	return max(min(step, 0), remaining)
}

// settle finishes the drain in flight immediately: the worker is canceled and
// the entire remaining delta is posted as one native relative motion, so an
// action firing mid-drain acts where the user aimed.
//
// The remainder is read only after acquiring injectSem: a chunk mid-injection
// finishes — including its deduction — before the read, and a chunk that had
// been selected but not yet injected sees the closed stopCh, stays in
// remaining, and is flushed here. Either way every accumulated delta is
// posted exactly once.
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

// stop cancels the drain in flight and discards the remaining delta. It is
// called on paths where an absolute move supersedes the pending deltas — the
// cursor is about to be placed somewhere explicit, so flushing would fight
// the warp. Like settle it zeroes the remainder only after the injectSem
// fence, so an in-flight chunk's deduction cannot drive remaining negative.
func (a *relativeCursorAnimator) stop() {
	done := a.detachWorker()
	done.close()

	a.injectSem <- struct{}{}

	a.mu.Lock()
	a.remaining = image.Point{}
	a.mu.Unlock()

	<-a.injectSem
}

// detachWorker cancels the current worker, returning the session completion
// to close. The returned done is nil (a safe no-op to close) when no session
// ever ran.
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
