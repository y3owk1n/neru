package overlay

import (
	"image"
	"unsafe"

	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/modeindicator"
	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/components/stickyindicator"
	"github.com/y3owk1n/neru/internal/core/domain"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// Mode represents the overlay mode.
type Mode string

const (
	// ModeIdle represents the idle mode.
	ModeIdle Mode = Mode(domain.ModeNameIdle)
	// ModeHints represents the hints mode.
	ModeHints Mode = Mode(domain.ModeNameHints)
	// ModeGrid represents the grid mode.
	ModeGrid Mode = Mode(domain.ModeNameGrid)
	// ModeScroll represents the scroll mode.
	ModeScroll Mode = Mode(domain.ModeNameScroll)
	// ModeRecursiveGrid represents the recursive-grid mode.
	ModeRecursiveGrid Mode = Mode(domain.ModeNameRecursiveGrid)
)

// StateChange represents a change in overlay mode.
type StateChange struct {
	prev Mode
	next Mode
}

// Prev returns the previous mode.
func (sc StateChange) Prev() Mode {
	return sc.prev
}

// Next returns the next mode.
func (sc StateChange) Next() Mode {
	return sc.next
}

// NoOpManager is a no-op implementation of ManagerInterface for headless environments.
type NoOpManager struct{}

// Ensure NoOpManager always implements ManagerInterface.
var _ ManagerInterface = (*NoOpManager)(nil)

// Show is a no-op implementation.
func (n *NoOpManager) Show() {}

// Hide is a no-op implementation.
func (n *NoOpManager) Hide() {}

// Clear is a no-op implementation.
func (n *NoOpManager) Clear() {}

// ResizeToActiveScreen is a no-op implementation.
func (n *NoOpManager) ResizeToActiveScreen() {}

// SwitchTo is a no-op implementation.
func (n *NoOpManager) SwitchTo(next Mode) {}

// Subscribe is a no-op implementation.
func (n *NoOpManager) Subscribe(fn func(StateChange)) uint64 { return 0 }

// Unsubscribe is a no-op implementation.
func (n *NoOpManager) Unsubscribe(id uint64) {}

// Destroy is a no-op implementation.
func (n *NoOpManager) Destroy() {}

// Mode returns ModeIdle.
func (n *NoOpManager) Mode() Mode { return ModeIdle }

// WindowPtr returns nil.
func (n *NoOpManager) WindowPtr() unsafe.Pointer { return nil }

// UseHintOverlay is a no-op implementation.
func (n *NoOpManager) UseHintOverlay(o *hints.Overlay) {}

// UseGridOverlay is a no-op implementation.
func (n *NoOpManager) UseGridOverlay(o *grid.Overlay) {}

// UseModeIndicatorOverlay is a no-op implementation.
func (n *NoOpManager) UseModeIndicatorOverlay(o *modeindicator.Overlay) {}

// UseRecursiveGridOverlay is a no-op implementation.
func (n *NoOpManager) UseRecursiveGridOverlay(o *recursivegrid.Overlay) {}

// UseStickyModifiersOverlay is a no-op implementation.
func (n *NoOpManager) UseStickyModifiersOverlay(o *stickyindicator.Overlay) {}

// HintOverlay returns nil.
func (n *NoOpManager) HintOverlay() *hints.Overlay { return nil }

// GridOverlay returns nil.
func (n *NoOpManager) GridOverlay() *grid.Overlay { return nil }

// ModeIndicatorOverlay returns nil.
func (n *NoOpManager) ModeIndicatorOverlay() *modeindicator.Overlay { return nil }

// RecursiveGridOverlay returns nil.
func (n *NoOpManager) RecursiveGridOverlay() *recursivegrid.Overlay { return nil }

// StickyModifiersOverlay returns nil.
func (n *NoOpManager) StickyModifiersOverlay() *stickyindicator.Overlay { return nil }

// DrawHintsWithStyle is a no-op implementation.
func (n *NoOpManager) DrawHintsWithStyle(
	hs []*hints.Hint,
	style hints.StyleMode,
) error {
	return nil
}

// DrawModeIndicator is a no-op implementation.
func (n *NoOpManager) DrawModeIndicator(x, y int) {}

// DrawStickyModifiersIndicator is a no-op implementation.
func (n *NoOpManager) DrawStickyModifiersIndicator(x, y int, symbols string) {}

// DrawGrid is a no-op implementation.
func (n *NoOpManager) DrawGrid(
	g *domainGrid.Grid,
	input string,
	style grid.Style,
) error {
	return nil
}

// DrawRecursiveGrid is a no-op implementation.
func (n *NoOpManager) DrawRecursiveGrid(
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

// UpdateGridMatches is a no-op implementation.
func (n *NoOpManager) UpdateGridMatches(prefix string) {}

// ShowSubgrid is a no-op implementation.
func (n *NoOpManager) ShowSubgrid(cell *domainGrid.Cell, style grid.Style) {}

// SetHideUnmatched is a no-op implementation.
func (n *NoOpManager) SetHideUnmatched(hide bool) {}

// SetSharingType is a no-op implementation.
func (n *NoOpManager) SetSharingType(hide bool) {}

// OverlayCapabilities reports that NoOpManager does not render overlays.
func (n *NoOpManager) OverlayCapabilities() ports.FeatureCapability {
	return ports.FeatureCapability{
		Status: ports.FeatureStatusHeadless,
		Detail: "headless no-op overlay manager",
	}
}

// CapabilityReporter exposes overlay support information.
type CapabilityReporter interface {
	OverlayCapabilities() ports.FeatureCapability
}

// ManagerInterface defines the interface for overlay window management.
type ManagerInterface interface {
	Show()
	Hide()
	Clear()
	ResizeToActiveScreen()
	SwitchTo(next Mode)
	Subscribe(fn func(StateChange)) uint64
	Unsubscribe(id uint64)
	Destroy()
	Mode() Mode
	WindowPtr() unsafe.Pointer

	UseHintOverlay(o *hints.Overlay)
	UseGridOverlay(o *grid.Overlay)
	UseModeIndicatorOverlay(o *modeindicator.Overlay)
	UseRecursiveGridOverlay(o *recursivegrid.Overlay)
	UseStickyModifiersOverlay(o *stickyindicator.Overlay)

	HintOverlay() *hints.Overlay
	GridOverlay() *grid.Overlay
	ModeIndicatorOverlay() *modeindicator.Overlay
	RecursiveGridOverlay() *recursivegrid.Overlay
	StickyModifiersOverlay() *stickyindicator.Overlay

	DrawHintsWithStyle(hs []*hints.Hint, style hints.StyleMode) error
	DrawModeIndicator(x, y int)
	DrawStickyModifiersIndicator(x, y int, symbols string)
	DrawGrid(g *domainGrid.Grid, input string, style grid.Style) error
	DrawRecursiveGrid(
		bounds image.Rectangle,
		depth int,
		keys string,
		gridCols int,
		gridRows int,
		nextKeys string,
		nextGridCols int,
		nextGridRows int,
		style recursivegrid.Style,
	) error
	UpdateGridMatches(prefix string)
	ShowSubgrid(cell *domainGrid.Cell, style grid.Style)
	SetHideUnmatched(hide bool)
	SetSharingType(hide bool)
}
