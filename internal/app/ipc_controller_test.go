package app_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	"go.uber.org/zap"
)

func newTestController() *app.IPCController {
	cfg := config.DefaultConfig()
	appState := state.NewAppState()
	logger, _ := zap.NewDevelopment()
	configService := config.NewService(cfg, "", logger)

	// Create controller with minimal dependencies for basic command testing
	controller := &app.IPCController{
		AppState:      appState,
		Config:        cfg,
		ConfigService: configService,
		Logger:        logger,
		ConfigPath:    "/test/config.toml",
		Handlers:      make(map[string]func(context.Context, ipc.Command) ipc.Response),
	}

	// Initialize handler components with nil dependencies where needed
	lifecycleHandler := app.NewIPCControllerLifecycle(appState, nil, logger)
	modesHandler := app.NewIPCControllerModes(nil, logger)
	infoHandler := app.NewIPCControllerInfo(
		configService,
		appState,
		cfg,
		nil, // modes
		nil, // hintService
		nil, // gridService
		nil, // actionService
		nil, // scrollService
		"/test/config.toml",
		logger,
	)

	// Register handlers from each component
	lifecycleHandler.RegisterHandlers(controller.Handlers)
	modesHandler.RegisterHandlers(controller.Handlers)
	infoHandler.RegisterHandlers(controller.Handlers)

	return controller
}

func TestIPCController_HandlePing(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	commandResponse := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandPing})

	if !commandResponse.Success {
		t.Errorf("Expected success=true, got %v", commandResponse.Success)
	}

	if commandResponse.Message != "pong" {
		t.Errorf("Expected message='pong', got %q", commandResponse.Message)
	}

	if commandResponse.Code != ipc.CodeOK {
		t.Errorf("Expected code=%s, got %s", ipc.CodeOK, commandResponse.Code)
	}
}

func TestIPCController_HandleStart(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	// Disable state first (NewAppState starts with enabled=true)
	controller.AppState.SetEnabled(false)

	// First start should succeed
	commandResponse := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandStart})
	if !commandResponse.Success {
		t.Errorf("Expected success=true, got %v", commandResponse.Success)
	}

	if !controller.AppState.IsEnabled() {
		t.Error("Expected state to be enabled after start")
	}

	// Second start should fail (already running)
	commandResponse = controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandStart})
	if commandResponse.Success {
		t.Error("Expected success=false when already running")
	}

	if commandResponse.Code != ipc.CodeAlreadyRunning {
		t.Errorf("Expected code=%s, got %s", ipc.CodeAlreadyRunning, commandResponse.Code)
	}
}

func TestIPCController_HandleStop(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	// Disable state first (NewAppState starts with enabled=true)
	controller.AppState.SetEnabled(false)

	// Stop when not running should fail
	commandResponse := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandStop})
	if commandResponse.Success {
		t.Error("Expected success=false when not running")
	}

	if commandResponse.Code != ipc.CodeNotRunning {
		t.Errorf("Expected code=%s, got %s", ipc.CodeNotRunning, commandResponse.Code)
	}

	// Start then stop should succeed
	controller.AppState.SetEnabled(true)

	commandResponse = controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandStop})
	if !commandResponse.Success {
		t.Errorf("Expected success=true, got %v", commandResponse.Success)
	}

	if controller.AppState.IsEnabled() {
		t.Error("Expected state to be disabled after stop")
	}
}

func TestIPCController_HandleConfig(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	commandResponse := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandConfig})
	if !commandResponse.Success {
		t.Errorf("Expected success=true, got %v", commandResponse.Success)
	}

	if commandResponse.Data == nil {
		t.Error("Expected non-nil data with config struct")
	}

	// Verify it's a valid config struct
	if cfg, ok := commandResponse.Data.(*config.Config); !ok {
		t.Errorf("Expected data to be *config.Config, got %T", commandResponse.Data)
	} else if cfg == nil {
		t.Error("Expected valid config struct, got nil")
	}
}

func TestIPCController_HandleActionAndScroll(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	// Test that the scroll handler can be called
	scrollResponse := controller.HandleCommand(ctx, ipc.Command{Action: "scroll"})
	if scrollResponse.Code == ipc.CodeUnknownCommand {
		t.Error("Scroll command should be recognized")
	}
}

func TestIPCController_UnknownCommand(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	commandResponse := controller.HandleCommand(ctx, ipc.Command{Action: "unknown_command"})
	if commandResponse.Success {
		t.Error("Expected success=false for unknown command")
	}

	if commandResponse.Code != ipc.CodeUnknownCommand {
		t.Errorf("Expected code=%s, got %s", ipc.CodeUnknownCommand, commandResponse.Code)
	}
}
