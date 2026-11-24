package modes

import (
	"context"
	"fmt"
	"image"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	statepkg "github.com/y3owk1n/neru/internal/domain/state"
)

// testScrollDownStep is a bound action that held_repeat is willing to repeat,
// so a test can drive the repeat loop through the real binding path.
const testScrollDownStep = "action scroll_down"

// testSpaceKey is bound in more than one mode where a test checks that one
// mode's sequence does not silence another mode's binding.
const testSpaceKey = "Space"

// The pan-macro body the cursor-slot tests write, as one step each so a test
// can spell the slot it saves through once.
const (
	panSaveStepFormat    = "action save_cursor_pos --slot %s"
	panMouseDownStep     = "action left_mouse_down --bail-on-error"
	panSleepStep         = "action sleep 10ms"
	panRelativeMoveStep  = "action move_mouse_relative --dx=$1 --dy=$2"
	panMouseUpStep       = "action left_mouse_up"
	panRestoreStepFormat = "action restore_cursor_pos --slot %s"
)

// panTestKey and its partner are the two bindings the pan tests press, and
// panEastStep is the step both press through the pan macro body.
const (
	panEastStep = "macro pan 400 0"

	panTestKeyS = "Shift+s"
	panTestKeyD = "Shift+d"
)

// panCursorSlot is the claim the default cursor slot has in the guard's map,
// written the way sequenceClaims writes one. Most tests here touch it through
// this constant, and the tests that name another slot build the claim the
// same way.
const panCursorSlot = sequenceClaim(cursorSlotClaimPrefix + statepkg.DefaultCursorSlot)

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
	appState := statepkg.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	return newHandlerWithState(handlerState{
		ctx:           context.Background(),
		config:        cfg,
		logger:        zap.NewNop(),
		appState:      appState,
		modifierState: statepkg.NewModifierState(),
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

// waitForDrained waits until reads reports nothing running, and fails the test
// on the count it settled on. Every caller wants zero, so the want is spelled
// here rather than passed: the invariant is the point.
func waitForDrained(t *testing.T, desc string, reads func() int32) {
	t.Helper()

	deadline := time.Now().Add(singleFlightTestWait)

	for {
		if got := reads(); got == 0 {
			return
		} else if time.Now().After(deadline) {
			t.Fatalf("%s: got %d, want 0", desc, got)
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

	waitForDrained(t, "runs still in flight", live.Load)

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

	waitForDrained(t, "runs still in flight", live.Load)
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
	waitForDrained(t, "runs still in flight after the key came up", live.Load)

	if got := peak.Load(); got != 1 {
		t.Fatalf("%d runs of one binding overlapped, want 1 at a time", got)
	}

	// A repeat that never fires again would pass the check above by holding
	// the key down and doing nothing, so the repeat is pinned as well.
	if got := runs.Load(); got < 2 {
		t.Fatalf("a held key ran its binding %d time(s), want the repeat to fire again", got)
	}
}

// panMacro is the config the cursor-slot tests share: a macro that saves the
// cursor, drags, and restores through the named slot, the shape the pan
// bindings a browser needs are written as.
func panMacro(slot string) map[string]configpkg.StringOrStringArray {
	return map[string]configpkg.StringOrStringArray{
		"pan": {
			fmt.Sprintf(panSaveStepFormat, slot),
			panMouseDownStep,
			panSleepStep,
			panRelativeMoveStep,
			panMouseUpStep,
			fmt.Sprintf(panRestoreStepFormat, slot),
		},
	}
}

// panSlotClaim is the claim for one named cursor slot, built the way
// sequenceClaims writes one.
func panSlotClaim(slot string) sequenceClaim {
	return sequenceClaim(cursorSlotClaimPrefix + slot)
}

// waitForSlotFree waits until the cursor slot named is claimed by no sequence.
// The release runs on the sequence's own goroutine, so this waits rather than
// reads once, but it reads the guard rather than pressing the key: what it
// pins is the release, which no press can observe without racing one.
func waitForSlotFree(t *testing.T, handler *Handler, claim sequenceClaim) {
	t.Helper()

	deadline := time.Now().Add(singleFlightTestWait)

	for {
		handler.outer.seqMu.Lock()
		_, held := handler.claimedSlots[claim]
		handler.outer.seqMu.Unlock()

		if !held {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("cursor slot %q is still claimed, want it free after the run ended", claim)
		}

		time.Sleep(time.Millisecond)
	}
}

// Two pan keys sharing one cursor slot are in the same race two presses of one
// key are, even though they are different bindings: a second pan pressed
// inside the first run's sleeps restores a position the first run had already
// moved away from, which is the drift a fast Shift+S then Shift+D showed. The
// second pan is dropped while the slot is held, and runs once the first is
// done.
func TestHandlerState_DispatchHotkeyActions_DropsASecondPanSharingItsCursorSlot(t *testing.T) {
	t.Parallel()

	started := make(chan string, 4)
	release := make(chan struct{})

	cfg := recursiveGridBindings(
		map[string]configpkg.StringOrStringArray{
			panTestKeyS: {panEastStep},
			panTestKeyD: {"macro pan 0 400"},
		},
		configpkg.HeldRepeatConfig{},
	)
	cfg.Macros = panMacro("pan")

	var runs atomic.Int32

	handler := newSingleFlightTestHandler(cfg, func(source string, _ []string) {
		runs.Add(1)

		started <- source

		<-release
	})

	handler.HandleKeyPress(panTestKeyS)
	waitForBindingDispatch(t, started, panTestKeyS)

	// A different binding, but the same slot: dropped, not stacked.
	handler.HandleKeyPress(panTestKeyD)

	select {
	case got := <-started:
		t.Fatalf("a second pan through slot pan ran while the first held it: %q", got)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	waitForSlotFree(t, handler, panCursorSlot)

	// The slot is free again, so the other pan key answers.
	pressUntilDispatched(t, handler, panTestKeyD, started, panTestKeyD)

	if got := runs.Load(); got != 2 {
		t.Fatalf("the pan ran %d time(s), want one per tap", got)
	}
}

// Two pans through different cursor slots never wait for each other: the slot
// is the only thing the runs share, so naming two is naming two resources.
// Shift+s runs macros.pan through slot "other" and Shift+d macros.other
// through slot "second", so the two bindings touch nothing in common.
func TestHandlerState_DispatchHotkeyActions_PansThroughDifferentSlotsRunTogether(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})

	cfg := recursiveGridBindings(
		map[string]configpkg.StringOrStringArray{
			panTestKeyS: {panEastStep},
			panTestKeyD: {"macro other 0 400"},
		},
		configpkg.HeldRepeatConfig{},
	)
	cfg.Macros = map[string]configpkg.StringOrStringArray{
		"pan": {
			"action save_cursor_pos --slot other",
			"action left_mouse_down --bail-on-error",
			"action sleep 10ms",
			"action move_mouse_relative --dx=$1 --dy=$2",
			"action left_mouse_up",
			"action restore_cursor_pos --slot other",
		},
		"other": {
			"action save_cursor_pos --slot second",
			"action left_mouse_down --bail-on-error",
			"action sleep 10ms",
			"action move_mouse_relative --dx=$1 --dy=$2",
			"action left_mouse_up",
			"action restore_cursor_pos --slot second",
		},
	}

	sDone := make(chan struct{})
	dStarted := make(chan struct{}, 1)

	handler := newSingleFlightTestHandler(cfg, func(source string, _ []string) {
		if source == panTestKeyS {
			<-release
			close(sDone)

			return
		}

		dStarted <- struct{}{}
	})

	handler.HandleKeyPress(panTestKeyS)

	// The second pan runs while the first holds its own slot: two slots are
	// two resources, so neither binding waits for the other.
	handler.HandleKeyPress(panTestKeyD)

	select {
	case <-dStarted:
	case <-time.After(singleFlightTestWait):
		t.Fatal("the second pan through another slot never ran")
	}

	close(release)

	<-sDone

	waitForSlotFree(t, handler, panSlotClaim("second"))
}

// A binding that saves through the default slot claims it, so two bindings
// that write no --slot are in the same race. Clicks and scrolls claim nothing
// and still run beside a pan.
func TestHandlerState_SequenceClaims_ReadsTheStepsTheBindingDispatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		hotkeys    map[string]configpkg.StringOrStringArray
		macros     map[string]configpkg.StringOrStringArray
		wantClaims []sequenceClaim
	}{
		{
			name: "a bare save claims the default slot",
			hotkeys: map[string]configpkg.StringOrStringArray{
				"j": {"action save_cursor_pos", "action left_click", "action restore_cursor_pos"},
			},
			wantClaims: []sequenceClaim{panCursorSlot},
		},
		{
			name: "an explicit slot is claimed by its name",
			hotkeys: map[string]configpkg.StringOrStringArray{
				"j": {"action save_cursor_pos --slot other"},
			},
			wantClaims: []sequenceClaim{panSlotClaim("other")},
		},
		{
			name:       "a macro claims what its body saves through",
			hotkeys:    map[string]configpkg.StringOrStringArray{"j": {"macro pan 1 2"}},
			macros:     panMacro("pan"),
			wantClaims: []sequenceClaim{panSlotClaim("pan")},
		},
		{
			name:       "a scroll claims nothing",
			hotkeys:    map[string]configpkg.StringOrStringArray{"j": {testScrollDownStep}},
			wantClaims: nil,
		},
		{
			name:       "an unknown macro claims nothing",
			hotkeys:    map[string]configpkg.StringOrStringArray{"j": {"macro nope 1 2"}},
			wantClaims: nil,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := recursiveGridBindings(testCase.hotkeys, configpkg.HeldRepeatConfig{})
			cfg.Macros = testCase.macros

			got := sequenceClaims(testCase.hotkeys["j"], cfg.Macros)

			if len(got) != len(testCase.wantClaims) {
				t.Fatalf("claims = %v, want %v", got, testCase.wantClaims)
			}

			for idx, claim := range got {
				if claim != testCase.wantClaims[idx] {
					t.Fatalf("claims[%d] = %q, want %q", idx, claim, testCase.wantClaims[idx])
				}
			}
		})
	}
}

// A repeat tick holds the same cursor slots the press held, so a held pan key
// never runs two drags through one slot at once.
func TestHandlerState_HeldRepeat_PanClaimsItsCursorSlot(t *testing.T) {
	t.Parallel()

	var live, peak atomic.Int32

	cfg := recursiveGridBindings(
		map[string]configpkg.StringOrStringArray{"j": {panEastStep}},
		configpkg.HeldRepeatConfig{
			Enabled:      true,
			InitialDelay: 1,
			Interval:     1,
		},
	)
	cfg.Macros = panMacro("pan")

	handler := newSingleFlightTestHandler(cfg, func(string, []string) {
		running := live.Add(1)
		for {
			highest := peak.Load()
			if running <= highest || peak.CompareAndSwap(highest, running) {
				break
			}
		}

		time.Sleep(2 * time.Millisecond)

		live.Add(-1)
	})

	handler.HandleKeyPress("j")
	time.Sleep(80 * time.Millisecond)
	handler.HandleKeyPress(keyUpPrefix + "j")

	waitForDrained(t, "runs still in flight after the key came up", live.Load)

	if got := peak.Load(); got != 1 {
		t.Fatalf("%d pans ran through slot pan at once, want 1", got)
	}

	waitForSlotFree(t, handler, panCursorSlot)
}
