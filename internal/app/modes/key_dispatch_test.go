//nolint:testpackage // These tests validate unexported handler behavior directly.
package modes

import (
	"image"
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

func mustNewModeHint(label string, elem *element.Element) *domainhint.Interface {
	hint, err := domainhint.NewHint(label, elem, image.Point{})
	if err != nil {
		panic(err)
	}

	return hint
}
