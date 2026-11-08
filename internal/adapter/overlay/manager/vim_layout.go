package manager

import (
	"image"
	"math"
	"strings"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
)

// VimOpKind is what one vim draw operation paints.
type VimOpKind int

const (
	// VimOpRect fills Bounds with Fill and strokes it with Border.
	VimOpRect VimOpKind = iota
	// VimOpRoundedRect is VimOpRect with corners of Radius.
	VimOpRoundedRect
	// VimOpText draws Text centered in Bounds.
	VimOpText
)

// VimOp is one shape a backend paints for a vim frame, in the order given.
// Colors are 0xAARRGGBB.
type VimOp struct {
	Kind      VimOpKind
	Bounds    image.Rectangle
	Radius    float64
	Fill      uint32
	Border    uint32
	LineWidth float64
	Text      string
	Family    string
	FontSize  float64
	Bold      bool
}

// The vim surface's geometry, in points, the same on every backend.
const (
	vimContainerInsetX = 6
	vimContainerInsetY = 4
	vimContainerRadius = 6
	vimContainerStroke = 1.5
	vimCursorOutset    = 1
	vimCursorStroke    = 2
	vimMatchUnderline  = 2
	vimMatchStroke     = 1
	vimBadgePad        = 2
	vimBadgeRadius     = 3
	vimLensPad         = 4
	vimLensGap         = 6
	vimLensRadius      = 4
	vimStatusPad       = 6
	vimNoticePad       = 14
	vimNoticeMark      = 18
	vimNoticeGap       = 12
	vimNoticeTop       = 56
	vimNoticeRadius    = 10
	vimNoticeStroke    = 1.5
	vimNoticeLineGap   = 4
	vimDefaultFontSize = 13
	vimFontPadding     = 1.3
	vimAlphaMask       = 0x00FFFFFF
	// vimCurrentMatchAlpha is the fill alpha of the match under the cursor.
	vimCurrentMatchAlpha = 0x59
	// vimSides counts a padding once on each side, and vimHalves halves a
	// length to center something in it.
	vimSides  = 2
	vimHalves = 2
	// vimNoticeRowsWithDetail is a notice's rows when it has a detail line.
	vimNoticeRowsWithDetail = 2
)

// VimStatusBar is the strip the status bar covers at the bottom of region. It
// is one line of the style's font with vimStatusPad above and below. The macOS
// bridge sizes its bar by the font's own line height, which is no taller.
func VimStatusBar(region image.Rectangle, style VimStyle) image.Rectangle {
	if region.Empty() {
		return image.Rectangle{}
	}

	height := min(vimLineHeight(style)+vimStatusPad*vimSides, region.Dy())

	return image.Rect(region.Min.X, region.Max.Y-height, region.Max.X, region.Max.Y)
}

// vimFontSize is the style's font size, or the default when it sets none.
func vimFontSize(style VimStyle) float64 {
	if style.FontSize <= 0 {
		return vimDefaultFontSize
	}

	return float64(style.FontSize)
}

// vimLineHeight is the height of a line of the style's font.
func vimLineHeight(style VimStyle) int {
	return int(vimFontSize(style) * vimFontPadding)
}

// LayoutVim lays a vim scene out as the shapes a backend paints, with
// measure answering a text's width at the style's font family and the given
// size. The macOS bridge lays out the same geometry with Core Animation
// layers. Here the notice spinner is an arc drawn at scene.Spin's step.
func LayoutVim(
	scene VimScene,
	style VimStyle,
	measure func(text string, fontSize float64, bold bool) int,
) []VimOp {
	fontSize := vimFontSize(style)
	width := func(text string, bold bool) int { return measure(text, fontSize, bold) }
	lineHeight := vimLineHeight(style)
	cursor := badge.ParseHexARGB(style.CursorColor)
	match := badge.ParseHexARGB(style.MatchColor)
	statusBackground := badge.ParseHexARGB(style.StatusBackgroundColor)
	statusText := badge.ParseHexARGB(style.StatusTextColor)

	text := func(value string, bounds image.Rectangle, color uint32, bold bool) VimOp {
		return VimOp{
			Kind: VimOpText, Bounds: bounds, Text: value, Fill: color,
			Family: style.FontFamily, FontSize: fontSize, Bold: bold,
		}
	}

	var ops []VimOp

	if !scene.Container.Empty() {
		ops = append(ops, VimOp{
			Kind: VimOpRoundedRect,
			Bounds: image.Rect(
				scene.Container.Min.X-vimContainerInsetX, scene.Container.Min.Y-vimContainerInsetY,
				scene.Container.Max.X+vimContainerInsetX, scene.Container.Max.Y+vimContainerInsetY,
			),
			Radius:    vimContainerRadius,
			Border:    badge.ParseHexARGB(style.ContainerColor),
			LineWidth: vimContainerStroke,
		})
	}

	for _, rect := range scene.Selection {
		ops = append(
			ops,
			VimOp{Kind: VimOpRect, Bounds: rect, Fill: badge.ParseHexARGB(style.SelectionColor)},
		)
	}

	for index, rect := range scene.Matches {
		if index == scene.Current {
			ops = append(ops, VimOp{
				Kind: VimOpRect, Bounds: rect,
				Fill:   match&vimAlphaMask | vimCurrentMatchAlpha<<24,
				Border: match, LineWidth: vimMatchStroke,
			})

			continue
		}

		ops = append(ops, VimOp{
			Kind: VimOpRect,
			Bounds: image.Rect(
				rect.Min.X,
				rect.Max.Y-vimMatchUnderline/vimHalves,
				rect.Max.X,
				rect.Max.Y+vimMatchUnderline/vimHalves,
			),
			Fill: match,
		})
	}

	if !scene.Cursor.Empty() {
		box := scene.Cursor.Inset(-vimCursorOutset)
		if scene.Pending {
			ops = append(
				ops,
				VimOp{Kind: VimOpRect, Bounds: box, Border: cursor, LineWidth: vimCursorStroke},
			)
		} else {
			ops = append(ops, VimOp{Kind: VimOpRect, Bounds: box, Fill: cursor})
		}
	}

	for _, label := range scene.Labels {
		badgeWidth := width(label.Text, true) + vimBadgePad*vimSides
		height := lineHeight + vimBadgePad*vimSides
		box := image.Rect(
			label.Bounds.Min.X,
			label.Bounds.Max.Y-height,
			label.Bounds.Min.X+badgeWidth,
			label.Bounds.Max.Y,
		)
		ops = append(ops,
			VimOp{Kind: VimOpRoundedRect, Bounds: box, Radius: vimBadgeRadius, Fill: match},
			text(label.Text, box, statusText, true),
		)
	}

	if scene.Lens[1] != "" && !scene.Cursor.Empty() {
		ops = append(
			ops,
			vimLens(scene, lineHeight, width, cursor, statusBackground, statusText, text)...)
	}

	if scene.Status != "" && !scene.Region.Empty() {
		bar := VimStatusBar(scene.Region, style)
		left := bar.Min.X + vimStatusPad*vimSides
		ops = append(
			ops,
			VimOp{Kind: VimOpRect, Bounds: bar, Fill: statusBackground},
			text(
				scene.Status,
				image.Rect(left, bar.Min.Y, left+width(scene.Status, false), bar.Max.Y),
				statusText,
				false,
			),
		)
	}

	if scene.Notice != "" && !scene.Region.Empty() {
		ops = append(
			ops,
			vimNotice(scene, lineHeight, width, cursor, statusBackground, statusText, text)...)
	}

	return ops
}

// vimLens is the box above the cursor, or below it when there is no room
// above. It shows the word under the cursor as read, with the cursor's
// character marked.
func vimLens(
	scene VimScene,
	lineHeight int,
	measure func(string, bool) int,
	cursor, background, color uint32,
	text func(string, image.Rectangle, uint32, bool) VimOp,
) []VimOp {
	before, marked, after := scene.Lens[0], scene.Lens[1], scene.Lens[2]
	widths := [3]int{measure(before, false), measure(marked, true), measure(after, false)}
	width := widths[0] + widths[1] + widths[2] + vimLensPad*vimSides
	height := lineHeight + vimLensPad*vimSides

	top := scene.Cursor.Min.Y - vimLensGap - height
	if top < 0 {
		top = scene.Cursor.Max.Y + vimLensGap
	}

	left := max(scene.Cursor.Min.X-vimLensPad, 0)
	box := image.Rect(left, top, left+width, top+height)
	inner := box.Inset(vimLensPad)

	atBox := image.Rect(
		inner.Min.X+widths[0],
		inner.Min.Y,
		inner.Min.X+widths[0]+widths[1],
		inner.Max.Y,
	)
	ops := []VimOp{
		{
			Kind:      VimOpRoundedRect,
			Bounds:    box,
			Radius:    vimLensRadius,
			Fill:      background,
			Border:    cursor,
			LineWidth: 1,
		},
		{Kind: VimOpRect, Bounds: atBox, Fill: cursor},
		text(marked, atBox, color, true),
	}

	if before != "" {
		ops = append(
			ops,
			text(
				before,
				image.Rect(inner.Min.X, inner.Min.Y, atBox.Min.X, inner.Max.Y),
				color,
				false,
			),
		)
	}

	if after != "" {
		ops = append(
			ops,
			text(
				after,
				image.Rect(atBox.Max.X, inner.Min.Y, atBox.Max.X+widths[2], inner.Max.Y),
				color,
				false,
			),
		)
	}

	return ops
}

// vimNotice is the card near the top of the region that says what a wait is:
// a ring beside a bold title and, under it, a detail line.
func vimNotice(
	scene VimScene,
	lineHeight int,
	measure func(string, bool) int,
	accent, background, color uint32,
	text func(string, image.Rectangle, uint32, bool) VimOp,
) []VimOp {
	title, detail, _ := strings.Cut(scene.Notice, "\n")

	textWidth := max(measure(title, true), measure(detail, false))

	rows := 1
	if detail != "" {
		rows = vimNoticeRowsWithDetail
	}

	textHeight := rows*lineHeight + (rows-1)*vimNoticeLineGap
	width := textWidth + vimNoticeMark + vimNoticeGap + vimNoticePad*vimSides
	height := max(textHeight, vimNoticeMark) + vimNoticePad*vimSides

	left := scene.Region.Min.X + max((scene.Region.Dx()-width)/vimHalves, 0)
	top := min(scene.Region.Min.Y+vimNoticeTop, max(scene.Region.Max.Y-height, scene.Region.Min.Y))
	card := image.Rect(left, top, left+width, top+height)

	markTop := card.Min.Y + (height-vimNoticeMark)/vimHalves
	mark := image.Rect(
		card.Min.X+vimNoticePad,
		markTop,
		card.Min.X+vimNoticePad+vimNoticeMark,
		markTop+vimNoticeMark,
	)

	textLeft := mark.Max.X + vimNoticeGap
	textTop := card.Min.Y + (height-textHeight)/vimHalves
	titleBox := image.Rect(textLeft, textTop, textLeft+measure(title, true), textTop+lineHeight)

	ops := []VimOp{
		{
			Kind: VimOpRoundedRect, Bounds: card, Radius: vimNoticeRadius,
			Fill: background, Border: accent, LineWidth: vimNoticeStroke,
		},
		text(title, titleBox, color, true),
	}

	ops = append(ops, vimSpinner(mark, scene.Spin, accent)...)

	if detail != "" {
		detailTop := titleBox.Max.Y + vimNoticeLineGap
		ops = append(
			ops,
			text(
				detail,
				image.Rect(
					textLeft,
					detailTop,
					textLeft+measure(detail, false),
					detailTop+lineHeight,
				),
				color,
				false,
			),
		)
	}

	return ops
}

// vimSpinDots is how many dots ring the notice spinner, vimSpinLit how many
// of them are lit, and vimSpinDotDivisor how many dot widths span the ring.
// The dots overlap, so the lit ones draw one arc, three quarters of the ring
// as on macOS.
const (
	vimSpinDots       = 12
	vimSpinLit        = 9
	vimSpinDotDivisor = 4
)

// vimSpinner draws the notice's arc in mark as overlapping dots that end at
// step. Each redraw at the next step turns it.
func vimSpinner(mark image.Rectangle, step int, accent uint32) []VimOp {
	dot := max(mark.Dx()/vimSpinDotDivisor, 1)
	center := mark.Min.Add(mark.Size().Div(vimHalves))
	radius := float64(mark.Dx()-dot) / vimHalves

	ops := make([]VimOp, 0, vimSpinLit)

	for index := range vimSpinDots {
		behind := ((step-index)%vimSpinDots + vimSpinDots) % vimSpinDots
		if behind >= vimSpinLit {
			continue
		}

		angle := float64(index)*2*math.Pi/vimSpinDots - math.Pi/vimHalves
		dotCenter := image.Pt(
			center.X+int(math.Round(radius*math.Cos(angle))),
			center.Y+int(math.Round(radius*math.Sin(angle))),
		)
		corner := dotCenter.Sub(image.Pt(dot/vimHalves, dot/vimHalves))

		ops = append(ops, VimOp{
			Kind:   VimOpRoundedRect,
			Bounds: image.Rectangle{Min: corner, Max: corner.Add(image.Pt(dot, dot))},
			Radius: float64(dot) / vimHalves,
			Fill:   accent,
		})
	}

	return ops
}
