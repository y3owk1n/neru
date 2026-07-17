//go:build windows

// internal/ui/overlay/manager_windows_features.go
// Win32/GDI rendering for hints and recursive-grid overlays on Windows.
// Does not own window lifecycle or grid rendering (see manager_windows_overlay.go).

package overlay

import (
	"image"
	"math"
	"strconv"
	"strings"

	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/core/ports"
)

const (
	winSubgridLineWidth = 1

	winHexColorOpaque     = 0xFFFFFFFF
	winHexRepeatCount     = 2
	winHexColorLenShort   = 3
	winHexColorLenNoAlpha = 6
	winHexColorLenFull    = 8

	winAutoPaddingHorizontalMultiplier = 0.6
	winAutoPaddingVerticalMultiplier   = 0.35
	winAutoPaddingMinHorizontal        = 6
	winAutoPaddingMinVertical          = 4
	winTextWidthMultiplier             = 0.7
	winTextHeightMultiplier            = 1.4
	winCenteredRectDivisor             = 2
	winPaddingMultiplier               = 2
	winSubKeyPreviewPaddingBottom      = 4

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

		if style.BoundaryHighlightEnabled() {
			boundary := image.Rect(
				hint.Position().X-hint.Size().X/2,
				hint.Position().Y-hint.Size().Y/2,
				hint.Position().X+hint.Size().X/2,
				hint.Position().Y+hint.Size().Y/2,
			)
			bdr := resolveWinBorderRadius(
				style.BoundaryBorderRadius(), boundary, winAutoRadiusBoundaryCap,
			)
			o.window.FillRoundedRect(
				boundary, bdr, parseHexColorARGB(style.BoundaryBackgroundColor()),
			)

			if bw := float64(max(style.BoundaryBorderWidth(), 0)); bw > 0 {
				o.window.StrokeRoundedRect(
					boundary, bdr, parseHexColorARGB(style.BoundaryBorderColor()), bw,
				)
			}
		}

		// Size the badge to the label text, not the element. hint.Size() is the
		// element's bounding box (hint.Bounds().Size()), so using it makes the
		// badge as large as the element (e.g. oversized boxes over big buttons).
		fontSize := float64(max(style.FontSize(), 1))
		paddingX := resolveWinAutoPadding(fontSize, style.PaddingX(), true)
		paddingY := resolveWinAutoPadding(fontSize, style.PaddingY(), false)
		badgeWidth := estimateWinTextWidth(hint.Label(), fontSize) + paddingX*winPaddingMultiplier
		badgeHeight := estimateWinTextHeight(fontSize) + paddingY*winPaddingMultiplier

		// Anchor the badge at the element's top-left corner rather than its
		// center so it does not cover the element's own content (e.g. the digit
		// on a calculator button). hint.Position() is the element center and
		// hint.Size() its bounds, so the top-left is center minus half-size.
		originX := hint.Position().X - hint.Size().X/winCenteredRectDivisor
		originY := hint.Position().Y - hint.Size().Y/winCenteredRectDivisor
		bounds := image.Rect(originX, originY, originX+badgeWidth, originY+badgeHeight)

		textColor := style.TextColor()
		if hint.MatchedPrefix() != "" {
			textColor = style.MatchedTextColor()
		}

		bdr := resolveWinBorderRadius(style.BorderRadius(), bounds, winAutoRadiusBadgeCap)
		o.window.FillRoundedRect(
			bounds, bdr, parseHexColorARGB(style.BackgroundColor()),
		)

		if bw := float64(max(style.BorderWidth(), 0)); bw > 0 {
			o.window.StrokeRoundedRect(
				bounds, bdr, parseHexColorARGB(style.BorderColor()), bw,
			)
		}

		o.window.DrawTextCentered(
			hint.Label(),
			bounds,
			ports.ResolveFont(style.FontFamily(), false),
			fontSize,
			parseHexColorARGB(textColor),
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
func (o *winOverlay) DrawRecursiveGrid(
	bounds image.Rectangle,
	keys string,
	gridCols int,
	gridRows int,
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

	if bounds.Empty() || gridCols <= 0 || gridRows <= 0 {
		return
	}

	o.cachedGrid = nil
	o.currentSubgrid = nil
	o.suppressDraw = false

	o.Clear()

	keyRunes := []rune(strings.ToUpper(keys))
	cellWidth := bounds.Dx() / gridCols
	cellHeight := bounds.Dy() / gridRows

	index := 0
	for row := range gridRows {
		for col := range gridCols {
			cell := image.Rect(
				bounds.Min.X+col*cellWidth,
				bounds.Min.Y+row*cellHeight,
				bounds.Min.X+(col+1)*cellWidth,
				bounds.Min.Y+(row+1)*cellHeight,
			)
			if col == gridCols-1 {
				cell.Max.X = bounds.Max.X
			}

			if row == gridRows-1 {
				cell.Max.Y = bounds.Max.Y
			}

			if style.HighlightColor != 0 {
				o.window.FillRect(cell, style.HighlightColor)
			}

			if style.LineWidth > 0 {
				o.window.StrokeRect(cell, style.LineColor, style.LineWidth)
			}

			if index < len(keyRunes) {
				label := style.LabelChar
				if label == "" {
					label = string(keyRunes[index])
				}

				if style.LabelBackground {
					o.drawRecursiveLabelBackground(label, cell, style)
				}

				o.drawTextCentered(
					label,
					cell,
					ports.ResolveFont(style.LabelFontName, false),
					style.LabelFontSize,
					style.LabelFontColor,
				)

				if shouldShowWinSubKeyPreview(cell, style) {
					o.drawRecursiveSubKeyPreview(label, cell, style)
				}
			}

			index++
		}
	}

	if virtualPointer.Visible {
		vpBounds := image.Rect(
			virtualPointer.Position.X-virtualPointer.Size/2,
			virtualPointer.Position.Y-virtualPointer.Size/2,
			virtualPointer.Position.X+virtualPointer.Size/2,
			virtualPointer.Position.Y+virtualPointer.Size/2,
		)
		o.drawFilledRect(
			vpBounds,
			parseHexColorARGB(virtualPointer.FillColor),
			style.LineColor,
			winSubgridLineWidth,
			0,
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
	fontSize := style.LabelFontSize
	paddingX := resolveWinAutoPadding(fontSize, style.LabelBackgroundPaddingX, true)
	paddingY := resolveWinAutoPadding(fontSize, style.LabelBackgroundPaddingY, false)
	width := estimateWinTextWidth(label, fontSize) + paddingX*winPaddingMultiplier
	height := estimateWinTextHeight(fontSize) + paddingY*winPaddingMultiplier
	rect := winCenteredRect(cell, width, height)

	o.drawFilledRect(
		rect,
		style.LabelBackgroundColor,
		style.LineColor,
		max(style.LabelBackgroundBorderWidth, 0),
		resolveWinBorderRadius(style.LabelBackgroundBorderRadius, rect, 0),
	)
}

func (o *winOverlay) drawRecursiveSubKeyPreview(
	label string,
	cell image.Rectangle,
	style recursivegridcomponent.Style,
) {
	previewLabel := style.SubKeyPreviewLabelChar
	if previewLabel == "" {
		previewLabel = label
	}

	previewRect := image.Rect(
		cell.Min.X,
		cell.Max.Y-estimateWinTextHeight(style.SubKeyPreviewFontSize)-winSubKeyPreviewPaddingBottom,
		cell.Max.X,
		cell.Max.Y,
	)

	o.drawTextCentered(
		previewLabel,
		previewRect,
		ports.ResolveFont(style.LabelFontName, false),
		style.SubKeyPreviewFontSize,
		style.SubKeyPreviewTextColor,
	)
}

func shouldShowWinSubKeyPreview(cell image.Rectangle, style recursivegridcomponent.Style) bool {
	if !style.SubKeyPreview {
		return false
	}

	if style.SubKeyPreviewAutohideMultiplier <= 0 {
		return true
	}

	threshold := style.SubKeyPreviewFontSize * style.SubKeyPreviewAutohideMultiplier

	return float64(cell.Dx()) >= threshold && float64(cell.Dy()) >= threshold
}

func resolveWinAutoPadding(fontSize float64, padding int, horizontal bool) int {
	if padding >= 0 {
		return padding
	}

	if horizontal {
		return max(int(fontSize*winAutoPaddingHorizontalMultiplier), winAutoPaddingMinHorizontal)
	}

	return max(int(fontSize*winAutoPaddingVerticalMultiplier), winAutoPaddingMinVertical)
}

// resolveWinBorderRadius resolves a configured border-radius value for the
// given rectangle. Negative values select an automatic radius: autoCap limits
// the auto-radius for badge-style corners (e.g. 6 px for hint badges); pass 0
// for a full pill shape (label backgrounds). Zero means sharp corners.
// Positive values are clamped to half the smaller dimension.
func resolveWinBorderRadius(configured int, bounds image.Rectangle, autoCap float64) float64 {
	maxR := float64(min(bounds.Dx(), bounds.Dy())) / 2 //nolint:mnd // half the smaller dimension

	if configured < 0 {
		if autoCap > 0 {
			return min(maxR, autoCap)
		}

		return maxR
	}

	if configured == 0 {
		return 0
	}

	return min(float64(configured), maxR)
}

func estimateWinTextWidth(text string, fontSize float64) int {
	return int(math.Ceil(float64(len([]rune(text))) * fontSize * winTextWidthMultiplier))
}

func estimateWinTextHeight(fontSize float64) int {
	return int(math.Ceil(fontSize * winTextHeightMultiplier))
}

func winCenteredRect(cell image.Rectangle, width, height int) image.Rectangle {
	centerX := cell.Min.X + cell.Dx()/winCenteredRectDivisor
	centerY := cell.Min.Y + cell.Dy()/winCenteredRectDivisor

	return image.Rect(
		centerX-width/winCenteredRectDivisor,
		centerY-height/winCenteredRectDivisor,
		centerX-width/winCenteredRectDivisor+width,
		centerY-height/winCenteredRectDivisor+height,
	)
}

func parseHexColorARGB(value string) uint32 {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	switch len(value) {
	case winHexColorLenShort:
		value = "FF" + strings.Repeat(string(value[0]), winHexRepeatCount) +
			strings.Repeat(string(value[1]), winHexRepeatCount) +
			strings.Repeat(string(value[2]), winHexRepeatCount)
	case winHexColorLenNoAlpha:
		value = "FF" + value
	case winHexColorLenFull:
	default:
		return winHexColorOpaque
	}

	parsed, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return winHexColorOpaque
	}

	return uint32(parsed)
}
