package modes

// A global [hotkeys] chord has to keep working while a mode is open, and on
// Linux the handler is the only thing that can run it: the in-mode capture is
// exclusive, so the mechanism that registered the chord sees nothing until the
// mode exits. These are that fallback stated as behavior — what it runs, what it
// refuses to shadow, and who wins when both tables bind the same chord.

import (
	"image"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
)

const (
	// fallbackChord is a global chord written the way a Linux user writes one;
	// it normalizes to the "Cmd+;" the evdev tap delivers.
	fallbackChord = "Super+;"
	// fallbackSteps is what the global binding runs — the reported case,
	// reduced to the flag that makes the difference.
	fallbackSteps = "recursive_grid --toggle"
	modeSteps     = "action left_click"
)

// fallbackHandler builds a recursive-grid handler over the two tables, recording
// what each dispatch runs and what reaches the mode itself.
func fallbackHandler(
	t *testing.T,
	global map[string][]string,
	modeHotkeys map[string]configpkg.StringOrStringArray,
) (*Handler, chan string, *recordingMode) {
	t.Helper()

	appState := state.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	mode := &recordingMode{keys: make(chan string, 1)}
	dispatched := make(chan string, 1)

	handler := newHandlerWithState(handlerState{
		config: &configpkg.Config{
			Hotkeys:       configpkg.HotkeysConfig{Bindings: global},
			RecursiveGrid: configpkg.RecursiveGridConfig{Hotkeys: modeHotkeys},
		},
		logger:        zap.NewNop(),
		appState:      appState,
		modifierState: state.NewModifierState(),
		modes: map[domain.Mode]Mode{
			domain.ModeRecursiveGrid: mode,
		},
		screenBounds: image.Rect(0, 0, 100, 100),
		executeActionSequence: func(_ string, steps []string) {
			dispatched <- strings.Join(steps, ",")
		},
	})

	return handler, dispatched, mode
}

// waitForDispatch reports the steps the handler ran, or fails if it ran none.
// The dispatch is asynchronous by contract — handleHotkey runs the sequence on a
// goroutine so it cannot re-enter h.mu — so this waits rather than polling once.
func waitForDispatch(t *testing.T, dispatched chan string) string {
	t.Helper()

	select {
	case steps := <-dispatched:
		return steps
	case <-time.After(2 * time.Second):
		t.Fatal("no action sequence ran for the key")

		return ""
	}
}

func TestHandleKeyPress_GlobalChordRunsWhenTheModeDoesNotBindIt(t *testing.T) {
	t.Parallel()

	handler, dispatched, mode := fallbackHandler(t,
		map[string][]string{fallbackChord: {fallbackSteps}},
		map[string]configpkg.StringOrStringArray{"Escape": {"idle"}},
	)

	// What the evdev tap delivers for the chord, in its own spelling.
	handler.HandleKeyPress("Cmd+;")

	if steps := waitForDispatch(t, dispatched); steps != fallbackSteps {
		t.Errorf("the chord ran %q, want the global binding %q", steps, fallbackSteps)
	}

	select {
	case key := <-mode.keys:
		t.Errorf("the chord reached the mode as %q; the fallback consumed nothing", key)
	default:
	}
}

// The mode's own table is the more specific one and keeps winning the chord,
// which is what the documented mode-cycling trick rests on
// (docs/TIPS_TRICKS.md).
func TestHandleKeyPress_ModeBindingWinsOverTheGlobalChord(t *testing.T) {
	t.Parallel()

	handler, dispatched, _ := fallbackHandler(t,
		map[string][]string{fallbackChord: {fallbackSteps}},
		map[string]configpkg.StringOrStringArray{fallbackChord: {modeSteps}},
	)

	handler.HandleKeyPress("Cmd+;")

	if steps := waitForDispatch(t, dispatched); steps != modeSteps {
		t.Errorf("the chord ran %q, want the mode's own binding %q", steps, modeSteps)
	}
}

// A bare key is a mode's own input — a hint label, a grid cell key — so a global
// binding for one may not reach it. The key goes to the mode instead.
func TestHandleKeyPress_GlobalBareKeyStaysTheModesOwn(t *testing.T) {
	t.Parallel()

	handler, dispatched, mode := fallbackHandler(t,
		map[string][]string{";": {fallbackSteps}, "Shift+M": {fallbackSteps}},
		nil,
	)

	handler.HandleKeyPress(";")

	select {
	case key := <-mode.keys:
		if key != ";" {
			t.Errorf("the mode got %q, want %q", key, ";")
		}
	case steps := <-dispatched:
		t.Fatalf("the bare key ran the global binding %q", steps)
	case <-time.After(2 * time.Second):
		t.Fatal("the bare key reached neither the mode nor a binding")
	}

	// Shift alone does not make a shortcut either: modes bind Shift combos of
	// their own throughout.
	handler.HandleKeyPress("Shift+m")

	select {
	case steps := <-dispatched:
		t.Errorf("the Shift-only combo ran the global binding %q", steps)
	case <-mode.keys:
	case <-time.After(2 * time.Second):
		t.Fatal("the Shift-only combo reached neither the mode nor a binding")
	}
}

// Idle takes no fallback: nothing is captured there, so the platform's own
// hotkey mechanism owns the chord and a key fed over IPC must not fire one
// behind its back.
func TestSettledGlobalHotkeys_IsEmptyInIdle(t *testing.T) {
	t.Parallel()

	handler, _, _ := fallbackHandler(t,
		map[string][]string{fallbackChord: {fallbackSteps}},
		nil,
	)

	handler.mu.Lock()
	_, inModeGlobal := handler.settledKeymaps()
	inMode := inModeGlobal.Len()
	handler.mu.Unlock()

	if inMode == 0 {
		t.Fatal("no global chord is in force while a mode is open")
	}

	handler.appState.SetMode(domain.ModeIdle)

	handler.mu.Lock()
	_, idleGlobal := handler.settledKeymaps()
	idle := idleGlobal.Len()
	handler.mu.Unlock()

	if idle != 0 {
		t.Errorf("idle holds %d global chords, want none", idle)
	}
}

// The fallback is settled with the mode's own table and from the same inputs, so
// a configuration replacement moves both. Without that, a reload would leave the
// chord resolving to what the old file said.
func TestUpdateConfig_ResettlesTheGlobalFallback(t *testing.T) {
	t.Parallel()

	handler, dispatched, _ := fallbackHandler(t,
		map[string][]string{fallbackChord: {fallbackSteps}},
		nil,
	)

	replaced := "scroll --toggle"
	handler.UpdateConfig(&configpkg.Config{
		Hotkeys: configpkg.HotkeysConfig{
			Bindings: map[string][]string{fallbackChord: {replaced}},
		},
	})

	handler.HandleKeyPress("Cmd+;")

	if steps := waitForDispatch(t, dispatched); steps != replaced {
		t.Errorf("the chord ran %q, want the replaced binding %q", steps, replaced)
	}
}

// A mode that binds nothing still gets the fallback. The dispatch used to give
// up on an empty keymap, which would have left the chord dead in exactly the
// mode with no table of its own.
func TestHandleKeyPress_GlobalChordRunsForAModeThatBindsNothing(t *testing.T) {
	t.Parallel()

	handler, dispatched, _ := fallbackHandler(t,
		map[string][]string{fallbackChord: {fallbackSteps}},
		nil,
	)

	handler.HandleKeyPress("Cmd+;")

	if steps := waitForDispatch(t, dispatched); steps != fallbackSteps {
		t.Errorf("the chord ran %q, want the global binding %q", steps, fallbackSteps)
	}
}

// The per-app override half: the global table can be rebound per application, so
// the fallback has to resolve for the app the watcher published rather than for
// the base table.
func TestHandleKeyPress_GlobalChordFollowsThePerAppOverride(t *testing.T) {
	t.Parallel()

	const focusedApp = "com.example.editor"

	overridden := "scroll --toggle"

	handler, dispatched, _ := fallbackHandler(t,
		map[string][]string{fallbackChord: {fallbackSteps}},
		nil,
	)

	handler.config.AppConfigs = []configpkg.AppConfig{
		{
			BundleID: focusedApp,
			Hotkeys: map[string]configpkg.StringOrStringArray{
				fallbackChord: {overridden},
			},
		},
	}
	handler.PublishFocusedApp(focusedApp)

	handler.HandleKeyPress("Cmd+;")

	if steps := waitForDispatch(t, dispatched); steps != overridden {
		t.Errorf("the chord ran %q, want the focused app's override %q", steps, overridden)
	}
}
