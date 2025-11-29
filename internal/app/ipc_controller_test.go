//go:build unit

package app_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	"github.com/y3owk1n/neru/internal/core/infra/metrics"
	"go.uber.org/zap"
)

func newTestController() *app.IPCController {
	cfg := config.DefaultConfig()
	appState := state.NewAppState()
	logger, _ := zap.NewDevelopment()
	metricsCollector := metrics.NewCollector()
	configService := config.NewService(cfg, "")

	// Create controller with nil services for basic command testing
	return &app.IPCController{
		AppState:      appState,
		Config:        cfg,
		ConfigService: configService,
		Logger:        logger,
		Metrics:       metricsCollector,
		ConfigPath:    "/test/config.toml",
		Handlers:      make(map[string]func(context.Context, ipc.Command) ipc.Response),
	}
}

func TestIPCController_HandlePing(t *testing.T) {
	controller := newTestController()
	controller.RegisterHandlers()

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
	controller.RegisterHandlers()

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
	controller.RegisterHandlers()

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
	controller.RegisterHandlers()

	ctx := context.Background()

	commandResponse := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandConfig})
	if !commandResponse.Success {
		t.Errorf("Expected success=true, got %v", commandResponse.Success)
	}

	cfg, ok := commandResponse.Data.(*config.Config)
	if !ok {
		t.Fatalf("Expected *config.Config, got %T", commandResponse.Data)
	}

	if cfg == nil {
		t.Error("Expected non-nil config")
	}
}

func TestIPCController_HandleMetrics(t *testing.T) {
	controller := newTestController()
	controller.RegisterHandlers()

	ctx := context.Background()

	commandResponse := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandMetrics})
	if !commandResponse.Success {
		t.Errorf("Expected success=true, got %v", commandResponse.Success)
	}

	// Metrics should return a snapshot (slice of metrics.Metric)
	if commandResponse.Data == nil {
		t.Error("Expected non-nil metrics data")
	}
}

func TestIPCController_HandleAction(t *testing.T) {
	controller := newTestController()
	controller.RegisterHandlers()

	// Test that the action handler is registered
	if controller.Handlers["action"] == nil {
		t.Error("Expected action handler to be registered")
	}

	// Test that the scroll handler is registered
	if controller.Handlers["scroll"] == nil {
		t.Error("Expected scroll handler to be registered")
	}

	// Test that the action handler can be called (even if it fails due to nil services)
	ctx := context.Background()
	commandResponse := controller.HandleCommand(ctx, ipc.Command{Action: "action"})
	if commandResponse.Code == ipc.CodeUnknownCommand {
		t.Error("Action command should be recognized")
	}

	// Test that the scroll handler can be called
	scrollResponse := controller.HandleCommand(ctx, ipc.Command{Action: "scroll"})
	if scrollResponse.Code == ipc.CodeUnknownCommand {
		t.Error("Scroll command should be recognized")
	}
}

func TestIPCController_UnknownCommand(t *testing.T) {
	controller := newTestController()
	controller.RegisterHandlers()

	ctx := context.Background()

	commandResponse := controller.HandleCommand(ctx, ipc.Command{Action: "unknown_command"})
	if commandResponse.Success {
		t.Error("Expected success=false for unknown command")
	}

	if commandResponse.Code != ipc.CodeUnknownCommand {
		t.Errorf("Expected code=%s, got %s", ipc.CodeUnknownCommand, commandResponse.Code)
	}
}
