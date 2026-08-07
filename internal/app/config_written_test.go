package app

import (
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
)

// TestNewConfigService_CarriesTheWrittenConfig pins the handoff the daemon
// depends on. The loader produces two halves — the configuration to run on and
// the one the user wrote — and only the second can answer whether a derived
// value was inferred. Dropping it here compiles, runs, and silently puts
// `neru config set grid.characters` back to relabelling nothing.
func TestNewConfigService_CarriesTheWrittenConfig(t *testing.T) {
	written := config.DefaultConfigForDecoding()
	written.Grid.Characters = "asdf"

	running, copyErr := loader.DeepCopyConfig(written)
	if copyErr != nil {
		t.Fatalf("deep copy failed: %v", copyErr)
	}

	loader.ResolveDerived(running)

	app := &App{config: running, writtenConfig: written}

	service := app.newConfigService(zap.NewNop())

	if got := service.Written().Grid.RowLabels; got != "" {
		t.Errorf("Written().Grid.RowLabels = %q, want %q", got, "")
	}

	if got := service.Get().Grid.RowLabels; got != "ASDF" {
		t.Errorf("Get().Grid.RowLabels = %q, want %q", got, "ASDF")
	}
}

// TestNewConfigService_WithoutAWrittenConfig covers the constructors that have
// no load behind them (tests, embedders): the service still answers, with the
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
