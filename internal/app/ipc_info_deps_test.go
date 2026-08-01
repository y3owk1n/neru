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

// TestIPCControllerInfoDeps_ZeroValuesAreUsable pins the contract the deps
// struct documents: omitted fields are legitimate zero values, not a
// misconfiguration.
//
// The constructor previously took fourteen positional arguments, most of them
// nil at any given call site; this asserts the struct form preserves that
// tolerance instead of panicking on a partially-filled dependency set.
func TestIPCControllerInfoDeps_ZeroValuesAreUsable(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	logger := zap.NewNop()

	handler := app.NewIPCControllerInfo(app.IPCControllerInfoDeps{
		ConfigService: config.NewService(cfg, "", logger, nil),
		AppState:      state.NewAppState(),
		Config:        cfg,
		Logger:        logger,
		// Modes, services, System, EventTap, IPCServer, ReloadConfig and
		// SetConfigField are all deliberately omitted.
	})

	if handler == nil {
		t.Fatal("NewIPCControllerInfo() = nil")
	}

	handlers := make(map[string]func(context.Context, ipc.Command) ipc.Response)
	handler.RegisterHandlers(handlers)

	for _, action := range []string{
		domain.CommandStatus,
		domain.CommandConfig,
		domain.CommandHealth,
		domain.CommandReloadConfig,
		domain.CommandConfigSet,
	} {
		if handlers[action] == nil {
			t.Errorf("RegisterHandlers() registered no handler for %q", action)
		}
	}

	// A command that needs an absent dependency must report it, not panic.
	resp := handlers[domain.CommandReloadConfig](context.Background(), ipc.Command{
		Action: domain.CommandReloadConfig,
	})
	if resp.Success {
		t.Error("reload_config succeeded with no ReloadConfig callback wired")
	}

	// One that only needs what was supplied must still work.
	resp = handlers[domain.CommandStatus](context.Background(), ipc.Command{
		Action: domain.CommandStatus,
	})
	if !resp.Success {
		t.Errorf("status failed with a minimally-wired handler: %s", resp.Message)
	}
}
