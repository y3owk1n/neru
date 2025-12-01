//go:build unit || integration

package app_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/core/domain"
)

// waitForMode waits for the application to reach the expected mode with a timeout.
func waitForMode(
	tb testing.TB,
	application *app.App,
	expectedMode domain.Mode,
) {
	tb.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if application.CurrentMode() == expectedMode {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	tb.Fatalf(
		"Timeout waiting for mode %v, current mode: %v",
		expectedMode,
		application.CurrentMode(),
	)
}

// waitForAppReady waits for the application to be enabled with a timeout.
func waitForAppReady(tb testing.TB, application *app.App) {
	tb.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if application.IsEnabled() {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	tb.Fatalf("App did not start within 5 seconds")
}
