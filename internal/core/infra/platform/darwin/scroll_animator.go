//go:build darwin

package darwin

/*
#include "accessibility.h"
*/
import "C"

import (
	"context"
	"math"
	"sync"
	"time"
)

const (
	minScrollAnimationDuration = 10 // Minimum animation duration in ms
	minScrollStepDelay         = 1  // Minimum delay between steps in ms
	scrollDrainTimeoutBufferMs = 50 // Extra grace period while draining a canceled animation.
	easeOutCubicExponent       = 3  // Exponent for ease-out cubic easing
)

type scrollAnimator struct {
	cancel context.CancelFunc
	done   chan struct{}
	gen    uint64
	mu     sync.Mutex
}

var scrollAnim scrollAnimator

func (a *scrollAnimator) stop() {
	a.mu.Lock()
	cancel := a.cancel
	a.gen++
	a.cancel = nil
	a.done = nil
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (a *scrollAnimator) animate(
	deltaX, deltaY int,
	steps int,
	maxDuration int,
	durationPerPixel float64,
) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	a.mu.Lock()
	prevCancel := a.cancel
	prevDone := a.done
	a.gen++
	gen := a.gen
	a.cancel = cancel
	a.done = done
	a.mu.Unlock()

	if prevCancel != nil {
		prevCancel()
	}

	a.waitForPreviousAnimation(prevDone)

	a.mu.Lock()
	if a.gen != gen || a.done != done {
		a.mu.Unlock()
		close(done)
		cancel()

		return
	}
	a.mu.Unlock()

	go func(runGen uint64) {
		defer close(done)
		defer cancel()
		defer a.clearIfCurrent(runGen, done)

		magnitude := math.Hypot(float64(deltaX), float64(deltaY))
		if magnitude == 0 {
			return
		}

		duration := math.Min(float64(maxDuration), magnitude*durationPerPixel)
		if duration < minScrollAnimationDuration {
			duration = minScrollAnimationDuration
		}

		actualSteps := steps
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
			case <-ctx.Done():
				return
			default:
			}

			cursorPos := CursorPosition()
			cgPos := C.CGPoint{x: C.double(cursorPos.X), y: C.double(cursorPos.Y)}

			t := float64(step) / float64(actualSteps)
			eased := 1 - math.Pow(1-t, easeOutCubicExponent)

			targetX := int(math.Round(float64(deltaX) * eased))
			targetY := int(math.Round(float64(deltaY) * eased))

			chunkX := targetX - prevX
			chunkY := targetY - prevY
			prevX = targetX
			prevY = targetY

			C.scrollAtPoint(cgPos, C.int(chunkX), C.int(chunkY))

			if step < actualSteps {
				timer := time.NewTimer(stepDelay)
				select {
				case <-ctx.Done():
					timer.Stop()

					return
				case <-timer.C:
				}
			}
		}
	}(gen)
}

func (a *scrollAnimator) clearIfCurrent(gen uint64, done chan struct{}) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.gen == gen && a.done == done {
		a.cancel = nil
		a.done = nil
	}
}

func (a *scrollAnimator) waitForPreviousAnimation(prevDone chan struct{}) {
	if prevDone == nil {
		return
	}

	timeout := previousScrollAnimationDrainTimeout()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-prevDone:
	case <-timer.C:
	}
}

func previousScrollAnimationDrainTimeout() time.Duration {
	cfg := currentConfig()
	maxDurationMs := 180
	if cfg != nil && cfg.SmoothScroll.MaxDuration >= 0 {
		maxDurationMs = cfg.SmoothScroll.MaxDuration
	}

	return time.Duration(maxDurationMs+scrollDrainTimeoutBufferMs) * time.Millisecond
}
