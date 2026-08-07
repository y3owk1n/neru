package modes

// The published focused app is the one piece of handler state written by a
// goroutine that holds no lock — on macOS the main queue, which may not take
// h.mu — and read by whichever thread is settling a keymap under it. That makes
// it the state a wrong answer costs a data race rather than a wrong binding,
// which is what this exercises.
//
// It publishes directly, which no journey may do: a journey drives a focus
// change through a watcher activation event so it fails if the watcher is never
// wired up (TestSimulation_FocusChangeMidModeRebindsTheKey covers that). What
// is under test here is the memory model, not the wiring, and the publisher is
// the seam that carries it.

import (
	"sync"
	"testing"

	"go.uber.org/zap"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
)

func TestPublishFocusedApp_IsSafeWhileKeysAreHandled(t *testing.T) {
	t.Parallel()

	const (
		key      = "j"
		switches = 200
		// The two applications bind the same key differently, so which steps
		// the keymap holds says which published app it was settled for.
		baseStep     = "action left_click"
		overrideStep = "action right_click"
	)

	appState := state.NewAppState()
	appState.SetMode(domain.ModeGrid)

	// Per-app overrides are what make the published app reach the keymap: with
	// none declared the handler has no reason to read the cell at all.
	cfg := &configpkg.Config{
		Grid: configpkg.GridConfig{
			Hotkeys: map[string]configpkg.StringOrStringArray{
				key: {baseStep},
			},
			AppConfigs: []configpkg.AppConfig{
				{
					BundleID: "com.example.one",
					Hotkeys: map[string]configpkg.StringOrStringArray{
						key: {overrideStep},
					},
				},
			},
		},
	}

	// The real modes, because whether the active one declares per-app overrides
	// is what decides that the published app is read at all.
	handler := newHandlerWithState(handlerState{
		config:                cfg,
		logger:                zap.NewNop(),
		appState:              appState,
		modifierState:         state.NewModifierState(),
		executeActionSequence: func(string, []string) {},
	})

	var publishing sync.WaitGroup

	publishing.Go(func() {
		for range switches {
			handler.PublishFocusedApp("com.example.one")
			handler.PublishFocusedApp("com.example.two")
		}
	})

	for range switches {
		handler.HandleKeyPress(key)
	}

	publishing.Wait()

	// Whichever app was published last is the one the next keystroke is bound
	// under, and the handler is still answering keys at all.
	handler.PublishFocusedApp("com.example.one")

	handler.mu.Lock()
	defer handler.mu.Unlock()

	binding, bound := handler.settledKeymap().Lookup(key)
	if !bound {
		t.Fatalf("%q is unbound after the focus changes", key)
	}

	if len(binding.Steps) != 1 || binding.Steps[0] != overrideStep {
		t.Errorf("%q runs %v, want the published app's override", key, binding.Steps)
	}
}
