package app

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/infra/ipc"
	"github.com/y3owk1n/neru/internal/infra/metrics"
	"go.uber.org/zap"
)

func newTestController() *IPCController {
	cfg := config.DefaultConfig()
	appState := state.NewAppState()
	logger, _ := zap.NewDevelopment()
	metricsCollector := metrics.NewCollector()
	configService := config.NewService(cfg, "")

	// Create controller with nil services for basic command testing
	return &IPCController{
		state:         appState,
		config:        cfg,
		configService: configService,
		logger:        logger,
		metrics:       metricsCollector,
		configPath:    "/test/config.toml",
		handlers:      make(map[string]func(context.Context, ipc.Command) ipc.Response),
	}
}

func TestIPCController_HandlePing(t *testing.T) {
	ctrl := newTestController()
	ctrl.registerHandlers()
	ctx := context.Background()

	resp := ctrl.HandleCommand(ctx, ipc.Command{Action: domain.CommandPing})

	if !resp.Success {
		t.Errorf("Expected success=true, got %v", resp.Success)
	}
	if resp.Message != "pong" {
		t.Errorf("Expected message='pong', got %q", resp.Message)
	}
	if resp.Code != ipc.CodeOK {
		t.Errorf("Expected code=%s, got %s", ipc.CodeOK, resp.Code)
	}
}

func TestIPCController_HandleStart(t *testing.T) {
	ctrl := newTestController()
	ctrl.registerHandlers()
	ctx := context.Background()

	// Disable state first (NewAppState starts with enabled=true)
	ctrl.state.SetEnabled(false)

	// First start should succeed
	resp := ctrl.HandleCommand(ctx, ipc.Command{Action: domain.CommandStart})
	if !resp.Success {
		t.Errorf("Expected success=true, got %v", resp.Success)
	}
	if !ctrl.state.IsEnabled() {
		t.Error("Expected state to be enabled after start")
	}

	// Second start should fail (already running)
	resp = ctrl.HandleCommand(ctx, ipc.Command{Action: domain.CommandStart})
	if resp.Success {
		t.Error("Expected success=false when already running")
	}
	if resp.Code != ipc.CodeAlreadyRunning {
		t.Errorf("Expected code=%s, got %s", ipc.CodeAlreadyRunning, resp.Code)
	}
}

func TestIPCController_HandleStop(t *testing.T) {
	ctrl := newTestController()
	ctrl.registerHandlers()
	ctx := context.Background()

	// Disable state first (NewAppState starts with enabled=true)
	ctrl.state.SetEnabled(false)

	// Stop when not running should fail
	resp := ctrl.HandleCommand(ctx, ipc.Command{Action: domain.CommandStop})
	if resp.Success {
		t.Error("Expected success=false when not running")
	}
	if resp.Code != ipc.CodeNotRunning {
		t.Errorf("Expected code=%s, got %s", ipc.CodeNotRunning, resp.Code)
	}

	// Start then stop should succeed
	ctrl.state.SetEnabled(true)
	resp = ctrl.HandleCommand(ctx, ipc.Command{Action: domain.CommandStop})
	if !resp.Success {
		t.Errorf("Expected success=true, got %v", resp.Success)
	}
	if ctrl.state.IsEnabled() {
		t.Error("Expected state to be disabled after stop")
	}
}

func TestIPCController_HandleConfig(t *testing.T) {
	ctrl := newTestController()
	ctrl.registerHandlers()
	ctx := context.Background()

	resp := ctrl.HandleCommand(ctx, ipc.Command{Action: domain.CommandConfig})
	if !resp.Success {
		t.Errorf("Expected success=true, got %v", resp.Success)
	}

	cfg, ok := resp.Data.(*config.Config)
	if !ok {
		t.Fatalf("Expected *config.Config, got %T", resp.Data)
	}
	if cfg == nil {
		t.Error("Expected non-nil config")
	}
}

func TestIPCController_HandleMetrics(t *testing.T) {
	ctrl := newTestController()
	ctrl.registerHandlers()
	ctx := context.Background()

	resp := ctrl.HandleCommand(ctx, ipc.Command{Action: domain.CommandMetrics})
	if !resp.Success {
		t.Errorf("Expected success=true, got %v", resp.Success)
	}

	// Metrics should return a snapshot (slice of metrics.Metric)
	if resp.Data == nil {
		t.Error("Expected non-nil metrics data")
	}
}

func TestIPCController_UnknownCommand(t *testing.T) {
	ctrl := newTestController()
	ctrl.registerHandlers()
	ctx := context.Background()

	resp := ctrl.HandleCommand(ctx, ipc.Command{Action: "unknown_command"})
	if resp.Success {
		t.Error("Expected success=false for unknown command")
	}
	if resp.Code != ipc.CodeUnknownCommand {
		t.Errorf("Expected code=%s, got %s", ipc.CodeUnknownCommand, resp.Code)
	}
}
