//go:build integration && darwin

package app_test

import (
	"context"
	"image"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app"
	componentrecursivegrid "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	portmocks "github.com/y3owk1n/neru/internal/core/ports/mocks"
)

type recursiveGridDrawCall struct {
	bounds       image.Rectangle
	depth        int
	keys         string
	gridCols     int
	gridRows     int
	nextKeys     string
	nextGridCols int
	nextGridRows int
}

type recursiveGridOverlayRecorder struct {
	mockOverlayManager

	mu    sync.RWMutex
	draws []recursiveGridDrawCall
}

func (r *recursiveGridOverlayRecorder) DrawRecursiveGrid(
	bounds image.Rectangle,
	depth int,
	keys string,
	gridCols int,
	gridRows int,
	nextKeys string,
	nextGridCols int,
	nextGridRows int,
	_ componentrecursivegrid.Style,
) error {
	r.mu.Lock()
	r.draws = append(r.draws, recursiveGridDrawCall{
		bounds:       bounds,
		depth:        depth,
		keys:         keys,
		gridCols:     gridCols,
		gridRows:     gridRows,
		nextKeys:     nextKeys,
		nextGridCols: nextGridCols,
		nextGridRows: nextGridRows,
	})
	r.mu.Unlock()

	return nil
}

func (r *recursiveGridOverlayRecorder) waitForDrawCount(tb testing.TB, expected int) {
	tb.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.RLock()
		count := len(r.draws)
		r.mu.RUnlock()
		if count >= expected {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	r.mu.RLock()
	count := len(r.draws)
	r.mu.RUnlock()

	tb.Fatalf("timeout waiting for %d recursive-grid draw calls, got %d", expected, count)
}

func (r *recursiveGridOverlayRecorder) lastDraw(tb testing.TB) recursiveGridDrawCall {
	tb.Helper()

	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.draws) == 0 {
		tb.Fatal("expected at least one recursive-grid draw call")
	}

	return r.draws[len(r.draws)-1]
}

func TestRecursiveGridSubKeyPreviewIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping recursive-grid integration test in short mode")
	}

	t.Run("hides preview on terminal second depth and restores it on backtrack", func(t *testing.T) {
		application, overlayRecorder, runDone := newRecursiveGridIntegrationApp(
			t,
			image.Rect(0, 0, 200, 200),
			func(cfg *config.Config) {
				cfg.RecursiveGrid.UI.SubKeyPreview = true
			},
		)
		defer stopIntegrationApp(t, application, runDone)

		application.ActivateMode(domain.ModeRecursiveGrid)
		waitForMode(t, application, domain.ModeRecursiveGrid)

		overlayRecorder.waitForDrawCount(t, 1)
		initialDraw := overlayRecorder.lastDraw(t)
		if initialDraw.nextKeys != "uijk" {
			t.Fatalf("expected initial recursive-grid preview keys %q, got %q", "uijk", initialDraw.nextKeys)
		}

		application.HandleKeyPress("u")
		overlayRecorder.waitForDrawCount(t, 2)
		firstDepthDraw := overlayRecorder.lastDraw(t)
		if firstDepthDraw.nextKeys != "uijk" {
			t.Fatalf("expected first nested depth to keep preview keys %q, got %q", "uijk", firstDepthDraw.nextKeys)
		}

		application.HandleKeyPress("u")
		overlayRecorder.waitForDrawCount(t, 3)
		secondDepthDraw := overlayRecorder.lastDraw(t)
		if secondDepthDraw.nextKeys != "" {
			t.Fatalf("expected terminal second depth preview to be hidden, got %q", secondDepthDraw.nextKeys)
		}
		if secondDepthDraw.nextGridCols != 0 || secondDepthDraw.nextGridRows != 0 {
			t.Fatalf(
				"expected terminal second depth preview layout to be cleared, got %dx%d",
				secondDepthDraw.nextGridCols,
				secondDepthDraw.nextGridRows,
			)
		}

		application.HandleKeyPress("Backspace")
		overlayRecorder.waitForDrawCount(t, 4)
		backtrackedDraw := overlayRecorder.lastDraw(t)
		if backtrackedDraw.nextKeys != "uijk" {
			t.Fatalf("expected preview keys to return after backtrack, got %q", backtrackedDraw.nextKeys)
		}
	})

	t.Run("hides preview immediately when the first depth is terminal", func(t *testing.T) {
		application, overlayRecorder, runDone := newRecursiveGridIntegrationApp(
			t,
			image.Rect(0, 0, 90, 90),
			func(cfg *config.Config) {
				cfg.RecursiveGrid.UI.SubKeyPreview = true
			},
		)
		defer stopIntegrationApp(t, application, runDone)

		application.ActivateMode(domain.ModeRecursiveGrid)
		waitForMode(t, application, domain.ModeRecursiveGrid)

		overlayRecorder.waitForDrawCount(t, 1)
		draw := overlayRecorder.lastDraw(t)
		if draw.nextKeys != "" {
			t.Fatalf("expected terminal first depth preview to be hidden, got %q", draw.nextKeys)
		}
		if draw.nextGridCols != 0 || draw.nextGridRows != 0 {
			t.Fatalf(
				"expected terminal first depth preview layout to be cleared, got %dx%d",
				draw.nextGridCols,
				draw.nextGridRows,
			)
		}
	})
}

func newRecursiveGridIntegrationApp(
	t *testing.T,
	screenBounds image.Rectangle,
	configure func(cfg *config.Config),
) (*app.App, *recursiveGridOverlayRecorder, <-chan error) {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.General.AccessibilityCheckOnStart = false
	cfg.RecursiveGrid.Enabled = true
	if configure != nil {
		configure(cfg)
	}

	system := &portmocks.SystemMock{
		ScreenBoundsFunc: func(ctx context.Context) (image.Rectangle, error) {
			return screenBounds, nil
		},
		MoveCursorToPointFunc: func(ctx context.Context, point image.Point, bypassSmooth bool) error {
			return nil
		},
	}

	overlayRecorder := &recursiveGridOverlayRecorder{}

	application, err := app.New(
		app.WithConfig(cfg),
		app.WithConfigPath(""),
		app.WithIPCServer(&mockIPCServer{}),
		app.WithWatcher(&mockAppWatcher{}),
		app.WithOverlayManager(overlayRecorder),
		app.WithHotkeyService(&mockHotkeyService{}),
		app.WithSystemPort(system),
	)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	runDone := make(chan error, 1)
	go func() {
		runDone <- application.Run()
	}()

	waitForAppReady(t, application)

	return application, overlayRecorder, runDone
}

func stopIntegrationApp(t *testing.T, application *app.App, runDone <-chan error) {
	t.Helper()

	application.Stop()

	select {
	case err := <-runDone:
		if err != nil {
			t.Logf("App Run() returned (expected after Stop): %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Application did not stop within timeout")
	}

	application.Cleanup()
}
