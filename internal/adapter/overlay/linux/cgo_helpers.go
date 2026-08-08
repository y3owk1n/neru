//go:build linux && cgo

package linux

import (
	"image"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/domain"
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
//
// The macOS one asks the same question in Objective-C
// (drawSubKeyPreviewInCellRect: in
// internal/adapter/platform/darwin/overlay_darwin.m), so Go cannot be its one
// implementation; ADR 0007 asks for a test holding the two copies together
// instead, and internal/architecture/sub_key_preview_autohide_rule_test.go is
// it — change the rule here and that test fails until the Objective-C copy
// follows. It reads this function out of the source rather than calling it,
// because the build tag above puts it out of reach of a test running on macOS.
func shouldShowSubKeyPreview(
	cell image.Rectangle,
	style recursivegrid.Style,
	subDims domain.GridDimensions,
) bool {
	if !style.SubKeyPreview() {
		return false
	}

	if style.SubKeyPreviewAutohideMultiplier() <= 0 {
		return true
	}

	threshold := style.SubKeyPreviewFontSizeF() * style.SubKeyPreviewAutohideMultiplier()
	subCellW := float64(cell.Dx()) / float64(subDims.Cols)
	subCellH := float64(cell.Dy()) / float64(subDims.Rows)

	return subCellW >= threshold && subCellH >= threshold
}
