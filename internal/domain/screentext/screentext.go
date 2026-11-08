// Package screentext lays the words read off the screen out as text, so the
// vim grammar can move over the screen as it moves over a buffer. Each byte
// of the text maps back to the pixels it was read from.
package screentext

import (
	"image"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/clipperhouse/displaywidth"
	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/y3owk1n/fude/vim"
)

const (
	// maxBlankLines is the most blank lines one vertical gap becomes. One is
	// what paragraph motions need, and more is a picture's empty space.
	maxBlankLines = 2
	// halfDivisor halves a length to find a middle.
	halfDivisor = 2
	// blankLineHeight is the share of a line's pitch a blank line's box
	// covers, as a fraction blankLineNumerator over blankLineDenominator.
	blankLineNumerator   = 2
	blankLineDenominator = 3
)

// IconGlyph stands in the text for an icon or a picture on screen, which has
// no text of its own.
const IconGlyph = "□"

// Word is one word read off the screen, with its bounds in global
// coordinates. Boxes, when it has one per character of Text, are where each
// character was read. Without them each character gets a share of Bounds by
// the cells it takes. Characters are grapheme clusters. Background
// is the color behind it as 0xRRGGBB, or zero when unknown.
type Word struct {
	Text       string
	Bounds     image.Rectangle
	Boxes      []image.Rectangle
	Background uint32

	icon bool
}

// Icon is an icon or a picture at bounds, written into the text as IconGlyph
// so motions reach it as they reach a word. The app has no such text, so it
// is left out of what is copied and of what an edit selects.
func Icon(bounds image.Rectangle) Word {
	return Word{Text: IconGlyph, Bounds: bounds, Boxes: []image.Rectangle{bounds}, icon: true}
}

// Container is a block of the screen's text that belongs together, such as
// a column, a sidebar or a card: where it is on screen, and the bytes of the
// text that are its own.
type Container struct {
	Bounds     image.Rectangle
	Start, End int
}

// Text is the screen as lines of words. It implements vim.Text.
type Text struct {
	s          string
	starts     []int             // byte offset where each line starts
	boxes      []image.Rectangle // per byte, the pixels it was read from
	words      []image.Rectangle // every word's bounds
	containers []Container
	icons      map[int]bool // the bytes that are an icon's glyph
	iconStarts []int        // where each icon's glyph starts, in order
	cell       int          // the width of one character
}

// New lays words out the way the screen shows them, as a buffer vim moves
// over as it would a file of that text. Words that belong together, a column,
// a sidebar or a card, are a container, and each container is laid out as a
// block of its own, in reading order, with a blank line before the next, so
// motions stay inside the block being read. Within a container, a word joins
// the line whose vertical span covers its vertical center, and words on a
// line sort left to right. Each word starts at the column its left edge is
// at, counted in the width of one character from the container's left edge,
// so indentation and wide gaps become spaces and columns line up from line to
// line for j and k. A gap between two lines that leaves room for more becomes
// blank lines, which is where paragraphs end. The padding is not text the app
// has.
func New(words []Word) *Text {
	kept := slices.DeleteFunc(slices.Clone(words), func(word Word) bool {
		return strings.TrimSpace(word.Text) == "" || word.Bounds.Empty()
	})

	cell := cellWidth(kept)
	text := &Text{cell: cell}

	var (
		builder strings.Builder
		last    image.Rectangle
	)

	for index, group := range findContainers(kept, cell) {
		bounds := boundsOf(group)

		if index > 0 {
			text.newline(&builder, image.Rect(last.Max.X, last.Min.Y, last.Max.X+1, last.Max.Y))
			text.starts = append(text.starts, builder.Len())
			text.newline(
				&builder,
				image.Rect(bounds.Min.X, bounds.Min.Y-cell, bounds.Min.X+cell, bounds.Min.Y),
			)
		}

		start := builder.Len()
		last = text.addBlock(&builder, group, cell)
		text.containers = append(
			text.containers,
			Container{Bounds: bounds, Start: start, End: builder.Len()},
		)
	}

	text.s = builder.String()
	if len(text.starts) == 0 {
		text.starts = []int{0}
	}

	return text
}

// Edit returns the text with edits made, sorted and apart, at offsets in t,
// as vim.Host's Apply makes them. An inserted rune takes the box of the
// character it was inserted before, since nothing on screen shows it yet.
// The containers go with it, since their bytes moved, until the screen is
// read again.
func (t *Text) Edit(edits []vim.Edit) *Text {
	out := &Text{words: t.words}

	var builder strings.Builder

	from := 0

	for _, edit := range edits {
		builder.WriteString(t.s[from:edit.Start])
		out.boxes = append(out.boxes, t.boxes[from:edit.Start]...)

		box := t.boxAt(edit.Start)
		for range len(edit.Text) {
			out.boxes = append(out.boxes, box)
		}

		builder.WriteString(edit.Text)

		from = edit.End
	}

	builder.WriteString(t.s[from:])
	out.boxes = append(out.boxes, t.boxes[from:]...)
	out.s = builder.String()

	out.starts = []int{0}
	for offset := range len(out.s) {
		if out.s[offset] == '\n' {
			out.starts = append(out.starts, offset+1)
		}
	}

	return out
}

// AppChars counts the characters of [start, end) the app on screen has, as
// grapheme clusters, since a Shift+Right selects one: every character but the
// spaces before a line's first word, which stand for where the line starts
// on screen rather than for text, and icons.
func (t *Text) AppChars(start, end int) int {
	end = min(end, len(t.s))
	count := 0

	for char := graphemes.FromString(t.s[start:end]); char.Next(); {
		pos := start + char.Start()

		switch {
		case t.icons[pos]:
		case char.Value() != " ":
			count++
		case !t.leading(pos) && (pos == start || t.s[pos-1] != ' '):
			count += t.appSpaces(pos, end)
		}
	}

	return count
}

// AppStart returns pos. When pos is in the padding or the icons before a
// line's first character, it returns that character instead, since the app
// has no text for either.
func (t *Text) AppStart(pos int) int {
	if pos < 0 || pos >= len(t.s) {
		return pos
	}

	for at := t.starts[t.LineOf(pos)]; at < pos; at++ {
		if t.s[at] != ' ' && !t.icons[at] {
			return pos
		}
	}

	for pos < len(t.s) && (t.s[pos] == ' ' || t.icons[pos]) {
		pos++
	}

	return pos
}

// AppText returns the text of [start, end) the app has: every rune but an
// icon's glyph, and the space an icon leaves doubled or at a line's end, so a
// line keeps its spacing without the icon.
func (t *Text) AppText(start, end int) string {
	end = min(end, len(t.s))

	var builder strings.Builder

	for pos := start; pos < end; pos++ {
		if t.s[pos] == ' ' && !t.icons[pos] && !t.leading(pos) {
			builder.WriteString(strings.Repeat(" ", t.appSpaces(pos, end)))

			for pos+1 < end && t.s[pos+1] == ' ' && !t.icons[pos+1] {
				pos++
			}

			continue
		}

		if !t.icons[pos] {
			builder.WriteByte(t.s[pos])

			continue
		}

		if strings.HasPrefix(t.s[pos:], IconGlyph) {
			pos += len(IconGlyph) - 1

			out := builder.String()
			if pos+1 < end && t.s[pos+1] == ' ' &&
				(out == "" || strings.HasSuffix(out, " ") || strings.HasSuffix(out, "\n")) {
				pos++
			}
		}
	}

	// The layout ends no line with a space, so one left there stood before an
	// icon that ended the line.
	lines := strings.Split(builder.String(), "\n")
	for index, line := range lines {
		lines[index] = strings.TrimRight(line, " ")
	}

	return strings.Join(lines, "\n")
}

// IconAfter returns where the count-th icon after pos starts, or before pos
// when back is true, or false when there are not that many that way.
func (t *Text) IconAfter(pos, count int, back bool) (int, bool) {
	found := 0

	for index := range t.iconStarts {
		start := t.iconStarts[index]
		if back {
			start = t.iconStarts[len(t.iconStarts)-1-index]
		}

		if (!back && start > pos) || (back && start < pos) {
			found++

			if found == max(count, 1) {
				return start, true
			}
		}
	}

	return 0, false
}

// CellWidth returns the width of one character on screen.
func (t *Text) CellWidth() int { return max(t.cell, 1) }

// Words returns the bounds of every word, in reading order.
func (t *Text) Words() []image.Rectangle { return t.words }

// Containers returns the containers, in reading order.
func (t *Text) Containers() []Container { return t.containers }

// ContainerAt returns the index of the container whose bytes hold pos, or
// false when none does, as on the blank line between two.
func (t *Text) ContainerAt(pos int) (int, bool) {
	for index, container := range t.containers {
		if pos >= container.Start && pos < max(container.End, container.Start+1) {
			return index, true
		}
	}

	return 0, false
}

// NearestIn returns the byte of container index whose pixels lie closest to
// point.
func (t *Text) NearestIn(index int, point image.Point) int {
	container := t.containers[index]
	best, bestDistance := container.Start, -1

	for offset := container.Start; offset < container.End; offset++ {
		box := t.boxes[offset]
		if t.s[offset] == '\n' {
			continue
		}

		dx := max(box.Min.X-point.X, 0, point.X-box.Max.X)
		dy := max(box.Min.Y-point.Y, 0, point.Y-box.Max.Y)

		distance := dx*dx + dy*dy
		if bestDistance < 0 || distance < bestDistance {
			best, bestDistance = offset, distance
		}
	}

	return best
}

// String returns the whole text.
func (t *Text) String() string { return t.s }

// Len returns the text's length in bytes.
func (t *Text) Len() int { return len(t.s) }

// Slice returns the bytes from start to end.
func (t *Text) Slice(start, end int) string { return t.s[start:end] }

// Lines returns how many lines the text has.
func (t *Text) Lines() int { return len(t.starts) }

// LineStart returns the byte offset where line starts.
func (t *Text) LineStart(line int) int { return t.starts[line] }

// LineOf returns the line that holds pos.
func (t *Text) LineOf(pos int) int {
	return sort.Search(len(t.starts), func(line int) bool { return t.starts[line] > pos }) - 1
}

// Box returns the pixels the byte at pos was read from, or false past the
// end of the text.
func (t *Text) Box(pos int) (image.Rectangle, bool) {
	if pos < 0 || pos >= len(t.boxes) {
		return image.Rectangle{}, false
	}

	return t.boxes[pos], true
}

// Point returns the center of the byte at pos, clamped into the text.
func (t *Text) Point(pos int) (image.Point, bool) {
	if len(t.boxes) == 0 {
		return image.Point{}, false
	}

	box, _ := t.Box(min(max(pos, 0), len(t.boxes)-1))

	return box.Min.Add(box.Size().Div(halfDivisor)), true
}

// Nearest returns the byte whose pixels lie closest to point.
func (t *Text) Nearest(point image.Point) int {
	best, bestDistance := 0, -1

	for offset, box := range t.boxes {
		if t.s[offset] == '\n' {
			continue
		}

		dx := max(box.Min.X-point.X, 0, point.X-box.Max.X)
		dy := max(box.Min.Y-point.Y, 0, point.Y-box.Max.Y)

		distance := dx*dx + dy*dy
		if bestDistance < 0 || distance < bestDistance {
			best, bestDistance = offset, distance
		}
	}

	return best
}

// leading reports whether pos is in the padding a line starts with.
func (t *Text) leading(pos int) bool {
	return strings.TrimLeft(t.s[t.starts[t.LineOf(pos)]:pos], " ") == ""
}

// appSpaces is how many spaces the app has for the run of padding from pos,
// up to end: the run's width on screen in characters, and at least one. The
// layout places words at columns counted from the block's left edge. A
// character width read a little narrow then pads a long line with spaces the
// app does not have, while the run's width on screen stays right.
func (t *Text) appSpaces(pos, end int) int {
	last := pos
	for last+1 < end && t.s[last+1] == ' ' && !t.icons[last+1] {
		last++
	}

	width := t.boxes[last].Max.X - t.boxes[pos].Min.X

	return max(int(math.Round(float64(width)/float64(max(t.cell, 1)))), 1)
}

// addBlock lays out one container's words as lines, and returns the box of
// the last word written.
func (t *Text) addBlock(builder *strings.Builder, words []Word, cell int) image.Rectangle {
	sort.SliceStable(words, func(a, b int) bool { return centerY(words[a]) < centerY(words[b]) })

	lines := groupLines(words)
	pitch := linePitch(lines)
	left := leftEdge(words)

	for index, line := range lines {
		sort.SliceStable(
			line,
			func(a, b int) bool { return line[a].Bounds.Min.X < line[b].Bounds.Min.X },
		)

		if index > 0 {
			t.breakLine(builder, lines[index-1], line, left, cell, pitch)
		}

		t.starts = append(t.starts, builder.Len())
		t.addLine(builder, line, left, cell)
	}

	last := lines[len(lines)-1]

	return last[len(last)-1].Bounds
}

// breakLine ends the line before next, and adds the blank lines the gap
// between their tops leaves room for at pitch.
func (t *Text) breakLine(builder *strings.Builder, prev, next []Word, left, cell, pitch int) {
	last := prev[len(prev)-1].Bounds
	t.newline(builder, image.Rect(last.Max.X, last.Min.Y, last.Max.X+1, last.Max.Y))

	top, prevTop := lineTop(next), lineTop(prev)

	for blank := range blankLines(top-prevTop, pitch) {
		y := prevTop + (blank+1)*pitch

		t.starts = append(t.starts, builder.Len())
		t.newline(
			builder,
			image.Rect(left, y, left+cell, y+pitch*blankLineNumerator/blankLineDenominator),
		)
	}
}

// addLine writes a line's words, each at its column, with the padding before
// it sharing the real gap on screen, so a place on the padding is a place
// between the two words.
func (t *Text) addLine(builder *strings.Builder, line []Word, left, cell int) {
	column, from := 0, left

	for index, word := range line {
		pad := int(math.Round(float64(word.Bounds.Min.X-left)/float64(cell))) - column
		if index > 0 {
			pad = max(pad, 1)
		}

		pad = max(pad, 0)
		gap := max(word.Bounds.Min.X-from, pad)

		for space := range pad {
			start, end := from+gap*space/pad, from+gap*(space+1)/pad

			builder.WriteByte(' ')

			t.boxes = append(
				t.boxes,
				image.Rect(start, word.Bounds.Min.Y, max(end, start+1), word.Bounds.Max.Y),
			)
			column++
		}

		from = word.Bounds.Max.X

		t.addWord(builder, word)
		t.words = append(t.words, word.Bounds)
		column += displaywidth.String(strings.Join(strings.Fields(word.Text), " "))
	}
}

// newline writes a line break with the box it shows at.
func (t *Text) newline(builder *strings.Builder, box image.Rectangle) {
	builder.WriteByte('\n')

	t.boxes = append(t.boxes, box)
}

// addWord writes word, each character with the box it was read from, or
// else its share of the word's width.
func (t *Text) addWord(builder *strings.Builder, word Word) {
	if word.icon {
		if t.icons == nil {
			t.icons = map[int]bool{}
		}

		t.iconStarts = append(t.iconStarts, builder.Len())

		for offset := range len(word.Text) {
			t.icons[builder.Len()+offset] = true
		}
	}

	text := strings.Join(strings.Fields(word.Text), " ")
	chars := Characters(text)

	boxes := word.Boxes
	if len(boxes) != len(chars) || text != word.Text {
		boxes = shares(word.Bounds, chars)
	}

	for index, char := range chars {
		for range len(char) {
			t.boxes = append(t.boxes, boxes[index])
		}

		builder.WriteString(char)
	}
}

// boxAt is the box of the byte at pos, or of the last byte past the end.
func (t *Text) boxAt(pos int) image.Rectangle {
	if len(t.boxes) == 0 {
		return image.Rectangle{}
	}

	return t.boxes[min(max(pos, 0), len(t.boxes)-1)]
}

// groupLines puts words sorted top to bottom into lines.
func groupLines(words []Word) [][]Word {
	var lines [][]Word

	for _, word := range words {
		if count := len(lines); count > 0 && sameLine(lines[count-1], word) {
			lines[count-1] = append(lines[count-1], word)

			continue
		}

		lines = append(lines, []Word{word})
	}

	return lines
}

// leftEdge is the leftmost word's left edge, or zero with no words.
func leftEdge(words []Word) int {
	if len(words) == 0 {
		return 0
	}

	left := words[0].Bounds.Min.X
	for _, word := range words {
		left = min(left, word.Bounds.Min.X)
	}

	return left
}

// cellWidth is the width of one character: the median distance from one
// character's left edge to the next where each was read, or else the median
// share of a word's width each of its characters has. Icons say nothing about
// it.
func cellWidth(words []Word) int {
	var samples []int

	for _, word := range words {
		if word.icon {
			continue
		}

		chars := Characters(word.Text)

		if len(word.Boxes) > 1 && len(word.Boxes) == len(chars) {
			for index := 1; index < len(chars); index++ {
				advance := word.Boxes[index].Min.X - word.Boxes[index-1].Min.X
				if advance > 0 {
					samples = append(samples, max(advance/cellsOf(chars[index-1]), 1))
				}
			}

			continue
		}

		if cells := displaywidth.String(word.Text); cells > 0 {
			samples = append(samples, max(word.Bounds.Dx()/cells, 1))
		}
	}

	if len(samples) == 0 {
		return 1
	}

	slices.Sort(samples)

	return max(samples[len(samples)/halfDivisor], 1)
}

// linePitch is how far apart two lines in a row are: the smallest distance
// from one line's top to the next's that is at least a line's height, since
// a paragraph only ever adds to it, or a line's height when no two lines
// are that far apart.
func linePitch(lines [][]Word) int {
	heights := make([]int, 0, len(lines))

	for _, line := range lines {
		heights = append(heights, lineBottom(line)-lineTop(line))
	}

	if len(heights) == 0 {
		return 1
	}

	slices.Sort(heights)
	height := max(heights[len(heights)/halfDivisor], 1)
	pitch := 0

	for index := 1; index < len(lines); index++ {
		distance := lineTop(lines[index]) - lineTop(lines[index-1])
		if distance >= height && (pitch == 0 || distance < pitch) {
			pitch = distance
		}
	}

	if pitch == 0 {
		return height
	}

	return pitch
}

// blankLines is how many blank lines a gap of dy between two lines' tops
// leaves room for at pitch.
func blankLines(dy, pitch int) int {
	return min(max(int(math.Round(float64(dy)/float64(pitch)))-1, 0), maxBlankLines)
}

func lineTop(line []Word) int {
	top := line[0].Bounds.Min.Y
	for _, word := range line[1:] {
		top = min(top, word.Bounds.Min.Y)
	}

	return top
}

func lineBottom(line []Word) int {
	bottom := line[0].Bounds.Max.Y
	for _, word := range line[1:] {
		bottom = max(bottom, word.Bounds.Max.Y)
	}

	return bottom
}

func centerY(word Word) int { return (word.Bounds.Min.Y + word.Bounds.Max.Y) / halfDivisor }

// sameLine reports whether word's vertical center falls inside the line's
// span.
func sameLine(line []Word, word Word) bool {
	center := centerY(word)

	return center >= lineTop(line) && center < lineBottom(line)
}
