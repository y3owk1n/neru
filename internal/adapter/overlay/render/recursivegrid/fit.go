package recursivegrid

import (
	"image"

	"github.com/y3owk1n/neru/internal/domain"
)

// FittedSizes is what one draw of the region grid draws its text at. It covers
// the cell labels and the sub-key preview, each with whether it is drawn at
// all. Every label in a draw shares one size, so this is settled once per draw
// and never per cell.
type FittedSizes struct {
	LabelSize   float64
	ShowLabel   bool
	PreviewSize float64
	ShowPreview bool
}

// FitDraw fits the labels and the preview to the cells of a settled draw.
// cells are in device pixels and scale is device pixels per font unit.
func (s Style) FitDraw(
	scale float64,
	cells []image.Rectangle,
	nextDims domain.GridDimensions,
) FittedSizes {
	var fitted FittedSizes

	fitted.LabelSize, fitted.ShowLabel = s.LabelFontSizeIn(scale, cells...)
	fitted.PreviewSize, fitted.ShowPreview = s.SubKeyPreviewFontSizeIn(scale, nextDims, cells...)

	return fitted
}

// FitTransition fits what a transition holds from its first frame to its last:
// the size that fits the cells it starts from and the cells it ends on, so
// nothing overflows on the way and no frame realizes a font of its own. A
// label or preview that either end cannot hold stays hidden until the
// transition settles, where FitDraw answers for the last frame alone.
func (s Style) FitTransition(
	scale float64,
	fromCells, toCells []image.Rectangle,
	nextDims domain.GridDimensions,
) FittedSizes {
	// Each end is fitted alone and the smaller answer kept, which is the fit
	// over both without joining them into one slice on every keypress.
	held := s.FitDraw(scale, fromCells, nextDims)
	settled := s.FitDraw(scale, toCells, nextDims)

	return FittedSizes{
		LabelSize:   min(held.LabelSize, settled.LabelSize),
		ShowLabel:   held.ShowLabel && settled.ShowLabel,
		PreviewSize: min(held.PreviewSize, settled.PreviewSize),
		ShowPreview: held.ShowPreview && settled.ShowPreview,
	}
}
