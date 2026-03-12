package overlay

import (
	"image"
	"unsafe"

	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/modeindicator"
	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/core/domain"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
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

	HintOverlay() *hints.Overlay
	GridOverlay() *grid.Overlay
	ModeIndicatorOverlay() *modeindicator.Overlay
	RecursiveGridOverlay() *recursivegrid.Overlay

	DrawHintsWithStyle(hs []*hints.Hint, style hints.StyleMode) error
	DrawModeIndicator(x, y int)
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
