package app_test

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

const (
	actionGrid  = "grid"
	actionHints = "hints"
)

func TestExtractModeOptions_InvalidCursorSelectionModeEqualsValue(t *testing.T) {
	cfg := config.DefaultConfig()
	appState := state.NewAppState()
	logger := zap.NewNop()
	configService := config.NewService(cfg, "", logger, nil)

	controller := app.NewIPCController(
		nil,
		nil,
		nil,
		nil,
		configService,
		appState,
		cfg,
		&modes.Handler{},
		nil,
		nil,
		nil,
		nil,
		logger,
	)

	resp := controller.HandleCommand(context.Background(), ipc.Command{
		Action: actionGrid,
		Args:   []string{actionGrid, "--cursor-selection-mode=invalid"},
	})

	if resp.Success {
		t.Fatal("HandleCommand() expected error response")
	}

	if resp.Message != "--cursor-selection-mode requires follow or hold" {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestExtractModeOptions_InvalidLabelDirection(t *testing.T) {
	cfg := config.DefaultConfig()
	appState := state.NewAppState()
	logger := zap.NewNop()
	configService := config.NewService(cfg, "", logger, nil)

	controller := app.NewIPCController(
		nil,
		nil,
		nil,
		nil,
		configService,
		appState,
		cfg,
		&modes.Handler{},
		nil,
		nil,
		nil,
		nil,
		logger,
	)

	resp := controller.HandleCommand(context.Background(), ipc.Command{
		Action: actionHints,
		Args:   []string{actionHints, "--label-direction=sideways"},
	})

	if resp.Success {
		t.Fatal("HandleCommand() expected error response")
	}

	if !strings.Contains(resp.Message, "--label-direction") {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestExtractModeOptions_InvalidModeAction(t *testing.T) {
	cfg := config.DefaultConfig()
	appState := state.NewAppState()
	logger := zap.NewNop()
	configService := config.NewService(cfg, "", logger, nil)

	controller := app.NewIPCController(
		nil,
		nil,
		nil,
		nil,
		configService,
		appState,
		cfg,
		&modes.Handler{},
		nil,
		nil,
		nil,
		nil,
		logger,
	)

	disallowedActions := []string{
		"move_monitor",
		"cycle_hint",
		"search_hints",
		"feed",
		"sleep",
		"reset",
	}
	for _, act := range disallowedActions {
		resp := controller.HandleCommand(context.Background(), ipc.Command{
			Action: actionHints,
			Args:   []string{actionHints, "--action=" + act},
		})

		if resp.Success {
			t.Fatalf(
				"HandleCommand() with disallowed action %q expected error response, but succeeded",
				act,
			)
		}

		expectedMsg := "is not allowed; use 'action " + act + "' instead"
		if !strings.Contains(resp.Message, expectedMsg) {
			t.Fatalf(
				"disallowed action %q error message %q does not contain %q",
				act,
				resp.Message,
				expectedMsg,
			)
		}
	}
}
