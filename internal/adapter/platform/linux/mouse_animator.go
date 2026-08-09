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

// The animation floor itself is not declared here: every smooth-cursor
// animation is floored at config.MinSmoothCursorAnimationDuration, the same
// constant ValidateSmoothCursor rejects a shorter relative_movement_duration
// against, so the validator and the animator cannot disagree about what the
// daemon will honor. darwin's animator reads it the same way.
const (
	// minCursorStepDelay is the shortest gap between injected steps — a
	// scheduling floor on this animator's own timer, not a config value. No
	// config option declares it and none derives from it (darwin likewise
	// keeps its own minStepDelay local), so there is nothing here for a
	// config change to desync from and it stays local.
	minCursorStepDelay = 1 // ms
	// defaultCursorSteps is the step count used when a caller passes none
	// (<= 0). It equals config.DefaultSmoothCursorSteps today, and that is a
	// coincidence rather than a derivation: both smooth-cursor call sites
	// pass cfg.SmoothCursor.Steps, which ValidateSmoothCursor already
	// requires to be >= 1, so no validated config can reach this fallback at
	// all. It answers a different question — what step count to use for a
	// caller that supplied a nonsense one — and moving the config default
	// must not move it.
	defaultCursorSteps = 10
)

// cursorAnimationDone is a once-closable completion signal shared with
// WaitForCursorIdle. It mirrors the darwin animator's done channel so callers
// can block until the cursor settles.
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

// cursorRequest is a single "animate to end" target. It carries no completion
// object of its own: completion is tracked per animation *session* (see
// smoothCursorAnimator), so coalescing a superseded target never releases a
// waiter early.
type cursorRequest struct {
	end              image.Point
	steps            int
	maxDuration      int
	durationPerPixel float64
	fixedDuration    int // when > 0, overrides the distance-derived duration
}

// smoothCursorAnimator glides the cursor toward a target, matching the darwin
// animator: one worker goroutine, latest-target-wins, a completion channel
// WaitForCursorIdle blocks on. Callers do not serialize cursor access, so a
// direct move cancels an in-flight animation; stop() fences on injectSem so a
// canceled tween can never land after the warp. Backend errors mid-animation
// are dropped (darwin's native move cannot fail back to Go either); the direct
// MoveCursorToPoint path still returns them.
//
// Completion is per session — every target arriving mid-session shares one
// completion, so a waiter tracks the latest target rather than being released
// when an intermediate one is superseded. pos and move are injected to keep
// X11/Wayland detail out; pos samples once per request, so a stale Wayland
// cache affects the glide path but never where the cursor lands.
type smoothCursorAnimator struct {
	pos  func() image.Point
	move func(image.Point) error

	mu     sync.Mutex
	reqCh  chan cursorRequest
	stopCh chan struct{}
	done   *cursorAnimationDone // current session completion; nil only between stop() and the next session
	busy   bool                 // a session is in progress (worker actively draining toward a target)

	// pendingEnd is the target of the animation currently in flight. Relative
	// moves extend it instead of restarting from the (possibly stale) cursor
	// position, so no part of a delta is lost under key repeat.
	pendingEnd image.Point
	hasPending bool

	// injectSem is a size-1 semaphore serializing actual backend injection (one
	// step at a time) and is the ordering handoff to stop(): the worker holds it
	// across each step and re-checks stopCh under it, so once stop() acquires it
	// no canceled step is mid-flight and none will start. It is never held
	// together with mu.
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

// stop cancels any in-flight animation and releases waiters. It is called on
// the non-smooth move path so a direct warp always wins over a running tween.
// It detaches the current worker via stopCh so a stale worker cannot mutate a
// later session.
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

	// Order a bypassed direct warp after any in-flight animation step. stopCh is
	// already closed above, so once we hold injectSem: any step that was
	// mid-injection has finished (it releases the token on the way out), and any
	// step that has not yet injected will see the closed stopCh (in stepMove) and
	// skip. The caller's subsequent moveCursorDirect therefore lands last.
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

// settle finishes any in-flight animation immediately: the worker is canceled
// and the cursor warps straight to the endpoint it was animating toward.
// Position-dependent actions call this so an action firing mid-animation acts
// at the point the user aimed for, without waiting the animation out. The
// warp happens under injectSem — the same fence stop() uses — so a canceled
// step can never land after it.
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

// stepMove injects one animation step while holding injectSem, re-checking
// stopCh so a step cannot inject once the session is stopped. This, paired with
// stop()'s injectSem barrier, guarantees a canceled tween never warps the cursor
// after a bypassed direct move. It returns false when the step was skipped due
// to stop.
func (a *smoothCursorAnimator) stepMove(point image.Point, stopCh <-chan struct{}) bool {
	a.injectSem <- struct{}{}
	defer func() { <-a.injectSem }()

	select {
	case <-stopCh:
		return false
	default:
	}

	// Best-effort, matching darwin (whose native move cannot fail back to Go):
	// a failed backend warp during the opt-in animation is not surfaced through
	// WaitForCursorIdle. The direct MoveCursorToPoint path still returns backend
	// errors synchronously.
	_ = a.move(point)

	return true
}

// wait blocks until the current session settles or ctx is canceled. It returns
// immediately when no session is active, preserving the historical
// "WaitForCursorIdle is a no-op on Linux" behavior for the non-smooth path.
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

// animateTo enqueues an animation toward end. A fresh session completion is
// created only when the animator is idle; a target arriving mid-session joins
// the running session so its waiter tracks the latest target.
//
// Worker start and enqueue happen under a.mu in one critical section. This is
// what makes it safe against a concurrent stop(): a bypassed move can never
// swap out the channel or kill the worker between "pick a channel" and "send",
// so a request can never be stranded on an orphaned channel. Because every
// producer holds the lock and the worker is the sole consumer of the size-1
// buffer, the enqueue is always non-blocking.
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

// animateRelativeBy animates a relative move with a fixed duration,
// independent of the distance, so that cursor speed scales with the per-move
// delta instead of collapsing to the constant velocity the
// distance-proportional duration would produce.
//
// The base is the pending endpoint of the animation in flight (falling back
// to the sampled cursor position), extended by delta and clamped by the
// caller's clamp. Base computation, bookkeeping, and submission all happen
// under one lock hold, so two concurrent relative movers compose their deltas
// instead of one silently overwriting the other's endpoint.
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
		// Nothing to animate: the delta was zero or the clamp ate it at a
		// screen edge. The in-flight animation (if any) already targets base,
		// so submitting would only cancel and restart it.
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

// enqueueLocked places req on the worker channel, coalescing so only the latest
// target survives. The caller must hold a.mu.
//
// The buffer is size 1 and, under the lock, we are the only producer while the
// worker is the only consumer (it removes, never adds). So after draining a
// stale target the buffer is empty and the follow-up send cannot block. Targets
// carry no completion object, so dropping a superseded one is a plain discard —
// the shared session completion is untouched.
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
	// queued; otherwise the session ends.
	next, ok := a.finishOrNext(reqCh, stopCh)
	if ok {
		req = next

		goto restart
	}
}

// finishOrNext, called after a target is reached, either hands back the next
// queued target to continue the current session, or ends the session by closing
// its completion (keeping a.done referencing the closed completion so a late
// WaitForCursorIdle still sees "idle"). It no-ops when this worker has been
// superseded by a stop()/restart, detected via stopCh identity, so a stale
// worker never closes a newer session's completion.
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
