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

// cursorRequest is a single "animate to end" target. It carries no completion
// object of its own: completion is tracked per animation *session* (see
// smoothCursorAnimator), so coalescing a superseded target never releases a
// waiter early.
type cursorRequest struct {
	end              image.Point
	steps            int
	maxDuration      int
	durationPerPixel float64
}

// smoothCursorAnimator animates cursor movement toward a target by stepping a
// backend move function over time. It mirrors the darwin animator's semantics:
// a single worker goroutine, latest-target-wins coalescing, and a completion
// channel that WaitForCursorIdle blocks on.
//
// Like the darwin animator it is preemptive and fire-and-forget: callers do not
// serialize cursor access (IPC, hotkey, and event-tap paths all drive moves
// concurrently), so a bypassed/direct move cancels an in-flight animation and
// the last writer wins. Backend move errors during an animation are best-effort
// and not reported through WaitForCursorIdle — matching darwin, whose native
// move call cannot fail back to Go. The direct (non-smooth) MoveCursorToPoint
// path still returns backend errors synchronously; only the opt-in animated
// path degrades to best-effort.
//
// Completion is tracked per *session*, not per request. A session begins when
// an idle animator receives a target and ends when the queue drains. All
// targets that arrive while the session is busy — including coalesced
// replacements — share the one session completion, so a waiter stays attached
// until the cursor reaches the latest target rather than being released when an
// intermediate target is superseded.
//
// The X11/Wayland injection detail is kept out of the animator by injecting
// pos/move at construction: pos samples the current cursor once per request
// (interpolation is then purely mathematical, so a stale Wayland cache only
// affects the glide path, never the final landing point) and move performs one
// instantaneous warp.
type smoothCursorAnimator struct {
	pos  func() image.Point
	move func(image.Point) error

	mu     sync.Mutex
	reqCh  chan cursorRequest
	stopCh chan struct{}
	done   *cursorAnimationDone // current session completion; nil only between stop() and the next session
	busy   bool                 // a session is in progress (worker actively draining toward a target)
}

func newSmoothCursorAnimator(
	pos func() image.Point,
	move func(image.Point) error,
) *smoothCursorAnimator {
	return &smoothCursorAnimator{pos: pos, move: move}
}

// stop cancels any in-flight animation and releases waiters. It is called on
// the non-smooth move path so a direct warp always wins over a running tween.
// It detaches the current worker via stopCh so a stale worker cannot mutate a
// later session.
func (a *smoothCursorAnimator) stop() {
	a.mu.Lock()
	stopCh := a.stopCh
	done := a.done
	a.reqCh = nil
	a.stopCh = nil
	a.done = nil
	a.busy = false

	if stopCh != nil {
		close(stopCh)
	}
	a.mu.Unlock()

	done.close()
}

// wait blocks until the current session settles or ctx is canceled. It returns
// immediately when no session is active, preserving the historical
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

// animateTo enqueues an animation toward end. A fresh session completion is
// created only when the animator is idle; a target arriving mid-session joins
// the running session so its waiter tracks the latest target.
//
// Worker start and enqueue happen under a.mu in one critical section. This is
// what makes it safe against a concurrent stop(): a bypassed move can never
// swap out the channel or kill the worker between "pick a channel" and "send",
// so a request can never be stranded on an orphaned channel. Because every
// producer holds the lock and the worker is the sole consumer of the size-1
// buffer, the enqueue is always non-blocking.
func (a *smoothCursorAnimator) animateTo(
	end image.Point,
	steps, maxDuration int,
	durationPerPixel float64,
) {
	req := cursorRequest{
		end:              end,
		steps:            steps,
		maxDuration:      maxDuration,
		durationPerPixel: durationPerPixel,
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.reqCh == nil {
		a.reqCh = make(chan cursorRequest, 1)
		a.stopCh = make(chan struct{})

		go a.run(a.reqCh, a.stopCh)
	}

	if !a.busy {
		a.done = newCursorAnimationDone()
		a.busy = true
	}

	a.enqueueLocked(req)
}

// enqueueLocked places req on the worker channel, coalescing so only the latest
// target survives. The caller must hold a.mu.
//
// The buffer is size 1 and, under the lock, we are the only producer while the
// worker is the only consumer (it removes, never adds). So after draining a
// stale target the buffer is empty and the follow-up send cannot block. Targets
// carry no completion object, so dropping a superseded one is a plain discard —
// the shared session completion is untouched.
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

	select {
	case a.reqCh <- req:
	default:
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

		// Best-effort, matching darwin (whose native move cannot fail back to
		// Go): a failed backend warp during the opt-in animation is not
		// surfaced through WaitForCursorIdle. The direct MoveCursorToPoint path
		// still returns backend errors synchronously.
		_ = a.move(intermediate)

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
	// queued; otherwise the session ends.
	next, ok := a.finishOrNext(reqCh, stopCh)
	if ok {
		req = next

		goto restart
	}
}

// finishOrNext, called after a target is reached, either hands back the next
// queued target to continue the current session, or ends the session by closing
// its completion (keeping a.done referencing the closed completion so a late
// WaitForCursorIdle still sees "idle"). It no-ops when this worker has been
// superseded by a stop()/restart, detected via stopCh identity, so a stale
// worker never closes a newer session's completion.
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
	a.mu.Unlock()

	done.close()

	return cursorRequest{}, false
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
