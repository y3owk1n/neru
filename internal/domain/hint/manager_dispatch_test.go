package hint_test

import (
	"image"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/logger"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/hint"
)

// updateRecorder captures how the manager delivered each hint update. The
// distinction that matters is *when* onUpdate ran: immediateUpdate calls it
// synchronously inside HandleInput, debouncedUpdate schedules it on a timer.
type updateRecorder struct {
	mu sync.Mutex

	// synchronous counts callbacks that arrived while inCall was set.
	synchronous int
	// asynchronous counts callbacks that arrived after the call returned,
	// i.e. ones delivered by the debounce timer.
	asynchronous int
	inCall       bool
	// lastCount is the number of hints carried by the most recent callback.
	lastCount int
	async     chan struct{}
}

func newUpdateRecorder() *updateRecorder {
	return &updateRecorder{async: make(chan struct{}, 16)}
}

func (r *updateRecorder) callback(hints []*hint.Interface) {
	r.mu.Lock()
	r.lastCount = len(hints)

	if r.inCall {
		r.synchronous++
		r.mu.Unlock()

		return
	}

	r.asynchronous++
	r.mu.Unlock()

	// Only debounced deliveries are signaled, so a waiter can never be woken
	// by a stale token left behind by a synchronous callback.
	select {
	case r.async <- struct{}{}:
	default:
	}
}

// counts returns the recorded synchronous and asynchronous callback totals.
func (r *updateRecorder) counts() (int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.synchronous, r.asynchronous
}

// lastHintCount returns how many hints the most recent callback carried.
func (r *updateRecorder) lastHintCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.lastCount
}

// during runs fn with the synchronous window marked, so a callback delivered
// inside fn is distinguishable from one delivered by the debounce timer.
func (r *updateRecorder) during(fn func()) {
	r.mu.Lock()
	r.inCall = true
	r.mu.Unlock()

	fn()

	r.mu.Lock()
	r.inCall = false
	r.mu.Unlock()
}

// handleInput runs one keystroke with the recorder marking the synchronous
// window, so a callback delivered inside the call is distinguishable from one
// delivered by the debounce timer afterwards.
func (r *updateRecorder) handleInput(
	tb testing.TB,
	manager *hint.Manager,
	key string,
) (*hint.Interface, bool) {
	tb.Helper()

	var (
		match *hint.Interface
		found bool
		err   error
	)

	r.during(func() {
		match, found, err = manager.HandleInput(key)
	})

	if err != nil {
		tb.Fatalf("HandleInput(%q): %v", key, err)
	}

	return match, found
}

// backspace runs HandleBackspace inside the synchronous window.
func (r *updateRecorder) backspace(tb testing.TB, manager *hint.Manager) {
	tb.Helper()

	var err error

	r.during(func() {
		err = manager.HandleBackspace()
	})

	if err != nil {
		tb.Fatalf("HandleBackspace: %v", err)
	}
}

// newDispatchManager wires a manager over the given labels, with an external
// mutex (required by immediateUpdate) already held by the caller. The returned
// unlock function releases it.
func newDispatchManager(
	tb testing.TB,
	labels ...string,
) (*hint.Manager, *updateRecorder, *sync.Mutex) {
	tb.Helper()

	elem, err := element.NewElement(element.ID("1"), image.Rect(0, 0, 10, 10), element.RoleButton)
	if err != nil {
		tb.Fatalf("NewElement: %v", err)
	}

	interfaces := make([]*hint.Interface, 0, len(labels))

	for _, label := range labels {
		created, hintErr := hint.NewHint(label, elem, image.Point{})
		if hintErr != nil {
			tb.Fatalf("NewHint(%q): %v", label, hintErr)
		}

		interfaces = append(interfaces, created)
	}

	mut := &sync.Mutex{}
	manager := hint.NewManager(logger.Get(), mut)
	recorder := newUpdateRecorder()

	manager.SetUpdateCallback(recorder.callback)

	mut.Lock()

	recorder.during(func() {
		err = manager.SetHints(hint.NewCollection(interfaces))
	})

	if err != nil {
		mut.Unlock()
		tb.Fatalf("SetHints: %v", err)
	}

	return manager, recorder, mut
}

// TestManager_HandleInput_UpdateDispatchFollowsHintCount pins the heuristic
// that decides between a synchronous repaint and a debounced one: when the
// filtered hint count is unchanged only text colors move, so the update is
// immediate; when the count changes the overlay has to restructure, so the
// update is debounced to batch redraws.
//
// Getting this backwards is invisible to a count-only assertion — the callback
// still runs, just at the wrong time — which is why this test distinguishes
// callbacks delivered inside HandleInput from ones delivered by the timer.
func TestManager_HandleInput_UpdateDispatchFollowsHintCount(t *testing.T) {
	// AAA and AAB share the "AA" prefix, ABC diverges at the second character,
	// so typing A -> A narrows 3 -> 3 -> 2.
	manager, recorder, mut := newDispatchManager(t, "AAA", "AAB", "ABC")
	defer mut.Unlock()

	// SetHints itself delivers one update.
	syncBefore, asyncBefore := recorder.counts()

	// First "A" matches all three: the count is unchanged, so this must be an
	// immediate, synchronous repaint.
	if _, found := recorder.handleInput(t, manager, "A"); found {
		t.Fatal(`HandleInput("A") reported an exact match, want none`)
	}

	gotSync, gotAsync := recorder.counts()

	if gotSync != syncBefore+1 {
		t.Errorf(
			"unchanged hint count: got %d synchronous updates, want %d (the repaint must not be debounced)",
			gotSync-syncBefore,
			1,
		)
	}

	if gotAsync != asyncBefore {
		t.Errorf(
			"unchanged hint count delivered %d asynchronous updates, want 0",
			gotAsync-asyncBefore,
		)
	}

	// Second "A" narrows 3 -> 2: a structural change, so it must be debounced
	// rather than delivered inside the call.
	syncBefore, asyncBefore = recorder.counts()

	if _, found := recorder.handleInput(t, manager, "A"); found {
		t.Fatal(`HandleInput("AA") reported an exact match, want none`)
	}

	gotSync, _ = recorder.counts()

	if gotSync != syncBefore {
		t.Errorf(
			"changed hint count delivered %d synchronous updates, want 0 (the restructure must be debounced)",
			gotSync-syncBefore,
		)
	}

	// Release the external mutex so the debounce timer's callback can take it,
	// then wait for the deferred update to actually arrive.
	mut.Unlock()

	select {
	case <-recorder.async:
	case <-time.After(2 * time.Second):
		mut.Lock()
		t.Fatal("debounced update never fired")
	}

	mut.Lock()

	if _, gotAsync = recorder.counts(); gotAsync != asyncBefore+1 {
		t.Errorf("got %d asynchronous updates, want 1", gotAsync-asyncBefore)
	}

	// The deferred update must carry the narrowed set, not the pre-keystroke one.
	if got := recorder.lastHintCount(); got != 2 {
		t.Errorf("debounced update carried %d hints, want the narrowed set of 2", got)
	}
}

// TestManager_HandleInput_ExactMatchRequiresUniqueAndEqualLabel pins both
// halves of the exact-match condition. A hint is selected only when the input
// has narrowed to exactly one hint *and* that hint's label equals the input in
// full. Relaxing either half would fire a click on the wrong element, or on a
// still-ambiguous one.
func TestManager_HandleInput_ExactMatchRequiresUniqueAndEqualLabel(t *testing.T) {
	t.Run("unique hint whose label equals the input matches", func(t *testing.T) {
		manager, recorder, mut := newDispatchManager(t, "AA", "BB")
		defer mut.Unlock()

		recorder.handleInput(t, manager, "A")

		match, found := recorder.handleInput(t, manager, "A")
		if !found {
			t.Fatal("typing the full label of the only remaining hint did not match")
		}

		if match == nil || match.Label() != "AA" {
			t.Errorf("matched %v, want the hint labeled AA", match)
		}
	})

	t.Run("input equal to a label but still ambiguous does not match", func(t *testing.T) {
		// "AA" is a prefix of "AAB", so after typing "AA" two hints remain.
		// The first of them has the label "AA", which is exactly the input —
		// but the set has not been narrowed to one, so this must not select.
		manager, recorder, mut := newDispatchManager(t, "AA", "AAB")
		defer mut.Unlock()

		recorder.handleInput(t, manager, "A")

		match, found := recorder.handleInput(t, manager, "A")
		if found {
			t.Errorf(
				"input %q selected %v while another hint still shares that prefix; selection must be unambiguous",
				"AA",
				match,
			)
		}
	})

	t.Run("unique hint whose label is longer than the input does not match", func(t *testing.T) {
		// After "AA" only "AAB" remains — a unique hint, but the user has not
		// finished typing its label, so nothing may be selected yet.
		manager, recorder, mut := newDispatchManager(t, "AAB", "BBB")
		defer mut.Unlock()

		recorder.handleInput(t, manager, "A")

		match, found := recorder.handleInput(t, manager, "A")
		if found {
			t.Errorf(
				"input %q selected %v before its full label was typed",
				"AA", match,
			)
		}

		// Completing the label does select it.
		match, found = recorder.handleInput(t, manager, "B")
		if !found {
			t.Fatal("completing the label of the only remaining hint did not match")
		}

		if match == nil || match.Label() != "AAB" {
			t.Errorf("matched %v, want the hint labeled AAB", match)
		}
	})
}

// TestManager_HandleBackspace_OnEmptyInputResetsToFullSet covers the guard on
// the backspace path. Backspace is reachable before any character has been
// typed (the user opens hints and immediately presses it), where slicing the
// input would panic; the documented behavior is instead to reset and re-show
// the full hint set. Repeating it must stay stable rather than drifting.
func TestManager_HandleBackspace_OnEmptyInputResetsToFullSet(t *testing.T) {
	manager, recorder, mut := newDispatchManager(t, "AA", "AB", "AC")
	defer mut.Unlock()

	syncBefore, asyncBefore := recorder.counts()

	// Several times, to also cover backspacing past the start of the input.
	for range 3 {
		recorder.backspace(t, manager)

		if got := manager.CurrentInput(); got != "" {
			t.Fatalf("CurrentInput() = %q after backspacing empty input, want %q", got, "")
		}

		if got := recorder.lastHintCount(); got != 3 {
			t.Errorf("backspace on empty input re-showed %d hints, want the full set of 3", got)
		}
	}

	gotSync, gotAsync := recorder.counts()

	// Each reset delivers its update synchronously; none may be deferred to a
	// timer, or the overlay would briefly keep showing a stale filtered set.
	if gotSync != syncBefore+3 {
		t.Errorf("got %d synchronous updates from 3 backspaces, want 3", gotSync-syncBefore)
	}

	if gotAsync != asyncBefore {
		t.Errorf("backspace on empty input deferred %d updates, want 0", gotAsync-asyncBefore)
	}
}

// TestManager_HandleBackspace_RestoresWiderHintSet checks the inverse of the
// narrowing path: removing a character must widen the filtered set back out and
// clear the matched prefix, or the overlay keeps highlighting a prefix the user
// has already deleted.
func TestManager_HandleBackspace_RestoresWiderHintSet(t *testing.T) {
	manager, recorder, mut := newDispatchManager(t, "AAA", "AAB", "ABC")
	defer mut.Unlock()

	recorder.handleInput(t, manager, "A")
	recorder.handleInput(t, manager, "A")

	if got := manager.CurrentInput(); got != "AA" {
		t.Fatalf("CurrentInput() = %q, want %q", got, "AA")
	}

	// Backspacing "AA" -> "A" widens the filtered set from 2 back to 3. That is
	// a structural change, so — exactly as on the typing path — it must be
	// debounced rather than delivered inside the call.
	syncBefore, asyncBefore := recorder.counts()

	recorder.backspace(t, manager)

	if got := manager.CurrentInput(); got != "A" {
		t.Errorf("CurrentInput() = %q after one backspace, want %q", got, "A")
	}

	gotSync, _ := recorder.counts()

	if gotSync != syncBefore {
		t.Errorf(
			"widening backspace delivered %d synchronous updates, want 0 (the restructure must be debounced)",
			gotSync-syncBefore,
		)
	}

	mut.Unlock()

	select {
	case <-recorder.async:
	case <-time.After(2 * time.Second):
		mut.Lock()
		t.Fatal("debounced update never fired after a widening backspace")
	}

	mut.Lock()

	if _, gotAsync := recorder.counts(); gotAsync != asyncBefore+1 {
		t.Errorf("got %d asynchronous updates after backspace, want 1", gotAsync-asyncBefore)
	}

	if got := recorder.lastHintCount(); got != 3 {
		t.Errorf("backspace re-showed %d hints, want the widened set of 3", got)
	}

	// Backspacing "A" -> "" keeps the set at 3 (everything already matched the
	// single-character prefix), so this one is only a color repaint and must
	// be delivered immediately.
	syncBefore, _ = recorder.counts()

	recorder.backspace(t, manager)

	if got := manager.CurrentInput(); got != "" {
		t.Errorf("CurrentInput() = %q after two backspaces, want %q", got, "")
	}

	if gotSync, _ = recorder.counts(); gotSync != syncBefore+1 {
		t.Errorf(
			"unchanged-count backspace delivered %d synchronous updates, want 1",
			gotSync-syncBefore,
		)
	}
}
