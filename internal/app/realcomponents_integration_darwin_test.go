//go:build integration && darwin

package app_test

// The one thing the simulation journeys cannot prove: that the app builds and
// runs against the real macOS adapters. This constructs the app WITHOUT
// injecting the system port, accessibility port, or event tap, so the real
// platform implementations are exercised — permission checks, AX client setup,
// event tap creation (which degrades gracefully without Input Monitoring).
//
// Everything behavioral (mode journeys, hint selection, clicks, scrolling)
// lives in the simulation journeys in simulation_journey_test.go; keep this
// file to construction, startup, one real-AX mode activation, and shutdown.
//
// Skipped under -short (the CI profile): hints activation drives the real
// accessibility tree, which needs Accessibility permission and a real desktop.

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/ports/mocks"
)

// requireDesktop skips unless this run opted into tests that drive the real
// desktop (cursor, keyboard, overlays). `just test-desktop` sets the variable;
// plain `just test` stays hands-off the machine.
func requireDesktop(t *testing.T) {
	t.Helper()

	if os.Getenv("NERU_DESKTOP_TESTS") == "" {
		t.Skip("skipping desktop-driving test; run `just test-desktop` to include it")
	}
}

func TestRealComponents_StartupModesShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real-component test in short mode (needs Accessibility permission)")
	}

	// Activating hints reads the real AX tree and enables a real event tap,
	// which briefly intercepts the developer's keyboard.
	requireDesktop(t)

	cfg := config.DefaultConfig()

	// Overlay, IPC, watcher and hotkeys stay mocked: they would create native
	// windows, bind sockets, and register global hotkeys on the developer's
	// machine. System port, accessibility and event tap stay real — that is
	// what this test exists to cover.
	application, err := app.New(
		app.WithConfig(cfg),
		app.WithConfigPath(""),
		app.WithIPCServer(&mocks.MockIPCPort{}),
		app.WithWatcher(&mocks.MockAppWatcherPort{}),
		app.WithOverlayPort(&simOverlayPort{}),
		app.WithHotkeyService(&simHotkeyPort{}),
	)
	if err != nil {
		t.Fatalf("app.New with real components failed: %v", err)
	}
	defer application.Cleanup()

	runDone := make(chan error, 1)

	go func() {
		runDone <- application.Run()
	}()

	waitForCondition(t, "app running", application.IsEnabled)

	if application.CurrentMode() != domain.ModeIdle {
		t.Fatalf("expected idle after startup, got %v", application.CurrentMode())
	}

	// Grid is accessibility-independent and must work against the real system
	// port on any macOS session.
	application.SetModeGrid()
	waitForCondition(t, "grid mode", func() bool {
		return application.CurrentMode() == domain.ModeGrid
	})

	application.SetModeIdle()
	waitForCondition(t, "idle mode", func() bool {
		return application.CurrentMode() == domain.ModeIdle
	})

	// Hints drives the real AX tree. On a permissioned developer machine this
	// must activate against whatever is on screen.
	application.SetModeHints()
	waitForCondition(t, "hints mode", func() bool {
		return application.CurrentMode() == domain.ModeHints
	})

	application.HandleKeyPress("Escape")
	waitForCondition(t, "idle after escape", func() bool {
		return application.CurrentMode() == domain.ModeIdle
	})

	application.Stop()

	select {
	case runErr := <-runDone:
		if runErr != nil && !errors.Is(runErr, context.Canceled) &&
			!derrors.IsCode(runErr, derrors.CodeContextCanceled) {
			t.Errorf("Run() returned an unexpected error after Stop(): %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("app did not stop within 5s")
	}
}

// waitForCondition polls cond until it holds or fails the test after 5s.
func waitForCondition(t *testing.T, desc string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", desc)
}
