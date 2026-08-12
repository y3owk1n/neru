//go:build darwin

package darwin

import (
	"image"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/domain/action"
)

// scrollChunk is one posted scroll, with the modifier set it was stamped with.
type scrollChunk struct {
	deltaX, deltaY int
	modifiers      action.Modifiers
}

// scrollRecorder stands in for the window server: it captures every chunk the
// animator posts so a test can total the distance actually traveled.
type scrollRecorder struct {
	mu     sync.Mutex
	chunks []scrollChunk
}

func (r *scrollRecorder) pos() image.Point { return image.Point{X: 100, Y: 100} }

func (r *scrollRecorder) post(_ image.Point, deltaX, deltaY int, modifiers action.Modifiers) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.chunks = append(r.chunks, scrollChunk{deltaX: deltaX, deltaY: deltaY, modifiers: modifiers})
}

// traveledWith totals the vertical distance posted under exactly modifiers.
func (r *scrollRecorder) traveledWith(modifiers action.Modifiers) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	var total int

	for _, chunk := range r.chunks {
		if chunk.modifiers == modifiers {
			total += chunk.deltaY
		}
	}

	return total
}

// traveled totals the vertical distance posted, whatever the modifiers.
func (r *scrollRecorder) traveled() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	var total int

	for _, chunk := range r.chunks {
		total += chunk.deltaY
	}

	return total
}

func (r *scrollRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.chunks)
}

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

// waitForScroll polls until the condition holds, failing rather than hanging.
func waitForScroll(t *testing.T, condition func() bool) {
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

// TestScrollAnimator_Animate_HeldRepeatTravelsAsFarAsDiscretePresses is the
// property the whole fix rests on: switching smooth scroll on changes how a
// held scroll key looks, never how far it goes. Each repeat tick preempts the
// animation still in flight, and before the remainder was composed in, roughly
// 70% of every tick's travel was thrown away.
func TestScrollAnimator_Animate_HeldRepeatTravelsAsFarAsDiscretePresses(t *testing.T) {
	const ticks = 10

	rec := &scrollRecorder{}
	animator := newScrollAnimator(rec.pos, rec.post)

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

	want := ticks * scrollTestHalfPage

	// The last tick's animation is still draining; the run is only equal to the
	// same number of discrete presses once it finishes.
	waitForScroll(t, func() bool { return rec.traveled() == want })

	if got := rec.traveled(); got != want {
		t.Errorf("%d held-repeat ticks traveled %d, want %d (the same %d discrete presses)",
			ticks, got, want, ticks)
	}
}

// TestScrollAnimator_Animate_KeepsTheBacklogBounded covers the risk composition
// introduces: the animation (180ms) outlives the repeat interval (50ms), so
// undelivered delta accumulates across ticks. It has to self-limit — duration is
// capped at max_duration, so a larger pending delta drains over the same window
// and the delivery rate rises with the backlog — and that convergence is worth a
// test rather than an assumption. Without it a long hold would keep scrolling
// long after the key came up.
func TestScrollAnimator_Animate_KeepsTheBacklogBounded(t *testing.T) {
	const (
		ticks = 15
		// Steady state is ~0.6 of one tick's delta at these settings, so four
		// ticks' worth is loose enough to absorb scheduler jitter — and an
		// uncomposed backlog, which grows by ~0.4 of a tick's delta every tick,
		// still crosses it well inside the run.
		maxBacklog = 4 * -scrollTestHalfPage
	)

	rec := &scrollRecorder{}
	animator := newScrollAnimator(rec.pos, rec.post)

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

		backlog := -(tick*scrollTestHalfPage - rec.traveled())
		if backlog > maxBacklog {
			t.Fatalf("after %d ticks %d pixels are still undelivered, more than the %d ceiling: "+
				"the backlog is not converging", tick, backlog, maxBacklog)
		}
	}
}

// TestScrollAnimator_Animate_DropsTheRemainderOnADifferentModifierSet pins the
// half of the rule that must not change: a plain scroll_down arriving mid-zoom
// cancels the zoom and finishes unmodified, which is what the second binding
// asked for. Carrying the zoom's undelivered distance into it would emit that
// distance as a plain scroll nobody asked for.
func TestScrollAnimator_Animate_DropsTheRemainderOnADifferentModifierSet(t *testing.T) {
	const (
		zoom  = -4000
		plain = -100
	)

	rec := &scrollRecorder{}
	animator := newScrollAnimator(rec.pos, rec.post)

	t.Cleanup(animator.stop)

	// Long enough that the preempting request certainly lands mid-animation.
	animator.animate(0, zoom, action.ModCtrl, 40, 2000, 1.0)

	waitForScroll(t, func() bool { return rec.count() > 0 })

	animator.animate(0, plain, 0, scrollTestSteps, scrollTestMaxDuration, 1.0)

	waitForScroll(t, func() bool { return rec.traveledWith(0) == plain })

	// Give the animator room to post more than it should before asserting.
	time.Sleep(50 * time.Millisecond)

	if got := rec.traveledWith(0); got != plain {
		t.Errorf("the unmodified scroll traveled %d, want exactly %d — "+
			"the canceled zoom's remainder leaked into it", got, plain)
	}

	if got := rec.traveledWith(action.ModCtrl); got == zoom {
		t.Errorf("the zoom traveled its full %d; the plain scroll was supposed to cancel it", got)
	}
}

// TestScrollAnimator_EnqueueLocked_ComposesAQueuedRequest covers the other seam
// a delta can be dropped at: a request the worker has not taken yet is replaced
// wholesale by the next one. None of it has been delivered, so all of it folds
// into the replacement — under the same modifier rule the worker applies.
func TestScrollAnimator_EnqueueLocked_ComposesAQueuedRequest(t *testing.T) {
	tests := []struct {
		name      string
		queuedMod action.Modifiers
		nextMod   action.Modifiers
		wantY     int
	}{
		{name: "same modifiers compose", queuedMod: 0, nextMod: 0, wantY: -80},
		{
			name: "same non-zero modifiers compose", queuedMod: action.ModCtrl,
			nextMod: action.ModCtrl, wantY: -80,
		},
		{name: "different modifiers replace", queuedMod: action.ModCtrl, nextMod: 0, wantY: -50},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// No worker: nothing consumes the channel, so the test owns both
			// requests and no scroll is posted.
			animator := newScrollAnimator(
				func() image.Point { return image.Point{} },
				func(image.Point, int, int, action.Modifiers) {},
			)
			animator.reqCh = make(chan scrollRequest, 1)

			animator.enqueueLocked(scrollRequest{deltaY: -30, modifiers: test.queuedMod})
			animator.enqueueLocked(scrollRequest{deltaY: -50, modifiers: test.nextMod})

			select {
			case got := <-animator.reqCh:
				if got.deltaY != test.wantY {
					t.Errorf("queued request deltaY = %d, want %d", got.deltaY, test.wantY)
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
				t.Fatalf("a second request (%d) survived the coalesce", extra.deltaY)
			default:
			}
		})
	}
}
