package heldmotion_test

import (
	"context"
	"image"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app/heldmotion"
	"github.com/y3owk1n/neru/internal/domain/motion"
	"github.com/y3owk1n/neru/internal/ports/mocks"
)

type recorder struct {
	mu     sync.Mutex
	points []image.Point
}

func (r *recorder) record(_ context.Context, point image.Point, bypass bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !bypass {
		panic("motion must bypass the jump animator")
	}

	r.points = append(r.points, point)

	return nil
}

func (r *recorder) last() (image.Point, int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.points) == 0 {
		return image.Point{}, 0
	}

	return r.points[len(r.points)-1], len(r.points)
}

func newController(t *testing.T, rec *recorder) *heldmotion.Controller {
	t.Helper()

	start := image.Point{X: 100, Y: 100}
	system := &mocks.MockSystemPort{
		CursorPositionFunc: func(context.Context) (image.Point, error) { return start, nil },
		ScreenNamesFunc:    func(context.Context) ([]string, error) { return nil, nil },
		ScreenBoundsFunc: func(context.Context) (image.Rectangle, error) {
			return image.Rect(0, 0, 1000, 1000), nil
		},
		MoveCursorToPointFunc: rec.record,
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// A 10 px step per 10 ms is 1000 px/s, and a multiplier of 1 never ramps.
	ramp := func() motion.Ramp {
		return motion.Ramp{Interval: 10 * time.Millisecond, Multiplier: 1}
	}

	return heldmotion.New(ctx, system, ramp, nil)
}

func TestGroup_Press_TwoKeysGlideDiagonallyUntilReleased(t *testing.T) {
	rec := &recorder{}
	keys := newController(t, rec).Group("test")

	keys.Press("l", motion.Direction{X: 1}, 10)
	keys.Press("j", motion.Direction{Y: 1}, 10)

	time.Sleep(100 * time.Millisecond)

	keys.Release("l")
	keys.Release("j")

	last, count := waitQuiet(t, rec)
	if count == 0 {
		t.Fatal("no cursor moves posted while keys were held")
	}

	if last.X <= 100 || last.Y <= 100 {
		t.Fatalf("last position %v, want down-right of the start", last)
	}

	if last.X != last.Y {
		t.Errorf("diagonal drifted: %v", last)
	}
}

func TestGroup_Release_OtherGroupsKeysStayHeld(t *testing.T) {
	rec := &recorder{}
	ctrl := newController(t, rec)
	modes, hotkeys := ctrl.Group("modes"), ctrl.Group("hotkeys")

	modes.Press("l", motion.Direction{X: 1}, 10)
	hotkeys.Press("l", motion.Direction{X: 1}, 10)

	modes.ReleaseAll()

	if !hotkeys.IsHeld("l") {
		t.Fatal("releasing one group's keys released another's")
	}

	if modes.IsHeld("l") {
		t.Fatal("ReleaseAll left the group's own key held")
	}

	if hotkeys.Release("j") {
		t.Error("releasing a key never pressed reported held")
	}

	hotkeys.ReleaseAll()
}

func TestGroup_NilIsInert(t *testing.T) {
	var keys *heldmotion.Group

	keys.Press("l", motion.Direction{X: 1}, 10)

	if keys.IsHeld("l") || keys.Release("l") {
		t.Error("nil group reported a held key")
	}

	keys.ReleaseAll()
}

// TestGroup_Press_AfterConcurrentChurnRestartsTheLoop pins the handoff the
// loop relies on: a press racing the last release either joins the running
// loop or starts the next one. Under -race it also covers the held-set
// bookkeeping from several goroutines at once.
func TestGroup_Press_AfterConcurrentChurnRestartsTheLoop(t *testing.T) {
	rec := &recorder{}
	keys := newController(t, rec).Group("test")

	var waitGroup sync.WaitGroup

	for worker := range 8 {
		waitGroup.Go(func() {
			key := string(rune('a' + worker))

			for range 200 {
				keys.Press(key, motion.Direction{X: 1}, 10)
				keys.Release(key)
			}
		})
	}

	waitGroup.Wait()

	if keys.IsHeld("a") {
		t.Fatal("a released key is still held")
	}

	_, before := waitQuiet(t, rec)

	keys.Press("l", motion.Direction{X: 1}, 10)

	deadline := time.Now().Add(time.Second)

	for {
		if _, after := rec.last(); after > before {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("cursor did not move after a fresh press following the churn")
		}

		time.Sleep(heldmotion.TickInterval)
	}

	keys.Release("l")
}

// waitQuiet blocks until the recorder has seen no new post across a window of
// several ticks, then returns the last post and the count. It fails the test
// when the loop is still posting a second later.
func waitQuiet(t *testing.T, rec *recorder) (image.Point, int) {
	t.Helper()

	deadline := time.Now().Add(time.Second)

	for {
		_, before := rec.last()

		time.Sleep(4 * heldmotion.TickInterval)

		last, after := rec.last()
		if after == before {
			return last, after
		}

		if time.Now().After(deadline) {
			t.Fatalf("cursor still moving after release: %d posts, then %d", before, after)
		}
	}
}
