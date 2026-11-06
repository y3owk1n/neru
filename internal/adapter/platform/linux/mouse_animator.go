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

// Every animation is floored at config.MinSmoothCursorAnimationDuration, the
// constant ValidateSmoothCursor checks against, so validator and animator
// cannot disagree. darwin reads it the same way.
const (
	// minCursorStepDelay is the shortest gap between injected steps, a
	// scheduling floor on this animator's timer and not a config value.
	minCursorStepDelay = 1 // ms
	// defaultCursorSteps is the step count for a caller that passes none
	// (<= 0). It equals config.DefaultSmoothCursorSteps by coincidence. No
	// validated config reaches it, and moving the config default must not
	// move it.
	defaultCursorSteps = 10
)

// cursorAnimationDone is a once-closable completion signal WaitForCursorIdle
// blocks on, mirroring the darwin animator's done channel.
type cursorAnimationDone struct {
	ch   chan struct{}
	once sync.Once
}

func newCursorAnimationDone() *cursorAnimationDone {
	return &cursorAnimationDone{ch: make(chan struct{})}
}

func (d *cursorAnimationDone) close() {
	if d == nil {
		return
	}

	d.once.Do(func() {
		close(d.ch)
	})
}

// cursorRequest is one "animate to end" target. Completion is tracked per
// session (see smoothCursorAnimator), not per target, so coalescing a
// superseded target never releases a waiter early.
type cursorRequest struct {
	end              image.Point
	steps            int
	maxDuration      int
	durationPerPixel float64
	fixedDuration    int // when > 0, overrides the distance-derived duration
}

// smoothCursorAnimator glides the cursor toward a target like the darwin
// animator, with one worker goroutine, latest target wins, and a completion
// WaitForCursorIdle blocks on. A direct move cancels an in-flight animation,
// and stop() fences on injectSem so a canceled step never lands after the
// warp. Step errors are dropped, as darwin's native move cannot report them,
// while MoveCursorToPoint still returns them.
//
// Targets arriving mid-session share one completion, so a waiter tracks the
// latest target. pos samples once per request, so a stale Wayland cache bends
// the glide path but never moves where the cursor lands.
type smoothCursorAnimator struct {
	pos  func() image.Point
	move func(image.Point) error

	mu     sync.Mutex
	reqCh  chan cursorRequest
	stopCh chan struct{}
	done   *cursorAnimationDone // current session completion; nil only between stop() and the next session
	busy   bool                 // a session is in progress (worker actively draining toward a target)

	// pendingEnd is the in-flight target. Relative moves extend it rather than
	// the possibly stale cursor position, so key repeat loses no delta.
	pendingEnd image.Point
	hasPending bool

	// injectSem is a size-1 semaphore serializing backend injection, and the
	// ordering handoff to stop(). The worker holds it per step and re-checks
	// stopCh under it, so once stop() holds it no canceled step can run. It is
	// never held together with mu.
	injectSem chan struct{}
}

func newSmoothCursorAnimator(
	pos func() image.Point,
	move func(image.Point) error,
) *smoothCursorAnimator {
	return &smoothCursorAnimator{
		pos:       pos,
		move:      move,
		injectSem: make(chan struct{}, 1),
	}
}

// stop cancels any in-flight animation and releases waiters, so a direct warp
// wins over a running tween. Closing stopCh detaches the stale worker.
func (a *smoothCursorAnimator) stop() {
	a.mu.Lock()
	stopCh := a.stopCh
	done := a.done
	a.reqCh = nil
	a.stopCh = nil
	a.done = nil
	a.busy = false
	a.hasPending = false

	if stopCh != nil {
		close(stopCh)
	}
	a.mu.Unlock()

	done.close()

	// stopCh is closed, so once we hold injectSem any mid-injection step has
	// finished and any later one skips in stepMove. The caller's direct move
	// therefore lands last.
	a.injectSem <- struct{}{}

	<-a.injectSem
}

// pendingTarget returns the endpoint of the animation currently in flight,
// or ok == false when no animation is active.
func (a *smoothCursorAnimator) pendingTarget() (image.Point, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.pendingEnd, a.hasPending
}

// settle cancels any in-flight animation and warps straight to its endpoint,
// so a position-dependent action acts where the user aimed without waiting.
// The warp happens under injectSem, as in stop(), so no canceled step lands
// after it.
func (a *smoothCursorAnimator) settle() {
	a.mu.Lock()

	if !a.hasPending {
		a.mu.Unlock()

		return
	}

	end := a.pendingEnd
	stopCh := a.stopCh
	done := a.done
	a.reqCh = nil
	a.stopCh = nil
	a.done = nil
	a.busy = false
	a.hasPending = false

	if stopCh != nil {
		close(stopCh)
	}
	a.mu.Unlock()

	done.close()

	a.injectSem <- struct{}{}

	_ = a.move(end)

	<-a.injectSem
}

// stepMove injects one step under injectSem, and skips it, returning false,
// once stopCh is closed. With stop()'s barrier, a canceled tween never warps
// the cursor after a direct move.
func (a *smoothCursorAnimator) stepMove(point image.Point, stopCh <-chan struct{}) bool {
	a.injectSem <- struct{}{}
	defer func() { <-a.injectSem }()

	select {
	case <-stopCh:
		return false
	default:
	}

	// Best-effort, see smoothCursorAnimator.
	_ = a.move(point)

	return true
}

// wait blocks until the current session settles or ctx is canceled, and
// returns at once when no session is active (the non-smooth path).
func (a *smoothCursorAnimator) wait(ctx context.Context) error {
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

// animateTo enqueues an animation toward end. A target arriving mid-session
// joins the running session so its waiter tracks the latest target.
//
// Worker start and enqueue share one a.mu critical section, so a concurrent
// stop() cannot swap the channel between picking it and sending, and no
// request is stranded on an orphaned channel.
func (a *smoothCursorAnimator) animateTo(
	end image.Point,
	steps, maxDuration int,
	durationPerPixel float64,
) {
	req := cursorRequest{
		end:              end,
		steps:            steps,
		maxDuration:      maxDuration,
		durationPerPixel: durationPerPixel,
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.submitLocked(req)
}

// animateRelativeBy animates a relative move over a fixed duration, so speed
// scales with the delta instead of collapsing to constant velocity. The base is
// the pending endpoint (else the sampled position) plus delta, clamped. It is
// computed and submitted under one lock hold, so concurrent relative movers
// compose their deltas instead of overwriting each other.
func (a *smoothCursorAnimator) animateRelativeBy(
	delta image.Point,
	clamp func(image.Point) image.Point,
	steps int,
	durationMs int,
) {
	a.mu.Lock()
	defer a.mu.Unlock()

	base := a.pendingEnd
	if !a.hasPending {
		base = a.pos()
	}

	end := clamp(base.Add(delta))
	if end == base {
		// The delta was zero or clamped away at an edge. Submitting would only
		// cancel and restart an animation that already targets base.
		return
	}

	a.submitLocked(cursorRequest{
		end:           end,
		steps:         steps,
		fixedDuration: durationMs,
	})
}

// submitLocked records req as the pending animation target and hands it to
// the worker, starting the worker and a fresh session completion when needed.
// Callers must hold a.mu.
func (a *smoothCursorAnimator) submitLocked(req cursorRequest) {
	if a.reqCh == nil {
		a.reqCh = make(chan cursorRequest, 1)
		a.stopCh = make(chan struct{})

		go a.run(a.reqCh, a.stopCh)
	}

	if !a.busy {
		a.done = newCursorAnimationDone()
		a.busy = true
	}

	a.pendingEnd = req.end
	a.hasPending = true

	a.enqueueLocked(req)
}

// enqueueLocked places req on the worker channel so only the latest target
// survives. The caller must hold a.mu. Under the lock we are the only producer
// and the worker only removes, so after draining a stale target the send
// cannot block. Targets carry no completion, so dropping one is a plain discard.
func (a *smoothCursorAnimator) enqueueLocked(req cursorRequest) {
	select {
	case a.reqCh <- req:
		return
	default:
	}

	select {
	case <-a.reqCh:
	default:
	}

	select {
	case a.reqCh <- req:
	default:
	}
}

func (a *smoothCursorAnimator) run(reqCh <-chan cursorRequest, stopCh <-chan struct{}) {
	timer := time.NewTimer(time.Hour)

	stopAndDrainTimer(timer)
	defer timer.Stop()

	for {
		select {
		case <-stopCh:
			return
		case req := <-reqCh:
			a.runRequest(req, reqCh, stopCh, timer)
		}
	}
}

func (a *smoothCursorAnimator) runRequest(
	req cursorRequest,
	reqCh <-chan cursorRequest,
	stopCh <-chan struct{},
	timer *time.Timer,
) {
restart:
	start := a.pos()

	distance := math.Hypot(float64(req.end.X-start.X), float64(req.end.Y-start.Y))

	duration := math.Min(float64(req.maxDuration), distance*req.durationPerPixel)
	if req.fixedDuration > 0 {
		duration = float64(req.fixedDuration)
	}

	if duration < config.MinSmoothCursorAnimationDuration {
		duration = config.MinSmoothCursorAnimationDuration
	}

	actualSteps := req.steps
	if actualSteps <= 0 {
		actualSteps = defaultCursorSteps
	}

	maxSteps := max(int(duration/float64(minCursorStepDelay)), 1)
	if actualSteps > maxSteps {
		actualSteps = maxSteps
	}

	stepDelayMs := max(int(math.Round(duration/float64(actualSteps))), minCursorStepDelay)
	stepDelay := time.Duration(stepDelayMs) * time.Millisecond

	for step := 1; step <= actualSteps; step++ {
		select {
		case <-stopCh:
			return
		case nextReq := <-reqCh:
			req = nextReq

			goto restart
		default:
		}

		progress := float64(step) / float64(actualSteps)
		intermediate := image.Point{
			X: int(float64(start.X) + float64(req.end.X-start.X)*progress),
			Y: int(float64(start.Y) + float64(req.end.Y-start.Y)*progress),
		}

		if !a.stepMove(intermediate, stopCh) {
			return
		}

		if step < actualSteps {
			timer.Reset(stepDelay)

			select {
			case <-stopCh:
				stopAndDrainTimer(timer)

				return
			case nextReq := <-reqCh:
				stopAndDrainTimer(timer)

				req = nextReq

				goto restart
			case <-timer.C:
			}
		}
	}

	// Target reached. Continue the same session if a newer target is already
	// queued, otherwise the session ends.
	next, ok := a.finishOrNext(reqCh, stopCh)
	if ok {
		req = next

		goto restart
	}
}

// finishOrNext, after a target is reached, returns the next queued target or
// ends the session by closing its completion. a.done keeps the closed one so a
// late WaitForCursorIdle sees idle. A worker superseded by stop(), detected by
// stopCh identity, does nothing, so it never closes a newer session's
// completion.
func (a *smoothCursorAnimator) finishOrNext(
	reqCh <-chan cursorRequest,
	stopCh <-chan struct{},
) (cursorRequest, bool) {
	a.mu.Lock()

	if a.stopCh != stopCh {
		a.mu.Unlock()

		return cursorRequest{}, false
	}

	select {
	case next := <-reqCh:
		a.mu.Unlock()

		return next, true
	default:
	}

	done := a.done
	a.busy = false
	a.hasPending = false
	a.mu.Unlock()

	done.close()

	return cursorRequest{}, false
}

// stopAndDrainTimer stops timer and clears any pending tick so the next Reset
// starts clean.
func stopAndDrainTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}
