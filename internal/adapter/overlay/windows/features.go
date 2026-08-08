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

// Win32/GDI rendering for hints and recursive-grid overlays on Windows.
// Does not own window lifecycle or grid rendering (see overlay.go).
const (
	winPaddingMultiplier            = 2
	winSubKeyPreviewPaddingBottom   = 4
	winAutoRadiusBadgeCap           = 6.0
	winAutoRadiusBoundaryCap        = 4.0
	winMouseActionSquareRadiusScale = 0.18
	winMouseActionMinSquareRadius   = 2.0
)

// DrawHints renders the hint overlay using GDI, mirroring the cross-platform
// software renderer: an element-sized box per hint with a centered label.
// Each hint is rendered as an atomic unit (fill + stroke + text) so that
// overlapping hints have correct Z-ordering — later hints are fully on top of
// earlier ones, matching macOS behavior.
func (o *winOverlay) DrawHints(
	hintsSlice []*hintscomponent.Hint,
	style hintscomponent.StyleMode,
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

	for _, hint := range hintsSlice {
		if hint == nil {
			continue
		}

		// The element's own box, rebuilt from what the hint carries:
		// hint.Position() is the element center and hint.Size() its bounds. The
		// boundary highlight draws it and the badge is anchored to its corner.
		element := badge.CenteredOn(hint.Position(), hint.Size().X, hint.Size().Y)

		if style.BoundaryHighlightEnabled() {
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

		// Anchor the badge at the element's top-left corner rather than its
		// center so it does not cover the element's own content (e.g. the digit
		// on a calculator button) — deliberately not badge.CenteredIn, which
		// would put the badge in the middle of the element.
		bounds := image.Rect(
			element.Min.X,
			element.Min.Y,
			element.Min.X+badgeWidth,
			element.Min.Y+badgeHeight,
		)

		textColor := style.TextColor()
		if hint.MatchedPrefix() != "" {
			textColor = style.MatchedTextColor()
		}

		bdr := badge.BorderRadius(style.BorderRadius(), bounds, winAutoRadiusBadgeCap)
		o.window.FillRoundedRect(
			bounds, bdr, badge.ParseHexARGB(style.BackgroundColor()),
		)

		if bw := float64(max(style.BorderWidth(), 0)); bw > 0 {
			o.window.StrokeRoundedRect(
				bounds, bdr, badge.ParseHexARGB(style.BorderColor()), bw,
			)
		}

		// The window's own text call rather than drawTextCentered, so the
		// label lands inside this hint's composite boundary — but the family
		// arrives resolved here for the same reason drawTextCentered
		// documents, and this is the surface that redraws on every keystroke.
		o.window.DrawTextCentered(
			hint.Label(),
			bounds,
			style.FontFamily(),
			fontSize,
			badge.ParseHexARGB(textColor),
		)

		// Composite this hint atomically so its content lands as a unit,
		// giving correct Z-ordering with overlapping hints.
		o.window.CompositeCurrent()
	}

	o.flushOverlay("hints")
}

// DrawRecursiveGrid renders the recursive-grid overlay using GDI, mirroring the
// cross-platform software renderer (cell subdivision, labels, sub-key preview,
// and the virtual pointer indicator).
//
// The dimensions arrive as domain.GridDimensions rather than as a column count
// beside a row count so that this backend has no pair to transpose on its way
// to ComputeGridCells (#1313).
func (o *winOverlay) DrawRecursiveGrid(
	bounds image.Rectangle,
	keys string,
	dims domain.GridDimensions,
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

			if shouldShowWinSubKeyPreview(cell, style) {
				o.drawRecursiveSubKeyPreview(label, cell, style)
			}
		}
	}

	if virtualPointer.Visible {
		vpChar := virtualPointer.Char
		if vpChar == "" {
			vpChar = "\u25CF"
		}

		// FontName arrives resolved: it comes from the Style, which settles
		// every family it hands out. Resolving again here would be a lock and
		// a cache lookup per drawn frame for the same answer.
		fontName := virtualPointer.FontName

		fontSize := float64(virtualPointer.Size)
		// Not badge.CenteredOn: the half is floored at 1 so a pointer
		// configured to size 0 or 1 still has a box to draw its glyph in.
		halfSize := max(virtualPointer.Size/2, 1) //nolint:mnd

		vpBounds := image.Rect(
			virtualPointer.Position.X-halfSize,
			virtualPointer.Position.Y-halfSize,
			virtualPointer.Position.X+halfSize,
			virtualPointer.Position.Y+halfSize,
		)
		o.drawTextCentered(
			vpChar,
			vpBounds,
			fontName,
			fontSize,
			badge.ParseHexARGB(virtualPointer.FillColor),
		)
	}

	o.flushOverlay("recursive-grid")
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

func (o *winOverlay) drawRecursiveSubKeyPreview(
	label string,
	cell image.Rectangle,
	style recursivegridcomponent.Style,
) {
	previewLabel := style.SubKeyPreviewLabelChar()
	if previewLabel == "" {
		previewLabel = label
	}

	previewRect := image.Rect(
		cell.Min.X,
		cell.Max.Y-badge.EstimateTextHeight(
			style.SubKeyPreviewFontSizeF(),
		)-winSubKeyPreviewPaddingBottom,
		cell.Max.X,
		cell.Max.Y,
	)

	o.drawTextCentered(
		previewLabel,
		previewRect,
		style.FontFamily(),
		style.SubKeyPreviewFontSizeF(),
		style.SubKeyPreviewTextColorARGB(),
	)
}

// shouldShowWinSubKeyPreview reports whether the single preview label this
// backend draws along the bottom of a cell is worth drawing: the cell must
// reach sub_key_preview_autohide_multiplier x the preview font size.
//
// Unlike the label autohide threshold (recursivegridcomponent.Style.ShowLabelIn)
// this is not shared with the Linux backend, because the two do not draw the
// same thing: Linux and macOS draw a mini-grid of the next level's keys and
// measure a sub-cell, this backend draws one label and measures the cell.
func shouldShowWinSubKeyPreview(cell image.Rectangle, style recursivegridcomponent.Style) bool {
	if !style.SubKeyPreview() {
		return false
	}

	if style.SubKeyPreviewAutohideMultiplier() <= 0 {
		return true
	}

	threshold := style.SubKeyPreviewFontSizeF() * style.SubKeyPreviewAutohideMultiplier()

	return float64(cell.Dx()) >= threshold && float64(cell.Dy()) >= threshold
}
