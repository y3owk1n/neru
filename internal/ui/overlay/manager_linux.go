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
	"github.com/y3owk1n/neru/internal/app/components/stickyindicator"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// Manager manages multiple overlay windows (Linux stub).
type Manager struct {
	logger *zap.Logger
}

// NewOverlayManager creates a new overlay manager (Linux stub).
func NewOverlayManager(logger *zap.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// Get returns the global overlay manager (Linux stub).
func Get() *Manager {
	return &Manager{}
}

// Init initializes the global overlay manager (Linux stub).
func Init(logger *zap.Logger) *Manager {
	return &Manager{logger: logger}
}

// Show shows all managed overlays (Linux stub).
func (m *Manager) Show() {}

// Hide hides all managed overlays (Linux stub).
func (m *Manager) Hide() {}

// Clear clears all managed overlays (Linux stub).
func (m *Manager) Clear() {}

// ResizeToActiveScreen resizes all overlays to the active screen (Linux stub).
func (m *Manager) ResizeToActiveScreen() {}

// SwitchTo switches to the specified mode (Linux stub).
func (m *Manager) SwitchTo(_ Mode) {}

// Subscribe subscribes to state changes (Linux stub).
func (m *Manager) Subscribe(_ func(StateChange)) uint64 { return 0 }

// Unsubscribe unsubscribes from state changes (Linux stub).
func (m *Manager) Unsubscribe(_ uint64) {}

// Destroy destroys all managed overlays (Linux stub).
func (m *Manager) Destroy() {}

// Mode returns the current mode (Linux stub).
func (m *Manager) Mode() Mode { return ModeIdle }

// WindowPtr returns the window pointer (Linux stub).
func (m *Manager) WindowPtr() unsafe.Pointer { return nil }

// UseHintOverlay sets the hint overlay (Linux stub).
func (m *Manager) UseHintOverlay(_ *hints.Overlay) {}

// UseGridOverlay sets the grid overlay (Linux stub).
func (m *Manager) UseGridOverlay(_ *grid.Overlay) {}

// UseModeIndicatorOverlay sets the mode indicator overlay (Linux stub).
func (m *Manager) UseModeIndicatorOverlay(_ *modeindicator.Overlay) {}

// UseStickyModifiersOverlay sets the sticky modifiers overlay (Linux stub).
func (m *Manager) UseStickyModifiersOverlay(_ *stickyindicator.Overlay) {}

// UseRecursiveGridOverlay sets the recursive grid overlay (Linux stub).
func (m *Manager) UseRecursiveGridOverlay(_ *recursivegrid.Overlay) {}

// HintOverlay returns the hint overlay (Linux stub).
func (m *Manager) HintOverlay() *hints.Overlay { return nil }

// GridOverlay returns the grid overlay (Linux stub).
func (m *Manager) GridOverlay() *grid.Overlay { return nil }

// ModeIndicatorOverlay returns the mode indicator overlay (Linux stub).
func (m *Manager) ModeIndicatorOverlay() *modeindicator.Overlay { return nil }

// StickyModifiersOverlay returns the sticky modifiers overlay (Linux stub).
func (m *Manager) StickyModifiersOverlay() *stickyindicator.Overlay { return nil }

// RecursiveGridOverlay returns the recursive grid overlay (Linux stub).
func (m *Manager) RecursiveGridOverlay() *recursivegrid.Overlay { return nil }

// OverlayCapabilities reports current Linux overlay support.
func (m *Manager) OverlayCapabilities() ports.FeatureCapability {
	return ports.FeatureCapability{
		Status: ports.FeatureStatusStub,
		Detail: "native Linux overlays are not implemented yet",
	}
}

// DrawHintsWithStyle draws hints with the specified style (Linux stub).
func (m *Manager) DrawHintsWithStyle(_ []*hints.Hint, _ hints.StyleMode) error {
	return derrors.New(derrors.CodeNotSupported, "overlay hints not implemented on linux")
}

// DrawModeIndicator draws the mode indicator (Linux stub).
func (m *Manager) DrawModeIndicator(_, _ int) {}

// DrawStickyModifiersIndicator draws the sticky modifiers indicator (Linux stub).
func (m *Manager) DrawStickyModifiersIndicator(_, _ int, _ string) {}

// DrawGrid draws the grid (Linux stub).
func (m *Manager) DrawGrid(_ *domainGrid.Grid, _ string, _ grid.Style) error {
	return derrors.New(derrors.CodeNotSupported, "overlay grid not implemented on linux")
}

// DrawRecursiveGrid draws the recursive grid (Linux stub).
func (m *Manager) DrawRecursiveGrid(
	_ image.Rectangle,
	_ int,
	_ string,
	_ int,
	_ int,
	_ string,
	_ int,
	_ int,
	_ recursivegrid.Style,
) error {
	return derrors.New(derrors.CodeNotSupported, "recursive grid overlay not implemented on linux")
}

// UpdateGridMatches updates the grid matches (Linux stub).
func (m *Manager) UpdateGridMatches(_ string) {}

// ShowSubgrid shows the subgrid (Linux stub).
func (m *Manager) ShowSubgrid(_ *domainGrid.Cell, _ grid.Style) {}

// SetHideUnmatched sets whether to hide unmatched cells (Linux stub).
func (m *Manager) SetHideUnmatched(_ bool) {}

// SetSharingType sets the sharing type (Linux stub).
func (m *Manager) SetSharingType(_ bool) {}
