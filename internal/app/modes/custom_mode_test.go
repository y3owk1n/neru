package modes

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// The declarations these cases enter.
const (
	declaredWindow = "window"
	declaredTabs   = "tabs"
	stepFocusWest  = "exec yabai -m window --focus west"
	stepNextTab    = "action feed ctrl+tab"
)

// configDeclaring is a configuration declaring window and tabs, each with one
// bare-letter binding over the default Escape.
func configDeclaring() *configpkg.Config {
	declare := func(key, step string) configpkg.CustomModeConfig {
		table := configpkg.DefaultCustomModeHotkeys()
		table[key] = configpkg.StringOrStringArray{step}

		return configpkg.CustomModeConfig{Hotkeys: table}
	}

	return &configpkg.Config{
		Modes: map[string]configpkg.CustomModeConfig{
			declaredWindow: declare("h", stepFocusWest),
			declaredTabs:   declare("n", stepNextTab),
		},
	}
}

// newCustomModeHandler builds a handler on the declared configuration that
// records every sequence a key dispatches.
func newCustomModeHandler(cfg *configpkg.Config) (*Handler, chan string) {
	ran := make(chan string, 4)

	handler := newHandlerWithState(handlerState{
		ctx:           context.Background(),
		config:        cfg,
		logger:        zap.NewNop(),
		appState:      state.NewAppState(),
		cursorState:   state.NewCursorState(),
		modifierState: state.NewModifierState(),
		scroll:        &components.ScrollComponent{Context: &scroll.Context{}},
		executeActionSequence: func(_ string, steps []string) {
			for _, step := range steps {
				ran <- step
			}
		},
	})

	return handler, ran
}

func activate(name string, toggle bool) modecmd.Activation {
	activation := modecmd.Activation{Mode: domain.ModeCustom, Name: name}
	if toggle {
		activation.Toggle = new(true)
	}

	return activation
}

// TestCustomMode_Activate_EntersTheDeclaredModeAndAnswersItsKeys pins the
// whole path a declared mode exists for: entering it by name settles that
// declaration's keymap, a bound bare key runs its steps, the default Escape
// binding leads out, and leaving clears the name.
func TestCustomMode_Activate_EntersTheDeclaredModeAndAnswersItsKeys(t *testing.T) {
	t.Parallel()

	handler, ran := newCustomModeHandler(configDeclaring())

	handler.ActivateMode(activate(declaredWindow, false))

	if got := handler.appState.CurrentMode(); got != domain.ModeCustom {
		t.Fatalf("mode = %v after activation, want ModeCustom", got)
	}

	if handler.customModeName != declaredWindow {
		t.Fatalf("customModeName = %q, want %q", handler.customModeName, declaredWindow)
	}

	handler.HandleKeyPress("h")
	waitForStep(t, ran, stepFocusWest)

	handler.HandleKeyPress("Escape")
	waitForStep(t, ran, configpkg.CmdIdle)

	handler.ActivateMode(modecmd.Activation{Mode: domain.ModeIdle})

	if got := handler.appState.CurrentMode(); got != domain.ModeIdle {
		t.Fatalf("mode = %v after idle, want ModeIdle", got)
	}

	if handler.customModeName != "" {
		t.Errorf("customModeName = %q after leaving, want it cleared", handler.customModeName)
	}
}

// TestCustomMode_Activate_SwitchesBetweenDeclarations pins that two declared
// modes are two modes even though they share one enum value: entering the
// second from the first switches the keymap, and --toggle exits only when the
// name matches the one already open.
func TestCustomMode_Activate_SwitchesBetweenDeclarations(t *testing.T) {
	t.Parallel()

	handler, ran := newCustomModeHandler(configDeclaring())

	handler.ActivateMode(activate(declaredWindow, false))
	handler.ActivateMode(activate(declaredTabs, true))

	if got := handler.appState.CurrentMode(); got != domain.ModeCustom {
		t.Fatalf("toggling another declaration exited to %v, want a switch", got)
	}

	if handler.customModeName != declaredTabs {
		t.Fatalf("customModeName = %q, want %q", handler.customModeName, declaredTabs)
	}

	handler.HandleKeyPress("n")
	waitForStep(t, ran, stepNextTab)

	handler.ActivateMode(activate(declaredTabs, true))

	if got := handler.appState.CurrentMode(); got != domain.ModeIdle {
		t.Fatalf("toggling the open declaration left the mode at %v, want idle", got)
	}
}

// TestCustomMode_Activate_RefusesAnUndeclaredName pins that an activation the
// grammar never saw cannot enter a mode nothing declares: the daemon stays
// idle with the keyboard uncaptured rather than in a mode that answers nothing.
func TestCustomMode_Activate_RefusesAnUndeclaredName(t *testing.T) {
	t.Parallel()

	handler, _ := newCustomModeHandler(configDeclaring())

	handler.ActivateMode(activate("nobody", false))

	if got := handler.appState.CurrentMode(); got != domain.ModeIdle {
		t.Fatalf("mode = %v after an undeclared name, want idle", got)
	}

	if handler.DeclaresMode("nobody") || !handler.DeclaresMode(declaredWindow) {
		t.Error("DeclaresMode() disagrees with the configuration")
	}
}

// TestUpdateConfig_ExitsADeclaredModeTheReloadDropped pins the reload rule: a
// mode the new configuration no longer declares has no keymap to settle, so
// the session ends rather than leaving the keyboard captured.
func TestUpdateConfig_ExitsADeclaredModeTheReloadDropped(t *testing.T) {
	t.Parallel()

	handler, _ := newCustomModeHandler(configDeclaring())

	handler.ActivateMode(activate(declaredWindow, false))

	kept := configDeclaring()
	handler.UpdateConfig(kept)

	if got := handler.appState.CurrentMode(); got != domain.ModeCustom {
		t.Fatalf("a reload that keeps the declaration exited to %v", got)
	}

	dropped := configDeclaring()
	delete(dropped.Modes, declaredWindow)
	handler.UpdateConfig(dropped)

	if got := handler.appState.CurrentMode(); got != domain.ModeIdle {
		t.Fatalf("a reload that drops the declaration left the mode at %v, want idle", got)
	}
}
