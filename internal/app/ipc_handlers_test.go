package app_test

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
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
		logger,
	)

	resp := controller.HandleCommand(context.Background(), ipc.Command{
		Action: "grid",
		Args:   []string{"grid", "--cursor-selection-mode=invalid"},
	})

	if resp.Success {
		t.Fatal("HandleCommand() expected error response")
	}

	if resp.Message != "--cursor-selection-mode requires follow or hold" {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}
