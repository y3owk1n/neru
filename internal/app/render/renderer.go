package render

import (
	"image"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// OverlayRenderer manages rendering operations for all application overlays.
type OverlayRenderer struct {
	manager overlay.ManagerInterface
	styles  overlay.StyleSource
}

// NewOverlayRenderer creates a new overlay renderer drawing with the Style the
// overlay resolves. The renderer holds no style of its own: a config reload or
// a theme change reaches it because the resolver it reads has already been
// told, not because someone remembered to push new values here.
func NewOverlayRenderer(
	manager overlay.ManagerInterface,
	styles overlay.StyleSource,
) *OverlayRenderer {
	return &OverlayRenderer{
		manager: manager,
		styles:  styles,
	}
}

// DrawHints draws hints with the resolved style.
func (r *OverlayRenderer) DrawHints(hs []*hints.Hint) error {
	return r.manager.DrawHintsWithStyle(hs, overlay.ResolvedStyle(r.styles).Hints)
}

// DrawGrid draws a grid with the resolved style.
func (r *OverlayRenderer) DrawGrid(g *domainGrid.Grid, input string) error {
	return r.manager.DrawGrid(g, input, overlay.ResolvedStyle(r.styles).Grid)
}

// ShowSubgrid shows a subgrid for the specified cell.
func (r *OverlayRenderer) ShowSubgrid(
	cell *domainGrid.Cell,
) {
	r.manager.ShowSubgrid(cell, overlay.ResolvedStyle(r.styles).Grid)
}

// UpdateGridMatches updates the grid matches with the specified prefix.
func (r *OverlayRenderer) UpdateGridMatches(prefix string) {
	r.manager.UpdateGridMatches(prefix)
}

// SetHideUnmatched sets whether to hide unmatched cells.
func (r *OverlayRenderer) SetHideUnmatched(hide bool) {
	r.manager.SetHideUnmatched(hide)
}

// Show shows the overlay.
func (r *OverlayRenderer) Show() {
	r.manager.Show()
}

// Clear clears the overlay.
func (r *OverlayRenderer) Clear() {
	r.manager.Clear()
}

// ResizeActive resizes the overlay to the active screen.
func (r *OverlayRenderer) ResizeActive() {
	r.manager.ResizeToActiveScreen()
}

// DrawModeIndicator draws a mode indicator at the specified position.
func (r *OverlayRenderer) DrawModeIndicator(x, y int) {
	r.manager.DrawModeIndicator(x, y)
}

// DrawRecursiveGrid draws a recursive-grid with the current bounds and depth.
// nextKeys/nextGridCols/nextGridRows describe the *next* depth's layout
// and are used by the sub-key preview mini-grid inside each cell.
func (r *OverlayRenderer) DrawRecursiveGrid(
	bounds image.Rectangle,
	depth int,
	keys string,
	gridCols int,
	gridRows int,
	nextKeys string,
	nextGridCols int,
	nextGridRows int,
	virtualPointer recursivegrid.VirtualPointerState,
) error {
	return r.manager.DrawRecursiveGrid(
		bounds,
		depth,
		keys,
		gridCols,
		gridRows,
		nextKeys,
		nextGridCols,
		nextGridRows,
		overlay.ResolvedStyle(r.styles).RecursiveGrid,
		virtualPointer,
	)
}
