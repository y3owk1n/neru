//go:build integration && windows

// internal/core/infra/platform/windows/overlay_windows_integration_test.go
// Real Win32 overlay integration tests.
// Does not run in default CI; execute on WIN-VM desktop session with:
// go test -tags=integration ./internal/core/infra/platform/windows/...

package windows_test

import (
	"image"
	"strings"
	"testing"

	winplatform "github.com/y3owk1n/neru/internal/core/infra/platform/windows"
)

func skipIfOverlayUnavailable(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		return
	}

	msg := err.Error()
	if strings.Contains(msg, "interactive") ||
		strings.Contains(msg, "CreateWindowExW") ||
		strings.Contains(msg, "RegisterClassExW") {
		t.Skipf("skipping: overlay requires interactive desktop (%v)", err)
	}
}

func TestOverlayWindowLifecycleIntegration(t *testing.T) {
	t.Parallel()

	overlay, err := winplatform.NewOverlayWindow()
	skipIfOverlayUnavailable(t, err)

	if err != nil {
		t.Fatalf("NewOverlayWindow: %v", err)
	}

	defer overlay.Destroy()

	if !overlay.Healthy() {
		t.Fatal("overlay is not healthy after creation")
	}

	bounds := overlay.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Fatalf("overlay bounds = %v", bounds)
	}

	overlay.Clear()
	overlay.FillRect(bounds, 0x8000FF00)
	overlay.DrawTextCentered("FF", bounds, "Segoe UI", 48, 0xFFFFFFFF)
	_ = overlay.Flush()
	overlay.Show()
	overlay.Hide()

	err = overlay.ResizeToActiveScreen()
	if err != nil {
		t.Fatalf("ResizeToActiveScreen: %v", err)
	}
}

func TestOverlayRectDrawingIntegration(t *testing.T) {
	t.Parallel()

	overlay, err := winplatform.NewOverlayWindow()
	skipIfOverlayUnavailable(t, err)

	if err != nil {
		t.Fatalf("NewOverlayWindow: %v", err)
	}

	defer overlay.Destroy()

	cell := image.Rect(10, 10, 110, 60)

	overlay.Clear()
	overlay.FillRect(cell, 0xCC3366FF)
	overlay.StrokeRect(cell, 0xFFFFFFFF, 2)
	_ = overlay.Flush()
}
