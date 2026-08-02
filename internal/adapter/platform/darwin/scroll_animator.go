//go:build darwin

package darwin

/*
#include "accessibility.h"
*/
import "C"

import (
	"math"
	"sync"
	"time"
)

const (
	minScrollAnimationDuration = 10 // Minimum animation duration in ms
	minScrollStepDelay         = 1  // Minimum delay between steps in ms
	easeOutCubicExponent       = 3  // Exponent for ease-out cubic easing
)

type scrollRequest struct {
	deltaX, deltaY   int
	steps            int
	maxDuration      int
	durationPerPixel float64
}

type scrollAnimator struct {
	mu     sync.Mutex
	reqCh  chan scrollRequest
	stopCh chan struct{}
}

var scrollAnim scrollAnimator

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
	steps int,
	maxDuration int,
	durationPerPixel float64,
) {
	req := scrollRequest{
		deltaX:           deltaX,
		deltaY:           deltaY,
		steps:            steps,
		maxDuration:      maxDuration,
		durationPerPixel: durationPerPixel,
	}

	reqCh := a.ensureWorker()
	select {
	case reqCh <- req:
	default:
		select {
		case <-reqCh:
		default:
		}
		reqCh <- req
	}
}

func (a *scrollAnimator) ensureWorker() chan scrollRequest {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.reqCh != nil {
		return a.reqCh
	}

	reqCh := make(chan scrollRequest, 1)
	stopCh := make(chan struct{})
	a.reqCh = reqCh
	a.stopCh = stopCh

	go a.run(reqCh, stopCh)

	return reqCh
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

func (a *scrollAnimator) runRequest(
	req scrollRequest,
	reqCh <-chan scrollRequest,
	stopCh <-chan struct{},
	timer *time.Timer,
) {
restart:
	magnitude := math.Hypot(float64(req.deltaX), float64(req.deltaY))
	if magnitude == 0 {
		return
	}

	duration := math.Min(float64(req.maxDuration), magnitude*req.durationPerPixel)
	if duration < minScrollAnimationDuration {
		duration = minScrollAnimationDuration
	}

	actualSteps := req.steps
	if actualSteps <= 0 {
		actualSteps = 12
	}

	maxSteps := max(int(duration/float64(minScrollStepDelay)), 1)
	if actualSteps > maxSteps {
		actualSteps = maxSteps
	}

	stepDelayMs := max(int(math.Round(duration/float64(actualSteps))), minScrollStepDelay)

	stepDelay := time.Duration(stepDelayMs) * time.Millisecond

	var prevX, prevY int

	for step := 1; step <= actualSteps; step++ {
		select {
		case <-stopCh:
			return
		case req = <-reqCh:
			goto restart
		default:
		}

		cursorPos := CursorPosition()
		cgPos := C.CGPoint{x: C.double(cursorPos.X), y: C.double(cursorPos.Y)}

		t := float64(step) / float64(actualSteps)
		eased := 1 - math.Pow(1-t, easeOutCubicExponent)

		targetX := int(math.Round(float64(req.deltaX) * eased))
		targetY := int(math.Round(float64(req.deltaY) * eased))

		chunkX := targetX - prevX
		chunkY := targetY - prevY
		prevX = targetX
		prevY = targetY

		C.NeruScrollAtPoint(cgPos, C.int(chunkX), C.int(chunkY))

		if step < actualSteps {
			timer.Reset(stepDelay)
			select {
			case <-stopCh:
				stopAndDrainTimer(timer)

				return
			case req = <-reqCh:
				stopAndDrainTimer(timer)

				goto restart
			case <-timer.C:
			}
		}
	}
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
