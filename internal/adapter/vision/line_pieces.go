package vision

import (
	"image"
	"strings"

	"github.com/y3owk1n/neru/internal/ports"
)

// halfDivisor halves a length.
const halfDivisor = 2

// lineSymbol is one character a recognizer read, with the box of its ink in
// the captured image's pixels, and its line and its word within the line.
type lineSymbol struct {
	text       string
	ink        image.Rectangle
	line, word int
}

// piecesFromSymbols turns the characters a recognizer read into ReadLines
// pieces, one per character, in the global coordinates of region, which img
// shows. A word's characters get cells that meet halfway between their ink and
// share the word's height, so a point between two characters falls on one of
// them. Each piece carries its word's background color. macOS measures the
// same cells from the pixels in its bridge.
//
// tesseract places characters roughly, so each boundary between two of a
// word's cells then moves to the nearest blank column, as macOS does for wide
// characters. Sometimes one character's ink starts left of the one before it.
// Such a word becomes one piece, and vim mode splits it by the characters'
// widths.
func piecesFromSymbols(
	symbols []lineSymbol,
	img *image.RGBA,
	region image.Rectangle,
) [][]ports.TextPiece {
	var lines [][]ports.TextPiece

	for start := 0; start < len(symbols); {
		end := start + 1
		for end < len(symbols) && symbols[end].line == symbols[start].line &&
			symbols[end].word == symbols[start].word {
			end++
		}

		word := symbols[start:end]
		box := word[0].ink

		for _, symbol := range word[1:] {
			box = box.Union(symbol.ink)
		}

		line := word[0].line
		for len(lines) <= line {
			lines = append(lines, nil)
		}

		background := backgroundOf(img, box)

		if !leftToRight(word) {
			var text strings.Builder
			for _, symbol := range word {
				text.WriteString(symbol.text)
			}

			lines[line] = append(lines[line], ports.TextPiece{
				Text:       text.String(),
				Bounds:     toRegion(box, img.Rect, region),
				Word:       word[0].word,
				Background: background,
			})
			start = end

			continue
		}

		cells := snapToBlankColumns(symbolCells(word), img, box, snapWindow(img.Rect, region))

		for index, symbol := range word {
			cell := image.Rect(cells[index][0], box.Min.Y, cells[index][1], box.Max.Y)
			lines[line] = append(lines[line], ports.TextPiece{
				Text:       symbol.text,
				Bounds:     toRegion(cell, img.Rect, region),
				Word:       symbol.word,
				Background: background,
			})
		}

		start = end
	}

	return lines
}

// leftToRight reports whether each character's ink in word starts and ends
// right of the one before it.
func leftToRight(word []lineSymbol) bool {
	for index := 1; index < len(word); index++ {
		current, previous := word[index].ink, word[index-1].ink
		if current.Min.X < previous.Min.X || current.Max.X < previous.Max.X {
			return false
		}
	}

	return true
}

// symbolCells is the left and right edge of each character's cell in word:
// halfway between its ink and its neighbors', and half the word's mean gap
// past the ink at either end.
func symbolCells(word []lineSymbol) [][2]int {
	halfGap := 1

	if len(word) > 1 {
		gaps := 0
		for index := 1; index < len(word); index++ {
			gaps += max(word[index].ink.Min.X-word[index-1].ink.Max.X, 0)
		}

		halfGap = max(gaps/(len(word)-1)/halfDivisor, 1)
	}

	cells := make([][2]int, len(word))

	for index, symbol := range word {
		left, right := symbol.ink.Min.X-halfGap, symbol.ink.Max.X+halfGap
		if index > 0 {
			left = (word[index-1].ink.Max.X + symbol.ink.Min.X) / halfDivisor
		}

		if index+1 < len(word) {
			right = (symbol.ink.Max.X + word[index+1].ink.Min.X) / halfDivisor
		}

		cells[index] = [2]int{left, max(right, left+1)}
	}

	return cells
}

// snapPoints is how far, in points, a boundary between two characters may move
// to reach a blank column, as on macOS.
const snapPoints = 3

// snapWindow is snapPoints in the pixels of an image that shows region.
func snapWindow(pixels, region image.Rectangle) int {
	if region.Dx() == 0 {
		return snapPoints
	}

	return max(snapPoints*pixels.Dx()/region.Dx(), 1)
}

// inkContrast is how far a pixel's luma has to be from the background's to
// count as ink, as on macOS.
const inkContrast = 64

// snapToBlankColumns moves each boundary between two of a word's cells to the
// nearest column of box with no ink, within window pixels. tesseract places a
// character's box only roughly, so the halfway point between two boxes can
// land inside one of the characters, while the gap between them is a column or
// two away.
func snapToBlankColumns(cells [][2]int, img *image.RGBA, box image.Rectangle, window int) [][2]int {
	if len(cells) <= 1 {
		return cells
	}

	box = box.Intersect(img.Rect)
	if box.Empty() {
		return cells
	}

	background := lumaOf(img, box)

	blank := func(column int) bool {
		return column < box.Min.X || column >= box.Max.X ||
			!inkedColumn(img, column, box, background)
	}

	for index := 1; index < len(cells); index++ {
		edge := cells[index][0]

		for distance := 0; distance <= window; distance++ {
			if blank(edge - distance) {
				edge -= distance

				break
			}

			if blank(edge + distance) {
				edge += distance

				break
			}
		}

		edge = min(max(edge, cells[index-1][0]+1), cells[index][1]-1)
		cells[index-1][1], cells[index][0] = edge, edge
	}

	return cells
}

// inkedColumn reports whether column of img has ink between box's top and
// bottom, a pixel whose luma is more than inkContrast from background. A
// column outside img has none.
func inkedColumn(img *image.RGBA, column int, box image.Rectangle, background int) bool {
	if column < img.Rect.Min.X || column >= img.Rect.Max.X {
		return false
	}

	for y := box.Min.Y; y < box.Max.Y; y++ {
		pixel := img.RGBAAt(column, y)
		if abs(luma(pixel.R, pixel.G, pixel.B)-background) > inkContrast {
			return true
		}
	}

	return false
}

// lumaOf is the most common luma inside box in img. Text covers less of its
// own box than the background does, while the pixels around the box can run
// along a border, such as an editor's current-line highlight.
func lumaOf(img *image.RGBA, box image.Rectangle) int {
	var histogram [256]int

	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			pixel := img.RGBAAt(x, y)
			histogram[luma(pixel.R, pixel.G, pixel.B)]++
		}
	}

	best := 0
	for value, count := range histogram {
		if count > histogram[best] {
			best = value
		}
	}

	return best
}

// The Rec. 601 luma weights, in thousandths.
const (
	lumaRed   = 299
	lumaGreen = 587
	lumaBlue  = 114
	lumaScale = 1000
)

// luma is a pixel's brightness, 0 to 255.
func luma(red, green, blue uint8) int {
	return (lumaRed*int(red) + lumaGreen*int(green) + lumaBlue*int(blue)) / lumaScale
}

func abs(value int) int {
	if value < 0 {
		return -value
	}

	return value
}

// toRegion maps rect from the pixels of an image that shows region to region's
// global coordinates.
func toRegion(rect, pixels, region image.Rectangle) image.Rectangle {
	scale := func(value, from, to, fromSize, toSize int) int {
		if fromSize == 0 {
			return to
		}

		return to + (value-from)*toSize/fromSize
	}

	return image.Rect(
		scale(rect.Min.X, pixels.Min.X, region.Min.X, pixels.Dx(), region.Dx()),
		scale(rect.Min.Y, pixels.Min.Y, region.Min.Y, pixels.Dy(), region.Dy()),
		scale(rect.Max.X, pixels.Min.X, region.Min.X, pixels.Dx(), region.Dx()),
		scale(rect.Max.Y, pixels.Min.Y, region.Min.Y, pixels.Dy(), region.Dy()),
	)
}

// backgroundChannelBits is how many high bits of each channel the background
// is told apart by, so antialiased ink and gradients fall into the same color.
const backgroundChannelBits = 4

// backgroundOf is the most common color along the edges of box in img, as
// 0xRRGGBB with each channel cut to its top backgroundChannelBits, or zero
// when box shows no pixels.
func backgroundOf(img *image.RGBA, box image.Rectangle) uint32 {
	box = box.Inset(-1).Intersect(img.Rect)
	if box.Empty() {
		return 0
	}

	const (
		shift  = 8 - backgroundChannelBits
		levels = 1 << backgroundChannelBits
	)

	var counts [levels * levels * levels]int

	tally := func(x, y int) {
		pixel := img.RGBAAt(x, y)
		counts[(int(pixel.R>>shift)*levels+int(pixel.G>>shift))*levels+int(pixel.B>>shift)]++
	}

	for x := box.Min.X; x < box.Max.X; x++ {
		tally(x, box.Min.Y)
		tally(x, box.Max.Y-1)
	}

	for y := box.Min.Y; y < box.Max.Y; y++ {
		tally(box.Min.X, y)
		tally(box.Max.X-1, y)
	}

	best := 0
	for index, count := range counts {
		if count > counts[best] {
			best = index
		}
	}

	red := uint32(best/(levels*levels)) << shift
	green := uint32(best/levels%levels) << shift
	blue := uint32(best%levels) << shift

	return red<<16 | green<<8 | blue
}
