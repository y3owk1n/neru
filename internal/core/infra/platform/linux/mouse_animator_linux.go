//go:build linux

package linux

import (
	"context"
	"image"
	"math"
	"sync"
	"time"
)

const (
	// minCursorAnimationDuration is the floor for a single move so even a
	// zero-distance request settles promptly instead of instantly.
	minCursorAnimationDuration = 10 // ms
	// minCursorStepDelay is the shortest gap between injected steps.
	minCursorStepDelay = 1 // ms
	// defaultCursorSteps is used when the config value is unset (<= 0).
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

// cursorRequest is a single "animate to end" job. Config values are captured
// per-request so a live config reload mid-animation does not change an
// in-flight tween.
type cursorRequest struct {
	end              image.Point
	steps            int
	maxDuration      int
	durationPerPixel float64
	done             *cursorAnimationDone
}

// smoothCursorAnimator animates cursor movement toward a target by stepping a
// backend move function over time. It mirrors the darwin animator design: a
// single worker goroutine, latest-request-wins coalescing, and a done channel
// that WaitForCursorIdle blocks on.
//
// The X11/Wayland injection detail is kept out of the animator by injecting
// pos/move at construction: pos samples the current cursor once per request
// (the interpolation is then purely mathematical, so a stale Wayland cache only
// affects the glide path, never the final landing point) and move performs one
// instantaneous warp. Per-step move errors are best-effort and ignored, exactly
// like the cgo darwin path where the C call cannot fail back to Go.
type smoothCursorAnimator struct {
	pos  func() image.Point
	move func(image.Point) error

	mu     sync.Mutex
	reqCh  chan cursorRequest
	stopCh chan struct{}
	done   *cursorAnimationDone
}

func newSmoothCursorAnimator(
	pos func() image.Point,
	move func(image.Point) error,
) *smoothCursorAnimator {
	return &smoothCursorAnimator{pos: pos, move: move}
}

// stop cancels any in-flight animation and releases waiters. It is called on
// the non-smooth move path so a direct warp always wins over a running tween.
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

// wait blocks until the current animation settles or ctx is canceled. It
// returns immediately when no animation is active, preserving the historical
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

// animateTo enqueues an animation toward end, coalescing so only the latest
// target survives when requests arrive faster than the worker drains them.
func (a *smoothCursorAnimator) animateTo(
	end image.Point,
	steps, maxDuration int,
	durationPerPixel float64,
) {
	done := newCursorAnimationDone()
	req := cursorRequest{
		end:              end,
		steps:            steps,
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
	start := a.pos()

	distance := math.Hypot(float64(req.end.X-start.X), float64(req.end.Y-start.Y))

	duration := math.Min(float64(req.maxDuration), distance*req.durationPerPixel)
	if duration < minCursorAnimationDuration {
		duration = minCursorAnimationDuration
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

		_ = a.move(intermediate)

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
