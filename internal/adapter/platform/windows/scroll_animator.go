//go:build windows

package windows

import (
	"math"
	"sync"
	"time"

	"github.com/y3owk1n/neru/internal/domain/action"
)

const (
	// minScrollAnimationDuration floors the animation length in ms, at the
	// number the darwin and linux animators floor at.
	minScrollAnimationDuration = 10
	// minScrollStepDelay is the shortest gap between injected steps, in ms. A
	// scheduling floor on this animator's own timer rather than a config
	// value, as minCursorStepDelay is for the cursor animator.
	minScrollStepDelay = 1
	// easeOutCubicExponent shapes the curve: fast at the start, settling at
	// the end. It matches the other two animators, so one configuration
	// produces the same shape everywhere.
	easeOutCubicExponent = 3
	// defaultScrollSteps answers a caller that supplied a nonsense step count
	// (<= 0). ValidateSmoothScroll already requires steps >= 1, so it exists
	// so a direct caller cannot divide by zero.
	defaultScrollSteps = 12
	// scrollGranularity is the smallest distance a wheel event can carry, in
	// the caller's pixels: MOUSEINPUT.mouseData is an integer count of
	// WHEEL_DELTA/120ths and wheelUnitsPerPixel of them make a pixel, so a
	// step can be as short as a quarter pixel, one 120th of a notch. That is
	// what makes this animate below a notch the way Wayland does and X11
	// cannot.
	scrollGranularity = 1.0 / wheelUnitsPerPixel
)

// scrollSession is one animation's hold on the injection path.
//
// It exists because a modifier has to stay held across every step: a
// ctrl-modified scroll is a real ctrl key pressed before the first chunk and
// released after the last, and pressing it around each chunk would read to an
// application as that many separate zoom gestures.
type scrollSession interface {
	// inject sends one chunk. Both deltas are exact multiples of
	// scrollGranularity, so the conversion to wheel data does not round.
	inject(deltaX, deltaY float64) error
	// close releases whatever begin acquired.
	close()
}

// sendInputScrollSession injects wheel events through SendInput with a
// modifier hold open for the length of the animation.
type sendInputScrollSession struct {
	hold modifierHold
}

func newSendInputScrollSession(modifiers action.Modifiers) (scrollSession, error) {
	hold, err := holdModifiers(modifiers)
	if err != nil {
		return nil, err
	}

	return &sendInputScrollSession{hold: hold}, nil
}

func (s *sendInputScrollSession) inject(deltaX, deltaY float64) error {
	// The same sign convention wheelEvents applies: positive deltaY is up and
	// positive deltaX is left, and only the horizontal axis needs negating.
	records := wheelRecords(
		int32(math.Round(deltaY*wheelUnitsPerPixel)),
		int32(math.Round(-deltaX*wheelUnitsPerPixel)),
	)

	for _, event := range records {
		err := sendMouseInput(event.flags, event.data)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *sendInputScrollSession) close() {
	s.hold.release()
}

// scrollChunks lays out one axis of an animated scroll: what each step should
// inject so that the steps together travel the requested distance.
//
// The curve is eased on the cumulative position and each chunk is the
// difference between two positions on it, so rounding never accumulates: a
// fraction of a wheel unit that one chunk cannot express stays in the
// difference and goes out with a later chunk. The schedule is laid out in
// whole wheel units and the last chunk takes whatever is left, so the steps
// together travel exactly the units the delta rounds to.
//
// delta is a float rather than the integer the caller's action carries
// because a request can arrive holding the undelivered remainder of the one
// it preempted (see composeUndelivered), and that remainder is a position on
// an eased curve.
func scrollChunks(delta float64, steps int) []float64 {
	if steps <= 0 {
		steps = defaultScrollSteps
	}

	chunks := make([]float64, steps)

	total := math.Round(delta * wheelUnitsPerPixel)
	if total == 0 {
		return chunks
	}

	var sent float64

	for step := 1; step < steps; step++ {
		progress := float64(step) / float64(steps)
		eased := 1 - math.Pow(1-progress, easeOutCubicExponent)

		chunk := math.Trunc(total*eased - sent)

		chunks[step-1] = chunk * scrollGranularity
		sent += chunk
	}

	chunks[steps-1] = (total - sent) * scrollGranularity

	return chunks
}

// scrollRequest is one animated scroll.
//
// The modifier set belongs to the request rather than to the animator,
// because a request preempts whatever is in flight: a plain scroll_down
// arriving mid-zoom cancels the zoom and finishes unmodified, which is what
// the second binding asked for.
type scrollRequest struct {
	deltaX, deltaY   float64
	modifiers        action.Modifiers
	steps            int
	maxDuration      int
	durationPerPixel float64
}

// composeUndelivered folds the part of inflight that never went out — remX,
// remY — into the request preempting it, but only when the two ask for the
// same modifiers.
//
// Same modifiers is the held-repeat case: without this, every tick throws
// away whatever the previous tick had not yet injected, and holding a scroll
// key travels visibly less than the same number of discrete presses.
// Different modifiers is the deliberate cancel scrollRequest documents, and
// the remainder is dropped so the zoom's distance is not injected as a plain
// scroll the user never asked for.
func composeUndelivered(next, inflight scrollRequest, remX, remY float64) scrollRequest {
	if next.modifiers != inflight.modifiers {
		return next
	}

	next.deltaX += remX
	next.deltaY += remY

	return next
}

// scrollAnimator spreads a scroll over time on one worker goroutine, with the
// latest request winning. It is the cursor animator's shape applied to the
// wheel: the daemon has one scroll at a time, and a second one arriving
// mid-animation replaces the first rather than queueing behind it.
type scrollAnimator struct {
	// begin opens a session. It is a field so a test can substitute one;
	// production wires newSendInputScrollSession.
	begin func(action.Modifiers) (scrollSession, error)

	mu     sync.Mutex
	reqCh  chan scrollRequest
	stopCh chan struct{}

	// injectSem is a size-1 semaphore the worker holds across opening a
	// session, each injected chunk, and closing it, re-checking stopCh under
	// it before injecting. It is the handoff to stop(): once stop() holds it,
	// no chunk is mid-flight and none will start. A session holds real
	// modifier keys down, so a stale worker's release is ordered against a
	// later worker's press by the same token. It is never held together with
	// mu.
	injectSem chan struct{}
}

var scrollAnim = newScrollAnimator(newSendInputScrollSession)

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

	// Order whatever the caller does next after any chunk still in flight,
	// so a canceled animation cannot land after the direct scroll that
	// replaced it.
	a.injectSem <- struct{}{}

	<-a.injectSem
}

// animate hands a request to the worker, replacing any request already
// queued.
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
// Under the lock we are the only producer and the worker is the only
// consumer, so after draining the buffer is empty and the follow-up send
// cannot block. A request still sitting in the buffer has had none of its
// delta injected, so composeUndelivered folds all of it in.
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
// order itself after it. When stopCh is non-nil the call is skipped if the
// session was canceled while we waited for the token; it reports whether the
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

// runRequest animates one request to completion, or until a newer one
// preempts it. The session is opened per request rather than per animator,
// so a preempting request presses its own modifiers and the one it replaced
// lets go of exactly what it pressed.
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

// runOnce plays one request's schedule. It reports the request that
// preempted this one, if any.
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

	// Opening presses real modifier keys, so it takes the fence too: without
	// it a worker starting here could press ctrl before a worker stopped a
	// moment ago released it. It also checks stopCh under the fence, because
	// a request the worker took just before stop() must not press a key the
	// direct scroll that replaced it then has to wait out.
	opened := a.underFence(func() { session, err = a.begin(req.modifiers) }, stopCh)
	if !opened {
		return scrollRequest{}, false
	}

	if err != nil {
		// Reported the way the cursor animator reports a failed step, which
		// is not at all: there is no caller left to return it to.
		return scrollRequest{}, false
	}

	// Closing releases real modifier keys, so it goes out under the same
	// fence as an injected chunk, and unconditionally, because a session left
	// open leaves the key held.
	defer a.underFence(session.close, nil)

	steps, stepDelay := scrollScheduleTiming(req)

	chunksX := scrollChunks(req.deltaX, steps)
	chunksY := scrollChunks(req.deltaY, steps)

	for step := range steps {
		select {
		case <-stopCh:
			return scrollRequest{}, false
		case next := <-reqCh:
			// Chunks from step on have not gone out.
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

			if !injected || injectErr != nil {
				// Canceled, or SendInput went away mid-animation. Playing the
				// remaining steps out would spend the whole duration failing
				// once per step; the session is closed on the way out.
				return scrollRequest{}, false
			}
		}

		if step == steps-1 {
			break
		}

		timer.Reset(stepDelay)

		select {
		case <-stopCh:
			stopAndDrainTimer(timer)

			return scrollRequest{}, false
		case next := <-reqCh:
			stopAndDrainTimer(timer)

			// This chunk has gone out; the undelivered remainder starts at
			// the next one.
			return composeUndelivered(
				next, req, sumFrom(chunksX, step+1), sumFrom(chunksY, step+1),
			), true
		case <-timer.C:
		}
	}

	return scrollRequest{}, false
}

// sumFrom totals the chunks from index on: the part of a schedule that has
// not been injected yet.
func sumFrom(chunks []float64, from int) float64 {
	var total float64

	for _, chunk := range chunks[from:] {
		total += chunk
	}

	return total
}

// scrollScheduleTiming turns a request's configuration into a step count and
// the gap between steps, with the same arithmetic the darwin and linux
// animators use so one configuration produces one animation everywhere.
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
