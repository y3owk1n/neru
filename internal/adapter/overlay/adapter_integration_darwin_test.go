//go:build integration && darwin

package overlay_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/logger"
	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ports"
)

// testThemeProvider is a simple ThemeProvider mock for integration tests.
type testThemeProvider struct {
	darkMode bool
}

func (t *testThemeProvider) IsDarkMode() bool {
	return t.darkMode
}

// requireDesktop skips unless this run opted into tests that drive the real
// desktop (cursor, keyboard, overlays). `just test-desktop` sets the variable;
// plain `just test` stays hands-off the machine.
func requireDesktop(t *testing.T) {
	t.Helper()

	if os.Getenv("NERU_DESKTOP_TESTS") == "" {
		t.Skip("skipping desktop-driving test; run `just test-desktop` to include it")
	}
}

// TestOverlayAdapterIntegration tests the overlay adapter with real dependencies.
func TestOverlayAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	requireDesktop(t)

	if os.Getenv("CI") != "" {
		t.Skip("Skipping overlay integration test in CI (no window server run loop)")
	}

	// Setup
	logger := logger.Get()
	manager := overlay.Init(logger)
	theme := &testThemeProvider{darkMode: false}
	styles := overlay.NewStyleResolver(manager, config.DefaultConfig(), theme, logger)

	adapter := overlay.NewAdapter(manager, styles, logger)

	ctx := context.Background()

	emptyHints := ports.HintsFrame{}

	t.Run("ShowFrame", func(t *testing.T) {
		// Showing a frame should not error, even with nothing in it
		showErr := adapter.ShowFrame(ctx, emptyHints)
		if showErr != nil {
			t.Errorf("ShowFrame() error = %v, want nil", showErr)
		}
	})

	t.Run("ClearFrame", func(t *testing.T) {
		// Clearing should not error
		clearErr := adapter.ClearFrame(ctx)
		if clearErr != nil {
			t.Errorf("ClearFrame() error = %v, want nil", clearErr)
		}
	})

	t.Run("Refresh", func(t *testing.T) {
		// Refresh should not error
		refreshErr := adapter.Refresh(ctx)
		if refreshErr != nil {
			t.Errorf("Refresh() error = %v, want nil", refreshErr)
		}
	})

	t.Run("IsVisible tracks the frame on screen", func(t *testing.T) {
		// Unlike the service-level tests, this runs against a real manager, so
		// visibility is genuinely observable. The mode handler relies on this
		// flag to decide whether an overlay still needs tearing down, so it
		// must follow the frame rather than being constant.
		err := adapter.ClearFrame(ctx)
		if err != nil {
			t.Fatalf("ClearFrame() error = %v, want nil", err)
		}

		if adapter.IsVisible() {
			t.Fatal("IsVisible() = true after ClearFrame(), want false")
		}

		err = adapter.ShowFrame(ctx, emptyHints)
		if err != nil {
			t.Fatalf("ShowFrame() error = %v, want nil", err)
		}

		if !adapter.IsVisible() {
			t.Error("IsVisible() = false after ShowFrame(), want true")
		}

		err = adapter.ClearFrame(ctx)
		if err != nil {
			t.Fatalf("ClearFrame() error = %v, want nil", err)
		}

		if adapter.IsVisible() {
			t.Error("IsVisible() = true after ClearFrame(), want false")
		}
	})
}

// TestOverlayAdapterContextCancellation tests context cancellation handling.
func TestOverlayAdapterContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	requireDesktop(t)

	if os.Getenv("CI") != "" {
		t.Skip("Skipping overlay integration test in CI (no window server run loop)")
	}

	logger := logger.Get()
	manager := overlay.Init(logger)
	theme := &testThemeProvider{darkMode: false}
	styles := overlay.NewStyleResolver(manager, config.DefaultConfig(), theme, logger)

	adapter := overlay.NewAdapter(manager, styles, logger)

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("ShowFrame with canceled context", func(t *testing.T) {
		showErr := adapter.ShowFrame(ctx, ports.HintsFrame{})
		if !errors.Is(showErr, context.Canceled) {
			t.Errorf(
				"ShowFrame() with canceled context error = %v, want %v",
				showErr,
				context.Canceled,
			)
		}
	})
}
