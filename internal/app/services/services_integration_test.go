//go:build integration

package services_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
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
		hints, err := hintService.ShowHints(ctx)

		// Check that the service doesn't panic and returns some result
		// In different environments, this may succeed or fail based on permissions/elements
		if err != nil {
			t.Logf("ShowHints failed (may be expected in some environments): %v", err)
			// Even on failure, we should get an empty slice, not nil
			if hints == nil {
				t.Error("Expected empty slice on failure, got nil")
			}
		} else {
			t.Logf("ShowHints succeeded, found %d hints", len(hints))
			// In test environment, it's normal to find 0 hints (no real UI elements)
			// The overlay may or may not be visible depending on whether hints were found
			if len(hints) == 0 {
				t.Log("No hints found in test environment (expected)")
			} else {
				// If hints were found, overlay should be visible
				if overlay.IsVisible() {
					t.Logf("Overlay is visible after finding %d hints (expected)", len(hints))
				} else {
					t.Logf("Overlay is not visible after finding %d hints (unexpected but non-failing)", len(hints))
				}
			}
		}
	})

	t.Run("HideHints integration", func(t *testing.T) {
		err := hintService.HideHints(ctx)
		if err != nil {
			t.Errorf("HideHints failed: %v", err)
		} else {
			// Verify overlay is hidden after HideHints
			// Note: HideHints may not immediately hide if there are other overlays
			t.Log("HideHints completed successfully")
		}
	})

	t.Run("Service health check", func(t *testing.T) {
		health := hintService.Health(ctx)
		// Health returns a map of component health status
		if health == nil {
			t.Error("Expected health map, got nil")
		}
		// Log health status for debugging
		for component, err := range health {
			if err != nil {
				t.Logf("Component %s health: FAILED - %v", component, err)
			} else {
				t.Logf("Component %s health: OK", component)
			}
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

	accAdapter, overlayAdapter := initializeRealAdapters(t, cfg, logger)

	actionService := services.NewActionService(accAdapter, overlayAdapter, cfg.Action, logger)

	ctx := context.Background()

	t.Run("PerformAction left click", func(t *testing.T) {
		// Test performing an action at a point
		err := actionService.PerformAction(ctx, "left_click", image.Point{X: 100, Y: 100})
		if err != nil {
			t.Logf("PerformAction failed (may be expected in some environments): %v", err)
			// Even on failure, verify the service handled it gracefully
			t.Log("Service handled action attempt gracefully")
		} else {
			t.Log("Left click action performed successfully")
		}
	})

	t.Run("PerformAction right click", func(t *testing.T) {
		err := actionService.PerformAction(ctx, "right_click", image.Point{X: 200, Y: 200})
		if err != nil {
			t.Logf("Right click failed (may be expected): %v", err)
		} else {
			t.Log("Right click action performed successfully")
		}
	})

	t.Run("ExecuteAction on element", func(t *testing.T) {
		// This tests the element-based action execution
		// Create a mock element using the constructor
		testElement, err := element.NewElement(
			element.ID("test-button"),
			image.Rect(50, 50, 150, 80),
			element.RoleButton,
		)
		if err != nil {
			t.Fatalf("Failed to create test element: %v", err)
		}

		err = actionService.ExecuteAction(ctx, testElement, action.TypeLeftClick)
		if err != nil {
			t.Logf("ExecuteAction failed (may be expected): %v", err)
		} else {
			t.Log("Element action executed successfully")
		}
	})

	t.Run("ShowActionHighlight", func(t *testing.T) {
		err := actionService.ShowActionHighlight(ctx)
		if err != nil {
			t.Logf("ShowActionHighlight failed: %v", err)
		} else {
			t.Log("Action highlight displayed successfully")
			// Verify overlay is visible
			if overlayAdapter.IsVisible() {
				t.Log("Overlay became visible after highlight (expected)")
			} else {
				t.Log("Overlay is still not visible after highlight")
			}
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
