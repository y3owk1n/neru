package app_test

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	portmocks "github.com/y3owk1n/neru/internal/core/ports/mocks"
)

const (
	actionGrid   = "grid"
	actionHints  = "hints"
	actionScroll = "scroll"
)

func newTestModesHandler(
	cfg *config.Config,
	logger *zap.Logger,
	appState *state.AppState,
	actionService *services.ActionService,
) *modes.Handler {
	return modes.NewHandler(
		context.Background(),
		cfg,
		logger,
		appState,
		state.NewCursorState(),
		nil,                     // overlayManager
		nil,                     // renderer
		nil,                     // hintService
		nil,                     // gridService
		actionService,           // actionService
		nil,                     // scrollService
		nil,                     // modeIndicatorService
		nil,                     // stickyIndicatorService
		nil,                     // hintsComponent
		nil,                     // grid
		nil,                     // scroll
		nil,                     // recursiveGridComponent
		func() {},               // enableEventTap
		func() {},               // disableEventTap
		func(bool, []string) {}, // setModifierPassthrough
		func([]string) {},       // setInterceptedModifierKeys
		func(func()) {},         // setPassthroughCallback
		func(bool) {},           // setStickyModifierToggle
		func(string, bool) {},   // postModifierEvent
		func() {},               // refreshHotkeys
		func(string, string) error { return nil }, // executeHotkeyAction
		func() {}, // shutdown
		nil,       // textInput
		nil,       // systemPort
	)
}

func TestExtractModeOptions_InvalidCursorSelectionModeEqualsValue(t *testing.T) {
	cfg := config.DefaultConfig()
	appState := state.NewAppState()
	logger := zap.NewNop()
	configService := config.NewService(cfg, "", logger, nil)
	actionService := services.NewActionService(
		&portmocks.MockAccessibilityPort{},
		&portmocks.MockOverlayPort{},
		&portmocks.MockSystemPort{},
		logger,
	)

	controller := app.NewIPCController(
		nil,
		nil,
		actionService,
		nil,
		configService,
		appState,
		cfg,
		newTestModesHandler(cfg, logger, appState, actionService),
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
	actionService := services.NewActionService(
		&portmocks.MockAccessibilityPort{},
		&portmocks.MockOverlayPort{},
		&portmocks.MockSystemPort{},
		logger,
	)

	controller := app.NewIPCController(
		nil,
		nil,
		actionService,
		nil,
		configService,
		appState,
		cfg,
		newTestModesHandler(cfg, logger, appState, actionService),
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
	actionService := services.NewActionService(
		&portmocks.MockAccessibilityPort{},
		&portmocks.MockOverlayPort{},
		&portmocks.MockSystemPort{},
		logger,
	)

	controller := app.NewIPCController(
		nil,
		nil,
		actionService,
		nil,
		configService,
		appState,
		cfg,
		newTestModesHandler(cfg, logger, appState, actionService),
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
		"move_mouse",
		"move_mouse_relative",
		actionScroll,
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

func TestExtractModeOptions_ModifierRequiresAction(t *testing.T) {
	cfg := config.DefaultConfig()
	appState := state.NewAppState()
	logger := zap.NewNop()
	configService := config.NewService(cfg, "", logger, nil)
	actionService := services.NewActionService(
		&portmocks.MockAccessibilityPort{},
		&portmocks.MockOverlayPort{},
		&portmocks.MockSystemPort{},
		logger,
	)

	controller := app.NewIPCController(
		nil,
		nil,
		actionService,
		nil,
		configService,
		appState,
		cfg,
		newTestModesHandler(cfg, logger, appState, actionService),
		nil,
		nil,
		nil,
		nil,
		logger,
	)

	resp := controller.HandleCommand(context.Background(), ipc.Command{
		Action: actionHints,
		Args:   []string{actionHints, "--modifier=shift"},
	})

	if resp.Success {
		t.Fatal(
			"HandleCommand() expected error response since --modifier was passed without --action",
		)
	}

	expectedMsg := "--modifier requires an action"
	if !strings.Contains(resp.Message, expectedMsg) {
		t.Fatalf("expected error message containing %q, got: %q", expectedMsg, resp.Message)
	}
}
