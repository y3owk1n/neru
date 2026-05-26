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
)

const (
	minAnimationDuration = 10 // Minimum animation duration in ms
	minStepDelay         = 1  // Minimum delay between steps in ms
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
	maxDuration      int
	durationPerPixel float64
	done             *cursorAnimationDone
}

type smoothCursorAnimator struct {
	mu     sync.Mutex
	reqCh  chan cursorRequest
	stopCh chan struct{}
	done   *cursorAnimationDone
}

var cursorAnimator smoothCursorAnimator

func (a *smoothCursorAnimator) stop() {
	a.mu.Lock()
	stopCh := a.stopCh
	done := a.done
	a.reqCh = nil
	a.stopCh = nil
	a.done = nil

	if stopCh != nil {
		close(stopCh)
	}
	a.mu.Unlock()

	done.close()
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

func (a *smoothCursorAnimator) animateTo(end image.Point, steps int, eventType uint32) {
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
		maxDuration:      maxDuration,
		durationPerPixel: durationPerPixel,
		done:             done,
	}

	reqCh := a.ensureWorker(done)
	select {
	case reqCh <- req:
	default:
		select {
		case dropped := <-reqCh:
			dropped.done.close()
		default:
		}
		reqCh <- req
	}
}

func (a *smoothCursorAnimator) ensureWorker(done *cursorAnimationDone) chan cursorRequest {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.done = done

	if a.reqCh != nil {
		return a.reqCh
	}

	reqCh := make(chan cursorRequest, 1)
	stopCh := make(chan struct{})
	a.reqCh = reqCh
	a.stopCh = stopCh

	go a.run(reqCh, stopCh)

	return reqCh
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
	start := CursorPosition()
	distance := math.Hypot(float64(req.end.X-start.X), float64(req.end.Y-start.Y))

	duration := math.Min(float64(req.maxDuration), distance*req.durationPerPixel)
	if duration < minAnimationDuration {
		duration = minAnimationDuration
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

		pos := C.CGPoint{x: C.double(intermediate.X), y: C.double(intermediate.Y)}
		C.NeruPostMouseMoveEvent(pos, C.CGEventType(req.eventType))

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
	}
}
