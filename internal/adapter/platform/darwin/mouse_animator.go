//go:build darwin

package darwin

/*
#include "accessibility.h"
*/
import "C"

import (
	"context"
	"image"
	"math"
	"sync"
	"time"

	"github.com/y3owk1n/neru/internal/config"
)

const (
	minStepDelay = 1 // Minimum delay between steps in ms
)

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
	eventType        uint32
	button           uint32
	maxDuration      int
	durationPerPixel float64
	fixedDuration    int // when > 0, overrides the distance-derived duration
}

// smoothCursorAnimator glides the cursor toward a target: one worker
// goroutine, latest-target-wins, and a completion channel WaitForCursorIdle
// blocks on.
//
// Completion is per session — every target arriving mid-session shares one
// completion, so a waiter tracks the latest target rather than being released
// when an intermediate one is superseded. A move, wait, act sequence
// (ActionService) would otherwise act at a point the cursor had already left.
type smoothCursorAnimator struct {
	// pos samples the live cursor position and post emits one move event.
	// Both are injected so the worker loop can be driven without cgo: the
	// animator is otherwise only observable through the events it posts to the
	// window server. Production wires them to CoreGraphics in cursorAnimator.
	//
	// Both may be called with a.mu held (post always is, so the check and the
	// post cannot be split — see postIfCurrent), so neither may call back into
	// the animator. The production pair are plain cgo calls, which is what
	// makes that safe; a substitute must be too.
	pos  func() image.Point
	post func(point image.Point, eventType, button uint32)

	mu     sync.Mutex
	reqCh  chan cursorRequest
	stopCh chan struct{}
	// done is the current session's completion; nil only between stop() and
	// the next session. After a session ends naturally it keeps referencing
	// the closed completion, so a late waiter still sees "idle".
	done *cursorAnimationDone
	// busy reports that a session is in progress — the worker is still
	// draining toward a target. A target submitted while busy joins that
	// session instead of starting a new completion.
	busy bool
	// generation is bumped by stop() to invalidate steps from the animation
	// being canceled. See postIfCurrent.
	generation uint64
	// pendingEnd is the target of the animation currently in flight. Relative
	// moves extend it instead of restarting from the mid-animation cursor
	// position, so no part of a delta is lost under key repeat.
	pendingEnd image.Point
	hasPending bool
}

func newSmoothCursorAnimator(
	pos func() image.Point,
	post func(point image.Point, eventType, button uint32),
) *smoothCursorAnimator {
	return &smoothCursorAnimator{pos: pos, post: post}
}

// postCursorMoveEvent posts one cursor move (or drag) event through
// CoreGraphics. It is the production half of the animator's post seam.
func postCursorMoveEvent(point image.Point, eventType, button uint32) {
	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
	C.NeruPostMouseMoveEventWithButton(pos, C.CGEventType(eventType), C.CGMouseButton(button))
}

var cursorAnimator = newSmoothCursorAnimator(CursorPosition, postCursorMoveEvent)

func (a *smoothCursorAnimator) stop() {
	a.mu.Lock()
	stopCh := a.stopCh
	done := a.done
	a.reqCh = nil
	a.stopCh = nil
	a.done = nil

	// Closing stopCh on its own is not enough to stop the worker: it can already
	// have passed its cancellation check and be about to post a step, which then
	// lands after whatever replaced this animation and drags the cursor — and
	// the zoom viewport — back toward the abandoned target. Bumping the
	// generation under the same lock the worker posts under closes that window.
	a.generation++
	a.busy = false
	a.hasPending = false

	if stopCh != nil {
		close(stopCh)
	}
	a.mu.Unlock()

	done.close()
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
// Position-dependent reads call this so an action firing mid-animation acts
// at the point the user aimed for, without waiting the animation out.
func (a *smoothCursorAnimator) settle() {
	end, ok := a.takePendingForSettle()
	if !ok {
		return
	}

	eventType, button := dragEventType()
	pos := C.CGPoint{x: C.double(end.X), y: C.double(end.Y)}
	C.NeruMoveMouseWithTypeAndButton(pos, eventType, button)
}

// takePendingForSettle atomically ends the in-flight animation and hands back
// its endpoint: the worker is canceled with the same generation bump as
// stop() — so a step already past its checks is dropped rather than landing
// after the settle warp — waiters are released, and the pending endpoint is
// cleared. ok reports whether there was an animation to settle.
func (a *smoothCursorAnimator) takePendingForSettle() (image.Point, bool) {
	a.mu.Lock()

	if !a.hasPending {
		a.mu.Unlock()

		return image.Point{}, false
	}

	end := a.pendingEnd
	stopCh := a.stopCh
	done := a.done
	a.reqCh = nil
	a.stopCh = nil
	a.done = nil
	a.generation++
	a.busy = false
	a.hasPending = false

	if stopCh != nil {
		close(stopCh)
	}
	a.mu.Unlock()

	done.close()

	return end, true
}

// currentGeneration returns the animation generation in effect right now.
func (a *smoothCursorAnimator) currentGeneration() uint64 {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.generation
}

// postIfCurrent posts one cursor move, but only if the animation that produced
// it has not been canceled in the meantime. The check and the post share the
// lock with stop(), so once stop() has returned no further step can escape.
func (a *smoothCursorAnimator) postIfCurrent(
	generation uint64,
	point image.Point,
	eventType uint32,
	button uint32,
) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.generation != generation {
		return false
	}

	a.post(point, eventType, button)

	return true
}

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

func (a *smoothCursorAnimator) animateTo(end image.Point, steps int, eventType, button uint32) {
	cfg := currentConfig()
	maxDuration := 200
	durationPerPixel := 0.1
	if cfg != nil {
		maxDuration = cfg.SmoothCursor.MaxDuration
		durationPerPixel = cfg.SmoothCursor.DurationPerPixel
	}

	a.submit(cursorRequest{
		end:              end,
		steps:            steps,
		eventType:        eventType,
		button:           button,
		maxDuration:      maxDuration,
		durationPerPixel: durationPerPixel,
	})
}

// animateRelativeBy animates a relative move with a fixed duration,
// independent of the distance, so that cursor speed scales with the per-move
// delta instead of collapsing to the constant velocity the
// distance-proportional duration would produce.
//
// The base is the pending endpoint of the animation in flight (falling back to
// the live cursor position), extended by delta and clamped by the caller's
// clamp. Base computation, bookkeeping, and submission all happen under one
// lock hold: two concurrent relative movers therefore compose their deltas
// instead of one silently overwriting the other's endpoint.
func (a *smoothCursorAnimator) animateRelativeBy(
	delta image.Point,
	clamp func(image.Point) image.Point,
	steps int,
	durationMs int,
	eventType, button uint32,
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
		eventType:     eventType,
		button:        button,
		fixedDuration: durationMs,
	})
}

func (a *smoothCursorAnimator) submit(req cursorRequest) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.submitLocked(req)
}

// submitLocked records req as the pending animation target and hands it to the
// worker, starting the worker and a fresh session completion when needed.
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
// target survives. Callers must hold a.mu.
//
// The buffer holds one request and, under the lock, we are the only producer
// while the worker is the only consumer (it removes, never adds), so after
// draining a stale target the follow-up send cannot block — it stays a plain
// send rather than another non-blocking one, because submitLocked has already
// published req.end as the pending endpoint and a dropped target would leave
// pendingTarget() naming a point nothing is animating toward. Targets carry no
// completion object, so dropping a superseded one is a plain discard — the
// shared session completion is untouched, and its waiters stay attached to the
// target that replaced it.
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

	a.reqCh <- req
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
	generation := a.currentGeneration()
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
		actualSteps = 10
	}

	maxSteps := max(int(duration/float64(minStepDelay)), 1)
	if actualSteps > maxSteps {
		actualSteps = maxSteps
	}

	stepDelayMs := max(int(math.Round(duration/float64(actualSteps))), minStepDelay)
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

		// A false answer means stop() or settle() bumped the generation; both
		// already closed this session's completion, so the worker just leaves.
		if !a.postIfCurrent(generation, intermediate, req.eventType, req.button) {
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
	// queued; otherwise the session ends and its waiters are released.
	next, ok := a.finishOrNext(reqCh, stopCh)
	if ok {
		req = next

		goto restart
	}
}

// finishOrNext, called once a target is reached, either hands back the next
// queued target to continue the current session, or ends the session by
// closing its completion (keeping a.done referencing the closed completion so
// a late waiter still sees "idle"). It no-ops when this worker has been
// superseded by a stop()/settle()/restart, detected via stopCh identity, so a
// stale worker never closes a newer session's completion.
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
