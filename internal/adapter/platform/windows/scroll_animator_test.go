//go:build windows

package windows

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/domain/action"
)

type fakeScrollSession struct {
	mu     sync.Mutex
	chunks [][2]float64
	closed bool
}

func (s *fakeScrollSession) inject(deltaX, deltaY float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.chunks = append(s.chunks, [2]float64{deltaX, deltaY})

	return nil
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

func sumChunks(values []float64) float64 {
	var total float64

	for _, value := range values {
		total += value
	}

	return total
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		if condition() {
			return
		}

		time.Sleep(2 * time.Millisecond)
	}

	t.Fatal("condition not met before the deadline")
}

// TestScrollChunks_TravelsTheRequestedDistance is the property the whole
// animation rests on: switching smooth scroll on changes when a scroll
// arrives, never how far it goes. Every chunk is a whole number of 120ths, so
// the total is exact for any whole-notch delta.
func TestScrollChunks_TravelsTheRequestedDistance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		delta float64
		steps int
	}{
		{name: "one notch down", delta: -1, steps: 20},
		{name: "half a page up", delta: 500, steps: 20},
		{name: "a single step", delta: 300, steps: 1},
		{name: "more steps than notches", delta: 7, steps: 64},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			chunks := scrollChunks(test.delta, test.steps)

			if len(chunks) != test.steps {
				t.Fatalf("got %d chunks, want %d", len(chunks), test.steps)
			}

			if got := sumChunks(chunks); math.Abs(got-test.delta) > 1e-9 {
				t.Errorf("chunks total %v, want %v", got, test.delta)
			}
		})
	}
}

// TestScrollChunks_EasesOutInWholeWheelUnits pins the shape and the unit at
// once: the early chunks are the long ones, and every chunk converts to an
// integer mouseData, which is what lets the steps go below a notch without
// SendInput rounding them.
func TestScrollChunks_EasesOutInWholeWheelUnits(t *testing.T) {
	t.Parallel()

	chunks := scrollChunks(-10, 12)

	for index, chunk := range chunks {
		if units := chunk * wheelDelta; math.Abs(units-math.Round(units)) > 1e-9 {
			t.Errorf("chunk %d = %v notches, which is not a whole number of 120ths", index, chunk)
		}

		if index > 0 && math.Abs(chunk) > math.Abs(chunks[index-1])+1e-9 {
			t.Fatalf("chunk %d (%v) is longer than chunk %d (%v); the curve is not easing out",
				index, chunk, index-1, chunks[index-1])
		}
	}

	if chunks[0] >= 0 || chunks[0] <= -10 {
		t.Errorf("first chunk = %v, want a negative fraction of the -10 notch delta", chunks[0])
	}
}

func TestScrollAnimator_Animate_TravelsTheWholeDistance(t *testing.T) {
	t.Parallel()

	session := &fakeScrollSession{}
	animator := newScrollAnimator(
		func(_ action.Modifiers) (scrollSession, error) { return session, nil },
	)

	t.Cleanup(animator.stop)

	// Both axes at once: a scroll_right bound alongside a scroll_down has to
	// arrive as one diagonal animation, not as one axis lost to the other.
	animator.animate(120, -500, 0, 8, 40, 0.1)

	waitFor(t, session.isClosed)

	horizontal, vertical := session.traveled()
	if vertical != -500 {
		t.Errorf("the animation traveled %v vertically, want -500", vertical)
	}

	if horizontal != 120 {
		t.Errorf("the animation traveled %v horizontally, want 120", horizontal)
	}
}

// TestScrollAnimator_Animate_HoldsTheModifiersOfTheRequestThatIsRunning pins
// the reason a session exists: the modifier is a real key, pressed once
// before the animation and released once after it, not toggled around every
// chunk.
func TestScrollAnimator_Animate_HoldsTheModifiersOfTheRequestThatIsRunning(t *testing.T) {
	t.Parallel()

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

	waitFor(t, session.isClosed)

	guard.Lock()
	defer guard.Unlock()

	if opens != 1 {
		t.Errorf("the animation opened %d sessions, want exactly 1", opens)
	}

	if lastMod != action.ModCtrl {
		t.Errorf("the session was opened with %v, want ctrl", lastMod)
	}
}

// TestScrollAnimator_Stop_EndsTheAnimation is what the unanimated path relies
// on after a reload switches the animation off: no chunk lands after stop()
// returns, and the session's modifier hold is released.
func TestScrollAnimator_Stop_EndsTheAnimation(t *testing.T) {
	t.Parallel()

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

	waitFor(t, session.isClosed)

	_, before := session.traveled()

	time.Sleep(60 * time.Millisecond)

	if _, after := session.traveled(); after != before {
		t.Errorf("the animation traveled %v more after stop(); it should have ended", after-before)
	}
}

// TestScrollAnimator_Animate_HeldRepeatTravelsAsFarAsDiscretePresses keeps a
// held scroll key honest: each tick preempts the animation before it and
// absorbs its undelivered remainder, so the run lands exactly where the same
// number of discrete presses would.
func TestScrollAnimator_Animate_HeldRepeatTravelsAsFarAsDiscretePresses(t *testing.T) {
	t.Parallel()

	const (
		ticks    = 10
		halfPage = -500
	)

	session := &fakeScrollSession{}
	animator := newScrollAnimator(
		func(_ action.Modifiers) (scrollSession, error) { return session, nil },
	)

	t.Cleanup(animator.stop)

	for range ticks {
		animator.animate(0, halfPage, 0, 20, 200, 0.5)
		time.Sleep(30 * time.Millisecond)
	}

	want := float64(ticks * halfPage)

	waitFor(t, func() bool {
		_, vertical := session.traveled()

		return math.Abs(vertical-want) < 1e-6
	})
}

// TestScrollAnimator_Animate_DropsTheRemainderOnADifferentModifierSet is the
// deliberate cancel: a plain scroll arriving mid-zoom finishes unmodified and
// does not inherit the zoom's undelivered distance.
func TestScrollAnimator_Animate_DropsTheRemainderOnADifferentModifierSet(t *testing.T) {
	t.Parallel()

	next := composeUndelivered(
		scrollRequest{deltaY: -100},
		scrollRequest{deltaY: -1000, modifiers: action.ModCtrl},
		0, -700,
	)

	if next.deltaY != -100 {
		t.Errorf(
			"a plain scroll absorbed %v of a ctrl scroll's remainder, want none",
			next.deltaY+100,
		)
	}

	same := composeUndelivered(
		scrollRequest{deltaY: -100},
		scrollRequest{deltaY: -1000},
		0, -700,
	)

	if same.deltaY != -800 {
		t.Errorf("a same-modifier scroll carries %v, want -800", same.deltaY)
	}
}
