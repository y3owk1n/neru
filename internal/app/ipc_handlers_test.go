package app_test

import (
	"context"
	"encoding/json"
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
	// Only the fields this test needs; every other dependency is legitimately
	// the zero value, which the deps struct expresses by omission.
	return modes.NewHandler(modes.HandlerDeps{
		Ctx:                   context.Background(),
		Config:                cfg,
		Logger:                logger,
		AppState:              appState,
		CursorState:           state.NewCursorState(),
		ActionService:         actionService,
		RefreshHotkeys:        func() {},
		ExecuteActionSequence: func(string, []string) {},
		Shutdown:              func() {},
	})
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

	controller := app.NewIPCController(app.IPCControllerDeps{
		ActionService: actionService,
		ConfigService: configService,
		AppState:      appState,
		Config:        cfg,
		Modes:         newTestModesHandler(cfg, logger, appState, actionService),
		Logger:        logger,
	})

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

	controller := app.NewIPCController(app.IPCControllerDeps{
		ActionService: actionService,
		ConfigService: configService,
		AppState:      appState,
		Config:        cfg,
		Modes:         newTestModesHandler(cfg, logger, appState, actionService),
		Logger:        logger,
	})

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

	controller := app.NewIPCController(app.IPCControllerDeps{
		ActionService: actionService,
		ConfigService: configService,
		AppState:      appState,
		Config:        cfg,
		Modes:         newTestModesHandler(cfg, logger, appState, actionService),
		Logger:        logger,
	})

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

	controller := app.NewIPCController(app.IPCControllerDeps{
		ActionService: actionService,
		ConfigService: configService,
		AppState:      appState,
		Config:        cfg,
		Modes:         newTestModesHandler(cfg, logger, appState, actionService),
		Logger:        logger,
	})

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

func TestExtractModeOptions_ModifierEmptyList(t *testing.T) {
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

	controller := app.NewIPCController(app.IPCControllerDeps{
		ActionService: actionService,
		ConfigService: configService,
		AppState:      appState,
		Config:        cfg,
		Modes:         newTestModesHandler(cfg, logger, appState, actionService),
		Logger:        logger,
	})

	resp := controller.HandleCommand(context.Background(), ipc.Command{
		Action: actionHints,
		Args:   []string{actionHints, "--action=left_click", "--modifier=,"},
	})

	if resp.Success {
		t.Fatal(
			"HandleCommand() expected error response since --modifier value had no actual modifier names",
		)
	}

	expectedMsg := "modifier values cannot be empty"
	if !strings.Contains(resp.Message, expectedMsg) {
		t.Fatalf("expected error message containing %q, got: %q", expectedMsg, resp.Message)
	}
}

// newToggleTestController builds a controller over the real registration path,
// so these tests exercise the wiring rather than a handler called directly.
func newToggleTestController(
	appState *state.AppState,
	executeMacro func(context.Context, string, []string) error,
) *app.IPCController {
	cfg := config.DefaultConfig()
	logger := zap.NewNop()
	actionService := services.NewActionService(
		&portmocks.MockAccessibilityPort{},
		&portmocks.MockOverlayPort{},
		&portmocks.MockSystemPort{},
		logger,
	)

	return app.NewIPCController(app.IPCControllerDeps{
		ActionService: actionService,
		ConfigService: config.NewService(cfg, "", logger, nil),
		AppState:      appState,
		Config:        cfg,
		Modes:         newTestModesHandler(cfg, logger, appState, actionService),
		ExecuteMacro:  executeMacro,
		Logger:        logger,
	})
}

func TestHandleCommand_MacroReachesTheRunner(t *testing.T) {
	var (
		gotName string
		gotArgs []string
	)

	controller := newToggleTestController(
		state.NewAppState(),
		func(_ context.Context, name string, args []string) error {
			gotName, gotArgs = name, args

			return nil
		},
	)

	resp := controller.HandleCommand(context.Background(), ipc.Command{
		Action: "macro",
		Args:   []string{"window_click", "100", "70"},
	})

	if !resp.Success {
		t.Fatalf("HandleCommand(macro) = %+v, want success", resp)
	}

	if gotName != "window_click" {
		t.Fatalf("macro name = %q, want %q", gotName, "window_click")
	}

	if strings.Join(gotArgs, ",") != "100,70" {
		t.Fatalf("macro args = %v, want [100 70]", gotArgs)
	}
}

func TestHandleCommand_ToggleStateConvergesThroughTheController(t *testing.T) {
	appState := state.NewAppState()
	appState.SetScrollInverted(true)

	controller := newToggleTestController(appState, nil)

	// Asking twice for the same state must land there both times: that is what
	// a script relies on and what a bare toggle cannot give it.
	for range 2 {
		resp := controller.HandleCommand(context.Background(), ipc.Command{
			Action: "toggle-scroll-invert",
			Args:   []string{"--state=off"},
		})

		if !resp.Success {
			t.Fatalf("HandleCommand(toggle-scroll-invert) = %+v, want success", resp)
		}

		if appState.IsScrollInverted() {
			t.Fatal("scroll inverted = true, want --state=off to converge on off")
		}
	}
}

func TestHandleCommand_StatusReportsTheToggles(t *testing.T) {
	appState := state.NewAppState()
	appState.SetScrollInverted(true)
	appState.SetHiddenForScreenShare(true)

	controller := newToggleTestController(appState, nil)

	resp := controller.HandleCommand(context.Background(), ipc.Command{Action: "status"})
	if !resp.Success {
		t.Fatalf("HandleCommand(status) = %+v, want success", resp)
	}

	status, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("status data = %T, want map[string]any", resp.Data)
	}

	for key, want := range map[string]bool{
		"scroll_inverted":         true,
		"hidden_for_screen_share": true,
	} {
		got, isBool := status[key].(bool)
		if !isBool {
			t.Fatalf("status[%q] = %v (%T), want a bool", key, status[key], status[key])
		}

		if got != want {
			t.Fatalf("status[%q] = %t, want %t", key, got, want)
		}
	}

	// No mode is running, so the session-scoped toggle has no state to report.
	// Null and false are different answers and the payload must not conflate
	// them.
	value, present := status["cursor_follow_selection"]
	if !present {
		t.Fatal("status is missing cursor_follow_selection")
	}

	if follow, isPointer := value.(*bool); !isPointer || follow != nil {
		t.Fatalf("status[cursor_follow_selection] = %v, want nil while no mode runs", value)
	}

	// What a script actually reads is the encoded form, so pin that rather than
	// the Go value: "neru status --json | jq .cursor_follow_selection" has to
	// print null, not false and not a missing key.
	encoded, encodeErr := json.Marshal(status)
	if encodeErr != nil {
		t.Fatalf("json.Marshal(status) error = %v", encodeErr)
	}

	if !strings.Contains(string(encoded), `"cursor_follow_selection":null`) {
		t.Fatalf("encoded status does not report a null toggle:\n%s", encoded)
	}

	if !strings.Contains(string(encoded), `"scroll_inverted":true`) {
		t.Fatalf("encoded status does not report the scroll toggle:\n%s", encoded)
	}
}
