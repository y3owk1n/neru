//go:build linux

package linux

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// fakeScrollSession stands in for a backend so the schedule can be checked
// without a compositor.
type fakeScrollSession struct {
	mu        sync.Mutex
	gran      float64
	chunks    [][2]float64
	closed    bool
	injectErr error
}

func (s *fakeScrollSession) granularity() float64 { return s.gran }

func (s *fakeScrollSession) inject(deltaX, deltaY float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.chunks = append(s.chunks, [2]float64{deltaX, deltaY})

	return s.injectErr
}

func (s *fakeScrollSession) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
}

func (s *fakeScrollSession) traveled() (float64, float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var totalX, totalY float64

	for _, chunk := range s.chunks {
		totalX += chunk[0]
		totalY += chunk[1]
	}

	return totalX, totalY
}

func (s *fakeScrollSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.closed
}

func sum(values []float64) float64 {
	var total float64

	for _, value := range values {
		total += value
	}

	return total
}

// TestScrollChunks_TravelsTheRequestedDistance is the property the whole
// animation rests on: switching smooth scroll on changes when a scroll arrives,
// never how far it goes.
func TestScrollChunks_TravelsTheRequestedDistance(t *testing.T) {
	tests := []struct {
		name  string
		delta float64
		steps int
	}{
		{name: "one line down", delta: -50, steps: 20},
		{name: "half a page up", delta: 500, steps: 20},
		{name: "a single step", delta: 300, steps: 1},
		{name: "more steps than pixels", delta: 7, steps: 64},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chunks := scrollChunks(test.delta, test.steps, 0, 0)

			if len(chunks) != test.steps {
				t.Fatalf("got %d chunks, want %d", len(chunks), test.steps)
			}

			if got := sum(chunks); math.Abs(got-test.delta) > 1e-9 {
				t.Errorf("chunks total %v, want %v", got, test.delta)
			}
		})
	}
}

// TestScrollChunks_EasesOut pins the shape rather than the endpoint: the early
// chunks are the long ones, which is what makes the movement read as one
// gesture settling rather than a constant-speed crawl.
func TestScrollChunks_EasesOut(t *testing.T) {
	chunks := scrollChunks(-1000, 10, 0, 0)

	for i := 1; i < len(chunks); i++ {
		if math.Abs(chunks[i]) > math.Abs(chunks[i-1])+1e-9 {
			t.Fatalf("chunk %d (%v) is longer than chunk %d (%v); the curve is not easing out",
				i, chunks[i], i-1, chunks[i-1])
		}
	}

	if chunks[0] >= 0 {
		t.Errorf("first chunk = %v, want a negative one for a negative delta", chunks[0])
	}
}

// TestScrollChunks_KeepsAGranularBackendOnWholeUnits is the X11 half: a wheel
// button is one notch and there is no fraction of one to send, so every chunk
// has to be a whole notch and the remainder has to survive into a later chunk
// instead of being rounded away step by step.
func TestScrollChunks_KeepsAGranularBackendOnWholeUnits(t *testing.T) {
	const notch = scrollPixelsPerNotch

	chunks := scrollChunks(-500, 20, notch, maxScrollUnitsPerRequest)

	for i, chunk := range chunks {
		if math.Mod(chunk, notch) != 0 {
			t.Errorf("chunk %d = %v, which is not a whole multiple of %v", i, chunk, notch)
		}
	}

	// The unanimated X11 path sends abs(delta)/30 clicks, truncated: 16 here.
	// Rounding per step would lose several of them.
	if got, want := sum(chunks), -16*float64(notch); got != want {
		t.Errorf("chunks total %v, want %v (16 notches)", got, want)
	}
}

// TestScrollChunks_SendsAOneUnitScrollImmediately covers the case that is not
// an animation at all, and the one the default configuration hits on X11: a
// scroll_step of 50 pixels is a single wheel notch there. One event goes out
// either way, so scheduling it anywhere but the first step would deliver the
// same scroll later — added latency and nothing else.
func TestScrollChunks_SendsAOneUnitScrollImmediately(t *testing.T) {
	const notch = scrollPixelsPerNotch

	// Shorter than a notch, exactly a notch, and the shipped scroll_step.
	for _, delta := range []float64{10, -10, 1, -1, notch, -notch, 50, -50} {
		chunks := scrollChunks(delta, 20, notch, maxScrollUnitsPerRequest)

		want := float64(notch)
		if delta < 0 {
			want = -want
		}

		if got := sum(chunks); got != want {
			t.Errorf("scrollChunks(%v, …) totals %v, want %v", delta, got, want)
		}

		if chunks[0] != want {
			t.Errorf("scrollChunks(%v, …) puts %v on the first step, want the whole %v "+
				"— a one-notch scroll must not be delayed", delta, chunks[0], want)
		}
	}
}

// TestScrollChunks_CapsAGranularBackend keeps the ceiling the unanimated X11
// path already applies: scroll_step_full is a million pixels, and thirty-three
// thousand button clicks is not an animation.
func TestScrollChunks_CapsAGranularBackend(t *testing.T) {
	const notch = scrollPixelsPerNotch

	chunks := scrollChunks(1000000, 20, notch, maxScrollUnitsPerRequest)

	if got, want := sum(chunks), float64(maxScrollUnitsPerRequest*notch); got != want {
		t.Errorf("chunks total %v, want the %d-notch ceiling (%v)",
			got, maxScrollUnitsPerRequest, want)
	}

	// A continuous backend has no ceiling: capping it would make a modified
	// go_bottom travel a different distance from an unmodified one.
	if got := sum(scrollChunks(1000000, 20, 0, maxScrollUnitsPerRequest)); got != 1000000 {
		t.Errorf("continuous chunks total %v, want the full 1000000", got)
	}
}

// TestScrollAnimator_Animate_TravelsTheWholeDistance is the end-to-end of the
// scheduler against a stand-in backend: every chunk reaches it, and the session
// is closed when the animation ends.
func TestScrollAnimator_Animate_TravelsTheWholeDistance(t *testing.T) {
	session := &fakeScrollSession{}
	animator := newScrollAnimator(
		func(_ action.Modifiers) (scrollSession, error) { return session, nil },
	)

	t.Cleanup(animator.stop)

	// Both axes at once: a scroll_right bound alongside a scroll_down has to
	// arrive as one diagonal animation, not as one axis lost to the other.
	animator.animate(120, -500, 0, 8, 40, 0.1)

	waitFor(t, func() bool { return session.isClosed() })

	horizontal, vertical := session.traveled()
	if vertical != -500 {
		t.Errorf("the animation traveled %v vertically, want -500", vertical)
	}

	if horizontal != 120 {
		t.Errorf("the animation traveled %v horizontally, want 120", horizontal)
	}
}

// TestScrollAnimator_Animate_HoldsTheModifiersOfTheRequestThatIsRunning pins
// the reason a session exists at all: the modifier is a real key, pressed once
// before the animation and released once after it, not toggled around every
// chunk.
func TestScrollAnimator_Animate_HoldsTheModifiersOfTheRequestThatIsRunning(t *testing.T) {
	session := &fakeScrollSession{}

	var (
		guard   sync.Mutex
		opens   int
		lastMod action.Modifiers
	)

	animator := newScrollAnimator(func(modifiers action.Modifiers) (scrollSession, error) {
		guard.Lock()
		opens++
		lastMod = modifiers
		guard.Unlock()

		return session, nil
	})

	t.Cleanup(animator.stop)

	animator.animate(0, -300, action.ModCtrl, 6, 30, 0.1)

	waitFor(t, func() bool { return session.isClosed() })

	guard.Lock()
	defer guard.Unlock()

	if opens != 1 {
		t.Errorf("the animation opened %d sessions, want exactly 1", opens)
	}

	if lastMod != action.ModCtrl {
		t.Errorf("the session was opened with %v, want ctrl", lastMod)
	}
}

// TestScrollAnimator_Animate_StopsOnABackendThatDiesMidAnimation covers the
// backend that passed ScrollAtCursor's availability check and then broke — a
// KDE portal grant dropped, a compositor connection lost. There is no caller
// left to return the error to, so the animation ends rather than spending its
// whole duration failing once per step. The loud refusal belongs to
// scrollBackendAvailable, which runs before any of this.
func TestScrollAnimator_Animate_StopsOnABackendThatDiesMidAnimation(t *testing.T) {
	var (
		guard sync.Mutex
		tries int
	)

	animator := newScrollAnimator(func(_ action.Modifiers) (scrollSession, error) {
		guard.Lock()
		tries++
		guard.Unlock()

		return nil, derrors.New(derrors.CodeNotSupported, "no backend")
	})

	t.Cleanup(animator.stop)

	animator.animate(0, -300, 0, 6, 30, 0.1)

	waitFor(t, func() bool {
		guard.Lock()
		defer guard.Unlock()

		return tries == 1
	})
}

// TestScrollAnimator_Animate_IgnoresAnEmptyScroll keeps a zero delta from
// opening a session, which on X11 is a display connection and a modifier press
// for no movement at all.
func TestScrollAnimator_Animate_IgnoresAnEmptyScroll(t *testing.T) {
	var (
		guard sync.Mutex
		opens int
	)

	animator := newScrollAnimator(func(_ action.Modifiers) (scrollSession, error) {
		guard.Lock()
		opens++
		guard.Unlock()

		return &fakeScrollSession{}, nil
	})

	t.Cleanup(animator.stop)

	animator.animate(0, 0, 0, 6, 30, 0.1)

	time.Sleep(50 * time.Millisecond)

	guard.Lock()
	defer guard.Unlock()

	if opens != 0 {
		t.Errorf("a zero-delta scroll opened %d sessions, want none", opens)
	}
}

// TestScrollAnimator_Stop_EndsTheAnimation covers the reload case: smooth
// scroll is switched off, and the scroll that arrives next must not be chased
// by chunks scheduled before it.
func TestScrollAnimator_Stop_EndsTheAnimation(t *testing.T) {
	session := &fakeScrollSession{}
	animator := newScrollAnimator(
		func(_ action.Modifiers) (scrollSession, error) { return session, nil },
	)

	// Long enough that stop() lands mid-animation.
	animator.animate(0, -1000, 0, 40, 400, 1.0)

	waitFor(t, func() bool {
		session.mu.Lock()
		defer session.mu.Unlock()

		return len(session.chunks) > 0
	})

	animator.stop()

	waitFor(t, func() bool { return session.isClosed() })

	_, before := session.traveled()

	time.Sleep(60 * time.Millisecond)

	if _, after := session.traveled(); after != before {
		t.Errorf("the animation traveled %v more after stop(); it should have ended", after-before)
	}
}

// TestScrollAnimator_Animate_AbandonsAnAnimationOnTheFirstFailedChunk keeps a
// dead backend from costing the whole animation duration: the session is closed
// on the first refusal instead of the remaining steps being played out against
// it.
func TestScrollAnimator_Animate_AbandonsAnAnimationOnTheFirstFailedChunk(t *testing.T) {
	session := &fakeScrollSession{injectErr: derrors.New(derrors.CodeActionFailed, "gone")}
	animator := newScrollAnimator(
		func(_ action.Modifiers) (scrollSession, error) { return session, nil },
	)

	t.Cleanup(animator.stop)

	animator.animate(0, -1000, 0, 20, 400, 1.0)

	waitFor(t, func() bool { return session.isClosed() })

	session.mu.Lock()
	defer session.mu.Unlock()

	if len(session.chunks) != 1 {
		t.Errorf("the animation injected %d chunks against a failing backend, want 1",
			len(session.chunks))
	}
}

// loggedChunk is one injected chunk together with the modifier set the session
// that carried it was opened with.
type loggedChunk struct {
	deltaX, deltaY float64
	modifiers      action.Modifiers
}

// scrollLog records every chunk across every session the animator opens. A
// preempting request opens its own session, so the modifier set is what tells
// the two runs apart.
type scrollLog struct {
	mu     sync.Mutex
	gran   float64
	chunks []loggedChunk
}

func (l *scrollLog) begin(modifiers action.Modifiers) (scrollSession, error) {
	return &loggingScrollSession{log: l, modifiers: modifiers}, nil
}

func (l *scrollLog) traveled() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()

	var total float64

	for _, chunk := range l.chunks {
		total += chunk.deltaY
	}

	return total
}

func (l *scrollLog) traveledWith(modifiers action.Modifiers) float64 {
	l.mu.Lock()
	defer l.mu.Unlock()

	var total float64

	for _, chunk := range l.chunks {
		if chunk.modifiers == modifiers {
			total += chunk.deltaY
		}
	}

	return total
}

func (l *scrollLog) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.chunks)
}

type loggingScrollSession struct {
	log       *scrollLog
	modifiers action.Modifiers
}

func (s *loggingScrollSession) granularity() float64 { return s.log.gran }

func (s *loggingScrollSession) inject(deltaX, deltaY float64) error {
	s.log.mu.Lock()
	defer s.log.mu.Unlock()

	s.log.chunks = append(
		s.log.chunks,
		loggedChunk{deltaX: deltaX, deltaY: deltaY, modifiers: s.modifiers},
	)

	return nil
}

func (s *loggingScrollSession) close() {}

// The shipped smooth_scroll defaults, which are what makes the truncation
// visible: the animation runs for max_duration (180ms) while held_repeat fires
// every 50ms, so most of each tick's travel is still unsent when the next tick
// arrives.
const (
	scrollTestSteps            = 20
	scrollTestMaxDuration      = 180
	scrollTestDurationPerPixel = 1.0
	scrollTestRepeatInterval   = 50 * time.Millisecond
	scrollTestHalfPage         = -500
)

// TestScrollAnimator_Animate_HeldRepeatTravelsAsFarAsDiscretePresses is the
// property the whole animation rests on, restated for a key held down:
// switching smooth scroll on changes how a held scroll key looks, never how far
// it goes. Each repeat tick preempts the animation still in flight, and before
// the undelivered remainder was composed into it, most of every tick's travel
// was thrown away. darwin's animator carries the same test.
func TestScrollAnimator_Animate_HeldRepeatTravelsAsFarAsDiscretePresses(t *testing.T) {
	const ticks = 10

	// A continuous backend (Wayland): granularity 0, so the chunks carry the
	// distance exactly and the total is the requested one rather than a whole
	// number of wheel notches.
	log := &scrollLog{}
	animator := newScrollAnimator(log.begin)

	t.Cleanup(animator.stop)

	for range ticks {
		animator.animate(
			0,
			scrollTestHalfPage,
			0,
			scrollTestSteps,
			scrollTestMaxDuration,
			scrollTestDurationPerPixel,
		)
		time.Sleep(scrollTestRepeatInterval)
	}

	want := float64(ticks * scrollTestHalfPage)

	// The last tick's animation is still draining; the run is only equal to the
	// same number of discrete presses once it finishes.
	waitFor(t, func() bool { return math.Abs(log.traveled()-want) < 1e-6 })

	if got := log.traveled(); math.Abs(got-want) > 1e-6 {
		t.Errorf("%d held-repeat ticks traveled %v, want %v (the same %d discrete presses)",
			ticks, got, want, ticks)
	}
}

// TestScrollAnimator_Animate_HeldRepeatOnAGranularBackend is the X11 half of
// the same property, and the one where it cannot be exact: a wheel button is
// one notch and there is no fraction of one to send, so each request's schedule
// leaves a sub-notch residue behind. A discrete press drops that residue too,
// which is what keeps the two comparable — the held run has to land within one
// notch per tick of the same number of presses, not merely somewhere short of
// them. Truncating each tick's remainder instead put it 40% short.
func TestScrollAnimator_Animate_HeldRepeatOnAGranularBackend(t *testing.T) {
	const (
		ticks = 10
		notch = scrollPixelsPerNotch
	)

	log := &scrollLog{gran: notch}
	animator := newScrollAnimator(log.begin)

	t.Cleanup(animator.stop)

	for range ticks {
		animator.animate(
			0,
			scrollTestHalfPage,
			0,
			scrollTestSteps,
			scrollTestMaxDuration,
			scrollTestDurationPerPixel,
		)
		time.Sleep(scrollTestRepeatInterval)
	}

	var (
		asked     = float64(ticks * -scrollTestHalfPage)
		tolerance = float64(ticks * notch)
	)

	// The last tick is still draining, so the floor is what has to be waited
	// for; the ceiling holds at every instant, since no schedule can send more
	// than was asked for.
	waitFor(t, func() bool { return -log.traveled() >= asked-tolerance })

	time.Sleep(60 * time.Millisecond)

	if got := -log.traveled(); got > asked {
		t.Errorf("%d held-repeat ticks traveled %v, more than the %v asked for",
			ticks, got, asked)
	}
}

// TestScrollAnimator_Animate_KeepsTheBacklogBounded covers the risk composition
// introduces: the animation (180ms) outlives the repeat interval (50ms), so
// undelivered delta accumulates across ticks. It has to self-limit — duration is
// capped at max_duration, so a larger pending delta drains over the same window
// and the delivery rate rises with the backlog. Without that a long hold would
// keep scrolling long after the key came up.
func TestScrollAnimator_Animate_KeepsTheBacklogBounded(t *testing.T) {
	const (
		ticks = 15
		// Steady state is ~0.6 of one tick's delta at these settings, so four
		// ticks' worth is loose enough to absorb scheduler jitter — and an
		// uncomposed backlog, which grows by ~0.4 of a tick's delta every tick,
		// still crosses it well inside the run.
		maxBacklog = 4 * -scrollTestHalfPage
	)

	log := &scrollLog{}
	animator := newScrollAnimator(log.begin)

	t.Cleanup(animator.stop)

	for tick := 1; tick <= ticks; tick++ {
		animator.animate(
			0,
			scrollTestHalfPage,
			0,
			scrollTestSteps,
			scrollTestMaxDuration,
			scrollTestDurationPerPixel,
		)
		time.Sleep(scrollTestRepeatInterval)

		backlog := -(float64(tick*scrollTestHalfPage) - log.traveled())
		if backlog > maxBacklog {
			t.Fatalf("after %d ticks %v pixels are still undelivered, more than the %d ceiling: "+
				"the backlog is not converging", tick, backlog, maxBacklog)
		}
	}
}

// TestScrollAnimator_Animate_DropsTheRemainderOnADifferentModifierSet pins the
// half of the rule that must not change: a plain scroll_down arriving mid-zoom
// cancels the zoom and finishes unmodified, which is what the second binding
// asked for. Carrying the zoom's undelivered distance into it would emit that
// distance as a plain scroll nobody asked for — and on Linux the modifier is a
// real key, so the two runs are two sessions.
func TestScrollAnimator_Animate_DropsTheRemainderOnADifferentModifierSet(t *testing.T) {
	const (
		zoom  = -4000
		plain = -100
	)

	log := &scrollLog{}
	animator := newScrollAnimator(log.begin)

	t.Cleanup(animator.stop)

	// Long enough that the preempting request certainly lands mid-animation.
	animator.animate(0, zoom, action.ModCtrl, 40, 2000, 1.0)

	waitFor(t, func() bool { return log.count() > 0 })

	animator.animate(0, plain, 0, scrollTestSteps, scrollTestMaxDuration, 1.0)

	waitFor(t, func() bool { return math.Abs(log.traveledWith(0)-plain) < 1e-6 })

	// Give the animator room to post more than it should before asserting.
	time.Sleep(50 * time.Millisecond)

	if got := log.traveledWith(0); math.Abs(got-plain) > 1e-6 {
		t.Errorf("the unmodified scroll traveled %v, want exactly %d — "+
			"the canceled zoom's remainder leaked into it", got, plain)
	}

	if got := log.traveledWith(action.ModCtrl); got == zoom {
		t.Errorf("the zoom traveled its full %v; the plain scroll was supposed to cancel it", got)
	}
}

// TestScrollAnimator_EnqueueLocked_ComposesAQueuedRequest covers the other seam
// a delta can be dropped at: a request the worker has not taken yet is replaced
// wholesale by the next one. None of it has been injected, so all of it folds
// into the replacement — under the same modifier rule the worker applies.
func TestScrollAnimator_EnqueueLocked_ComposesAQueuedRequest(t *testing.T) {
	tests := []struct {
		name      string
		queuedMod action.Modifiers
		nextMod   action.Modifiers
		wantY     float64
	}{
		{name: "same modifiers compose", queuedMod: 0, nextMod: 0, wantY: -80},
		{
			name:      "same non-zero modifiers compose",
			queuedMod: action.ModCtrl,
			nextMod:   action.ModCtrl,
			wantY:     -80,
		},
		{name: "different modifiers replace", queuedMod: action.ModCtrl, nextMod: 0, wantY: -50},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// No worker: nothing consumes the channel, so the test owns both
			// requests and this never runs.
			animator := newScrollAnimator(
				func(_ action.Modifiers) (scrollSession, error) {
					return &fakeScrollSession{}, nil
				},
			)
			animator.reqCh = make(chan scrollRequest, 1)

			animator.enqueueLocked(scrollRequest{deltaY: -30, modifiers: test.queuedMod})
			animator.enqueueLocked(scrollRequest{deltaY: -50, modifiers: test.nextMod})

			select {
			case got := <-animator.reqCh:
				if got.deltaY != test.wantY {
					t.Errorf("queued request deltaY = %v, want %v", got.deltaY, test.wantY)
				}

				if got.modifiers != test.nextMod {
					t.Errorf("queued request modifiers = %v, want the preempting %v",
						got.modifiers, test.nextMod)
				}
			default:
				t.Fatal("nothing was queued")
			}

			select {
			case extra := <-animator.reqCh:
				t.Fatalf("a second request (%v) survived the coalesce", extra.deltaY)
			default:
			}
		})
	}
}

// waitFor polls until the condition holds, failing rather than hanging.
func waitFor(t *testing.T, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		if condition() {
			return
		}

		time.Sleep(2 * time.Millisecond)
	}

	t.Fatal("the condition never held within 5s")
}
