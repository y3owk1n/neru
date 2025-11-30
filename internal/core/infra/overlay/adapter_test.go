//go:build unit

package overlay_test

import (
	"context"
	"image"
	"testing"
	"unsafe"

	"github.com/y3owk1n/neru/internal/app/components/action"
	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	"github.com/y3owk1n/neru/internal/core/infra/overlay"
	uiOverlay "github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

// mockManager is a mock implementation of uiOverlay.ManagerInterface.
type mockManager struct {
	hintOverlay *hints.Overlay
	lastStyle   hints.StyleMode

	// Track DrawActionHighlight calls
	lastActionX, lastActionY, lastActionW, lastActionH int
	actionHighlightCalled                              bool

	// Track DrawScrollHighlight calls
	lastScrollX, lastScrollY, lastScrollW, lastScrollH int
	scrollHighlightCalled                              bool
}

func (m *mockManager) Show()                                           {}
func (m *mockManager) Hide()                                           {}
func (m *mockManager) Clear()                                          {}
func (m *mockManager) ResizeToActiveScreenSync()                       {}
func (m *mockManager) SwitchTo(next uiOverlay.Mode)                    {}
func (m *mockManager) Subscribe(fn func(uiOverlay.StateChange)) uint64 { return 0 }
func (m *mockManager) Unsubscribe(id uint64)                           {}
func (m *mockManager) Destroy()                                        {}
func (m *mockManager) Mode() uiOverlay.Mode                            { return uiOverlay.ModeIdle }
func (m *mockManager) WindowPtr() unsafe.Pointer                       { return nil }

func (m *mockManager) UseHintOverlay(o *hints.Overlay)    { m.hintOverlay = o }
func (m *mockManager) UseGridOverlay(o *grid.Overlay)     {}
func (m *mockManager) UseActionOverlay(o *action.Overlay) {}
func (m *mockManager) UseScrollOverlay(o *scroll.Overlay) {}

func (m *mockManager) HintOverlay() *hints.Overlay    { return m.hintOverlay }
func (m *mockManager) GridOverlay() *grid.Overlay     { return nil }
func (m *mockManager) ActionOverlay() *action.Overlay { return nil }
func (m *mockManager) ScrollOverlay() *scroll.Overlay { return nil }

func (m *mockManager) DrawHintsWithStyle(hs []*hints.Hint, style hints.StyleMode) error {
	m.lastStyle = style

	return nil
}

func (m *mockManager) DrawActionHighlight(x, y, w, h int) {
	m.lastActionX, m.lastActionY, m.lastActionW, m.lastActionH = x, y, w, h
	m.actionHighlightCalled = true
}

func (m *mockManager) DrawScrollHighlight(x, y, w, h int) {
	m.lastScrollX, m.lastScrollY, m.lastScrollW, m.lastScrollH = x, y, w, h
	m.scrollHighlightCalled = true
}

func (m *mockManager) DrawGrid(
	g *domainGrid.Grid,
	input string,
	style grid.Style,
) error {
	return nil
}
func (m *mockManager) UpdateGridMatches(prefix string)                     {}
func (m *mockManager) ShowSubgrid(cell *domainGrid.Cell, style grid.Style) {}
func (m *mockManager) SetHideUnmatched(hide bool)                          {}

func TestShowHints_PassesCorrectStyle(t *testing.T) {
	// Setup
	cfg := config.HintsConfig{
		BackgroundColor: "#123456",
		TextColor:       "#abcdef",
	}

	logger := zap.NewNop()

	// Create a real hints.Overlay (but with nil window) to hold the config
	// We use NewOverlayWithWindow with nil pointer which is safe for just holding config
	hintOverlay, err := hints.NewOverlayWithWindow(cfg, logger, nil)
	if err != nil {
		t.Fatalf("Failed to create hint overlay: %v", err)
	}

	mock := &mockManager{
		hintOverlay: hintOverlay,
	}

	adapter := overlay.NewAdapter(mock, logger)

	// Execute
	// Pass empty hints list as we only care about style passing
	err = adapter.ShowHints(context.Background(), nil)
	if err != nil {
		t.Fatalf("ShowHints failed: %v", err)
	}

	// Verify
	if mock.lastStyle.BackgroundColor() != "#123456" {
		t.Errorf("Expected BackgroundColor #123456, got %s", mock.lastStyle.BackgroundColor())
	}

	if mock.lastStyle.TextColor() != "#abcdef" {
		t.Errorf("Expected TextColor #abcdef, got %s", mock.lastStyle.TextColor())
	}
}

func TestDrawActionHighlight_UsesLocalCoordinates(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	mock := &mockManager{}
	adapter := overlay.NewAdapter(mock, logger)

	// Test with extended screen bounds (multi-monitor scenario)
	// Screen at (1920, 0) with size 1920x1080
	screenBounds := image.Rect(1920, 0, 3840, 1080)

	// Execute
	err := adapter.DrawActionHighlight(context.Background(), screenBounds, "#ff0000", 2)
	if err != nil {
		t.Fatalf("DrawActionHighlight failed: %v", err)
	}

	// Verify - should use local coordinates (0,0) not screen bounds
	if !mock.actionHighlightCalled {
		t.Error("DrawActionHighlight was not called on manager")
	}

	expectedX, expectedY := 0, 0
	expectedW, expectedH := 1920, 1080

	if mock.lastActionX != expectedX || mock.lastActionY != expectedY {
		t.Errorf("DrawActionHighlight called with wrong position: got (%d,%d), expected (%d,%d)",
			mock.lastActionX, mock.lastActionY, expectedX, expectedY)
	}

	if mock.lastActionW != expectedW || mock.lastActionH != expectedH {
		t.Errorf("DrawActionHighlight called with wrong size: got (%d,%d), expected (%d,%d)",
			mock.lastActionW, mock.lastActionH, expectedW, expectedH)
	}
}

func TestDrawScrollHighlight_UsesLocalCoordinates(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	mock := &mockManager{}
	adapter := overlay.NewAdapter(mock, logger)

	// Test with extended screen bounds (multi-monitor scenario)
	// Screen at (1920, 0) with size 1920x1080
	screenBounds := image.Rect(1920, 0, 3840, 1080)

	// Execute
	err := adapter.DrawScrollHighlight(context.Background(), screenBounds, "#00ff00", 3)
	if err != nil {
		t.Fatalf("DrawScrollHighlight failed: %v", err)
	}

	// Verify - should use local coordinates (0,0) not screen bounds
	if !mock.scrollHighlightCalled {
		t.Error("DrawScrollHighlight was not called on manager")
	}

	expectedX, expectedY := 0, 0
	expectedW, expectedH := 1920, 1080

	if mock.lastScrollX != expectedX || mock.lastScrollY != expectedY {
		t.Errorf("DrawScrollHighlight called with wrong position: got (%d,%d), expected (%d,%d)",
			mock.lastScrollX, mock.lastScrollY, expectedX, expectedY)
	}

	if mock.lastScrollW != expectedW || mock.lastScrollH != expectedH {
		t.Errorf("DrawScrollHighlight called with wrong size: got (%d,%d), expected (%d,%d)",
			mock.lastScrollW, mock.lastScrollH, expectedW, expectedH)
	}
}
