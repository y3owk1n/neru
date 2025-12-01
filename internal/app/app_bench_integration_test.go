//go:build integration

package app_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"go.uber.org/zap"
)

// BenchmarkModeTransitionsIntegration benchmarks mode transitions in integration.
func BenchmarkModeTransitionsIntegration(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.General.AccessibilityCheckOnStart = false

	application, err := app.New(
		app.WithConfig(cfg),
		app.WithConfigPath(""),
		app.WithLogger(zap.NewNop()), // Use no-op logger to suppress benchmark spam
		app.WithIPCServer(&mockIPCServer{}),
		app.WithWatcher(&mockAppWatcher{}),
		app.WithOverlayManager(&mockOverlayManager{}),
		app.WithHotkeyService(&mockHotkeyService{}),
	)
	if err != nil {
		b.Fatalf("Failed to create app: %v", err)
	}
	defer application.Cleanup()

	// Start the app
	runDone := make(chan error, 1)
	go func() {
		runDone <- application.Run()
	}()

	waitForAppReady(b, application)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			application.SetModeHints()
			application.SetModeGrid()
			application.SetModeAction()
			application.SetModeScroll()
			application.SetModeIdle()
		}
	})

	application.Stop()

	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		b.Fatal("App did not stop within timeout")
	}
}
