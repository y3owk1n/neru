//go:build linux && cgo

package overlay

import (
	"image"

	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
)

func centeredRect(cell image.Rectangle, width, height int) image.Rectangle {
	centerX := cell.Min.X + cell.Dx()/centeredRectDivisor
	centerY := cell.Min.Y + cell.Dy()/centeredRectDivisor

	return image.Rect(
		centerX-width/centeredRectDivisor,
		centerY-height/centeredRectDivisor,
		centerX-width/centeredRectDivisor+width,
		centerY-height/centeredRectDivisor+height,
	)
}

func shouldShowLabel(
	cell image.Rectangle,
	style recursivegrid.Style,
) bool {
	if style.LabelAutohideMultiplier <= 0 {
		return true
	}

	threshold := style.LabelFontSize * style.LabelAutohideMultiplier

	return float64(cell.Dx()) >= threshold && float64(cell.Dy()) >= threshold
}

func shouldShowSubKeyPreview(
	cell image.Rectangle,
	style recursivegrid.Style,
	subGridCols int,
	subGridRows int,
) bool {
	if !style.SubKeyPreview {
		return false
	}

	if style.SubKeyPreviewAutohideMultiplier <= 0 {
		return true
	}

	threshold := style.SubKeyPreviewFontSize * style.SubKeyPreviewAutohideMultiplier
	subCellW := float64(cell.Dx()) / float64(subGridCols)
	subCellH := float64(cell.Dy()) / float64(subGridRows)

	return subCellW >= threshold && subCellH >= threshold
}
