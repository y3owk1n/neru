package app

import (
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

// TestNewConfigService_WithoutAWrittenConfig covers the constructors that have
// no load behind them (tests, embedders). The service still answers, with the
// configuration it was given, so a field change normalizes what was typed even
// though it cannot re-infer.
func TestNewConfigService_WithoutAWrittenConfig(t *testing.T) {
	running := config.DefaultConfig()

	app := &App{config: running}

	service := app.newConfigService(zap.NewNop())

	if service.Written() != running {
		t.Error("Written() did not fall back to the running configuration")
	}
}
