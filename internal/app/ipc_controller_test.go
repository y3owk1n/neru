package app_test

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

func newTestController() *app.IPCController {
	cfg := config.DefaultConfig()
	appState := state.NewAppState()
	logger, _ := zap.NewDevelopment()
	configService := config.NewService(cfg, "", logger, nil)

	return app.NewIPCController(
		nil, // hintService
		nil, // gridService
		nil, // actionService
		nil, // scrollService
		configService,
		appState,
		cfg,
		nil, // modesHandler
		nil, // systemPort
		nil, // eventTap
		nil, // ipcServer
		nil, // reloadConfig
		logger,
	)
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

func TestIPCController_UpdateConfig(t *testing.T) {
	controller := newTestController()
	ctx := context.Background()
	// Verify initial config is returned
	commandResponse := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandConfig})
	if !commandResponse.Success {
		t.Fatalf("Expected success=true, got %v", commandResponse.Success)
	}

	initialCfg, initialCfgOk := commandResponse.Data.(*config.Config)
	if !initialCfgOk || initialCfg == nil {
		t.Fatal("Expected valid initial config")
	}
	// Create a new config with different values
	newCfg := config.DefaultConfig()
	newCfg.Hints.Enabled = !initialCfg.Hints.Enabled
	// Propagate via UpdateConfig
	controller.UpdateConfig(newCfg)
	// Verify the config handler now returns the updated config
	commandResponse = controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandConfig})
	if !commandResponse.Success {
		t.Fatalf("Expected success=true after update, got %v", commandResponse.Success)
	}

	updatedCfg, initialCfgOk := commandResponse.Data.(*config.Config)
	if !initialCfgOk || updatedCfg == nil {
		t.Fatal("Expected valid updated config")
	}

	if updatedCfg.Hints.Enabled != newCfg.Hints.Enabled {
		t.Errorf("Expected Hints.Enabled=%v after UpdateConfig, got %v",
			newCfg.Hints.Enabled, updatedCfg.Hints.Enabled)
	}
}
