//go:build integration && darwin

package overlay_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/logger"
	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/adapter/platform"
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

	systemPort, systemPortErr := platform.NewSystemPort()
	if systemPortErr != nil {
		t.Fatalf("Failed to create system port: %v", systemPortErr)
	}

	adapter := overlay.NewAdapter(manager, theme, systemPort, logger)

	ctx := context.Background()

	t.Run("ShowHints", func(t *testing.T) {
		// ShowHints should not error with empty hints
		showHintsErr := adapter.ShowHints(ctx, nil)
		if showHintsErr != nil {
			t.Errorf("ShowHints() error = %v, want nil", showHintsErr)
		}
	})

	t.Run("ShowGrid", func(t *testing.T) {
		// ShowGrid should not error with valid dimensions
		showGridErr := adapter.ShowGrid(ctx)
		if showGridErr != nil {
			t.Errorf("ShowGrid() error = %v, want nil", showGridErr)
		}
	})

	t.Run("Hide", func(t *testing.T) {
		// Hide should not error
		hideErr := adapter.Hide(ctx)
		if hideErr != nil {
			t.Errorf("Hide() error = %v, want nil", hideErr)
		}
	})

	t.Run("Refresh", func(t *testing.T) {
		// Refresh should not error
		refreshErr := adapter.Refresh(ctx)
		if refreshErr != nil {
			t.Errorf("Refresh() error = %v, want nil", refreshErr)
		}
	})

	t.Run("IsVisible tracks show and hide", func(t *testing.T) {
		// Unlike the service-level tests, this runs against a real manager, so
		// visibility is genuinely observable. The mode handler relies on this
		// flag to decide whether an overlay still needs tearing down, so it
		// must follow show/hide rather than being constant.
		err := adapter.Hide(ctx)
		if err != nil {
			t.Fatalf("Hide() error = %v, want nil", err)
		}

		if adapter.IsVisible() {
			t.Fatal("IsVisible() = true after Hide(), want false")
		}

		err = adapter.ShowGrid(ctx)
		if err != nil {
			t.Fatalf("ShowGrid() error = %v, want nil", err)
		}

		if !adapter.IsVisible() {
			t.Error("IsVisible() = false after ShowGrid(), want true")
		}

		err = adapter.Hide(ctx)
		if err != nil {
			t.Fatalf("Hide() error = %v, want nil", err)
		}

		if adapter.IsVisible() {
			t.Error("IsVisible() = true after Hide(), want false")
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

	systemPort, systemPortErr := platform.NewSystemPort()
	if systemPortErr != nil {
		t.Fatalf("Failed to create system port: %v", systemPortErr)
	}

	adapter := overlay.NewAdapter(manager, theme, systemPort, logger)

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("ShowHints with canceled context", func(t *testing.T) {
		showHintsErr := adapter.ShowHints(ctx, nil)
		if !errors.Is(showHintsErr, context.Canceled) {
			t.Errorf(
				"ShowHints() with canceled context error = %v, want %v",
				showHintsErr,
				context.Canceled,
			)
		}
	})

	t.Run("ShowGrid with canceled context", func(t *testing.T) {
		showGridErr := adapter.ShowGrid(ctx)
		if !errors.Is(showGridErr, context.Canceled) {
			t.Errorf(
				"ShowGrid() with canceled context error = %v, want %v",
				showGridErr,
				context.Canceled,
			)
		}
	})
}
