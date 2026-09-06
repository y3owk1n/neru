package ipcctrl_test

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/ipcctrl"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// newDeclaredModeController builds a controller over a configuration that
// declares one mode, so the custom mode command has something to enter.
func newDeclaredModeController() (*ipcctrl.Controller, *state.AppState) {
	cfg := config.DefaultConfig()
	cfg.Modes = map[string]config.CustomModeConfig{
		"window": {Hotkeys: config.DefaultCustomModeHotkeys()},
	}

	logger := zap.NewNop()
	appState := state.NewAppState()
	actionService := services.NewActionService(
		&portmocks.MockAccessibilityPort{},
		&portmocks.MockOverlayPort{},
		&portmocks.MockSystemPort{},
		logger,
	)

	return ipcctrl.New(ipcctrl.Deps{
		ActionService: actionService,
		ConfigService: loader.NewService(cfg, "", logger, nil),
		AppState:      appState,
		Config:        cfg,
		Modes:         newTestModesHandler(cfg, logger, appState, actionService),
		Logger:        logger,
	}), appState
}

// TestHandleCommand_CustomModeCommandEntersADeclaredMode pins the wire shape
// of "mode <name>": the name travels as the first argument, a declared one is
// entered, and an undeclared one is refused in the grammar's words with the
// code a script branches on.
func TestHandleCommand_CustomModeCommandEntersADeclaredMode(t *testing.T) {
	controller, appState := newDeclaredModeController()

	resp := controller.HandleCommand(context.Background(), ipc.Command{
		Action: domain.ModeNameCustom,
		Args:   []string{"window"},
	})
	if !resp.Success {
		t.Fatalf("mode window was refused: %s", resp.Message)
	}

	if resp.Message != "custom mode activated" {
		t.Errorf("message = %q, want %q", resp.Message, "custom mode activated")
	}

	if got := appState.CurrentMode(); got != domain.ModeCustom {
		t.Errorf("mode = %v after the command, want ModeCustom", got)
	}

	refused := controller.HandleCommand(context.Background(), ipc.Command{
		Action: domain.ModeNameCustom,
		Args:   []string{"nobody"},
	})
	if refused.Success {
		t.Fatal("mode nobody was accepted; want a refusal")
	}

	if refused.Code != ipc.CodeInvalidInput {
		t.Errorf("code = %q, want %q", refused.Code, ipc.CodeInvalidInput)
	}

	if want := `mode "nobody" is not declared; declare it as [modes.nobody]`; refused.Message != want {
		t.Errorf("message = %q, want %q", refused.Message, want)
	}
}
