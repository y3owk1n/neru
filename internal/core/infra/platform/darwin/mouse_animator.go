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
)

type smoothCursorAnimator struct {
	cancel context.CancelFunc
	done   chan struct{}
	wg     sync.WaitGroup
	mu     sync.Mutex
}

var cursorAnimator smoothCursorAnimator

func (a *smoothCursorAnimator) stop() {
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}

	// Wait inside the lock so a concurrent animateTo cannot sneak in a new
	// goroutine between Unlock and Wait.  The animation goroutine never
	// acquires a.mu, so this cannot deadlock.
	a.wg.Wait()
	a.done = nil

	a.mu.Unlock()
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
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	done := make(chan struct{})
	a.done = done

	// Wait inside the lock so no other caller can race past and launch a
	// second goroutine between Wait and Go.  The animation goroutine never
	// acquires a.mu, so this cannot deadlock.
	a.wg.Wait()

	cfg := config.Global()
	maxDuration := 200
	durationPerPixel := 0.1
	if cfg != nil {
		maxDuration = cfg.SmoothCursor.MaxDuration
		durationPerPixel = cfg.SmoothCursor.DurationPerPixel
	}

	a.wg.Go(func() {
		defer close(done)
		defer cancel()
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
	})

	a.mu.Unlock()
}
