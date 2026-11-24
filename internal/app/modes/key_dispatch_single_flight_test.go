package modes

import (
	"context"
	"image"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// testScrollDownStep is a bound action that held_repeat is willing to repeat,
// so a test can drive the repeat loop through the real binding path.
const testScrollDownStep = "action scroll_down"

// testSpaceKey is bound in more than one mode where a test checks that one
// mode's sequence does not silence another mode's binding.
const testSpaceKey = "Space"

// singleFlightTestWait bounds every wait in this file. A run is held open by the
// test itself, so the only real wait is the scheduler putting a dispatched
// goroutine on a core.
const singleFlightTestWait = 2 * time.Second

// newSingleFlightTestHandler builds a handler whose bindings come from cfg and
// whose executor is run, so a test can press keys through the whole dispatch
// path and watch what reaches the sequence runner.
func newSingleFlightTestHandler(
	cfg *configpkg.Config,
	run func(source string, steps []string),
) *Handler {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	return newHandlerWithState(handlerState{
		ctx:           context.Background(),
		config:        cfg,
		logger:        zap.NewNop(),
		appState:      appState,
		modifierState: state.NewModifierState(),
		modes: map[domain.Mode]Mode{
			domain.ModeGrid:          &recordingMode{keys: make(chan string, 1)},
			domain.ModeRecursiveGrid: &recordingMode{keys: make(chan string, 1)},
		},
		screenBounds: image.Rect(0, 0, 100, 100),
		executeActionSequence: func(source string, steps []string, _ []string) {
			run(source, steps)
		},
	})
}

// recursiveGridBindings is the config the single-binding tests share: one
// recursive-grid binding and held_repeat as the caller wants it.
func recursiveGridBindings(
	hotkeys map[string]configpkg.StringOrStringArray,
	heldRepeat configpkg.HeldRepeatConfig,
) *configpkg.Config {
	return &configpkg.Config{
		RecursiveGrid: configpkg.RecursiveGridConfig{Hotkeys: hotkeys},
		HeldRepeat:    heldRepeat,
	}
}

// waitForBindingDispatch waits for a binding's sequence to reach the executor, which
// runs on a goroutine of its own.
func waitForBindingDispatch(t *testing.T, started <-chan string, want string) {
	t.Helper()

	select {
	case got := <-started:
		if got != want {
			t.Fatalf("dispatched %q, want %q", got, want)
		}
	case <-time.After(singleFlightTestWait):
		t.Fatalf("timed out waiting for %q to run", want)
	}
}

// pressUntilDispatched presses key again and again until the binding reaches the
// executor. A press that lands while that binding's previous run is still going
// is dropped by design, so proving the binding answers again is done by asking
// again rather than by reading the guard: the test then passes or fails on the
// behavior, and survives the guard changing shape.
func pressUntilDispatched(
	t *testing.T,
	handler *Handler,
	key string,
	started <-chan string,
	want string,
) {
	t.Helper()

	deadline := time.Now().Add(singleFlightTestWait)

	for {
		handler.HandleKeyPress(key)

		select {
		case got := <-started:
			if got != want {
				t.Fatalf("dispatched %q, want %q", got, want)
			}

			return
		case <-time.After(10 * time.Millisecond):
		}

		if time.Now().After(deadline) {
			t.Fatalf("%q never ran again, want the binding to answer once its run ended", want)
		}
	}
}

// waitForCount waits until reads returns want, and fails the test on the value it
// settled on.
func waitForCount(t *testing.T, desc string, reads func() int32, want int32) {
	t.Helper()

	deadline := time.Now().Add(singleFlightTestWait)

	for {
		if got := reads(); got == want {
			return
		} else if time.Now().After(deadline) {
			t.Fatalf("%s: got %d, want %d", desc, got, want)
		}

		time.Sleep(time.Millisecond)
	}
}

// A key the kernel repeats, or a second tap landing inside a run, must not
// stack a second copy of the same binding on top of the first. The two copies
// share the cursor slot their steps save and restore through, so overlapping
// them is what walked the cursor away while a pan key was held.
func TestHandlerState_DispatchHotkeyActions_DropsARepeatOfABindingStillRunning(t *testing.T) {
	t.Parallel()

	started := make(chan string, 4)
	release := make(chan struct{})

	handler := newSingleFlightTestHandler(
		recursiveGridBindings(
			map[string]configpkg.StringOrStringArray{"j": {testScrollDownStep}},
			configpkg.HeldRepeatConfig{},
		),
		func(source string, _ []string) {
			started <- source

			<-release
		},
	)

	handler.HandleKeyPress("j")
	waitForBindingDispatch(t, started, "j")

	handler.HandleKeyPress("j")

	select {
	case got := <-started:
		t.Fatalf("a second press of %q ran while the first was still going", got)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)

	// With the run finished the binding answers again, so the drop is the
	// run's length and not the key being stuck forever.
	pressUntilDispatched(t, handler, "j", started, "j")
}

// One slow binding may not hold up other bindings. A user panning with one key
// still expects a click on another to land while the pan is running.
func TestHandlerState_DispatchHotkeyActions_RunsADifferentBindingWhileOneIsInFlight(t *testing.T) {
	t.Parallel()

	started := make(chan string, 4)
	release := make(chan struct{})

	handler := newSingleFlightTestHandler(
		recursiveGridBindings(
			map[string]configpkg.StringOrStringArray{
				"j": {testScrollDownStep},
				"k": {modeSteps},
			},
			configpkg.HeldRepeatConfig{},
		),
		func(source string, _ []string) {
			started <- source

			if source == "j" {
				<-release
			}
		},
	)

	handler.HandleKeyPress("j")
	waitForBindingDispatch(t, started, "j")

	handler.HandleKeyPress("k")
	waitForBindingDispatch(t, started, "k")

	close(release)
}

// The same key bound in two modes is two bindings, so a sequence that holds it
// in one mode leaves the other answering. wait_for_mode_exit is the case that
// makes it visible: a sequence waiting for hints to end can hold Space for as
// long as the hints session is up.
func TestHandlerState_DispatchHotkeyActions_SeparatesOneKeyAcrossModes(t *testing.T) {
	t.Parallel()

	var dispatched, live, peak atomic.Int32

	started := make(chan string, 4)
	release := make(chan struct{})

	cfg := &configpkg.Config{
		Grid: configpkg.GridConfig{
			Hotkeys: map[string]configpkg.StringOrStringArray{testSpaceKey: {testScrollDownStep}},
		},
		RecursiveGrid: configpkg.RecursiveGridConfig{
			Hotkeys: map[string]configpkg.StringOrStringArray{testSpaceKey: {modeSteps}},
		},
	}

	handler := newSingleFlightTestHandler(cfg, func(source string, _ []string) {
		running := live.Add(1)
		for {
			highest := peak.Load()
			if running <= highest || peak.CompareAndSwap(highest, running) {
				break
			}
		}

		dispatched.Add(1)

		started <- source

		<-release

		live.Add(-1)
	})

	handler.appState.SetMode(domain.ModeGrid)
	handler.HandleKeyPress(testSpaceKey)
	waitForBindingDispatch(t, started, testSpaceKey)

	// The grid run is still holding its slot. The same key in the other mode
	// is another binding, so this press runs rather than being dropped.
	handler.appState.SetMode(domain.ModeRecursiveGrid)
	handler.HandleKeyPress(testSpaceKey)
	waitForBindingDispatch(t, started, testSpaceKey)

	if got := peak.Load(); got != 2 {
		t.Fatalf("%d runs of one key across two modes overlapped, want 2", got)
	}

	close(release)

	waitForCount(t, "runs still in flight", live.Load, 0)

	if got := dispatched.Load(); got != 2 {
		t.Fatalf("the key ran %d time(s) across two modes, want 2", got)
	}
}

// A fed key is a real press from `neru action feed`, not the kernel repeating
// one, so it runs every time. A script feeds keys back to back, far faster than
// any sequence finishes, and dropping the tail of a feed would make
// `neru action feed --mode j j` scroll once instead of three times.
func TestHandler_HandleFedKeyPress_RunsEveryFedPressWhileOneIsRunning(t *testing.T) {
	t.Parallel()

	var ran, live atomic.Int32

	started := make(chan string, 4)
	release := make(chan struct{})

	handler := newSingleFlightTestHandler(
		recursiveGridBindings(
			map[string]configpkg.StringOrStringArray{"j": {testScrollDownStep}},
			configpkg.HeldRepeatConfig{},
		),
		func(source string, _ []string) {
			ran.Add(1)
			live.Add(1)

			started <- source

			<-release

			live.Add(-1)
		},
	)

	// Three presses with no wait between them, the way a script feeds them.
	for range 3 {
		handler.HandleFedKeyPress("j")
	}

	for range 3 {
		waitForBindingDispatch(t, started, "j")
	}

	if got := ran.Load(); got != 3 {
		t.Fatalf("three fed presses ran the binding %d time(s), want 3", got)
	}

	close(release)

	waitForCount(t, "runs still in flight", live.Load, 0)
}

// The slot is released after a panic too, or one bad step in one binding would
// leave that key dead for the rest of the session.
func TestHandlerState_RunSequence_ReleasesTheSlotAfterAPanic(t *testing.T) {
	t.Parallel()

	var panics atomic.Int32

	started := make(chan string, 2)

	handler := newSingleFlightTestHandler(
		recursiveGridBindings(
			map[string]configpkg.StringOrStringArray{"j": {testScrollDownStep}},
			configpkg.HeldRepeatConfig{},
		),
		func(source string, _ []string) {
			panics.Add(1)

			started <- source

			panic("a step blew up")
		},
	)

	handler.HandleKeyPress("j")
	waitForBindingDispatch(t, started, "j")

	if got := panics.Load(); got != 1 {
		t.Fatalf("the panicking sequence ran %d time(s), want 1", got)
	}

	// The slot comes back as the panic unwinds, after the count above was
	// observed, so a press landing inside that release window is dropped by
	// design. The second run is asked for until it lands rather than pressed
	// once and assumed.
	pressUntilDispatched(t, handler, "j", started, "j")

	if got := panics.Load(); got != 2 {
		t.Fatalf("the binding ran %d time(s) after one panic, want 2", got)
	}
}

// A held key repeats the binding, and the repeat never runs a second copy of it
// while the previous one is still going. A sequence slower than the interval
// then sets the cadence, and the repeats it outran are dropped rather than
// queued, so nothing is left running once the key comes up.
func TestHandler_HandleKeyPress_HeldRepeatNeverStacksTwoRunsOfOneBinding(t *testing.T) {
	t.Parallel()

	var live, peak, runs atomic.Int32

	handler := newSingleFlightTestHandler(
		recursiveGridBindings(
			map[string]configpkg.StringOrStringArray{"j": {testScrollDownStep}},
			configpkg.HeldRepeatConfig{
				Enabled:      true,
				InitialDelay: 1,
				Interval:     1,
			},
		),
		func(string, []string) {
			running := live.Add(1)
			for {
				highest := peak.Load()
				if running <= highest || peak.CompareAndSwap(highest, running) {
					break
				}
			}

			// Long enough that several repeats land while this one runs.
			time.Sleep(3 * time.Millisecond)

			live.Add(-1)
			runs.Add(1)
		},
	)

	handler.HandleKeyPress("j")
	time.Sleep(100 * time.Millisecond)
	handler.HandleKeyPress(keyUpPrefix + "j")

	// The key is up, so no tick is starting and the last one in flight has to
	// come back down.
	waitForCount(t, "runs still in flight after the key came up", live.Load, 0)

	if got := peak.Load(); got != 1 {
		t.Fatalf("%d runs of one binding overlapped, want 1 at a time", got)
	}

	// A repeat that never fires again would pass the check above by holding
	// the key down and doing nothing, so the repeat is pinned as well.
	if got := runs.Load(); got < 2 {
		t.Fatalf("a held key ran its binding %d time(s), want the repeat to fire again", got)
	}
}
