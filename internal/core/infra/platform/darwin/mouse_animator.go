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
	minAnimationDuration = 10 // Minimum animation duration in ms
	minStepDelay         = 1  // Minimum delay between steps in ms
	drainTimeoutBufferMs = 50 // Extra grace period while draining a canceled animation.
)

type smoothCursorAnimator struct {
	cancel context.CancelFunc
	done   chan struct{}
	gen    uint64
	mu     sync.Mutex
}

var cursorAnimator smoothCursorAnimator

func (a *smoothCursorAnimator) stop() {
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

func (a *smoothCursorAnimator) wait(ctx context.Context) error {
	a.mu.Lock()
	done := a.done
	a.mu.Unlock()

	if done == nil {
		return nil
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *smoothCursorAnimator) animateTo(end image.Point, steps int, eventType uint32) {
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

	cfg := config.Global()
	maxDuration := 200
	durationPerPixel := 0.1
	if cfg != nil {
		maxDuration = cfg.SmoothCursor.MaxDuration
		durationPerPixel = cfg.SmoothCursor.DurationPerPixel
	}

	go func(runGen uint64) {
		defer close(done)
		defer cancel()
		defer a.clearIfCurrent(runGen, done)
		start := CursorPosition()
		distance := math.Hypot(float64(end.X-start.X), float64(end.Y-start.Y))

		duration := math.Min(float64(maxDuration), distance*durationPerPixel)
		if duration < minAnimationDuration {
			duration = minAnimationDuration
		}

		actualSteps := steps
		if actualSteps <= 0 {
			actualSteps = 10
		}

		// Reduce steps so total time stays within the computed duration.
		// Without this, a high step count with a short duration would be
		// inflated by the minStepDelay floor (e.g. 100 steps × 1ms = 100ms
		// even when the adaptive duration is only 10ms).
		maxSteps := max(int(duration/float64(minStepDelay)), 1)
		if actualSteps > maxSteps {
			actualSteps = maxSteps
		}

		stepDelayMs := max(int(math.Round(duration/float64(actualSteps))), minStepDelay)

		stepDelay := time.Duration(stepDelayMs) * time.Millisecond

		for step := 1; step <= actualSteps; step++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			progress := float64(step) / float64(actualSteps)
			intermediate := image.Point{
				X: int(float64(start.X) + float64(end.X-start.X)*progress),
				Y: int(float64(start.Y) + float64(end.Y-start.Y)*progress),
			}

			pos := C.CGPoint{x: C.double(intermediate.X), y: C.double(intermediate.Y)}
			C.postMouseMoveEvent(pos, C.CGEventType(eventType))

			if step < actualSteps {
				// Use a timer so that context cancellation interrupts the
				// wait immediately instead of blocking until the full
				// stepDelay elapses.
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

func (a *smoothCursorAnimator) clearIfCurrent(
	gen uint64,
	done chan struct{},
) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.gen == gen && a.done == done {
		a.cancel = nil
		a.done = nil
	}
}

func (a *smoothCursorAnimator) waitForPreviousAnimation(prevDone chan struct{}) {
	if prevDone == nil {
		return
	}

	timeout := previousAnimationDrainTimeout()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-prevDone:
	case <-timer.C:
	}
}

func previousAnimationDrainTimeout() time.Duration {
	cfg := config.Global()
	maxDurationMs := 200
	if cfg != nil && cfg.SmoothCursor.MaxDuration > 0 {
		maxDurationMs = cfg.SmoothCursor.MaxDuration
	}

	return time.Duration(maxDurationMs+drainTimeoutBufferMs) * time.Millisecond
}
