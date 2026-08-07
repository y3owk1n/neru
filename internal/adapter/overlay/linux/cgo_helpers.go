//go:build linux && cgo

package linux

import (
	"image"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
)

// shouldShowSubKeyPreview reports whether the sub-key mini-grid this backend
// draws inside a cell is legible enough to be worth drawing: every sub-cell
// must reach sub_key_preview_autohide_multiplier x the preview font size.
//
// Unlike the label autohide threshold (recursivegrid.Style.ShowLabelIn) this
// is not shared with the Windows backend, because the two do not draw the same
// thing: Windows draws a single preview label along the bottom of the cell and
// measures the cell, this backend and the macOS one draw a mini-grid and
// measure a sub-cell.
func shouldShowSubKeyPreview(
	cell image.Rectangle,
	style recursivegrid.Style,
	subGridCols int,
	subGridRows int,
) bool {
	if !style.SubKeyPreview() {
		return false
	}

	if style.SubKeyPreviewAutohideMultiplier() <= 0 {
		return true
	}

	threshold := style.SubKeyPreviewFontSizeF() * style.SubKeyPreviewAutohideMultiplier()
	subCellW := float64(cell.Dx()) / float64(subGridCols)
	subCellH := float64(cell.Dy()) / float64(subGridRows)

	return subCellW >= threshold && subCellH >= threshold
}
