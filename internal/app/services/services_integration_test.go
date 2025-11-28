//go:build integration

package services_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/infra/accessibility"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/infra/metrics"
	overlayAdapter "github.com/y3owk1n/neru/internal/core/infra/overlay"
	"github.com/y3owk1n/neru/internal/core/ports"
	uiOverlay "github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

// TestHintServiceIntegration tests the hint service with real adapters
func TestHintServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()

	// Create real adapters (they will be initialized with real infra)
	cfg := config.DefaultConfig()
	cfg.Hints.Enabled = true
	cfg.General.AccessibilityCheckOnStart = false

	// Initialize real adapters like the app does
	accAdapter, overlay := initializeRealAdapters(t, cfg, logger)

	// Create hint generator
	hintGen, err := hint.NewAlphabetGenerator(cfg.Hints.HintCharacters)
	if err != nil {
		t.Fatalf("Failed to create hint generator: %v", err)
	}

	// Create hint service
	hintService := services.NewHintService(accAdapter, overlay, hintGen, cfg.Hints, logger)

	ctx := context.Background()

	t.Run("ShowHints integration", func(t *testing.T) {
		// This tests the full pipeline: accessibility -> hint generation -> overlay
		_, err := hintService.ShowHints(ctx)
		if err != nil {
			// It's okay if this fails due to permissions or no elements
			t.Logf("ShowHints failed (expected in some environments): %v", err)
		}
	})

	t.Run("HideHints integration", func(t *testing.T) {
		err := hintService.HideHints(ctx)
		if err != nil {
			t.Logf("HideHints failed: %v", err)
		}
	})
}

// TestActionServiceIntegration tests the action service with real adapters
func TestActionServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()

	cfg := config.DefaultConfig()
	cfg.General.AccessibilityCheckOnStart = false

	accAdapter, _ := initializeRealAdapters(t, cfg, logger)

	actionService := services.NewActionService(accAdapter, nil, cfg.Action, logger)

	ctx := context.Background()

	t.Run("PerformAction integration", func(t *testing.T) {
		// Test performing an action at a point
		err := actionService.PerformAction(ctx, "left_click", image.Point{X: 100, Y: 100})
		if err != nil {
			t.Logf("PerformAction failed (expected in some environments): %v", err)
		}
	})
}

// TestGridServiceIntegration tests the grid service with real adapters
func TestGridServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()

	cfg := config.DefaultConfig()
	cfg.Grid.Enabled = true
	cfg.General.AccessibilityCheckOnStart = false

	_, overlay := initializeRealAdapters(t, cfg, logger)

	gridService := services.NewGridService(overlay, logger)

	ctx := context.Background()

	t.Run("ShowGrid integration", func(t *testing.T) {
		err := gridService.ShowGrid(ctx)
		if err != nil {
			t.Logf("ShowGrid failed: %v", err)
		}
	})

	t.Run("HideGrid integration", func(t *testing.T) {
		err := gridService.HideGrid(ctx)
		if err != nil {
			t.Logf("HideGrid failed: %v", err)
		}
	})
}

// TestScrollServiceIntegration tests the scroll service with real adapters
func TestScrollServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()

	cfg := config.DefaultConfig()
	cfg.General.AccessibilityCheckOnStart = false

	accAdapter, overlay := initializeRealAdapters(t, cfg, logger)

	scrollService := services.NewScrollService(accAdapter, overlay, cfg.Scroll, logger)

	ctx := context.Background()

	t.Run("Scroll integration", func(t *testing.T) {
		err := scrollService.Scroll(
			ctx,
			services.ScrollDirectionDown,
			services.ScrollAmountHalfPage,
		)
		if err != nil {
			t.Logf("Scroll failed (expected in some environments): %v", err)
		}
	})
}

// Helper function to initialize real adapters like the app does
func initializeRealAdapters(
	t *testing.T,
	cfg *config.Config,
	logger *zap.Logger,
) (ports.AccessibilityPort, ports.OverlayPort) {
	t.Helper()

	// Initialize metrics collector
	metricsCollector := metrics.NewCollector()

	// Create infrastructure client
	axClient := accessibility.NewInfraAXClient(logger)

	// Create base accessibility adapter
	baseAccessibilityAdapter := accessibility.NewAdapter(
		logger,
		cfg.General.ExcludedApps,
		cfg.Hints.ClickableRoles,
		axClient,
	)

	// Wrap with metrics decorator
	accAdapter := accessibility.NewMetricsDecorator(
		baseAccessibilityAdapter,
		metricsCollector,
	)

	// Initialize overlay manager
	overlayManager := uiOverlay.Init(logger)

	// Create overlay adapter
	baseOverlayAdapter := overlayAdapter.NewAdapter(overlayManager, logger)

	// Wrap with metrics decorator
	overlay := overlayAdapter.NewMetricsDecorator(baseOverlayAdapter, metricsCollector)

	return accAdapter, overlay
}
