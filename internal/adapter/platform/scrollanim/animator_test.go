package scrollanim

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// testNotch and testMaxUnits are the Linux X11 notch and per-request cap, so
// the granular cases read the way that backend drives the animator.
const (
	testNotch    = 30
	testMaxUnits = 50
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

func (s *fakeScrollSession) Granularity() float64 { return s.gran }

func (s *fakeScrollSession) Inject(deltaX, deltaY float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.chunks = append(s.chunks, [2]float64{deltaX, deltaY})

	return s.injectErr
}

func (s *fakeScrollSession) Close() {
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
// animation rests on. Switching smooth scroll on changes when a scroll arrives,
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

// TestScrollChunks_EasesOut pins the shape rather than the endpoint. The early
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

// TestScrollChunks_KeepsAGranularBackendOnWholeUnits is the X11 half. A wheel
// button is one notch and there is no fraction of one to send, so every chunk
// has to be a whole notch and the remainder has to survive into a later chunk
// instead of being rounded away step by step.
func TestScrollChunks_KeepsAGranularBackendOnWholeUnits(t *testing.T) {
	const notch = testNotch

	chunks := scrollChunks(-500, 20, notch, testMaxUnits)

	for i, chunk := range chunks {
		if math.Mod(chunk, notch) != 0 {
			t.Errorf("chunk %d = %v, which is not a whole multiple of %v", i, chunk, notch)
		}
	}

	// The unanimated X11 path sends 500 px rounded to the nearest notch, 17.
	// Rounding per step would lose several.
	if got, want := sum(chunks), -17*float64(notch); got != want {
		t.Errorf("chunks total %v, want %v (17 notches)", got, want)
	}
}

// TestScrollChunks_SendsAOneUnitScrollImmediately covers the case that is not
// an animation at all: a scroll that rounds to a single wheel notch on X11.
// One event goes out either way, so scheduling it anywhere but the first step
// would deliver the same scroll later, which adds latency and nothing else.
func TestScrollChunks_SendsAOneUnitScrollImmediately(t *testing.T) {
	const notch = testNotch

	// Shorter than a notch, exactly a notch, and the last step that still
	// rounds down to one.
	for _, delta := range []float64{10, -10, 1, -1, notch, -notch, 44, -44} {
		chunks := scrollChunks(delta, 20, notch, testMaxUnits)

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
// path already applies, since scroll_step_full is a million pixels and
// thirty-three thousand button clicks is not an animation.
func TestScrollChunks_CapsAGranularBackend(t *testing.T) {
	const notch = testNotch

	chunks := scrollChunks(1000000, 20, notch, testMaxUnits)

	if got, want := sum(chunks), float64(testMaxUnits*notch); got != want {
		t.Errorf("chunks total %v, want the %d-notch ceiling (%v)",
			got, testMaxUnits, want)
	}

	// A continuous backend has no ceiling. Capping it would make a modified
	// go_bottom travel a different distance from an unmodified one.
	if got := sum(scrollChunks(1000000, 20, 0, testMaxUnits)); got != 1000000 {
		t.Errorf("continuous chunks total %v, want the full 1000000", got)
	}
}

// TestAnimator_Animate_TravelsTheWholeDistance is the end-to-end of the
// scheduler against a stand-in backend. Every chunk reaches it, and the session
// is closed when the animation ends.
func TestAnimator_Animate_TravelsTheWholeDistance(t *testing.T) {
	session := &fakeScrollSession{}
	animator := New(
		func(_ action.Modifiers) (Session, error) { return session, nil },
		testMaxUnits,
	)

	t.Cleanup(animator.Stop)

	// Both axes at once. A scroll_right bound alongside a scroll_down has to
	// arrive as one diagonal animation, not as one axis lost to the other.
	animator.Animate(120, -500, 0, 8, 40, 0.1)

	waitFor(t, func() bool { return session.isClosed() })

	horizontal, vertical := session.traveled()
	if vertical != -500 {
		t.Errorf("the animation traveled %v vertically, want -500", vertical)
	}

	if horizontal != 120 {
		t.Errorf("the animation traveled %v horizontally, want 120", horizontal)
	}
}

// TestAnimator_Animate_HoldsTheModifiersOfTheRequestThatIsRunning pins
// the reason a session exists at all. The modifier is a real key, pressed once
// before the animation and released once after it, not toggled around every
// chunk.
func TestAnimator_Animate_HoldsTheModifiersOfTheRequestThatIsRunning(t *testing.T) {
	session := &fakeScrollSession{}

	var (
		guard   sync.Mutex
		opens   int
		lastMod action.Modifiers
	)

	animator := New(func(modifiers action.Modifiers) (Session, error) {
		guard.Lock()
		opens++
		lastMod = modifiers
		guard.Unlock()

		return session, nil
	}, testMaxUnits)

	t.Cleanup(animator.Stop)

	animator.Animate(0, -300, action.ModCtrl, 6, 30, 0.1)

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

// TestAnimator_Animate_StopsOnABackendThatDiesMidAnimation covers the
// backend that passed the caller's availability check and then broke, such
// as a dropped KDE portal grant or a lost compositor connection. There is no caller
// left to return the error to, so the animation ends rather than spending its
// whole duration failing once per step. The loud refusal belongs to
// the caller's backend check, which runs before any of this.
func TestAnimator_Animate_StopsOnABackendThatDiesMidAnimation(t *testing.T) {
	var (
		guard sync.Mutex
		tries int
	)

	animator := New(func(_ action.Modifiers) (Session, error) {
		guard.Lock()
		tries++
		guard.Unlock()

		return nil, derrors.New(derrors.CodeNotSupported, "no backend")
	}, testMaxUnits)

	t.Cleanup(animator.Stop)

	animator.Animate(0, -300, 0, 6, 30, 0.1)

	waitFor(t, func() bool {
		guard.Lock()
		defer guard.Unlock()

		return tries == 1
	})
}

// TestAnimator_Animate_IgnoresAnEmptyScroll keeps a zero delta from
// opening a session, which on X11 is a display connection and a modifier press
// for no movement at all.
func TestAnimator_Animate_IgnoresAnEmptyScroll(t *testing.T) {
	var (
		guard sync.Mutex
		opens int
	)

	animator := New(func(_ action.Modifiers) (Session, error) {
		guard.Lock()
		opens++
		guard.Unlock()

		return &fakeScrollSession{}, nil
	}, testMaxUnits)

	t.Cleanup(animator.Stop)

	animator.Animate(0, 0, 0, 6, 30, 0.1)

	time.Sleep(50 * time.Millisecond)

	guard.Lock()
	defer guard.Unlock()

	if opens != 0 {
		t.Errorf("a zero-delta scroll opened %d sessions, want none", opens)
	}
}

// TestAnimator_Stop_EndsTheAnimation covers the reload case. Smooth
// scroll is switched off, and the scroll that arrives next must not be chased
// by chunks scheduled before it.
func TestAnimator_Stop_EndsTheAnimation(t *testing.T) {
	session := &fakeScrollSession{}
	animator := New(
		func(_ action.Modifiers) (Session, error) { return session, nil },
		testMaxUnits,
	)

	// Long enough that Stop lands mid-animation.
	animator.Animate(0, -1000, 0, 40, 400, 1.0)

	waitFor(t, func() bool {
		session.mu.Lock()
		defer session.mu.Unlock()

		return len(session.chunks) > 0
	})

	animator.Stop()

	waitFor(t, func() bool { return session.isClosed() })

	_, before := session.traveled()

	time.Sleep(60 * time.Millisecond)

	if _, after := session.traveled(); after != before {
		t.Errorf("the animation traveled %v more after Stop; it should have ended", after-before)
	}
}

// TestAnimator_Animate_AbandonsAnAnimationOnTheFirstFailedChunk keeps a
// dead backend from costing the whole animation duration. The session is closed
// on the first refusal instead of the remaining steps being played out against
// it.
func TestAnimator_Animate_AbandonsAnAnimationOnTheFirstFailedChunk(t *testing.T) {
	session := &fakeScrollSession{injectErr: derrors.New(derrors.CodeActionFailed, "gone")}
	animator := New(
		func(_ action.Modifiers) (Session, error) { return session, nil },
		testMaxUnits,
	)

	t.Cleanup(animator.Stop)

	animator.Animate(0, -1000, 0, 20, 400, 1.0)

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

func (l *scrollLog) begin(modifiers action.Modifiers) (Session, error) {
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

func (s *loggingScrollSession) Granularity() float64 { return s.log.gran }

func (s *loggingScrollSession) Inject(deltaX, deltaY float64) error {
	s.log.mu.Lock()
	defer s.log.mu.Unlock()

	s.log.chunks = append(
		s.log.chunks,
		loggedChunk{deltaX: deltaX, deltaY: deltaY, modifiers: s.modifiers},
	)

	return nil
}

func (s *loggingScrollSession) Close() {}

// The shipped smooth_scroll defaults, which are what makes the truncation
// visible. The animation runs for max_duration (180ms) while held_repeat fires
// every 50ms, so most of each tick's travel is still unsent when the next tick
// arrives.
const (
	scrollTestSteps            = 20
	scrollTestMaxDuration      = 180
	scrollTestDurationPerPixel = 1.0
	scrollTestRepeatInterval   = 50 * time.Millisecond
	scrollTestHalfPage         = -500
)

// TestAnimator_Animate_HeldRepeatTravelsAsFarAsDiscretePresses is the
// property the whole animation rests on, restated for a key held down.
// Switching smooth scroll on changes how a held scroll key looks, never how far
// it goes. Each repeat tick preempts the animation still in flight, and before
// the undelivered remainder was composed into it, most of every tick's travel
// was thrown away. darwin's animator carries the same test.
func TestAnimator_Animate_HeldRepeatTravelsAsFarAsDiscretePresses(t *testing.T) {
	const ticks = 10

	// A continuous backend (Wayland) has granularity 0, so the chunks carry the
	// distance exactly and the total is the requested one rather than a whole
	// number of wheel notches.
	log := &scrollLog{}
	animator := New(log.begin, testMaxUnits)

	t.Cleanup(animator.Stop)

	for range ticks {
		animator.Animate(
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

	// The last tick's animation is still draining. The run is only equal to the
	// same number of discrete presses once it finishes.
	waitFor(t, func() bool { return math.Abs(log.traveled()-want) < 1e-6 })

	if got := log.traveled(); math.Abs(got-want) > 1e-6 {
		t.Errorf("%d held-repeat ticks traveled %v, want %v (the same %d discrete presses)",
			ticks, got, want, ticks)
	}
}

// TestAnimator_Animate_HeldRepeatOnAGranularBackend is the X11 half of
// the same property, and the one where it cannot be exact. A wheel button is
// one notch and there is no fraction of one to send, so each request's schedule
// leaves a sub-notch residue behind. A discrete press drops that residue too,
// which is what keeps the two comparable. The held run has to land within one
// notch per tick of the same number of presses, not merely somewhere short of
// them. Truncating each tick's remainder instead put it 40% short.
func TestAnimator_Animate_HeldRepeatOnAGranularBackend(t *testing.T) {
	const (
		ticks = 10
		notch = testNotch
	)

	log := &scrollLog{gran: notch}
	animator := New(log.begin, testMaxUnits)

	t.Cleanup(animator.Stop)

	for range ticks {
		animator.Animate(
			0,
			scrollTestHalfPage,
			0,
			scrollTestSteps,
			scrollTestMaxDuration,
			scrollTestDurationPerPixel,
		)
		time.Sleep(scrollTestRepeatInterval)
	}

	// What ten discrete presses send. Each rounds to the nearest notch, and a
	// half page of 500 px is 17 of them.
	const halfPageNotches = 17

	var (
		asked     = float64(ticks * halfPageNotches * notch)
		tolerance = float64(ticks * notch)
	)

	// The last tick is still draining, so the floor is what has to be waited
	// for. The ceiling holds at every instant, since no schedule can send more
	// notches than the presses would.
	waitFor(t, func() bool { return -log.traveled() >= asked-tolerance })

	time.Sleep(60 * time.Millisecond)

	if got := -log.traveled(); got > asked {
		t.Errorf("%d held-repeat ticks traveled %v, more than the %v asked for",
			ticks, got, asked)
	}
}

// TestAnimator_Animate_KeepsTheBacklogBounded covers the risk composition
// introduces. The animation (180ms) outlives the repeat interval (50ms), so
// undelivered delta accumulates across ticks. It has to self-limit. Duration is
// capped at max_duration, so a larger pending delta drains over the same window
// and the delivery rate rises with the backlog. Without that a long hold would
// keep scrolling long after the key came up.
func TestAnimator_Animate_KeepsTheBacklogBounded(t *testing.T) {
	const (
		ticks = 15
		// Steady state is ~0.6 of one tick's delta at these settings, so four
		// ticks' worth is loose enough to absorb scheduler jitter, and an
		// uncomposed backlog, which grows by ~0.4 of a tick's delta every tick,
		// still crosses it well inside the run.
		maxBacklog = 4 * -scrollTestHalfPage
	)

	log := &scrollLog{}
	animator := New(log.begin, testMaxUnits)

	t.Cleanup(animator.Stop)

	for tick := 1; tick <= ticks; tick++ {
		animator.Animate(
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

// TestAnimator_Animate_DropsTheRemainderOnADifferentModifierSet pins the
// half of the rule that must not change. A plain scroll_down arriving mid-zoom
// cancels the zoom and finishes unmodified, which is what the second binding
// asked for. Carrying the zoom's undelivered distance into it would emit that
// distance as a plain scroll nobody asked for. On Linux the modifier is a
// real key, so the two runs are two sessions.
func TestAnimator_Animate_DropsTheRemainderOnADifferentModifierSet(t *testing.T) {
	const (
		zoom  = -4000
		plain = -100
	)

	log := &scrollLog{}
	animator := New(log.begin, testMaxUnits)

	t.Cleanup(animator.Stop)

	// Long enough that the preempting request certainly lands mid-animation.
	animator.Animate(0, zoom, action.ModCtrl, 40, 2000, 1.0)

	waitFor(t, func() bool { return log.count() > 0 })

	animator.Animate(0, plain, 0, scrollTestSteps, scrollTestMaxDuration, 1.0)

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

// TestAnimator_Animate_AReversalNetsTheTwoDeltas is the same rule read
// backwards, and the case composition is most often suspected of breaking. A
// scroll_up landing mid-scroll_down inherits a *negative* remainder, so the new
// animation travels less than its own delta, or not at all.
//
// That is the point. The pair asked for -600 then +600, which is a net zero,
// and the first animation has already sent part of its -600. Ignoring the
// remainder would inject the whole +600 on top of what went out and leave the
// page above where it started, which is the same truncation bug facing the
// other way. Conserving the requested total is what makes a reversal land where
// the two presses said. darwin's animator carries the same test.
func TestAnimator_Animate_AReversalNetsTheTwoDeltas(t *testing.T) {
	const stride = 600

	log := &scrollLog{}
	animator := New(log.begin, testMaxUnits)

	t.Cleanup(animator.Stop)

	// Long enough that the reversal certainly lands mid-animation.
	animator.Animate(0, -stride, 0, 40, 2000, 1.0)

	waitFor(t, func() bool { return log.count() > 0 })

	if got := log.traveled(); got >= 0 {
		t.Fatalf("the first animation traveled %v before the reversal, want some downward travel",
			got)
	}

	animator.Animate(0, stride, 0, 40, 2000, 1.0)

	waitFor(t, func() bool { return math.Abs(log.traveled()) < 1e-6 })

	// Give the animator room to overshoot before believing the zero.
	time.Sleep(80 * time.Millisecond)

	if got := log.traveled(); math.Abs(got) > 1e-6 {
		t.Errorf("a scroll of -%d reversed by +%d netted %v, want 0", stride, stride, got)
	}
}

// TestAnimator_EnqueueLocked_ComposesAQueuedRequest covers the other seam
// a delta can be dropped at. A request the worker has not taken yet is replaced
// wholesale by the next one. None of it has been injected, so all of it folds
// into the replacement, under the same modifier rule the worker applies.
func TestAnimator_EnqueueLocked_ComposesAQueuedRequest(t *testing.T) {
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
			// With no worker, nothing consumes the channel, so the test owns both
			// requests and this never runs.
			animator := New(
				func(_ action.Modifiers) (Session, error) {
					return &fakeScrollSession{}, nil
				},
				testMaxUnits,
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

// testWheelUnit is the Windows granularity, a quarter pixel: WHEEL_DELTA is
// 120 per notch and a notch is 30 pixels. Windows drives the animator with no
// per-request cap.
const testWheelUnit = 0.25

// TestScrollChunks_TravelsTheRequestedDistanceInWheelUnits is the Windows half
// of the distance property. Every chunk is a whole number of wheel units, so
// the total is exact for any whole-pixel delta, a single pixel included.
func TestScrollChunks_TravelsTheRequestedDistanceInWheelUnits(t *testing.T) {
	tests := []struct {
		name  string
		delta float64
		steps int
	}{
		{name: "one line down", delta: -50, steps: 20},
		{name: "half a page up", delta: 500, steps: 20},
		{name: "a single step", delta: 300, steps: 1},
		{name: "more steps than pixels", delta: 7, steps: 64},
		{name: "a single pixel", delta: 1, steps: 12},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chunks := scrollChunks(test.delta, test.steps, testWheelUnit, 0)

			if len(chunks) != test.steps {
				t.Fatalf("got %d chunks, want %d", len(chunks), test.steps)
			}

			if got := sum(chunks); math.Abs(got-test.delta) > 1e-9 {
				t.Errorf("chunks total %v, want %v", got, test.delta)
			}
		})
	}
}

// TestScrollChunks_EasesOutInWholeWheelUnits pins the shape and the unit at
// once. The early chunks are the long ones, and every chunk is a whole number
// of wheel units, which is what lets the steps go below a notch without
// SendInput rounding them.
func TestScrollChunks_EasesOutInWholeWheelUnits(t *testing.T) {
	chunks := scrollChunks(-10, 12, testWheelUnit, 0)

	for index, chunk := range chunks {
		if units := chunk / testWheelUnit; math.Abs(units-math.Round(units)) > 1e-9 {
			t.Errorf("chunk %d = %v pixels, which is not a whole number of wheel units",
				index, chunk)
		}

		// The last chunk carries whatever truncation left over, which can be
		// one unit after a run of zeros. The curve is read from the rest.
		if index > 0 && index < len(chunks)-1 &&
			math.Abs(chunk) > math.Abs(chunks[index-1])+1e-9 {
			t.Fatalf("chunk %d (%v) is longer than chunk %d (%v); the curve is not easing out",
				index, chunk, index-1, chunks[index-1])
		}
	}

	if chunks[0] >= 0 || chunks[0] <= -10 {
		t.Errorf("first chunk = %v, want a negative fraction of the -10 pixel delta", chunks[0])
	}
}

// TestAnimator_Stop_OpensNoSessionForARequestAlreadyTaken pins the stop check
// around opening a session. A request the worker took just before Stop must
// not open one, because opening presses real modifier keys that the direct
// scroll replacing it would then have to wait out.
func TestAnimator_Stop_OpensNoSessionForARequestAlreadyTaken(t *testing.T) {
	var (
		guard sync.Mutex
		opens int
	)

	animator := New(func(_ action.Modifiers) (Session, error) {
		guard.Lock()
		opens++
		guard.Unlock()

		return &fakeScrollSession{}, nil
	}, testMaxUnits)

	// Hold the fence, so the worker takes the request and then waits on it
	// before opening a session.
	animator.injectSem <- struct{}{}

	animator.Animate(0, -300, action.ModCtrl, 6, 30, 0.1)

	waitFor(t, func() bool {
		animator.mu.Lock()
		defer animator.mu.Unlock()

		return animator.reqCh != nil && len(animator.reqCh) == 0
	})

	stopped := make(chan struct{})

	go func() {
		animator.Stop()
		close(stopped)
	}()

	waitFor(t, func() bool {
		animator.mu.Lock()
		defer animator.mu.Unlock()

		return animator.stopCh == nil
	})

	<-animator.injectSem

	<-stopped

	time.Sleep(20 * time.Millisecond)

	guard.Lock()
	defer guard.Unlock()

	if opens != 0 {
		t.Errorf("a request taken before Stop opened %d sessions, want none", opens)
	}
}
