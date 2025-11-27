package overlay_test

import (
	"context"
	"testing"
	"unsafe"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/features/action"
	"github.com/y3owk1n/neru/internal/features/grid"
	"github.com/y3owk1n/neru/internal/features/hints"
	"github.com/y3owk1n/neru/internal/features/scroll"
	uiOverlay "github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

// mockManager is a mock implementation of uiOverlay.ManagerInterface.
type mockManager struct {
	hintOverlay *hints.Overlay
	lastStyle   hints.StyleMode
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
func (m *mockManager) DrawActionHighlight(x, y, w, h int) {}
func (m *mockManager) DrawScrollHighlight(x, y, w, h int) {}

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

func TestAdapter_Hide(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockManager{}
	adapter := overlay.NewAdapter(mock, logger)

	err := adapter.Hide(context.Background())
	if err != nil {
		t.Errorf("Hide() should not return error, got %v", err)
	}
}

func TestAdapter_IsVisible(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockManager{}
	adapter := overlay.NewAdapter(mock, logger)

	// Mock returns ModeIdle, so IsVisible should return false
	visible := adapter.IsVisible()
	if visible {
		t.Error("IsVisible() should return false when mode is idle")
	}
}

func TestAdapter_Refresh(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockManager{}
	adapter := overlay.NewAdapter(mock, logger)

	err := adapter.Refresh(context.Background())
	if err != nil {
		t.Errorf("Refresh() should not return error, got %v", err)
	}
}

func TestAdapter_Health(t *testing.T) {
	logger := zap.NewNop()
	mock := &mockManager{}
	adapter := overlay.NewAdapter(mock, logger)

	err := adapter.Health(context.Background())
	if err != nil {
		t.Errorf("Health() should not return error, got %v", err)
	}
}
