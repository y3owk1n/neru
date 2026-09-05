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
	// minScrollAnimationDuration floors the animation length in ms. Below it
	// the steps land closer together than a compositor will deliver them, so
	// the animation costs events without being seen. darwin's animator floors
	// at the same number.
	minScrollAnimationDuration = 10
	// minScrollStepDelay is the shortest gap between injected steps, in ms. A
	// scheduling floor on this animator's own timer rather than a config value,
	// so nothing here derives from the schema and nothing in the schema derives
	// from it.
	minScrollStepDelay = 1
	// easeOutCubicExponent shapes the curve: fast at the start, settling at the
	// end, which is what makes a page scroll read as one movement instead of a
	// jump. It matches darwin's exponent, so the same configuration produces
	// the same shape on both.
	easeOutCubicExponent = 3
	// defaultScrollSteps answers a caller that supplied a nonsense step count
	// (<= 0). ValidateSmoothScroll already requires steps >= 1, so no validated
	// configuration reaches it; it exists so a direct caller cannot divide by
	// zero.
	defaultScrollSteps = 12
)

// maxScrollUnitsPerRequest caps how many indivisible units one animated scroll
// travels on a granular backend. It is the X11 ceiling the unanimated path
// already applies (x11ScrollAtCursor's maxClicks), restated here because the
// animator is what decides the schedule; a continuous backend reports
// granularity 0 and the cap never applies to it.
const maxScrollUnitsPerRequest = 50

// scrollBackendAvailable reports whether an animated scroll can be injected at
// all, synchronously and without side effects, so that the answer can be
// returned to whoever pressed the key.
//
// This is what keeps smooth_scroll from turning a loud refusal into silence.
// The animation runs on a worker goroutine, where an error has nobody to go
// back to, and the backend enum alone does not answer the question: a
// CGO_ENABLED=0 build reads WAYLAND_DISPLAY and reports the Wayland backend
// exactly as a CGO build does, while every injection path in it is a
// CodeNotSupported stub. Without this check that configuration would answer
// "scrolled" and move nothing — the silent no-op ADR 0013 exists to end.
//
// It answers "could this ever work", not "will this call succeed": a backend
// that fails once the animation is running is reported the way darwin reports
// a failed native scroll, which is not at all.
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
// The backend is asked here rather than at each step so one animation cannot
// straddle two of them, and so the modifier press and its release are certainly
// the same backend's.
func beginScrollSession(modifiers action.Modifiers) (scrollSession, error) {
	if currentLinuxBackend() == linuxBackendX11 {
		return newX11ScrollSession(modifiers)
	}

	if currentLinuxBackend() == linuxBackendWayland {
		// Hyprland only needs its own session when there is a modifier to hold:
		// without one the virtual pointer carries the chunks as it does
		// everywhere else, and continuously, which beats whole notches. With
		// uinput unavailable it falls back to the same session, which is the
		// path a modified animated scroll took before Hyprland was named here.
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

// scrollSession is one backend's hold on the injection path for the length of
// one animation.
//
// It exists because a modifier has to stay held across every step: Linux has no
// event-flags concept, so a ctrl-modified scroll is a real ctrl key pressed
// before the first chunk and released after the last. Pressing and releasing it
// around each of twenty chunks would read to an application as twenty separate
// zoom gestures. On X11 the session owns the display connection for the same
// reason — the modifier is server state, and the connection that pressed it has
// to be the one that lets it go.
type scrollSession interface {
	// granularity is the smallest distance this backend can express, in the
	// same pixels the caller's delta is in. Zero means continuous: the backend
	// carries whatever fraction it is handed.
	granularity() float64
	// inject sends one chunk. Both deltas are exact multiples of granularity
	// when that is non-zero, so no backend has to round.
	inject(deltaX, deltaY float64) error
	// close releases whatever begin acquired.
	close()
}

// scrollChunks lays out one axis of an animated scroll: what each step should
// inject so that the steps together travel the requested distance.
//
// The curve is eased on the cumulative position and each chunk is the
// difference between two positions on it, so rounding never accumulates: a
// chunk a granular backend cannot express is not dropped, it stays in the
// difference and goes out with a later chunk.
//
// maxUnits caps how far a granular backend will travel, in units of
// granularity, mirroring the ceiling the unanimated path already applies (the
// X11 scroll is button clicks, and a scroll_step_full of a million pixels would
// otherwise be tens of thousands of them). Zero means uncapped, which is what a
// continuous backend takes.
//
// delta is a float rather than the integer the caller's action carries because
// a request can arrive holding the undelivered remainder of the one it
// preempted (see composeUndelivered), and that remainder is a position on an
// eased curve.
func scrollChunks(delta float64, steps int, granularity float64, maxUnits int) []float64 {
	if steps <= 0 {
		steps = defaultScrollSteps
	}

	chunks := make([]float64, steps)
	if delta == 0 {
		return chunks
	}

	total := delta

	// A granular backend travels the nearest whole number of units, never
	// fewer than one, exactly as the unanimated path counts them in
	// scrollNotches. The animation spreads the same notches over time, it
	// does not shorten the scroll. Rounding up front rather than per step
	// is what lets the eased chunks below truncate and still add up to it.
	if granularity > 0 {
		units := math.Max(math.Round(math.Abs(total)/granularity), 1)
		if maxUnits > 0 {
			units = math.Min(units, float64(maxUnits))
		}

		total = math.Copysign(units*granularity, total)
	}

	// A scroll worth one unit on a granular backend is not an animation:
	// whatever the curve says, exactly one event goes out. Putting it anywhere
	// but the first step would deliver the same single wheel click the
	// unanimated path sends, only later — pure added latency, which is the one
	// thing this must not buy. A scroll_step under 45 pixels is that case.
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
// The modifier set belongs to the request rather than to the animator, because
// a request preempts whatever is in flight: a plain scroll_down arriving
// mid-zoom cancels the zoom and finishes unmodified, which is what the second
// binding asked for. darwin's animator carries them for the same reason.
//
// The deltas are floats although every caller's action is in whole pixels: a
// request that preempts a same-modifier one absorbs its undelivered remainder,
// which is a point on an eased curve rather than a whole pixel.
type scrollRequest struct {
	deltaX, deltaY   float64
	modifiers        action.Modifiers
	steps            int
	maxDuration      int
	durationPerPixel float64
}

// composeUndelivered folds the part of inflight that never went out — remX,
// remY — into the request preempting it, but only when the two ask for the same
// modifiers.
//
// Same modifiers is the held-repeat case, since a repeat re-sends the binding
// it is repeating: without this, every tick throws away whatever the previous
// tick had not yet injected, and holding a scroll key travels visibly less than
// the same number of discrete presses.
//
// Different modifiers is the deliberate cancel scrollRequest documents: a plain
// scroll_down arriving mid-zoom finishes unmodified. Carrying the zoom's
// remainder into it would inject that distance as a plain scroll the user never
// asked for, so it is dropped exactly as before.
func composeUndelivered(next, inflight scrollRequest, remX, remY float64) scrollRequest {
	if next.modifiers != inflight.modifiers {
		return next
	}

	next.deltaX += remX
	next.deltaY += remY

	return next
}

// scrollAnimator spreads a scroll over time on one worker goroutine, with the
// latest request winning. It mirrors darwin's animator: the daemon has one
// scroll at a time, and a second one arriving mid-animation replaces the first
// rather than queueing behind it.
type scrollAnimator struct {
	// begin opens a session on the live backend. It is a field so a test can
	// substitute one; production wires beginScrollSession.
	begin func(action.Modifiers) (scrollSession, error)

	mu     sync.Mutex
	reqCh  chan scrollRequest
	stopCh chan struct{}

	// injectSem is a size-1 semaphore the worker holds across opening a
	// session, each injected chunk, and closing it, re-checking stopCh under it
	// before injecting. It is the handoff to stop(): once stop() holds it, no
	// chunk is mid-flight and none will start. The cursor animator in
	// platform/linux uses the same fence for the same reason, and here there is
	// a second one — a session holds real modifier keys down, so a stale
	// worker's release has to be ordered against a later worker's press. It is
	// never held together with mu.
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

	// Order whatever the caller does next after any chunk still in flight. The
	// wait is one native call long — a display flush or a handful of XTest
	// button events — and it is what keeps a canceled animation from landing
	// after the direct scroll that replaced it.
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
// worker has not taken yet. The caller must hold a.mu.
//
// Under the lock we are the only producer and the worker is the only consumer,
// so after draining the buffer is empty and the follow-up send cannot block.
//
// A request still sitting in the buffer has had none of its delta injected, so
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

// underFence runs one native call while holding injectSem, so stop() can order
// itself after it. When stopCh is non-nil the call is skipped if the session
// was canceled while we waited for the token, which is what keeps a canceled
// chunk from landing after the direct scroll that replaced it; it reports
// whether the call ran.
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

// runRequest animates one request to completion, or until a newer one preempts
// it. The session is opened per request rather than per animator, so a
// preempting request presses its own modifiers and the one it replaced lets go
// of exactly what it pressed.
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

	// Opening presses real modifier keys, so it takes the fence too: without it
	// a worker starting here could press ctrl before a worker stopped a moment
	// ago released it.
	a.underFence(func() { session, err = a.begin(req.modifiers) }, nil)

	if err != nil {
		// A backend that fails here rather than at ScrollAtCursor's check is
		// one that broke between the two — the KDE portal grant dropped, the
		// compositor connection lost. It is reported the way darwin reports a
		// failed native scroll, which is not at all: there is no caller left to
		// return it to. What must never reach here is a backend that was never
		// going to work, and scrollBackendAvailable is what keeps it out.
		return scrollRequest{}, false
	}

	// Closing releases real modifier keys, so it goes out under the same fence
	// as an injected chunk — and unconditionally, because a session left open
	// leaves the key held. nil rather than stopCh is what makes it
	// unconditional.
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
			// Chunks from step on have not gone out. What the schedule already
			// declined to send is absent from them by construction, and stays
			// dropped: the sub-unit residue a granular backend cannot express,
			// which a discrete press drops too — so a held key and the same
			// number of presses still land within one wheel notch per tick of
			// each other on X11 — and the distance past maxUnits, which is a
			// rate ceiling no animation was going to deliver.
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
				// The backend went away mid-animation. Playing the remaining
				// steps out against it would spend the whole duration failing
				// once per step; the session is closed on the way out and the
				// next scroll starts clean.
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

			// This chunk has gone out; the undelivered remainder starts at the
			// next one.
			return composeUndelivered(
				next, req, sumFrom(chunksX, step+1), sumFrom(chunksY, step+1),
			), true
		case <-timer.C:
		}
	}

	return scrollRequest{}, false
}

// sumFrom totals the chunks from index on — the part of a schedule that has not
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
