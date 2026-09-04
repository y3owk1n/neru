//go:build windows

package windows

import (
	"image"
	"strings"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/recursivegrid"
)

// Hint and recursive-grid drawing for the Windows overlay window.
// Does not own window lifecycle or grid rendering (see overlay.go).
const (
	winPaddingMultiplier            = 2
	winAutoRadiusBadgeCap           = 6.0
	winAutoRadiusBoundaryCap        = 4.0
	winMouseActionSquareRadiusScale = 0.18
	winMouseActionMinSquareRadius   = 2.0
)

// DrawHints renders the hint overlay, mirroring the cross-platform software
// renderer: a label badge per hint, placed against the element the hint
// labels. Each hint is queued as a unit (fill + arrow + stroke + text) and the
// window paints in queue order, so overlapping hints have correct Z-ordering —
// later hints are fully on top of earlier ones, matching macOS behavior.
//
// offset is the resolved `hints.ui.placement`: the caller reads the vocabulary
// and refuses a placement this backend cannot draw before anything is painted
// (Manager.DrawHintsWithStyle), so there is no configured string here and no
// unrecognized case to answer.
func (o *winOverlay) DrawHints(
	hintsSlice []*hintscomponent.Hint,
	style hintscomponent.StyleMode,
	offset badge.HintOffset,
) {
	if o == nil {
		return
	}

	o.ensureWindowForDraw()

	if o.window == nil {
		if o.logger != nil {
			o.logger.Error("DrawHints aborted, overlay window is nil")
		}

		return
	}

	// Hints own the surface; drop any cached grid so Show() does not redraw it.
	o.cachedGrid = nil
	o.currentSubgrid = nil
	o.suppressDraw = false

	o.Clear()

	o.lastHints = hintsSlice
	o.lastHintStyle = style
	o.lastHintOffset = offset

	for _, hint := range hintsSlice {
		if hint == nil {
			continue
		}

		if style.BoundaryHighlightEnabled() {
			// The element's own box, rebuilt from what the hint carries:
			// hint.Position() is the element center and hint.Size() its
			// bounds. Only the boundary highlight draws it; the badge is
			// placed against the same center point below.
			element := badge.CenteredOn(hint.Position(), hint.Size().X, hint.Size().Y)

			bdr := badge.BorderRadius(
				style.BoundaryBorderRadius(), element, winAutoRadiusBoundaryCap,
			)
			o.window.FillRoundedRect(
				element, bdr, badge.ParseHexARGB(style.BoundaryBackgroundColor()),
			)

			if bw := float64(max(style.BoundaryBorderWidth(), 0)); bw > 0 {
				o.window.StrokeRoundedRect(
					element, bdr, badge.ParseHexARGB(style.BoundaryBorderColor()), bw,
				)
			}
		}

		// Size the badge to the label text, not the element. hint.Size() is the
		// element's bounding box (hint.Bounds().Size()), so using it makes the
		// badge as large as the element (e.g. oversized boxes over big buttons).
		fontSize := float64(max(style.FontSize(), 1))
		paddingX := badge.AutoPadding(fontSize, style.PaddingX(), true)
		paddingY := badge.AutoPadding(fontSize, style.PaddingY(), false)
		badgeWidth := badge.EstimateTextWidth(
			hint.Label(),
			fontSize,
		) + paddingX*winPaddingMultiplier
		badgeHeight := badge.EstimateTextHeight(fontSize) + paddingY*winPaddingMultiplier

		// The corner radius is resolved from the badge's size alone — where the
		// badge lands does not change how round it is — and then capped so an
		// offset badge keeps a flat edge for the connector arrow to attach to.
		radius := badge.HintRadius(
			int(badge.BorderRadius(
				style.BorderRadius(),
				image.Rect(0, 0, badgeWidth, badgeHeight),
				winAutoRadiusBadgeCap,
			)),
			badgeWidth,
			offset,
		)

		// hint.Position() is the element center, the same point the boundary
		// highlight is drawn around. The badge is centered horizontally on it
		// and placed above / on / below it exactly as the macOS and Linux
		// overlays place theirs; an offset badge also gets a connector arrow
		// pointing back at the target.
		bounds, arrow, hasArrow := badge.PlaceHint(
			hint.Position(), badgeWidth, badgeHeight, radius, offset,
		)

		textColor := style.TextColor()
		if hint.MatchedPrefix() != "" {
			textColor = style.MatchedTextColor()
		}

		bdr := float64(radius)
		borderWidth := float64(max(style.BorderWidth(), 0))

		o.window.FillRoundedRect(
			bounds, bdr, badge.ParseHexARGB(style.BackgroundColor()),
		)

		if hasArrow {
			o.drawHintArrow(arrow, style, borderWidth)
		}

		if borderWidth > 0 {
			o.window.StrokeRoundedRect(
				bounds, bdr, badge.ParseHexARGB(style.BorderColor()), borderWidth,
			)
		}

		// The family arrives resolved here for the same reason
		// drawTextCentered documents, and this is the surface that redraws on
		// every keystroke. The window paints commands in the order they were
		// queued, so this hint's fill, arrow, stroke and label land as one
		// unit over every hint queued before it.
		o.drawTextCentered(
			hint.Label(),
			bounds,
			style.FontFamily(),
			fontSize,
			badge.ParseHexARGB(textColor),
		)
	}

	o.flushOverlay("hints")
}

// drawHintArrow draws the connector arrow that ties an offset hint badge back
// to the element it labels.
//
// The Cairo and Quartz backends build the badge and the arrow as one outline,
// so their border runs around both. This surface has no path primitive to do
// that with, so the arrow is a triangle drawn over a slightly larger triangle
// in the border color — which gives its two slanted edges the same border,
// leaving only the badge's own edge running across the arrow's base.
func (o *winOverlay) drawHintArrow(
	arrow badge.HintArrow,
	style hintscomponent.StyleMode,
	borderWidth float64,
) {
	if borderWidth > 0 {
		outset := outsetHintArrow(arrow, int(borderWidth))
		o.window.FillTriangle(
			outset.BaseLeft, outset.Tip, outset.BaseRight,
			badge.ParseHexARGB(style.BorderColor()),
		)
	}

	o.window.FillTriangle(
		arrow.BaseLeft, arrow.Tip, arrow.BaseRight,
		badge.ParseHexARGB(style.BackgroundColor()),
	)
}

// outsetHintArrow grows an arrow by width pixels along the two edges that show:
// outwards at the base corners and forwards at the tip, so the original
// triangle drawn on top of it leaves a border of that width on both slanted
// edges. The base moves back into the badge, where the badge's own fill and
// border already cover it.
func outsetHintArrow(arrow badge.HintArrow, width int) badge.HintArrow {
	// Which way the arrow points: down for a badge above its target, up for a
	// badge below it.
	toward := 1
	if arrow.Tip.Y < arrow.BaseLeft.Y {
		toward = -1
	}

	return badge.HintArrow{
		BaseLeft:  image.Pt(arrow.BaseLeft.X-width, arrow.BaseLeft.Y-toward*width),
		Tip:       image.Pt(arrow.Tip.X, arrow.Tip.Y+toward*width),
		BaseRight: image.Pt(arrow.BaseRight.X+width, arrow.BaseRight.Y-toward*width),
	}
}

// DrawRecursiveGrid renders the recursive-grid overlay, mirroring the
// cross-platform software renderer (cell subdivision, labels, sub-key preview,
// and the virtual pointer indicator).
//
// nextKeys/nextDims describe the *next* depth's layout, which each cell previews
// as a mini-grid of the keys that would select its sub-cells. They are zero when
// the region can no longer be divided, and then nothing is previewed.
//
// The dimensions arrive as domain.GridDimensions rather than as a column count
// beside a row count so that this backend has no pair to transpose on its way
// to ComputeGridCells (#1313).
func (o *winOverlay) DrawRecursiveGrid(
	bounds image.Rectangle,
	keys string,
	dims domain.GridDimensions,
	nextKeys string,
	nextDims domain.GridDimensions,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
) {
	if o == nil {
		return
	}

	o.ensureWindowForDraw()

	if o.window == nil {
		if o.logger != nil {
			o.logger.Error("DrawRecursiveGrid aborted, overlay window is nil")
		}

		return
	}

	if bounds.Empty() || dims.Cols <= 0 || dims.Rows <= 0 {
		return
	}

	o.cachedGrid = nil
	o.currentSubgrid = nil
	o.suppressDraw = false

	o.Clear()

	keyRunes := []rune(strings.ToUpper(keys))
	nextKeyRunes := []rune(strings.ToUpper(nextKeys))
	drawSubPreview := style.PreviewsNextDepth(len(nextKeyRunes), nextDims)

	cellRects := recursivegrid.ComputeGridCells(bounds, dims)
	for idx, cell := range cellRects {
		if style.HighlightColorARGB() != 0 {
			o.window.FillRect(cell, style.HighlightColorARGB())
		}

		if style.LineWidthF() > 0 {
			o.window.StrokeRect(cell, style.LineColorARGB(), style.LineWidthF())
		}

		if idx < len(keyRunes) {
			label := style.LabelChar()
			if label == "" {
				label = string(keyRunes[idx])
			}

			if style.ShowLabelIn(cell) {
				if style.LabelBackground() {
					o.drawRecursiveLabelBackground(label, cell, style)
				}

				o.drawTextCentered(
					label,
					cell,
					style.FontFamily(),
					style.LabelFontSize(),
					style.TextColorARGB(),
				)
			}

			if drawSubPreview && style.ShowSubKeyPreviewIn(cell, nextDims) {
				o.drawSubKeyMiniGrid(cell, nextKeyRunes, nextDims, style)
			}
		}
	}

	o.drawGridPointer(virtualPointer)

	o.flushOverlay("recursive-grid")
}

// drawGridPointer paints a grid mode's pointer stand-in into the pass the
// caller has begun, or nothing when the state is not visible. Grid and
// recursive grid both draw theirs here, on the one primitive the window owns.
//
// FontName arrives resolved: it comes from the Style, which settles every
// family it hands out. Resolving again here would be a lock and a cache lookup
// per drawn frame for the same answer.
func (o *winOverlay) drawGridPointer(pointer recursivegridcomponent.VirtualPointerState) {
	if !pointer.Visible || o.window == nil {
		return
	}

	o.window.DrawPointerGlyph(
		pointer.Position,
		pointer.Size,
		pointer.Char,
		pointer.FontName,
		badge.ParseHexARGB(pointer.FillColor),
	)
}

// drawFilledRect fills bounds then strokes its border, optionally with rounded
// corners. When radius > 0 the anti-aliased SDF rounded-rect primitives are
// used; otherwise the faster axis-aligned FillRect/StrokeRect path is taken.
func (o *winOverlay) drawFilledRect(
	bounds image.Rectangle,
	fill uint32,
	border uint32,
	lineWidth float64,
	radius float64,
) {
	if o == nil || o.window == nil {
		return
	}

	o.window.FillRoundedRect(bounds, radius, fill)

	if lineWidth > 0 {
		o.window.StrokeRoundedRect(bounds, radius, border, lineWidth)
	}
}

func (o *winOverlay) drawRecursiveLabelBackground(
	label string,
	cell image.Rectangle,
	style recursivegridcomponent.Style,
) {
	fontSize := style.LabelFontSize()
	paddingX := badge.AutoPadding(fontSize, style.LabelBackgroundPaddingX(), true)
	paddingY := badge.AutoPadding(fontSize, style.LabelBackgroundPaddingY(), false)
	width := badge.EstimateTextWidth(label, fontSize) + paddingX*winPaddingMultiplier
	height := badge.EstimateTextHeight(fontSize) + paddingY*winPaddingMultiplier
	rect := badge.CenteredIn(cell, width, height)

	o.drawFilledRect(
		rect,
		style.LabelBackgroundColorARGB(),
		style.LineColorARGB(),
		max(style.LabelBackgroundBorderWidthF(), 0),
		badge.BorderRadius(style.LabelBackgroundBorderRadius(), rect, 0),
	)
}

// drawSubKeyMiniGrid paints the next depth's keys inside one cell, each on the
// sub-cell it would select.
//
// Where they go is not this backend's arithmetic: Style.SubKeyPreviewCells
// divides the cell and decides which sub-cell is left blank, and the Cairo
// backend draws the same list (ADR 0007). What is left here is the painting.
func (o *winOverlay) drawSubKeyMiniGrid(
	cell image.Rectangle,
	nextKeyRunes []rune,
	nextDims domain.GridDimensions,
	style recursivegridcomponent.Style,
) {
	for _, subCell := range style.SubKeyPreviewCells(cell, nextKeyRunes, nextDims) {
		o.drawTextCentered(
			subCell.Label,
			subCell.Bounds,
			style.FontFamily(),
			style.SubKeyPreviewFontSizeF(),
			style.SubKeyPreviewTextColorARGB(),
		)
	}
}
