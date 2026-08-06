package modes

import (
	"context"
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
	domainhint "github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/state"
)

type recordingMode struct {
	keys chan string
}

func (m *recordingMode) Activate(modecmd.Activation) {}
func (m *recordingMode) HandleKey(key string)        { m.keys <- key }
func (m *recordingMode) Exit()                       {}
func (m *recordingMode) ModeType() domain.Mode       { return domain.ModeRecursiveGrid }

func TestHandleKeyPressUsesStickyStrippedKeyForBindings(t *testing.T) {
	t.Parallel()

	appState := state.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	mode := &recordingMode{keys: make(chan string, 1)}
	hotkeyActions := make(chan string, 1)

	handler := newHandlerWithState(handlerState{
		config: &configpkg.Config{
			RecursiveGrid: configpkg.RecursiveGridConfig{
				Hotkeys: map[string]configpkg.StringOrStringArray{
					"Ctrl+C": {"exit"},
				},
			},
		},
		logger:        zap.NewNop(),
		appState:      appState,
		modifierState: state.NewModifierState(),
		modes: map[domain.Mode]Mode{
			domain.ModeRecursiveGrid: mode,
		},
		screenBounds: image.Rect(0, 0, 100, 100),
		executeActionSequence: func(_ string, steps []string) {
			hotkeyActions <- strings.Join(steps, ",")
		},
	})
	handler.modifierState.Toggle(action.ModCtrl)

	handler.HandleKeyPress("Ctrl+c")

	select {
	case got := <-mode.keys:
		if got != "c" {
			t.Fatalf("mode key = %q, want %q", got, "c")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stripped mode key")
	}

	select {
	case got := <-hotkeyActions:
		t.Fatalf("sticky modifier leaked into hotkey action %q", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHandleKeyPressRoutesAllKeysToHintSearch(t *testing.T) {
	t.Parallel()

	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	handler := newHandlerWithState(handlerState{
		config: &configpkg.Config{
			Hints: configpkg.HintsConfig{
				Hotkeys: map[string]configpkg.StringOrStringArray{
					"/": {"action search_hints"},
				},
			},
		},
		logger:        zap.NewNop(),
		appState:      appState,
		modifierState: state.NewModifierState(),
		hints: &components.HintsComponent{
			Context: &hintscomponent.Context{},
		},
		modes: map[domain.Mode]Mode{},
		executeActionSequence: func(_ string, steps []string) {
			t.Fatalf("hotkey action should be skipped during hint search, got %v", steps)
		},
	})

	elem, _ := element.NewElement(
		"search",
		image.Rect(0, 0, 20, 20),
		element.RoleButton,
		element.WithTitle("Slash Target"),
	)
	collection := domainhint.NewCollection([]*domainhint.Interface{
		mustNewModeHint("AA", elem),
	})

	handler.mu.Lock()
	manager := domainhint.NewManager(handler.logger, &handler.mu)
	handler.hints.Context.SetManager(manager)

	err := handler.hints.Context.SetHints(collection)
	if err != nil {
		t.Fatalf("SetHints: %v", err)
	}

	handler.hints.Context.SetSearchActive(true)
	handler.mu.Unlock()

	handler.HandleKeyPress("/")

	if got := handler.hints.Context.SearchQuery(); got != "/" {
		t.Fatalf("search query = %q, want %q", got, "/")
	}
}

// newHeldRepeatTestHandler builds a handler in recursive-grid mode where "j" is
// bound to a held-repeat action, with long delays so the repeat goroutine
// blocks on its initial timer and never dispatches during the test.
func newHeldRepeatTestHandler() *Handler {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	return newHandlerWithState(handlerState{
		ctx: context.Background(),
		config: &configpkg.Config{
			RecursiveGrid: configpkg.RecursiveGridConfig{
				Hotkeys: map[string]configpkg.StringOrStringArray{
					"j": {"action scroll_down"},
				},
			},
			HeldRepeat: configpkg.HeldRepeatConfig{
				Enabled:      true,
				InitialDelay: 10_000,
				Interval:     10_000,
			},
		},
		logger:        zap.NewNop(),
		appState:      appState,
		modifierState: state.NewModifierState(),
		modes: map[domain.Mode]Mode{
			domain.ModeRecursiveGrid: &recordingMode{keys: make(chan string, 1)},
		},
		screenBounds:          image.Rect(0, 0, 100, 100),
		executeActionSequence: func(_ string, _ []string) {},
	})
}

// TestHandleFedKeyPressStopsOwnHeldRepeat verifies that a key injected via
// `action feed --mode` does not leave its own held-repeat goroutine running. A
// fed key has no physical key-up, so HandleFedKeyPress must synthesize the
// release that tears down the repeat the fed press started.
func TestHandleFedKeyPressStopsOwnHeldRepeat(t *testing.T) {
	t.Parallel()

	handler := newHeldRepeatTestHandler()

	handler.HandleFedKeyPress("j")

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if handler.heldRepeatingKey != "" {
		t.Fatalf(
			"fed key left a held repeat active; heldRepeatingKey = %q, want empty",
			handler.heldRepeatingKey,
		)
	}

	if handler.heldRepeatingCancel != nil {
		t.Fatal("fed key left the held-repeat cancel func set")
	}
}

// TestHandleFedKeyPressPreservesPhysicalHeldRepeat verifies that feeding a key
// that matches a repeat a physically held key already started does not cancel
// that repeat. The fed key owns only repeats it starts itself.
func TestHandleFedKeyPressPreservesPhysicalHeldRepeat(t *testing.T) {
	t.Parallel()

	handler := newHeldRepeatTestHandler()

	// Simulate a physical hold that started a repeat.
	handler.HandleKeyPress("j")

	handler.mu.Lock()
	startedKey := handler.heldRepeatingKey
	handler.mu.Unlock()

	if startedKey != "j" {
		t.Fatalf(
			"physical press did not start held repeat; heldRepeatingKey = %q, want %q",
			startedKey,
			"j",
		)
	}

	// Feeding the same key must not cancel the physically held repeat.
	handler.HandleFedKeyPress("j")

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if handler.heldRepeatingKey != "j" {
		t.Fatalf(
			"fed key canceled a physically held repeat; heldRepeatingKey = %q, want %q",
			handler.heldRepeatingKey,
			"j",
		)
	}

	// Stop the still-running repeat goroutine so it does not outlive the test.
	handler.stopHeldRepeat()
}

// The mode dispatcher hands the whole binding to the daemon's sequence
// executor rather than stepping through it itself, so that a binding runs
// under the same rules wherever it is dispatched from. The bail and
// continue-on-error semantics are covered against the real executor in
// internal/app.
func TestDispatchHotkeyActions_ForwardsWholeSequence(t *testing.T) {
	t.Parallel()

	got := make(chan []string, 1)

	handler := newHandlerWithState(handlerState{
		logger:   zap.NewNop(),
		appState: state.NewAppState(),
		executeActionSequence: func(source string, steps []string) {
			if source != "test-bind" {
				t.Errorf("sequence source = %q, want %q", source, "test-bind")
			}

			got <- steps
		},
	})

	want := []string{"first", "second"}
	handler.dispatchHotkeyActions("test-mode", "test-bind", "t", want)

	select {
	case steps := <-got:
		if !slices.Equal(steps, want) {
			t.Fatalf("forwarded steps = %v, want %v", steps, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the sequence to be dispatched")
	}
}

func TestModeHotkeyOverride(t *testing.T) {
	t.Parallel()

	cfg := &configpkg.Config{
		Hints: configpkg.HintsConfig{
			Hotkeys: map[string]configpkg.StringOrStringArray{
				"Primary+Ctrl+F": {"recursive_grid"},
				"Escape":         {"idle"},
			},
		},
		Grid: configpkg.GridConfig{
			Hotkeys: map[string]configpkg.StringOrStringArray{
				"Primary+Ctrl+F": {"scroll"},
			},
		},
	}

	// Global-hotkey dispatch passes the platform-canonical key (e.g. "Cmd+Ctrl+F"
	// on macOS), while the config stores the shared "Primary+..." alias. Build the
	// lookup keys exactly as registerHotkeys does so the test exercises the real
	// normalization path on every platform.
	overrideKey := configpkg.CanonicalHotkeyForPlatform("Primary+Ctrl+F")
	unboundGlobalKey := configpkg.CanonicalHotkeyForPlatform("Primary+Shift+G")

	tests := []struct {
		name        string
		mode        domain.Mode
		key         string
		wantActions []string
		wantOK      bool
	}{
		{
			name:        "active mode binds the key: per-mode action overrides the global binding",
			mode:        domain.ModeHints,
			key:         overrideKey,
			wantActions: []string{"recursive_grid"},
			wantOK:      true,
		},
		{
			name:   "active mode does not bind the key: no override, global hotkey still fires (#21 preserved)",
			mode:   domain.ModeHints,
			key:    unboundGlobalKey,
			wantOK: false,
		},
		{
			name:   "idle: a global launcher is never overridden",
			mode:   domain.ModeIdle,
			key:    overrideKey,
			wantOK: false,
		},
		{
			name:        "override is scoped to the active mode's own hotkey table",
			mode:        domain.ModeGrid,
			key:         overrideKey,
			wantActions: []string{"scroll"},
			wantOK:      true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			appState := state.NewAppState()
			appState.SetMode(testCase.mode)

			handler := newHandlerWithState(handlerState{
				config:   cfg,
				logger:   zap.NewNop(),
				appState: appState,
			})

			actions, ok := handler.ModeHotkeyOverride(testCase.key)
			if ok != testCase.wantOK {
				t.Fatalf(
					"ModeHotkeyOverride(%q) ok = %v, want %v",
					testCase.key,
					ok,
					testCase.wantOK,
				)
			}

			if !testCase.wantOK {
				return
			}

			if len(actions) != len(testCase.wantActions) {
				t.Fatalf("actions = %v, want %v", actions, testCase.wantActions)
			}

			for i := range actions {
				if actions[i] != testCase.wantActions[i] {
					t.Fatalf("actions = %v, want %v", actions, testCase.wantActions)
				}
			}
		})
	}
}

func mustNewModeHint(label string, elem *element.Element) *domainhint.Interface {
	hint, err := domainhint.NewHint(label, elem, image.Point{})
	if err != nil {
		panic(err)
	}

	return hint
}
