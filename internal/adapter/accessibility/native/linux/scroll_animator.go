//go:build linux

package linux

import (
	"math"
	"sync"
	"time"

	eventtaplinux "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

const (
	// minScrollAnimationDuration floors the animation length in ms. Shorter
	// steps land closer than a compositor delivers them. darwin uses the same.
	minScrollAnimationDuration = 10
	// minScrollStepDelay is the shortest gap between injected steps, in ms, a
	// scheduling floor for this animator's timer rather than a config value.
	minScrollStepDelay = 1
	// easeOutCubicExponent shapes the curve, fast then settling, so a page scroll
	// reads as one movement. It matches darwin's, so one config gives one shape.
	easeOutCubicExponent = 3
	// defaultScrollSteps answers a step count <= 0 so a direct caller cannot
	// divide by zero. ValidateSmoothScroll keeps every config from reaching it.
	defaultScrollSteps = 12
)

// maxScrollUnitsPerRequest caps how many units one animated scroll travels on
// a granular backend, the same ceiling as x11ScrollAtCursor's maxClicks. A
// continuous backend reports granularity 0 and is never capped.
const maxScrollUnitsPerRequest = 50

// scrollBackendAvailable reports, synchronously and without side effects,
// whether an animated scroll can be injected at all, so the answer reaches
// whoever pressed the key.
//
// The animation runs on a worker goroutine where an error has nobody to go
// back to, and the backend enum alone cannot answer. A CGO_ENABLED=0 build
// reports the Wayland backend while every injection path is a CodeNotSupported
// stub. Without this check it would answer "scrolled" and move nothing, the
// silent no-op ADR 0013 exists to end. It answers "could this ever work". A
// failure once the animation runs goes unreported, as on darwin.
func scrollBackendAvailable() error {
	if currentLinuxBackend() == linuxBackendX11 {
		return x11ScrollBackendAvailable()
	}

	if currentLinuxBackend() == linuxBackendWayland {
		return waylandScrollBackendAvailable()
	}

	return derrors.New(
		derrors.CodeNotSupported,
		"animated scroll is not supported without an X11 or Wayland backend",
	)
}

// beginScrollSession opens a session on the live backend.
//
// The backend is read once here so one animation cannot straddle two, and the
// modifier press and release go to the same backend.
func beginScrollSession(modifiers action.Modifiers) (scrollSession, error) {
	if currentLinuxBackend() == linuxBackendX11 {
		return newX11ScrollSession(modifiers)
	}

	if currentLinuxBackend() == linuxBackendWayland {
		// Hyprland needs its own session only to hold a modifier. Without one the
		// virtual pointer carries the chunks continuously, which beats whole notches.
		// Without uinput it falls back to the Wayland session.
		if modifiers != 0 &&
			hyprlandKeepsUinputScroll() &&
			eventtaplinux.IsUinputScrollAvailable() {
			return newHyprlandScrollSession(modifiers)
		}

		return newWaylandScrollSession(modifiers)
	}

	return nil, derrors.New(
		derrors.CodeNotSupported,
		"animated scroll is not supported without an X11 or Wayland backend",
	)
}

// scrollSession is one backend's hold on the injection path for one animation.
// A modifier stays held across every step, since pressing ctrl around each of
// twenty chunks would read as twenty zoom gestures. On X11 the session owns
// the display connection too, because the connection that pressed the modifier
// has to release it.
type scrollSession interface {
	// granularity is the smallest distance this backend can express, in the
	// caller's pixels. Zero means continuous.
	granularity() float64
	// inject sends one chunk. Both deltas are exact multiples of granularity
	// when that is non-zero, so no backend has to round.
	inject(deltaX, deltaY float64) error
	// close releases whatever begin acquired.
	close()
}

// scrollChunks lays out one axis of an animated scroll, what each step injects
// so the steps together travel the requested distance.
//
// The curve is eased on the cumulative position and each chunk is the
// difference between two positions, so rounding never accumulates and a chunk
// too small for a granular backend goes out with a later one.
//
// maxUnits caps a granular backend's travel in units of granularity, as the
// unanimated path does (a scroll_step_full of a million pixels would be tens
// of thousands of X11 clicks). Zero means uncapped. delta is a float because
// it can carry a preempted request's undelivered remainder (composeUndelivered).
func scrollChunks(delta float64, steps int, granularity float64, maxUnits int) []float64 {
	if steps <= 0 {
		steps = defaultScrollSteps
	}

	chunks := make([]float64, steps)
	if delta == 0 {
		return chunks
	}

	total := delta

	// A granular backend travels the nearest whole number of units, never fewer
	// than one, as scrollNotches counts them. Rounding up front lets the eased
	// chunks below truncate and still add up.
	if granularity > 0 {
		units := math.Max(math.Round(math.Abs(total)/granularity), 1)
		if maxUnits > 0 {
			units = math.Min(units, float64(maxUnits))
		}

		total = math.Copysign(units*granularity, total)
	}

	// A one-unit scroll on a granular backend sends its single event on the first
	// step. Anywhere later is pure added latency. A scroll_step under 45 pixels is
	// that case.
	if granularity > 0 && math.Abs(total) == granularity {
		unit := granularity
		if total < 0 {
			unit = -granularity
		}

		chunks[0] = unit

		return chunks
	}

	var sent float64

	for step := 1; step <= steps; step++ {
		progress := float64(step) / float64(steps)
		eased := 1 - math.Pow(1-progress, easeOutCubicExponent)

		want := total*eased - sent

		chunk := want
		if granularity > 0 {
			chunk = math.Trunc(want/granularity) * granularity
		}

		chunks[step-1] = chunk
		sent += chunk
	}

	return chunks
}

// scrollRequest is one animated scroll.
//
// The modifiers belong to the request, since a request preempts whatever is
// in flight. A plain scroll_down arriving mid-zoom cancels the zoom and
// finishes unmodified, as on darwin. The deltas are floats because a request
// can absorb the undelivered remainder of a same-modifier one it preempts.
type scrollRequest struct {
	deltaX, deltaY   float64
	modifiers        action.Modifiers
	steps            int
	maxDuration      int
	durationPerPixel float64
}

// composeUndelivered folds remX and remY, the part of inflight that never went
// out, into the request preempting it, but only for the same modifiers.
//
// Same modifiers is a held repeat. Without this, each tick drops what the last
// had not injected, and holding a key travels less than as many presses.
// Different modifiers is the deliberate cancel scrollRequest describes, and
// the zoom's remainder must not go out as a plain scroll, so it is dropped.
func composeUndelivered(next, inflight scrollRequest, remX, remY float64) scrollRequest {
	if next.modifiers != inflight.modifiers {
		return next
	}

	next.deltaX += remX
	next.deltaY += remY

	return next
}

// scrollAnimator spreads a scroll over time on one worker goroutine, latest
// request winning, as darwin's does. A scroll arriving mid-animation replaces
// the first rather than queueing.
type scrollAnimator struct {
	// begin opens a session on the live backend. Tests substitute it.
	begin func(action.Modifiers) (scrollSession, error)

	mu     sync.Mutex
	reqCh  chan scrollRequest
	stopCh chan struct{}

	// injectSem is a size-1 semaphore the worker holds across opening a session,
	// each chunk, and closing it, re-checking stopCh under it. Once stop() holds
	// it, no chunk is in flight and none will start (the platform/linux cursor
	// animator uses the same fence). It also orders a stale worker's modifier
	// release before a later worker's press. It is never held together with mu.
	injectSem chan struct{}
}

var scrollAnim = newScrollAnimator(beginScrollSession)

func newScrollAnimator(begin func(action.Modifiers) (scrollSession, error)) *scrollAnimator {
	return &scrollAnimator{begin: begin, injectSem: make(chan struct{}, 1)}
}

// stop cancels any animation in flight. The unanimated path calls it so a
// scroll that arrives with smooth scroll switched off is not still being
// chased by chunks from before the reload.
func (a *scrollAnimator) stop() {
	a.mu.Lock()
	stopCh := a.stopCh
	a.reqCh = nil
	a.stopCh = nil

	if stopCh != nil {
		close(stopCh)
	}

	a.mu.Unlock()

	// Wait out any chunk still in flight, one native call long, so a canceled
	// animation cannot land after the direct scroll that replaced it.
	a.injectSem <- struct{}{}

	<-a.injectSem
}

// animate hands a request to the worker, replacing any request already queued.
func (a *scrollAnimator) animate(
	deltaX, deltaY int,
	modifiers action.Modifiers,
	steps int,
	maxDuration int,
	durationPerPixel float64,
) {
	req := scrollRequest{
		deltaX:           float64(deltaX),
		deltaY:           float64(deltaY),
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

// enqueueLocked places req on the worker channel, folding in a request the
// worker has not taken yet. The caller must hold a.mu, which makes this the
// only producer, so after draining the follow-up send cannot block. A queued
// request has injected nothing, so composeUndelivered folds all of it in, on
// the same rule as a mid-animation preempt.
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

// underFence runs one native call while holding injectSem, so stop() can
// order itself after it. With a non-nil stopCh the call is skipped if the
// session was canceled while waiting for the token. It reports whether the
// call ran.
func (a *scrollAnimator) underFence(call func(), stopCh <-chan struct{}) bool {
	a.injectSem <- struct{}{}
	defer func() { <-a.injectSem }()

	if stopCh != nil {
		select {
		case <-stopCh:
			return false
		default:
		}
	}

	call()

	return true
}

func (a *scrollAnimator) run(reqCh <-chan scrollRequest, stopCh <-chan struct{}) {
	timer := time.NewTimer(time.Hour)

	stopAndDrainScrollTimer(timer)
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

// runRequest animates one request until done or preempted. The session is per
// request, so a preempting request presses its own modifiers and the replaced
// one releases exactly what it pressed.
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
	if req.deltaX == 0 && req.deltaY == 0 {
		return scrollRequest{}, false
	}

	var (
		session scrollSession
		err     error
	)

	// Opening presses real modifier keys, so it takes the fence, or it could press
	// ctrl before a just-stopped worker released it.
	a.underFence(func() { session, err = a.begin(req.modifiers) }, nil)

	if err != nil {
		// The backend broke after ScrollAtCursor's check (a dropped KDE portal
		// grant, a lost compositor connection). There is no caller left, so it goes
		// unreported as on darwin. scrollBackendAvailable keeps out backends that
		// never worked.
		return scrollRequest{}, false
	}

	// Closing releases real modifier keys, so it runs under the fence and, with a
	// nil stopCh, unconditionally, since an unclosed session leaves the key held.
	defer a.underFence(session.close, nil)

	steps, stepDelay := scrollScheduleTiming(req)

	granularity := session.granularity()
	chunksX := scrollChunks(req.deltaX, steps, granularity, maxScrollUnitsPerRequest)
	chunksY := scrollChunks(req.deltaY, steps, granularity, maxScrollUnitsPerRequest)

	for step := range steps {
		select {
		case <-stopCh:
			return scrollRequest{}, false
		case next := <-reqCh:
			// Chunks from step on have not gone out. What the schedule declined to
			// send stays dropped, the sub-unit residue a discrete press drops too and
			// the distance past maxUnits.
			return composeUndelivered(
				next, req, sumFrom(chunksX, step), sumFrom(chunksY, step),
			), true
		default:
		}

		if chunksX[step] != 0 || chunksY[step] != 0 {
			var injectErr error

			injected := a.underFence(func() {
				injectErr = session.inject(chunksX[step], chunksY[step])
			}, stopCh)

			if !injected {
				return scrollRequest{}, false
			}

			if injectErr != nil {
				// The backend went away. Stop rather than fail once per remaining
				// step. The deferred close releases the session.
				return scrollRequest{}, false
			}
		}

		if step == steps-1 {
			break
		}

		timer.Reset(stepDelay)

		select {
		case <-stopCh:
			stopAndDrainScrollTimer(timer)

			return scrollRequest{}, false
		case next := <-reqCh:
			stopAndDrainScrollTimer(timer)

			// This chunk has gone out. The undelivered remainder starts at the
			// next one.
			return composeUndelivered(
				next, req, sumFrom(chunksX, step+1), sumFrom(chunksY, step+1),
			), true
		case <-timer.C:
		}
	}

	return scrollRequest{}, false
}

// sumFrom totals the chunks from index on, the part of a schedule that has not
// been injected yet.
func sumFrom(chunks []float64, from int) float64 {
	var total float64

	for _, chunk := range chunks[from:] {
		total += chunk
	}

	return total
}

// scrollScheduleTiming turns a request's configuration into a step count and
// the gap between steps, using the same arithmetic darwin's animator does so
// one configuration produces one animation on both.
func scrollScheduleTiming(req scrollRequest) (int, time.Duration) {
	magnitude := math.Hypot(req.deltaX, req.deltaY)

	duration := math.Min(float64(req.maxDuration), magnitude*req.durationPerPixel)
	if duration < minScrollAnimationDuration {
		duration = minScrollAnimationDuration
	}

	steps := req.steps
	if steps <= 0 {
		steps = defaultScrollSteps
	}

	maxSteps := max(int(duration/float64(minScrollStepDelay)), 1)
	if steps > maxSteps {
		steps = maxSteps
	}

	stepDelayMs := max(int(math.Round(duration/float64(steps))), minScrollStepDelay)

	return steps, time.Duration(stepDelayMs) * time.Millisecond
}

// stopAndDrainScrollTimer stops timer and clears any pending tick so the next
// Reset starts clean.
func stopAndDrainScrollTimer(timer *time.Timer) {
	if timer.Stop() {
		return
	}

	select {
	case <-timer.C:
	default:
	}
}
