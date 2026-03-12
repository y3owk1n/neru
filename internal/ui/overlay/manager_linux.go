//go:build linux

package overlay

import (
	"image"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/modeindicator"
	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
)

// OverlayManager manages multiple overlay windows (Linux stub).
type OverlayManager struct {
	logger *zap.Logger
}

// NewOverlayManager creates a new overlay manager (Linux stub).
func NewOverlayManager(logger *zap.Logger) *OverlayManager {
	return &OverlayManager{
		logger: logger,
	}
}

// Get returns the global overlay manager (Linux stub).
func Get() *OverlayManager {
	return &OverlayManager{}
}

// Init initializes the global overlay manager (Linux stub).
func Init(logger *zap.Logger) *OverlayManager {
	return &OverlayManager{logger: logger}
}

// Show shows all managed overlays (Linux stub).
func (m *OverlayManager) Show() {}

// Hide hides all managed overlays (Linux stub).
func (m *OverlayManager) Hide() {}

// Clear clears all managed overlays (Linux stub).
func (m *OverlayManager) Clear() {}

// ResizeToActiveScreen resizes all overlays to the active screen (Linux stub).
func (m *OverlayManager) ResizeToActiveScreen() {}

// SwitchTo switches to the specified mode (Linux stub).
func (m *OverlayManager) SwitchTo(next Mode) {}

// Subscribe subscribes to state changes (Linux stub).
func (m *OverlayManager) Subscribe(fn func(StateChange)) uint64 { return 0 }

// Unsubscribe unsubscribes from state changes (Linux stub).
func (m *OverlayManager) Unsubscribe(id uint64) {}

// Destroy destroys all managed overlays (Linux stub).
func (m *OverlayManager) Destroy() {}

// Mode returns the current mode (Linux stub).
func (m *OverlayManager) Mode() Mode { return ModeIdle }

// WindowPtr returns the window pointer (Linux stub).
func (m *OverlayManager) WindowPtr() unsafe.Pointer { return nil }

// UseHintOverlay sets the hint overlay (Linux stub).
func (m *OverlayManager) UseHintOverlay(o *hints.Overlay) {}

// UseGridOverlay sets the grid overlay (Linux stub).
func (m *OverlayManager) UseGridOverlay(o *grid.Overlay) {}

// UseModeIndicatorOverlay sets the mode indicator overlay (Linux stub).
func (m *OverlayManager) UseModeIndicatorOverlay(o *modeindicator.Overlay) {}

// UseRecursiveGridOverlay sets the recursive grid overlay (Linux stub).
func (m *OverlayManager) UseRecursiveGridOverlay(o *recursivegrid.Overlay) {}

// HintOverlay returns the hint overlay (Linux stub).
func (m *OverlayManager) HintOverlay() *hints.Overlay { return nil }

// GridOverlay returns the grid overlay (Linux stub).
func (m *OverlayManager) GridOverlay() *grid.Overlay { return nil }

// ModeIndicatorOverlay returns the mode indicator overlay (Linux stub).
func (m *OverlayManager) ModeIndicatorOverlay() *modeindicator.Overlay { return nil }

// RecursiveGridOverlay returns the recursive grid overlay (Linux stub).
func (m *OverlayManager) RecursiveGridOverlay() *recursivegrid.Overlay { return nil }

// DrawHintsWithStyle draws hints with the specified style (Linux stub).
func (m *OverlayManager) DrawHintsWithStyle(hs []*hints.Hint, style hints.StyleMode) error {
	return nil
}

// DrawModeIndicator draws the mode indicator (Linux stub).
func (m *OverlayManager) DrawModeIndicator(x, y int) {}

// DrawGrid draws the grid (Linux stub).
func (m *OverlayManager) DrawGrid(g *domainGrid.Grid, input string, style grid.Style) error {
	return nil
}

// DrawRecursiveGrid draws the recursive grid (Linux stub).
func (m *OverlayManager) DrawRecursiveGrid(
	bounds image.Rectangle,
	depth int,
	keys string,
	gridCols int,
	gridRows int,
	nextKeys string,
	nextGridCols int,
	nextGridRows int,
	style recursivegrid.Style,
) error {
	return nil
}

// UpdateGridMatches updates the grid matches (Linux stub).
func (m *OverlayManager) UpdateGridMatches(prefix string) {}

// ShowSubgrid shows the subgrid (Linux stub).
func (m *OverlayManager) ShowSubgrid(cell *domainGrid.Cell, style grid.Style) {}

// SetHideUnmatched sets whether to hide unmatched cells (Linux stub).
func (m *OverlayManager) SetHideUnmatched(hide bool) {}

// SetSharingType sets the sharing type (Linux stub).
func (m *OverlayManager) SetSharingType(hide bool) {}
