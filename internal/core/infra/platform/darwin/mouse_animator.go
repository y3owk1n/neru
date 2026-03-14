//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c

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
	minSteps             = 1  // Minimum number of steps
)

type smoothCursorAnimator struct {
	cancel context.CancelFunc
	mu     sync.Mutex
}

var cursorAnimator smoothCursorAnimator

func (a *smoothCursorAnimator) animateTo(end image.Point, steps int, eventType uint32) {
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.mu.Unlock()

	cfg := config.Global()
	maxDuration := 200
	durationPerPixel := 0.1
	if cfg != nil {
		maxDuration = cfg.SmoothCursor.MaxDuration
		durationPerPixel = cfg.SmoothCursor.DurationPerPixel
	}

	go func() {
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
		stepDelayMs := max(int(duration)/actualSteps, minStepDelay)

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

			time.Sleep(time.Duration(stepDelayMs) * time.Millisecond)
		}
	}()
}
