package vision

import (
	"image"
	"math"
	"slices"
	"strings"
	"unicode"

	"github.com/clipperhouse/displaywidth"
	"github.com/clipperhouse/uax29/v2/graphemes"

	"github.com/y3owk1n/neru/internal/ports"
)

// halfDivisor halves a length, thirdDivisor takes a third of one, and
// quarterDivisor a quarter.
const (
	halfDivisor    = 2
	thirdDivisor   = 3
	quarterDivisor = 4
)

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
//
// tesseract reads each Chinese, Japanese or Korean character as a word of its
// own, and its boxes for them drift by most of a character. A run of such
// words is placed from the ink instead (placeWideRuns).
func piecesFromSymbols(
	symbols []lineSymbol,
	img *image.RGBA,
	region image.Rectangle,
) [][]ports.TextPiece {
	var lines [][]ports.TextPiece

	placed := placeWideRuns(symbols, img)

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

		if allPlaced(placed, start, end) {
			for index, symbol := range word {
				place := placed[start+index]
				lines[line] = append(lines[line], ports.TextPiece{
					Text:       fullwidthPunctuation(symbol.text),
					Bounds:     toRegion(place.cell, img.Rect, region),
					Word:       place.word,
					Background: backgroundOf(img, place.cell),
				})
			}

			start = end

			continue
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

// placeWideRuns places each run of characters in a line that are words of
// their own or take two cells, when the run has one that takes two cells,
// keyed by the symbol's index. A run is one word, as Chinese and Japanese
// have no spaces, where tesseract reads each character as a word. A space
// ends a run, so Korean words stay apart. evenPitchCells cuts the run's ink
// into cells.
func placeWideRuns(symbols []lineSymbol, img *image.RGBA) map[int]wideCell {
	single := func(index int) bool {
		symbol := symbols[index]
		sameWord := func(other int) bool {
			return other >= 0 && other < len(symbols) && symbols[other].line == symbol.line &&
				symbols[other].word == symbol.word
		}

		return !sameWord(index-1) && !sameWord(index+1)
	}

	member := func(index int) bool { return single(index) || textHasWide(symbols[index].text) }

	placed := map[int]wideCell{}

	for start := 0; start < len(symbols); {
		if !member(start) {
			start++

			continue
		}

		// A run stops at a space between two words of several characters, as
		// between Korean words. tesseract's boxes for these characters drift,
		// so the space is looked for in the pixels.
		joins := func(index int) bool {
			if symbols[index].word == symbols[index-1].word || single(index) || single(index-1) {
				return true
			}

			return !spaceBetween(img, symbols[index-1].ink, symbols[index].ink)
		}

		end := start + 1
		for end < len(symbols) && symbols[end].line == symbols[start].line && member(end) &&
			joins(end) {
			end++
		}

		run := symbols[start:end]
		if slices.ContainsFunc(
			run,
			func(symbol lineSymbol) bool { return textHasWide(symbol.text) },
		) {
			for index, cell := range evenPitchCells(run, img) {
				placed[start+index] = wideCell{cell: cell, word: run[0].word}
			}
		}

		start = end
	}

	return placed
}

// wideCell is where placeWideRuns put a character, and the word it is in.
type wideCell struct {
	cell image.Rectangle
	word int
}

// spaceBetween reports whether img shows a blank stretch wider than a third
// of the characters' height between two characters' boxes, reaching across
// both since the boxes can drift.
func spaceBetween(img *image.RGBA, previous, current image.Rectangle) bool {
	box := previous.Union(current).Intersect(img.Rect)
	if box.Empty() {
		return false
	}

	background := lumaOf(img, box)
	space := max(previous.Dy(), current.Dy()) / thirdDivisor
	blank := 0

	for column := box.Min.X; column < box.Max.X; column++ {
		if inkedColumn(img, column, box, background) {
			blank = 0

			continue
		}

		blank++
		if blank > space {
			return true
		}
	}

	return false
}

// allPlaced reports whether every symbol from start to end has a placed cell.
func allPlaced(placed map[int]wideCell, start, end int) bool {
	for index := start; index < end; index++ {
		if _, ok := placed[index]; !ok {
			return false
		}
	}

	return true
}

// evenPitchCells cuts the ink under run into one cell per character, by
// evenPitchCuts. A gap inside a character, as between the halves of 测, would
// leave a sliver of a cell, which the pitch rules out.
func evenPitchCells(run []lineSymbol, img *image.RGBA) []image.Rectangle {
	box := run[0].ink
	for _, symbol := range run[1:] {
		box = box.Union(symbol.ink)
	}

	box = box.Intersect(img.Rect)
	if box.Empty() {
		return nil
	}

	background := lumaOf(img, box)

	inked := func(column int) bool { return inkedColumn(img, column, box, background) }

	// tesseract's box can stop inside a character or short of it, so the run
	// reaches to the ink at either end.
	left, right := box.Min.X, box.Max.X
	for inked(left - 1) {
		left--
	}

	for left < right && !inked(left) {
		left++
	}

	for inked(right) {
		right++
	}

	for right > left && !inked(right-1) {
		right--
	}

	edges := evenPitchCuts(left, right, inked, len(run))

	cells := make([]image.Rectangle, len(run))
	for index := range run {
		cells[index] = image.Rect(
			edges[index],
			box.Min.Y,
			max(edges[index+1], edges[index]+1),
			box.Max.Y,
		)
	}

	return cells
}

// evenPitchCuts cuts left to right into count cells closest to an even pitch,
// by the sum of squared differences, where a cut through ink costs as much as
// being half a pitch off. So cuts land in the gaps between characters, and
// where two characters touch, near the pitch. It returns the count+1 edges,
// evenly spaced when no cuts fit.
func evenPitchCuts(left, right int, inked func(int) bool, count int) []int {
	width := right - left
	pitch := float64(width) / float64(count)
	inkCost := pitch * pitch / quarterDivisor

	even := func() []int {
		edges := make([]int, count+1)
		for index := range edges {
			edges[index] = left + width*index/count
		}

		return edges
	}

	// A cell is a third of a pitch to twice one wide.
	narrowest := max(int(pitch/thirdDivisor), 1)
	widest := int(pitch*halfDivisor) + 1

	// best[cells][offset] is the least cost of cutting left to left+offset into
	// that many cells, and from[cells][offset] where the last of them starts.
	best := make([][]float64, count+1)
	from := make([][]int, count+1)

	for cells := range best {
		best[cells] = make([]float64, width+1)
		from[cells] = make([]int, width+1)

		for offset := range best[cells] {
			best[cells][offset] = math.Inf(1)
		}
	}

	best[0][0] = 0

	for cells := 1; cells <= count; cells++ {
		for offset := 1; offset <= width; offset++ {
			cut := 0.0
			if cells < count && inked(left+offset) {
				cut = inkCost
			}

			for size := narrowest; size <= min(widest, offset); size++ {
				diff := float64(size) - pitch

				total := best[cells-1][offset-size] + diff*diff + cut
				if total < best[cells][offset] {
					best[cells][offset], from[cells][offset] = total, offset-size
				}
			}
		}
	}

	if math.IsInf(best[count][width], 1) {
		return even()
	}

	edges := make([]int, count+1)
	edges[count] = right

	for cells, offset := count, width; cells > 0; cells-- {
		offset = from[cells][offset]
		edges[cells-1] = left + offset
	}

	return edges
}

// fullwidthPunctuation is the fullwidth form of the ASCII punctuation tesseract
// reads a Chinese or Japanese mark in a run as, such as "，" for ",". Other text
// is returned as is.
func fullwidthPunctuation(text string) string {
	const fullwidthOffset = 0xFEE0

	if len(text) == 1 && strings.ContainsRune("!(),:;?", rune(text[0])) {
		return string(rune(text[0]) + fullwidthOffset)
	}

	return text
}

// textHasWide reports whether text has a character that takes two cells.
func textHasWide(text string) bool {
	for _, r := range text {
		if displaywidth.Rune(r) > 1 {
			return true
		}
	}

	return false
}

// wordSymbols splits a word into one symbol per character for
// piecesFromSymbols, when the recognizer gave a box for the whole word only.
// box is in img's pixels. Each character takes a run of ink columns. The
// narrowest gaps between runs merge until there is one run per character, as
// macOS measures a word Vision gave no character boxes for. A word with fewer
// runs than characters, such as one whose letters touch, splits by the
// characters' widths.
func wordSymbols(img *image.RGBA, text string, box image.Rectangle, line, word int) []lineSymbol {
	var chars []string
	for char := graphemes.FromString(text); char.Next(); {
		chars = append(chars, char.Value())
	}

	symbol := func(char string, left, right int) lineSymbol {
		return lineSymbol{
			text: char, ink: image.Rect(left, box.Min.Y, right, box.Max.Y), line: line, word: word,
		}
	}

	visible := box.Intersect(img.Rect)
	if len(chars) <= 1 || visible.Empty() {
		return []lineSymbol{symbol(text, box.Min.X, box.Max.X)}
	}

	runs := inkRuns(img, visible)

	for len(runs) > len(chars) {
		narrowest := 0

		for index := 1; index+1 < len(runs); index++ {
			if runs[index+1][0]-runs[index][1] < runs[narrowest+1][0]-runs[narrowest][1] {
				narrowest = index
			}
		}

		next := narrowest + 1
		runs[narrowest][1] = runs[next][1]
		runs = slices.Delete(runs, next, next+1)
	}

	symbols := make([]lineSymbol, len(chars))

	if len(runs) == len(chars) {
		for index, char := range chars {
			symbols[index] = symbol(char, runs[index][0], runs[index][1])
		}

		return symbols
	}

	total := 0
	for _, char := range chars {
		total += max(displaywidth.String(char), 1)
	}

	left, cells := box.Min.X, 0

	for index, char := range chars {
		cells += max(displaywidth.String(char), 1)
		right := box.Min.X + box.Dx()*cells/total
		symbols[index] = symbol(char, left, right)
		left = right
	}

	return symbols
}

// inkRuns is the runs of columns of box in img that have ink, each as its
// first column and the one past its last.
func inkRuns(img *image.RGBA, box image.Rectangle) [][2]int {
	background := lumaOf(img, box)

	var runs [][2]int

	for column := box.Min.X; column < box.Max.X; column++ {
		if !inkedColumn(img, column, box, background) {
			continue
		}

		if len(runs) > 0 && runs[len(runs)-1][1] == column {
			runs[len(runs)-1][1] = column + 1
		} else {
			runs = append(runs, [2]int{column, column + 1})
		}
	}

	return runs
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

// mergeSecondLines is the eng lines, with the lines a second-language line
// overlaps replaced by it, when it is in another script and none of them are,
// which is a line eng read as Latin noise, sometimes in pieces. A line is in
// another script when it has a character taking two cells, as a Chinese word
// in an English line does, or when most of its letters are outside Latin
// script. A Russian read can give an English line a few Cyrillic lookalikes,
// which do not count. A line eng read in another script stays, and a line
// only the second read found joins the rest. Two reads are the same line when
// their boxes overlap by at least half the smaller box, as on macOS.
func mergeSecondLines(detected, second [][]ports.TextPiece) [][]ports.TextPiece {
	merged := slices.Clone(detected)

	for _, line := range second {
		if !lineInOtherScript(line) {
			continue
		}

		box := lineBounds(line)
		overlaps := func(other []ports.TextPiece) bool {
			otherBox := lineBounds(other)
			smaller := min(area(box), area(otherBox))

			return smaller > 0 && area(box.Intersect(otherBox))*halfDivisor >= smaller
		}

		if slices.ContainsFunc(merged, func(other []ports.TextPiece) bool {
			return overlaps(other) && lineInOtherScript(other)
		}) {
			continue
		}

		merged = append(slices.DeleteFunc(merged, overlaps), line)
	}

	return merged
}

// lineInOtherScript reports whether a line has a character taking two cells,
// or whether most of its letters are outside Latin script.
func lineInOtherScript(line []ports.TextPiece) bool {
	letters, other := 0, 0

	for _, piece := range line {
		if textHasWide(piece.Text) {
			return true
		}

		for _, r := range piece.Text {
			if unicode.IsLetter(r) {
				letters++

				if !unicode.Is(unicode.Latin, r) {
					other++
				}
			}
		}
	}

	return other > 0 && other*halfDivisor >= letters
}

// lineBounds is the box around a line's pieces.
func lineBounds(line []ports.TextPiece) image.Rectangle {
	var box image.Rectangle
	for _, piece := range line {
		box = box.Union(piece.Bounds)
	}

	return box
}
