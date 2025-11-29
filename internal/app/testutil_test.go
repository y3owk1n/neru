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
	t *testing.T,
	application *app.App,
	expectedMode domain.Mode,
	timeout time.Duration,
) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if application.CurrentMode() == expectedMode {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf(
		"Timeout waiting for mode %v, current mode: %v",
		expectedMode,
		application.CurrentMode(),
	)
}

// waitForAppReady waits for the application to be enabled with a timeout.
func waitForAppReady(t testing.TB, application *app.App, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if application.IsEnabled() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("App did not start within %v timeout", timeout)
}
