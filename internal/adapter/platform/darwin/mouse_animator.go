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

type cursorRequest struct {
	end              image.Point
	steps            int
	eventType        uint32
	button           uint32
	maxDuration      int
	durationPerPixel float64
	fixedDuration    int // when > 0, overrides the distance-derived duration
	done             *cursorAnimationDone
}

type smoothCursorAnimator struct {
	mu     sync.Mutex
	reqCh  chan cursorRequest
	stopCh chan struct{}
	done   *cursorAnimationDone
	// generation is bumped by stop() to invalidate steps from the animation
	// being canceled. See postIfCurrent.
	generation uint64
	// pendingEnd is the target of the animation currently in flight. Relative
	// moves extend it instead of restarting from the mid-animation cursor
	// position, so no part of a delta is lost under key repeat.
	pendingEnd image.Point
	hasPending bool
}

var cursorAnimator smoothCursorAnimator

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

	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
	C.NeruPostMouseMoveEventWithButton(pos, C.CGEventType(eventType), C.CGMouseButton(button))

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

	done := newCursorAnimationDone()
	req := cursorRequest{
		end:              end,
		steps:            steps,
		eventType:        eventType,
		button:           button,
		maxDuration:      maxDuration,
		durationPerPixel: durationPerPixel,
		done:             done,
	}

	a.submit(req)
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
		base = CursorPosition()
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
		done:          newCursorAnimationDone(),
	})
}

func (a *smoothCursorAnimator) submit(req cursorRequest) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.submitLocked(req)
}

// submitLocked records req as the pending animation and hands it to the
// worker. Callers must hold a.mu. The channel send cannot block: the buffer
// holds one request, the worker never takes a.mu around its receive, and
// submitters are serialized by a.mu, so after the drop below a slot is free.
func (a *smoothCursorAnimator) submitLocked(req cursorRequest) {
	a.done = req.done
	a.pendingEnd = req.end
	a.hasPending = true

	if a.reqCh == nil {
		a.reqCh = make(chan cursorRequest, 1)
		a.stopCh = make(chan struct{})

		go a.run(a.reqCh, a.stopCh)
	}

	select {
	case a.reqCh <- req:
	default:
		select {
		case dropped := <-a.reqCh:
			dropped.done.close()
		default:
		}
		a.reqCh <- req
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
	generation := a.currentGeneration()
	start := CursorPosition()
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
			req.done.close()

			return
		case nextReq := <-reqCh:
			req.done.close()
			req = nextReq

			goto restart
		default:
		}

		progress := float64(step) / float64(actualSteps)
		intermediate := image.Point{
			X: int(float64(start.X) + float64(req.end.X-start.X)*progress),
			Y: int(float64(start.Y) + float64(req.end.Y-start.Y)*progress),
		}

		if !a.postIfCurrent(generation, intermediate, req.eventType, req.button) {
			req.done.close()

			return
		}

		if step < actualSteps {
			timer.Reset(stepDelay)
			select {
			case <-stopCh:
				stopAndDrainTimer(timer)
				req.done.close()

				return
			case nextReq := <-reqCh:
				stopAndDrainTimer(timer)
				req.done.close()
				req = nextReq

				goto restart
			case <-timer.C:
			}
		}
	}

	req.done.close()
	a.clearDoneIfCurrent(req.done)
}

func (a *smoothCursorAnimator) clearDoneIfCurrent(done *cursorAnimationDone) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.done == done {
		a.done = nil
		a.hasPending = false
	}
}
