//go:build integration && darwin

package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
)

// requireCleanShutdown asserts that App.Run returned the way Stop is supposed to
// end it: either with no error, or with a cancellation of the run context.
//
// Any other error means teardown itself failed — a phase that could not unwind,
// an adapter that could not release a native resource — and must fail the test
// rather than being logged and ignored. Not returning at all is also a failure:
// it means Stop did not actually unblock Run.
func requireCleanShutdown(tb testing.TB, runDone <-chan error, timeout time.Duration) {
	tb.Helper()

	select {
	case err := <-runDone:
		if err == nil {
			return
		}

		if errors.Is(err, context.Canceled) ||
			derrors.IsCode(err, derrors.CodeContextCanceled) {
			return
		}

		tb.Errorf("Run() returned an unexpected error after Stop(): %v", err)
	case <-time.After(timeout):
		tb.Fatalf("App did not stop within %v", timeout)
	}
}

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
