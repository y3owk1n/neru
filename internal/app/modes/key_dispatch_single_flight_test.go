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

// newSingleFlightTestHandler builds a handler in recursive-grid mode with the
// given bindings, so a test can press keys through the whole dispatch path.
func newSingleFlightTestHandler(
	hotkeys map[string]configpkg.StringOrStringArray,
	heldRepeat configpkg.HeldRepeatConfig,
	run func(source string, steps []string),
) *Handler {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	return newHandlerWithState(handlerState{
		ctx: context.Background(),
		config: &configpkg.Config{
			RecursiveGrid: configpkg.RecursiveGridConfig{Hotkeys: hotkeys},
			HeldRepeat:    heldRepeat,
		},
		logger:        zap.NewNop(),
		appState:      appState,
		modifierState: state.NewModifierState(),
		modes: map[domain.Mode]Mode{
			domain.ModeRecursiveGrid: &recordingMode{keys: make(chan string, 1)},
		},
		screenBounds: image.Rect(0, 0, 100, 100),
		executeActionSequence: func(source string, steps []string, _ []string) {
			run(source, steps)
		},
	})
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
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %q to run", want)
	}
}

// waitUntilSlotsFree waits until no binding's sequence is running.
func waitUntilSlotsFree(t *testing.T, handler *Handler) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for {
		handler.seqMu.Lock()
		running := len(handler.seqInFlight)
		handler.seqMu.Unlock()

		if running == 0 {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("%d binding(s) still in flight, want none", running)
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
		map[string]configpkg.StringOrStringArray{"j": {testScrollDownStep}},
		configpkg.HeldRepeatConfig{},
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
	waitUntilSlotsFree(t, handler)

	// With the run finished the binding answers again, so the drop is the
	// run's length and not the key being stuck forever.
	handler.HandleKeyPress("j")
	waitForBindingDispatch(t, started, "j")
}

// One slow binding may not hold up other bindings. A user panning with one key
// still expects a click on another to land while the pan is running.
func TestHandlerState_DispatchHotkeyActions_RunsADifferentBindingWhileOneIsInFlight(t *testing.T) {
	t.Parallel()

	started := make(chan string, 4)
	release := make(chan struct{})

	handler := newSingleFlightTestHandler(
		map[string]configpkg.StringOrStringArray{
			"j": {testScrollDownStep},
			"k": {"action left_click"},
		},
		configpkg.HeldRepeatConfig{},
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
	waitUntilSlotsFree(t, handler)
}

// The slot is released after a panic too, or one bad step in one binding would
// leave that key dead for the rest of the session.
func TestHandlerState_RunSequence_ReleasesTheSlotAfterAPanic(t *testing.T) {
	t.Parallel()

	var panics atomic.Int32

	handler := newSingleFlightTestHandler(
		map[string]configpkg.StringOrStringArray{"j": {testScrollDownStep}},
		configpkg.HeldRepeatConfig{},
		func(string, []string) {
			panics.Add(1)

			panic("a step blew up")
		},
	)

	handler.HandleKeyPress("j")
	waitUntilSlotsFree(t, handler)

	if got := panics.Load(); got != 1 {
		t.Fatalf("the panicking sequence ran %d time(s), want 1", got)
	}

	handler.HandleKeyPress("j")

	deadline := time.Now().Add(2 * time.Second)
	for panics.Load() != 2 {
		if time.Now().After(deadline) {
			t.Fatalf("the binding ran %d time(s) after one panic, want 2", panics.Load())
		}

		time.Sleep(time.Millisecond)
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
		map[string]configpkg.StringOrStringArray{"j": {testScrollDownStep}},
		configpkg.HeldRepeatConfig{
			Enabled:      true,
			InitialDelay: 1,
			Interval:     1,
		},
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

	waitUntilSlotsFree(t, handler)

	if got := peak.Load(); got != 1 {
		t.Fatalf("%d runs of one binding overlapped, want 1 at a time", got)
	}

	// A repeat that never fires again would pass the check above by holding
	// the key down and doing nothing, so the repeat is pinned as well.
	if got := runs.Load(); got < 2 {
		t.Fatalf("a held key ran its binding %d time(s), want the repeat to fire again", got)
	}
}
