package recursivegrid

import (
	"image"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/recursivegrid"
)

// SubKeyPreviewCell is one labeled sub-cell of the mini-grid a recursive-grid
// cell previews the next depth with: where the key lands inside the cell, and
// the key drawn there.
type SubKeyPreviewCell struct {
	// Bounds is the sub-cell, in whatever space the cell it was laid out in
	// was given in — screen-local or global, this does not translate.
	Bounds image.Rectangle

	// Label is the key that selects the sub-cell, or the configured
	// sub_key_preview_label_char when one is set.
	Label string
}

// PreviewsNextDepth reports whether a draw has a next depth to preview at all:
// the preview is switched on, the next depth has keys, and it has a shape to
// divide a cell into.
//
// It is one question per draw rather than per cell, and it is what keeps a
// preview off the deepest level — the mode leaves the next layout zero when the
// region can no longer be divided, so there are no keys and no shape, and the
// cells are drawn bare. Windows drew a label there until #1297, because it was
// handed the next layout and discarded it; asking here is what stops the next
// backend doing the same.
//
// A zero shape is also what makes this a gate rather than a hint: the
// subdivision divides by the column and row counts, so a draw that reached
// SubKeyPreviewCells with a zero one would divide by zero.
//
// nextKeyCount is taken as a number rather than as the keys themselves because
// only "are there any" is asked of it, and the three backends hold them in three
// forms — a rune slice on Cairo and GDI, the string the mode handed over on the
// Quartz one, which is passed on to Objective-C rather than decoded. Every form
// answers this the same way, and none of them has to be converted to ask.
func (s Style) PreviewsNextDepth(nextKeyCount int, nextDims domain.GridDimensions) bool {
	return s.subKeyPreview && nextKeyCount > 0 &&
		nextDims.Cols > 0 && nextDims.Rows > 0
}

// SubKeyPreviewFontSizeIn returns the size to draw the mini-grid previewing the
// next depth at, and whether to draw it at all. It is LabelFontSizeIn's rule
// with the preview's own settings. Here sub_key_preview_font_size is the ceiling,
// sub_key_preview_autohide_multiplier the room it keeps, and min_font_size the
// floor the two share.
//
// The rectangle fitted is a sub-cell rather than the whole cell, because a
// sub-cell is what a preview key is drawn in. It is sized by dividing each cell
// the way SubKeyPreviewCells will, less the pixel the remainder can give one
// sub-cell over another, so no layout is built just to be measured. Windows
// measured the whole cell until #1297, because it drew a single label along the
// bottom of the cell rather than a mini-grid.
//
// Like LabelFontSizeIn it takes every cell the preview will be drawn in, a
// transition's first frame with its last, and answers for the smallest.
func (s Style) SubKeyPreviewFontSizeIn(
	scale float64,
	nextDims domain.GridDimensions,
	cells ...image.Rectangle,
) (float64, bool) {
	if !s.subKeyPreview || nextDims.Cols <= 0 || nextDims.Rows <= 0 {
		return 0, false
	}

	fit := badge.FontFit{
		Requested:  s.SubKeyPreviewFontSizeF(),
		Floor:      float64(s.minFontSize),
		Multiplier: s.subKeyPreviewAutohideMultiplier,
		Metrics:    s.subKeyPreviewMetrics,
	}

	// The smallest sub-cell decides, as in FontFit.SizeAcross. This runs on
	// every keypress, so it keeps the running answer and builds no slice.
	size, show := 0.0, false

	for _, cell := range cells {
		subCell := image.Rect(0, 0, cell.Dx()/nextDims.Cols, cell.Dy()/nextDims.Rows)

		fitted, fits := fit.SizeIn(subCell, scale)
		if !fits {
			return 0, false
		}

		if !show || fitted < size {
			size = fitted
		}

		show = true
	}

	return size, show
}

// SubKeyPreviewCells lays the mini-grid out inside one cell: the next depth's
// keys, each on the sub-cell it would select, in reading order.
//
// It divides the cell with the same recursivegrid.ComputeGridCells the next
// depth will actually be drawn with, so a previewed key sits where its cell will
// be. The center sub-cell of an odd-by-odd division carries no key: the cell's
// own label is drawn there, and a preview key under it would be unreadable.
//
// Callers get only the sub-cells that carry a key, so nothing downstream has to
// know which one was left out. A draw the preview is switched off for, or one
// with no next depth to preview, gets nothing back — PreviewsNextDepth is the
// same question asked once per draw instead of once per cell.
//
// One thing this does not match the macOS overlay on, and it is deliberate:
// given fewer keys than sub-cells, drawSubKeyPreviewInCellRect: refuses the
// whole mini-grid while this labels what it can and leaves the rest blank, which
// is what the Cairo backend has always done. Correcting either would be a
// behavior change on a platform this is not fixing, and no configuration
// reaches it — recursivegrid.Manager takes a key mapping only when its length is
// the cell count, and drops a per-depth override whose length does not match
// that depth's shape, so a short mapping never leaves the mode layer.
func (s Style) SubKeyPreviewCells(
	cell image.Rectangle,
	nextKeys []rune,
	nextDims domain.GridDimensions,
) []SubKeyPreviewCell {
	if !s.PreviewsNextDepth(len(nextKeys), nextDims) {
		return nil
	}

	subCells := recursivegrid.ComputeGridCells(cell, nextDims)
	blank := blankSubKeyPreviewIndex(nextDims)

	preview := make([]SubKeyPreviewCell, 0, len(subCells))

	for idx, subCell := range subCells {
		if idx == blank || idx >= len(nextKeys) {
			continue
		}

		label := s.subKeyPreviewLabelChar
		if label == "" {
			label = string(nextKeys[idx])
		}

		preview = append(preview, SubKeyPreviewCell{Bounds: subCell, Label: label})
	}

	return preview
}

// blankSubKeyPreviewIndex is the sub-cell the mini-grid leaves unlabeled: the
// center one, when both next-level dimensions are odd. An even dimension has no
// center sub-cell to collide with the cell's own label, so nothing is skipped
// and -1 comes back.
//
// The macOS overlay decides this a second time in Objective-C
// (drawSubKeyPreviewInCellRect:), and ADR 0007 names that copy as unpinned: the
// autohide rule beside it has a test holding the two together, this one does
// not. What changed with #1297 is the count — Linux and Windows now read this
// one, so there are two copies rather than three.
//
//nolint:mnd // the 2s are "odd?" and "the middle of", not tunable values.
func blankSubKeyPreviewIndex(dims domain.GridDimensions) int {
	if dims.Cols%2 != 1 || dims.Rows%2 != 1 {
		return -1
	}

	return (dims.Rows/2)*dims.Cols + dims.Cols/2
}
