//go:build darwin

package darwin

/*
#include "accessibility.h"
*/
import "C"

import (
	"image"
	"math"
	"sync"
	"time"

	"github.com/y3owk1n/neru/internal/domain/action"
)

const (
	minScrollAnimationDuration = 10 // Minimum animation duration in ms
	minScrollStepDelay         = 1  // Minimum delay between steps in ms
	easeOutCubicExponent       = 3  // Exponent for ease-out cubic easing
	// defaultScrollSteps answers a caller that supplied a nonsense step count
	// (<= 0). ValidateSmoothScroll already requires steps >= 1, so no validated
	// configuration reaches it; it exists so a direct caller cannot divide by
	// zero. Linux's animator keeps the same fallback.
	defaultScrollSteps = 12
)

// scrollRequest is one animated scroll. The modifier set belongs to the
// request rather than to the call that started the animation, because a
// request preempts whatever is in flight: a plain scroll_down arriving mid-zoom
// cancels the zoom and finishes unmodified, which is what the second binding
// asked for.
type scrollRequest struct {
	deltaX, deltaY   int
	modifiers        action.Modifiers
	steps            int
	maxDuration      int
	durationPerPixel float64
}

type scrollAnimator struct {
	// pos samples the live cursor position and post emits one scroll chunk
	// there. Both are injected so the schedule can be driven without cgo — the
	// animator is otherwise only observable through the events it posts to the
	// window server — mirroring smoothCursorAnimator's pos/post pair.
	//
	// Neither may call back into the animator; the production pair are plain
	// cgo calls, which is what makes that safe.
	pos  func() image.Point
	post func(at image.Point, deltaX, deltaY int, modifiers action.Modifiers)

	mu     sync.Mutex
	reqCh  chan scrollRequest
	stopCh chan struct{}
}

// postScrollEvent posts one scroll chunk at point through CoreGraphics. It is
// the production half of the animator's post seam.
func postScrollEvent(at image.Point, deltaX, deltaY int, modifiers action.Modifiers) {
	cgPos := C.CGPoint{x: C.double(at.X), y: C.double(at.Y)}
	C.NeruScrollAtPoint(cgPos, C.int(deltaX), C.int(deltaY), modifiersToCGEventFlags(modifiers))
}

var scrollAnim = newScrollAnimator(CursorPosition, postScrollEvent)

func newScrollAnimator(
	pos func() image.Point,
	post func(at image.Point, deltaX, deltaY int, modifiers action.Modifiers),
) *scrollAnimator {
	return &scrollAnimator{pos: pos, post: post}
}

func (a *scrollAnimator) stop() {
	a.mu.Lock()
	stopCh := a.stopCh
	a.reqCh = nil
	a.stopCh = nil

	if stopCh != nil {
		close(stopCh)
	}
	a.mu.Unlock()
}

func (a *scrollAnimator) animate(
	deltaX, deltaY int,
	modifiers action.Modifiers,
	steps int,
	maxDuration int,
	durationPerPixel float64,
) {
	req := scrollRequest{
		deltaX:           deltaX,
		deltaY:           deltaY,
		modifiers:        modifiers,
		steps:            steps,
		maxDuration:      maxDuration,
		durationPerPixel: durationPerPixel,
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.reqCh == nil {
		a.reqCh = make(chan scrollRequest, 1)
		a.stopCh = make(chan struct{})

		go a.run(a.reqCh, a.stopCh)
	}

	a.enqueueLocked(req)
}

// enqueueLocked places req on the worker channel, folding in a queued request
// the worker has not taken yet. The caller must hold a.mu.
//
// Under the lock we are the only producer and the worker is the only consumer,
// so after draining the buffer is empty and the follow-up send cannot block.
//
// A request still sitting in the buffer has had none of its delta delivered, so
// composeUndelivered folds all of it in — on the same rule the worker applies
// when a request preempts it mid-animation, which is what keeps the two seams
// from disagreeing about whether a same-modifier scroll can be dropped.
func (a *scrollAnimator) enqueueLocked(req scrollRequest) {
	select {
	case a.reqCh <- req:
		return
	default:
	}

	select {
	case queued := <-a.reqCh:
		req = composeUndelivered(req, queued, queued.deltaX, queued.deltaY)
	default:
	}

	select {
	case a.reqCh <- req:
	default:
	}
}

func (a *scrollAnimator) run(reqCh <-chan scrollRequest, stopCh <-chan struct{}) {
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

// runRequest animates one request to completion, or until a newer one preempts
// it — in which case the preempting request is played next, carrying whatever
// composeUndelivered folded into it.
func (a *scrollAnimator) runRequest(
	req scrollRequest,
	reqCh <-chan scrollRequest,
	stopCh <-chan struct{},
	timer *time.Timer,
) {
	for {
		next, restart := a.runOnce(req, reqCh, stopCh, timer)
		if !restart {
			return
		}

		req = next
	}
}

// runOnce plays one request's schedule. It reports the request that preempted
// this one, if any.
func (a *scrollAnimator) runOnce(
	req scrollRequest,
	reqCh <-chan scrollRequest,
	stopCh <-chan struct{},
	timer *time.Timer,
) (scrollRequest, bool) {
	magnitude := math.Hypot(float64(req.deltaX), float64(req.deltaY))
	if magnitude == 0 {
		return scrollRequest{}, false
	}

	duration := math.Min(float64(req.maxDuration), magnitude*req.durationPerPixel)
	if duration < minScrollAnimationDuration {
		duration = minScrollAnimationDuration
	}

	actualSteps := req.steps
	if actualSteps <= 0 {
		actualSteps = defaultScrollSteps
	}

	maxSteps := max(int(duration/float64(minScrollStepDelay)), 1)
	if actualSteps > maxSteps {
		actualSteps = maxSteps
	}

	stepDelayMs := max(int(math.Round(duration/float64(actualSteps))), minScrollStepDelay)

	stepDelay := time.Duration(stepDelayMs) * time.Millisecond

	// sentX/sentY is what has actually gone out. The curve is eased on the
	// cumulative position and each chunk is the difference between two points
	// on it, so at the last step they equal the request's delta exactly — which
	// is what makes req.delta - sent the undelivered remainder.
	var sentX, sentY int

	for step := 1; step <= actualSteps; step++ {
		select {
		case <-stopCh:
			return scrollRequest{}, false
		case next := <-reqCh:
			return composeUndelivered(next, req, req.deltaX-sentX, req.deltaY-sentY), true
		default:
		}

		t := float64(step) / float64(actualSteps)
		eased := 1 - math.Pow(1-t, easeOutCubicExponent)

		targetX := int(math.Round(float64(req.deltaX) * eased))
		targetY := int(math.Round(float64(req.deltaY) * eased))

		chunkX := targetX - sentX
		chunkY := targetY - sentY
		sentX = targetX
		sentY = targetY

		// The modifiers travel with every chunk, so a request that preempts
		// this one stamps its own on the chunks that remain.
		a.post(a.pos(), chunkX, chunkY, req.modifiers)

		if step < actualSteps {
			timer.Reset(stepDelay)
			select {
			case <-stopCh:
				stopAndDrainTimer(timer)

				return scrollRequest{}, false
			case next := <-reqCh:
				stopAndDrainTimer(timer)

				return composeUndelivered(next, req, req.deltaX-sentX, req.deltaY-sentY), true
			case <-timer.C:
			}
		}
	}

	return scrollRequest{}, false
}

// composeUndelivered folds the part of inflight that never went out — remX,
// remY — into the request preempting it, but only when the two ask for the same
// modifiers.
//
// Same modifiers is the held-repeat case, since a repeat re-sends the binding
// it is repeating: without this, every tick throws away whatever the previous
// tick had not yet emitted, and holding a scroll key travels visibly less than
// the same number of discrete presses.
//
// Different modifiers is the deliberate cancel scrollRequest documents: a plain
// scroll_down arriving mid-zoom finishes unmodified, which is what the second
// binding asked for. Carrying the zoom's remainder into it would emit that
// distance as a plain scroll — a scroll the user never asked for — so it is
// dropped exactly as before.
func composeUndelivered(next, inflight scrollRequest, remX, remY int) scrollRequest {
	if next.modifiers != inflight.modifiers {
		return next
	}

	next.deltaX += remX
	next.deltaY += remY

	return next
}

func stopAndDrainTimer(timer *time.Timer) {
	if timer.Stop() {
		return
	}

	select {
	case <-timer.C:
	default:
	}
}
