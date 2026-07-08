//nolint:testpackage // These tests validate unexported handler behavior directly.
package modes

import (
	"image"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	domainhint "github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

type recordingMode struct {
	keys chan string
}

func (m *recordingMode) Activate(ModeActivationOptions) {}
func (m *recordingMode) HandleKey(key string)           { m.keys <- key }
func (m *recordingMode) Exit()                          {}
func (m *recordingMode) ModeType() domain.Mode          { return domain.ModeRecursiveGrid }

func TestHandleKeyPressUsesStickyStrippedKeyForBindings(t *testing.T) {
	t.Parallel()

	appState := state.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	mode := &recordingMode{keys: make(chan string, 1)}
	hotkeyActions := make(chan string, 1)

	handler := &Handler{
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
		executeHotkeyAction: func(_, actionStr string) error {
			hotkeyActions <- actionStr

			return nil
		},
	}
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

	handler := &Handler{
		config: &configpkg.Config{
			Hints: configpkg.HintsConfig{
				Hotkeys: map[string]configpkg.StringOrStringArray{
					"/": {"action search_hints"},
				},
			},
		},
		logger:         zap.NewNop(),
		appState:       appState,
		modifierState:  state.NewModifierState(),
		overlayManager: &overlay.NoOpManager{},
		hints: &components.HintsComponent{
			Context: &hintscomponent.Context{},
		},
		modes: map[domain.Mode]Mode{},
		executeHotkeyAction: func(_, actionStr string) error {
			t.Fatalf("hotkey action should be skipped during hint search, got %q", actionStr)

			return nil
		},
	}

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

func TestDispatchHotkeyActions_AbortsOnBail(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int64

	handler := &Handler{
		logger:   zap.NewNop(),
		appState: state.NewAppState(),
		executeHotkeyAction: func(_, actionStr string) error {
			callCount.Add(1)

			if callCount.Load() == 1 {
				return derrors.New(derrors.CodeChainBail, "bail")
			}

			return nil
		},
	}

	handler.dispatchHotkeyActions("test-mode", "test-bind", "t", []string{"first", "second"})

	time.Sleep(50 * time.Millisecond)

	if got := callCount.Load(); got != 1 {
		t.Fatalf(
			"executeHotkeyAction called %d times, want 1 (chain should abort on bail)",
			got,
		)
	}
}

func TestDispatchHotkeyActions_ContinuesOnRegularError(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int64

	handler := &Handler{
		logger:   zap.NewNop(),
		appState: state.NewAppState(),
		executeHotkeyAction: func(_, actionStr string) error {
			callCount.Add(1)

			if callCount.Load() == 1 {
				return derrors.New(derrors.CodeIPCFailed, "generic error")
			}

			return nil
		},
	}

	handler.dispatchHotkeyActions("test-mode", "test-bind", "t", []string{"first", "second"})

	time.Sleep(50 * time.Millisecond)

	if got := callCount.Load(); got != 2 {
		t.Fatalf(
			"executeHotkeyAction called %d times, want 2 (chain should continue on regular error)",
			got,
		)
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

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			appState := state.NewAppState()
			appState.SetMode(tc.mode)

			handler := &Handler{
				config:   cfg,
				logger:   zap.NewNop(),
				appState: appState,
			}

			actions, ok := handler.ModeHotkeyOverride(tc.key)
			if ok != tc.wantOK {
				t.Fatalf("ModeHotkeyOverride(%q) ok = %v, want %v", tc.key, ok, tc.wantOK)
			}

			if !tc.wantOK {
				return
			}

			if len(actions) != len(tc.wantActions) {
				t.Fatalf("actions = %v, want %v", actions, tc.wantActions)
			}

			for i := range actions {
				if actions[i] != tc.wantActions[i] {
					t.Fatalf("actions = %v, want %v", actions, tc.wantActions)
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
