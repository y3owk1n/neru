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
// tesseract places characters roughly, and sometimes one's ink starts left of
// the one before it. Such a word becomes one piece, and vim mode splits it by
// the characters' widths.
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

		background := backgroundOf(img, box)

		line := word[0].line
		for len(lines) <= line {
			lines = append(lines, nil)
		}

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

		cells := symbolCells(word)

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
