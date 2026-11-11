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
	vimOutlineStroke   = 1
	// The block around the container is outlined further out, at a third of
	// the container outline's opacity.
	vimBlockInsetX    = 12
	vimBlockInsetY    = 10
	vimBlockFade      = 3
	vimCursorOutset   = 1
	vimCursorStroke   = 2
	vimMatchUnderline = 2
	vimMatchStroke    = 1
	// vimBadgeRadiusCap caps an automatic badge corner, as it is capped for a
	// hint badge.
	vimBadgeRadiusCap  = 6
	vimLensGap         = 6
	vimStatusPad       = 6
	vimStatusRule      = 1
	vimChipInset       = 3
	vimChipPadX        = 8
	vimChipStroke      = 1
	vimNoticePad       = 14
	vimNoticeMark      = 18
	vimNoticeGap       = 12
	vimNoticeTop       = 56
	vimNoticeLineGap   = 4
	vimDefaultFontSize = 13
	vimFontPadding     = 1.3
	vimAlphaMask       = 0x00FFFFFF
	vimAlphaShift      = 24
	// vimCurrentMatchAlpha is the fill alpha of the match under the cursor.
	vimCurrentMatchAlpha = 0x59
	// The cursor's fill keeps vimCursorFillShare/vimAlphaFull of its color's
	// alpha, so the character under it stays readable. The bar down its left
	// edge is the color itself.
	vimCursorFillShare = 0x59
	vimAlphaFull       = 0xFF
	// The notice's detail line is its text color at three quarters of its
	// alpha.
	vimDetailShare   = 3
	vimDetailDivisor = 4
	// vimSides counts a padding once on each side, and vimHalves halves a
	// length to center something in it.
	vimSides  = 2
	vimHalves = 2
	// vimNoticeRowsWithDetail is a notice's rows when it has a detail line.
	vimNoticeRowsWithDetail = 2
	// vimEllipsis ends status text cut short to fit the bar.
	vimEllipsis = "…"
)

// VimMeasure answers how wide text draws in a font family at a size.
type VimMeasure func(text, family string, fontSize float64, bold bool) int

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

// vimBadgeFontSize is the badge's font size, or the default when it sets
// none.
func vimBadgeFontSize(style VimStyle) float64 {
	if style.Badge.FontSize <= 0 {
		return vimDefaultFontSize
	}

	return float64(style.Badge.FontSize)
}

// vimLineHeight is the height of a line of the style's font.
func vimLineHeight(style VimStyle) int {
	return int(vimFontSize(style) * vimFontPadding)
}

// vimWithAlpha is color with its alpha scaled by share/vimAlphaFull.
func vimWithAlpha(color, share uint32) uint32 {
	return color&vimAlphaMask | (color>>vimAlphaShift*share/vimAlphaFull)<<vimAlphaShift
}

// vimFace is one font a vim frame draws text in.
type vimFace struct {
	family string
	size   float64
	bold   bool
}

// vimPainter lays out one vim frame's text and boxes.
type vimPainter struct {
	style   VimStyle
	measure VimMeasure
	ops     []VimOp
}

func (p *vimPainter) width(text string, face vimFace) int {
	return p.measure(text, face.family, face.size, face.bold)
}

func (p *vimPainter) text(value string, bounds image.Rectangle, color uint32, face vimFace) {
	p.ops = append(p.ops, VimOp{
		Kind: VimOpText, Bounds: bounds, Text: value, Fill: color,
		Family: face.family, FontSize: face.size, Bold: face.bold,
	})
}

// badge draws box as a hint badge.
func (p *vimPainter) badge(box image.Rectangle) {
	p.ops = append(p.ops, VimOp{
		Kind:      VimOpRoundedRect,
		Bounds:    box,
		Radius:    badge.BorderRadius(p.style.Badge.BorderRadius, box, vimBadgeRadiusCap),
		Fill:      badge.ParseHexARGB(p.style.Badge.Background),
		Border:    badge.ParseHexARGB(p.style.Badge.Border),
		LineWidth: float64(p.style.Badge.BorderWidth),
	})
}

// fit is text cut to fit within limit, with an ellipsis where it was cut. It
// cuts the start when fromStart, and otherwise the end.
func (p *vimPainter) fit(text string, limit int, face vimFace, fromStart bool) string {
	if p.width(text, face) <= limit {
		return text
	}

	runes := []rune(text)
	for keep := len(runes) - 1; keep > 0; keep-- {
		shown := string(runes[:keep]) + vimEllipsis
		if fromStart {
			shown = vimEllipsis + string(runes[len(runes)-keep:])
		}

		if p.width(shown, face) <= limit {
			return shown
		}
	}

	return ""
}

// LayoutVim lays a vim scene out as the shapes a backend paints, with
// measure answering a text's width. The macOS bridge lays out the same
// geometry with Core Animation layers. Here the notice spinner is an arc
// drawn at scene.Spin's step.
func LayoutVim(scene VimScene, style VimStyle, measure VimMeasure) []VimOp {
	painter := &vimPainter{style: style, measure: measure}
	cursor := badge.ParseHexARGB(style.CursorColor)
	match := badge.ParseHexARGB(style.MatchColor)

	if !scene.Block.Empty() {
		border := badge.ParseHexARGB(style.ContainerColor)

		painter.ops = append(painter.ops, VimOp{
			Kind: VimOpRoundedRect,
			Bounds: image.Rect(
				scene.Block.Min.X-vimBlockInsetX, scene.Block.Min.Y-vimBlockInsetY,
				scene.Block.Max.X+vimBlockInsetX, scene.Block.Max.Y+vimBlockInsetY,
			),
			Radius:    vimContainerRadius,
			Border:    vimWithAlpha(border, vimAlphaFull/vimBlockFade),
			LineWidth: vimOutlineStroke,
		})
	}

	if !scene.Container.Empty() {
		painter.ops = append(painter.ops, VimOp{
			Kind: VimOpRoundedRect,
			Bounds: image.Rect(
				scene.Container.Min.X-vimContainerInsetX, scene.Container.Min.Y-vimContainerInsetY,
				scene.Container.Max.X+vimContainerInsetX, scene.Container.Max.Y+vimContainerInsetY,
			),
			Radius:    vimContainerRadius,
			Border:    badge.ParseHexARGB(style.ContainerColor),
			LineWidth: vimOutlineStroke,
		})
	}

	for _, rect := range scene.Selection {
		painter.ops = append(
			painter.ops,
			VimOp{Kind: VimOpRect, Bounds: rect, Fill: badge.ParseHexARGB(style.SelectionColor)},
		)
	}

	for index, rect := range scene.Matches {
		if index == scene.Current {
			painter.ops = append(painter.ops, VimOp{
				Kind: VimOpRect, Bounds: rect,
				Fill:   match&vimAlphaMask | vimCurrentMatchAlpha<<vimAlphaShift,
				Border: match, LineWidth: vimMatchStroke,
			})

			continue
		}

		painter.ops = append(painter.ops, VimOp{
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
		painter.cursor(scene.Cursor.Inset(-vimCursorOutset), scene.Pending, cursor)
	}

	painter.labels(scene.Labels)

	if scene.Lens[1] != "" && !scene.Cursor.Empty() {
		painter.lens(scene, cursor)
	}

	if scene.Mode != "" && !scene.Region.Empty() {
		painter.statusBar(scene)
	}

	if scene.Notice != "" && !scene.Region.Empty() {
		painter.notice(scene)
	}

	return painter.ops
}

// cursor draws the cursor on box. While an operator waits for its motion it
// is an outline. Otherwise it is a light fill with a bar down its left edge,
// which a selection's fill and a match's underline never have.
func (p *vimPainter) cursor(box image.Rectangle, pending bool, color uint32) {
	if pending {
		p.ops = append(
			p.ops,
			VimOp{Kind: VimOpRect, Bounds: box, Border: color, LineWidth: vimCursorStroke},
		)

		return
	}

	p.ops = append(p.ops,
		VimOp{Kind: VimOpRect, Bounds: box, Fill: vimWithAlpha(color, vimCursorFillShare)},
		VimOp{
			Kind:   VimOpRect,
			Bounds: image.Rect(box.Min.X, box.Min.Y, box.Min.X+vimCursorStroke, box.Max.Y),
			Fill:   color,
		},
	)
}

// labels draws each jump label as a hint badge centered on the character it
// names.
func (p *vimPainter) labels(labels []VimLabel) {
	face := vimFace{family: p.style.Badge.FontFamily, size: vimBadgeFontSize(p.style), bold: true}
	textColor := badge.ParseHexARGB(p.style.Badge.Text)
	height := badge.EstimateTextHeight(face.size) + p.style.Badge.PaddingY*vimSides

	for _, label := range labels {
		width := p.width(label.Text, face) + p.style.Badge.PaddingX*vimSides
		center := label.Bounds.Min.Add(label.Bounds.Size().Div(vimHalves))
		box := badge.CenteredOn(center, width, height)

		p.badge(box)
		p.text(label.Text, box, textColor, face)
	}
}

// lens is the hint badge above the cursor, or below it when there is no room
// above. It shows the word under the cursor as read, with the cursor's
// character marked.
func (p *vimPainter) lens(scene VimScene, cursor uint32) {
	before, marked, after := scene.Lens[0], scene.Lens[1], scene.Lens[2]
	plain := vimFace{family: p.style.Badge.FontFamily, size: vimBadgeFontSize(p.style)}
	bold := plain
	bold.bold = true
	widths := [3]int{p.width(before, plain), p.width(marked, bold), p.width(after, plain)}
	padX, padY := p.style.Badge.PaddingX, p.style.Badge.PaddingY
	width := widths[0] + widths[1] + widths[2] + padX*vimSides
	height := badge.EstimateTextHeight(plain.size) + padY*vimSides

	top := scene.Cursor.Min.Y - vimLensGap - height
	if top < 0 {
		top = scene.Cursor.Max.Y + vimLensGap
	}

	left := max(scene.Cursor.Min.X-padX, 0)
	box := image.Rect(left, top, left+width, top+height)
	inner := image.Rect(box.Min.X+padX, box.Min.Y+padY, box.Max.X-padX, box.Max.Y-padY)
	textColor := badge.ParseHexARGB(p.style.Badge.Text)

	atBox := image.Rect(
		inner.Min.X+widths[0],
		inner.Min.Y,
		inner.Min.X+widths[0]+widths[1],
		inner.Max.Y,
	)

	p.badge(box)
	p.ops = append(p.ops, VimOp{Kind: VimOpRect, Bounds: atBox, Fill: cursor})
	p.text(marked, atBox, badge.ParseHexARGB(p.style.OnModeColor), bold)

	if before != "" {
		p.text(
			before,
			image.Rect(inner.Min.X, inner.Min.Y, atBox.Min.X, inner.Max.Y),
			textColor,
			plain,
		)
	}

	if after != "" {
		p.text(
			after,
			image.Rect(atBox.Max.X, inner.Min.Y, atBox.Max.X+widths[2], inner.Max.Y),
			textColor,
			plain,
		)
	}
}

// statusBar draws the bar along the region's bottom edge: a rule along its
// top, the mode chip at its left, the status after the chip and the right
// status at its right end. The status is cut short before it reaches the
// right status.
func (p *vimPainter) statusBar(scene VimScene) {
	style := p.style
	bar := VimStatusBar(scene.Region, style)
	face := vimFace{family: style.FontFamily, size: vimFontSize(style)}
	bold := face
	bold.bold = true
	textColor := badge.ParseHexARGB(style.StatusTextColor)

	p.ops = append(p.ops,
		VimOp{Kind: VimOpRect, Bounds: bar, Fill: badge.ParseHexARGB(style.StatusBackgroundColor)},
		VimOp{
			Kind:   VimOpRect,
			Bounds: image.Rect(bar.Min.X, bar.Min.Y, bar.Max.X, bar.Min.Y+vimStatusRule),
			Fill:   badge.ParseHexARGB(style.Badge.Border),
		},
	)

	chipLeft := bar.Min.X + vimStatusPad
	chip := image.Rect(
		chipLeft,
		bar.Min.Y+vimChipInset,
		chipLeft+p.width(scene.Mode, bold)+vimChipPadX*vimSides,
		bar.Max.Y-vimChipInset,
	)
	chipOp := VimOp{
		Kind:   VimOpRoundedRect,
		Bounds: chip,
		Radius: badge.BorderRadius(-1, chip, vimBadgeRadiusCap),
	}
	chipText := badge.ParseHexARGB(style.OnModeColor)

	switch scene.ModeKind {
	case VimModeNormal:
		chipOp.Fill = badge.ParseHexARGB(style.ModeColor)
	case VimModeVisual:
		chipOp.Fill = badge.ParseHexARGB(style.VisualModeColor)
	case VimModeOther:
		chipOp.Border = badge.ParseHexARGB(style.Badge.Border)
		chipOp.LineWidth = vimChipStroke
		chipText = textColor
	}

	p.ops = append(p.ops, chipOp)
	p.text(scene.Mode, chip, chipText, bold)

	right := bar.Max.X - vimStatusPad*vimSides
	if scene.StatusRight != "" {
		rightWidth := p.width(scene.StatusRight, face)
		p.text(
			scene.StatusRight,
			image.Rect(right-rightWidth, bar.Min.Y, right, bar.Max.Y),
			textColor,
			face,
		)
		right -= rightWidth + vimStatusPad*vimSides
	}

	left := chip.Max.X + vimStatusPad*vimSides
	if status := p.fit(scene.Status, right-left, face, scene.StatusTyped); status != "" {
		p.text(
			status,
			image.Rect(left, bar.Min.Y, left+p.width(status, face), bar.Max.Y),
			textColor,
			face,
		)
	}
}

// notice is the hint badge near the top of the region that says what a wait
// is: a ring beside a bold title and, under it, a detail line.
func (p *vimPainter) notice(scene VimScene) {
	style := p.style
	face := vimFace{family: style.FontFamily, size: vimFontSize(style)}
	bold := face
	bold.bold = true
	lineHeight := vimLineHeight(style)
	textColor := badge.ParseHexARGB(style.Badge.Text)
	title, detail, _ := strings.Cut(scene.Notice, "\n")

	textWidth := max(p.width(title, bold), p.width(detail, face))

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
	titleBox := image.Rect(textLeft, textTop, textLeft+p.width(title, bold), textTop+lineHeight)

	p.badge(card)
	p.text(title, titleBox, textColor, bold)
	p.ops = append(p.ops, vimSpinner(mark, scene.Spin, badge.ParseHexARGB(style.ModeColor))...)

	if detail != "" {
		detailTop := titleBox.Max.Y + vimNoticeLineGap
		p.text(
			detail,
			image.Rect(textLeft, detailTop, textLeft+p.width(detail, face), detailTop+lineHeight),
			vimWithAlpha(textColor, vimAlphaFull*vimDetailShare/vimDetailDivisor),
			face,
		)
	}
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
