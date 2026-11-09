package screentext

import (
	"image"
	"slices"
	"strconv"
	"strings"
)

const (
	// containerGapColumns is the widest gap, in characters, between two
	// words on one line of the same container. A wider one is a gutter.
	containerGapColumns = 4
	// gutterGapColumns is the narrowest gap, in characters, between a line
	// number and its text, and gutterTextLines how many other lines have to
	// start where that text does.
	gutterGapColumns = 2
	gutterTextLines  = 2
	// lineSpacings is how many of a line's usual gaps between words one gap
	// may span to join a word of another line beside or under it, and
	// spacingWords how many words a line needs for its usual gap to count.
	lineSpacings = 2
	spacingWords = 4
	// containerGapLines is the tallest gap, in the taller word's heights,
	// between two stacked words of the same container. It spans a paragraph
	// break, and a wider one is space between blocks.
	containerGapLines = 2
	// iconGapColumns is the widest gap, in characters, between an icon and
	// the text it joins. An icon joins only text beside it on its row, as a
	// button's icon sits by its label. It has no background to tell it
	// apart, so a looser rule lets the pattern on a chat's wallpaper join
	// every bubble.
	iconGapColumns = 2
	// headingGrowth limits how much taller a line may be than the line a
	// paragraph break above it and still join it, by one part in this many,
	// so 2 allows half again. A taller line is a heading and starts a new
	// block. The margin leaves room for letters that reach above or below
	// the line.
	headingGrowth = 2
	// backgroundTolerance is how far apart two backgrounds may be, in any
	// channel, and still be one.
	backgroundTolerance = 0x18
	// channelMask keeps one 8-bit channel of a 0xRRGGBB color.
	channelMask = 0xFF
)

// findContainers groups words into containers, in reading order. Two words
// belong together when they sit on the same background and are near: on one
// row, or on the next line down, with a gap no wider than a few characters,
// or stacked a paragraph break apart and sharing a character's width side
// to side. An icon joins only the text beside it on its row. A list's
// highlighted item joins the items around it. A line-number gutter is a
// container of its own, so a line's number is not part of its text. A group
// mostly inside another is one container with it, since their text would
// interleave.
func findContainers(words []Word, cell int) [][]Word {
	gutter := lineNumberGutter(words, cell)

	parent := make([]int, len(words))
	for index := range parent {
		parent[index] = index
	}

	root := func(index int) int {
		for parent[index] != index {
			parent[index] = parent[parent[index]]
			index = parent[index]
		}

		return index
	}

	spacing := lineSpacing(words)

	for first := range words {
		for second := first + 1; second < len(words); second++ {
			if gutter[first] == gutter[second] &&
				sameBackground(words[first].Background, words[second].Background) &&
				near(words[first], words[second], cell, spacing) &&
				!wordBetween(words, words[first].Bounds, words[second].Bounds) {
				parent[root(first)] = root(second)
			}
		}
	}

	for _, run := range highlightedItems(words, gutter, cell) {
		parent[root(run[0])] = root(run[1])
		parent[root(run[1])] = root(run[2])
	}

	byRoot := map[int][]Word{}
	gutterRoots := map[int]bool{}

	for index, word := range words {
		byRoot[root(index)] = append(byRoot[root(index)], word)
		gutterRoots[root(index)] = gutter[index]
	}

	// A gutter sits beside the text rather than in it, so it stays apart even
	// where its bounds overlap a container's, as an editor's toolbar above it
	// widens the text's.
	groups := make([][]Word, 0, len(byRoot))

	var gutters [][]Word

	for at, group := range byRoot {
		if gutterRoots[at] {
			gutters = append(gutters, group)
		} else {
			groups = append(groups, group)
		}
	}

	groups = append(mergeNested(groups), gutters...)

	slices.SortFunc(groups, func(a, b []Word) int {
		boundsA, boundsB := boundsOf(a), boundsOf(b)
		if boundsA.Min.Y != boundsB.Min.Y {
			return boundsA.Min.Y - boundsB.Min.Y
		}

		return boundsA.Min.X - boundsB.Min.X
	})

	return groups
}

// lineNumberGutter reports which words are line numbers in a gutter. An editor
// draws them in a column left of its text, a few characters from it. Often
// only the current line's number is dark enough to read. So a number counts
// when it is first on its row and at least gutterGapColumns left of the text
// beside it, and that text starts where at least gutterTextLines other lines
// do. A number right-aligned with one, such as a blank line's, counts too.
func lineNumberGutter(words []Word, cell int) map[int]bool {
	isNumber := func(text string) bool {
		_, err := strconv.Atoi(strings.TrimSpace(text))

		return err == nil && !strings.ContainsAny(text, "+-")
	}

	sameRow := func(a, b image.Rectangle) bool {
		return min(a.Max.Y, b.Max.Y)-max(a.Min.Y, b.Min.Y) > 0
	}

	// leftmost reports a word first on its row, not counting numbers when
	// numbers is set.
	leftmost := func(index int, numbers bool) bool {
		return !slices.ContainsFunc(words, func(other Word) bool {
			return other.Bounds.Max.X <= words[index].Bounds.Min.X &&
				sameRow(other.Bounds, words[index].Bounds) &&
				(!numbers || !isNumber(other.Text))
		})
	}

	// beside is the word after a number on its row.
	beside := func(index int) (Word, bool) {
		var next Word

		found := false

		for _, other := range words {
			if other.Bounds.Min.X > words[index].Bounds.Max.X &&
				sameRow(other.Bounds, words[index].Bounds) &&
				(!found || other.Bounds.Min.X < next.Bounds.Min.X) {
				next, found = other, true
			}
		}

		return next, found
	}

	var numbers []int

	for index, word := range words {
		if isNumber(word.Text) && leftmost(index, false) {
			numbers = append(numbers, index)
		}
	}

	gutter := map[int]bool{}

	for _, index := range numbers {
		next, found := beside(index)
		if !found || next.Bounds.Min.X-words[index].Bounds.Max.X < gutterGapColumns*cell {
			continue
		}

		aligned := 0

		for other, word := range words {
			if other != index && !sameRow(word.Bounds, words[index].Bounds) &&
				leftmost(other, true) &&
				distance(word.Bounds.Min.X, next.Bounds.Min.X)*halfDivisor <= cell {
				aligned++
			}
		}

		if aligned >= gutterTextLines {
			gutter[index] = true
		}
	}

	for _, index := range numbers {
		for marked := range gutter {
			if distance(words[index].Bounds.Max.X, words[marked].Bounds.Max.X)*halfDivisor <= cell {
				gutter[index] = true
			}
		}
	}

	return gutter
}

// distance is how far apart two coordinates are.
func distance(a, b int) int {
	return max(a-b, b-a)
}

// lineSpacing maps each line the reader read to the median gap between its
// words, for lines with at least spacingWords words.
func lineSpacing(words []Word) map[int]int {
	edges := map[int][][2]int{}

	for _, word := range words {
		if word.Line > 0 {
			edges[word.Line] = append(
				edges[word.Line],
				[2]int{word.Bounds.Min.X, word.Bounds.Max.X},
			)
		}
	}

	spacing := map[int]int{}

	for line, spans := range edges {
		if len(spans) < spacingWords {
			continue
		}

		slices.SortFunc(spans, func(a, b [2]int) int { return a[0] - b[0] })

		gaps := make([]int, 0, len(spans)-1)
		for index := 1; index < len(spans); index++ {
			gaps = append(gaps, spans[index][0]-spans[index-1][1])
		}

		slices.Sort(gaps)
		spacing[line] = gaps[len(gaps)/halfDivisor]
	}

	return spacing
}

// near reports whether two words sit close enough to read as one block.
// spacing, when known, narrows how far apart words of two lines may be.
func near(first, second Word, cell int, spacing map[int]int) bool {
	boxFirst, boxSecond := first.Bounds, second.Bounds
	overlapY := min(boxFirst.Max.Y, boxSecond.Max.Y) - max(boxFirst.Min.Y, boxSecond.Min.Y)
	gapX := max(boxFirst.Min.X, boxSecond.Min.X) - min(boxFirst.Max.X, boxSecond.Max.X)
	gapY := -overlapY

	shorter := min(boxFirst.Dy(), boxSecond.Dy())

	switch {
	case first.icon != second.icon:
		return overlapY*halfDivisor >= shorter && gapX <= iconGapColumns*cell
	case overlapY*halfDivisor >= shorter:
		return gapX <= rowGap(first, second, cell, spacing, false)
	case gapY*halfDivisor <= shorter:
		return gapX <= rowGap(first, second, cell, spacing, true)
	default:
		upper, lower := boxFirst, boxSecond
		if lower.Min.Y < upper.Min.Y {
			upper, lower = lower, upper
		}

		// Corners that touch are not overlap. The words must share a
		// character's width, or all of the narrower word.
		overlap := min(cell, upper.Dx(), lower.Dx())

		return gapX <= -overlap && gapY <= containerGapLines*max(upper.Dy(), lower.Dy()) &&
			lower.Dy()*headingGrowth <= upper.Dy()*(headingGrowth+1)
	}
}

// highlightedItems finds a list's highlighted item, a word whose background
// sets it apart from the items right above and below it, which share one.
// Each run is the word above, the item and the word below, all starting in
// the same column. A first or last item has no such run, since a header or
// footer bar on its own background looks the same.
func highlightedItems(words []Word, gutter map[int]bool, cell int) [][3]int {
	var runs [][3]int

	for item := range words {
		var above, below []int

		for other := range words {
			if other == item || gutter[other] != gutter[item] ||
				!leftAligned(words[other].Bounds, words[item].Bounds, cell) ||
				!near(words[other], words[item], cell, nil) {
				continue
			}

			if words[other].Bounds.Max.Y <= words[item].Bounds.Min.Y {
				above = append(above, other)
			} else {
				below = append(below, other)
			}
		}

		for _, up := range above {
			for _, down := range below {
				if sameBackground(words[up].Background, words[down].Background) &&
					!sameBackground(words[up].Background, words[item].Background) {
					runs = append(runs, [3]int{up, item, down})
				}
			}
		}
	}

	return runs
}

// wordBetween reports whether a word lies between two stacked words, in the
// columns both cover. Two lines a line apart join through the one between
// them or not at all, so a wide line cannot reach past it to a heading.
func wordBetween(words []Word, first, second image.Rectangle) bool {
	upper, lower := first, second
	if lower.Min.Y < upper.Min.Y {
		upper, lower = lower, upper
	}

	left, right := max(upper.Min.X, lower.Min.X), min(upper.Max.X, lower.Max.X)
	if upper.Max.Y > lower.Min.Y || left >= right {
		return false
	}

	return slices.ContainsFunc(words, func(word Word) bool {
		box := word.Bounds

		return box.Min.Y >= upper.Max.Y && box.Max.Y <= lower.Min.Y &&
			box.Min.X < right && box.Max.X > left
	})
}

// rowGap is the widest gap two words on one row, or on lines one under the
// other, may have between them and join. Words the reader put on two lines
// join across lineSpacings of the narrower line's spacing at most, so a
// gutter between two columns of text keeps them apart. On one row, both
// lines must have a measured spacing, since a line of one or two words,
// such as a list's number, sits close to the text beside it. For lines one
// under the other, one measured line is enough, so the end of a line cannot
// step down across a gutter to a short line of the next column.
func rowGap(first, second Word, cell int, spacing map[int]int, stacked bool) int {
	limit := containerGapColumns * cell
	if first.Line == second.Line {
		return limit
	}

	spacingFirst, knownFirst := spacing[first.Line]
	spacingSecond, knownSecond := spacing[second.Line]

	switch {
	case knownFirst && knownSecond:
		return min(limit, lineSpacings*min(spacingFirst, spacingSecond))
	case stacked && knownFirst:
		return min(limit, lineSpacings*spacingFirst)
	case stacked && knownSecond:
		return min(limit, lineSpacings*spacingSecond)
	default:
		return limit
	}
}

// leftAligned reports whether two words are stacked and start in the same
// column.
func leftAligned(a, b image.Rectangle, cell int) bool {
	stacked := min(a.Max.Y, b.Max.Y) <= max(a.Min.Y, b.Min.Y)

	return stacked && distance(a.Min.X, b.Min.X)*halfDivisor <= cell
}

// sameBackground reports whether two backgrounds are one. An unknown one,
// zero, matches any.
func sameBackground(first, second uint32) bool {
	if first == 0 || second == 0 {
		return true
	}

	for shift := 0; shift <= 16; shift += 8 {
		channelA, channelB := int(first>>shift&channelMask), int(second>>shift&channelMask)
		if max(channelA-channelB, channelB-channelA) > backgroundTolerance {
			return false
		}
	}

	return true
}

// mergeNested joins groups until none lies mostly inside another.
func mergeNested(groups [][]Word) [][]Word {
	for merged := true; merged; {
		merged = false

		for a := 0; a < len(groups) && !merged; a++ {
			for b := a + 1; b < len(groups); b++ {
				if nested(groups[a], groups[b]) {
					groups[a] = append(groups[a], groups[b]...)
					groups = slices.Delete(groups, b, b+1)
					merged = true

					break
				}
			}
		}
	}

	return groups
}

// nested reports whether the smaller of two groups lies mostly inside the
// larger and the larger's words surround it, as a paragraph does a word of
// inline code. A sidebar inside the bounds of a block beside it has none of
// that block's words on its far side, so it stays apart.
func nested(first, second []Word) bool {
	inner, outer := boundsOf(first), boundsOf(second)
	outerWords := second

	if area(inner) > area(outer) {
		inner, outer, outerWords = outer, inner, first
	}

	return mostlyInside(inner, outer) && surrounded(inner, outerWords)
}

// surrounded reports whether words lie on both sides of bounds, above and
// below within its columns or left and right within its rows.
func surrounded(bounds image.Rectangle, words []Word) bool {
	var above, below, left, right bool

	for _, word := range words {
		box := word.Bounds
		columns := box.Min.X < bounds.Max.X && box.Max.X > bounds.Min.X
		rows := box.Min.Y < bounds.Max.Y && box.Max.Y > bounds.Min.Y

		above = above || columns && box.Max.Y <= bounds.Min.Y
		below = below || columns && box.Min.Y >= bounds.Max.Y
		left = left || rows && box.Max.X <= bounds.Min.X
		right = right || rows && box.Min.X >= bounds.Max.X
	}

	return above && below || left && right
}

// area is how many pixels bounds covers.
func area(bounds image.Rectangle) int { return bounds.Dx() * bounds.Dy() }

// mostlyInside reports whether the smaller of two bounds lies at least half
// inside the larger.
func mostlyInside(first, second image.Rectangle) bool {
	overlap := first.Intersect(second)
	if overlap.Empty() {
		return false
	}

	return area(overlap)*halfDivisor >= min(area(first), area(second))
}

// boundsOf covers every word of a group.
func boundsOf(words []Word) image.Rectangle {
	bounds := words[0].Bounds
	for _, word := range words[1:] {
		bounds = bounds.Union(word.Bounds)
	}

	return bounds
}

// neighbor returns the container key leads to from container from. For h,
// j, k and l it is the nearest container wholly that way past the facing
// edge, preferring one that shares rows (or columns) with from, and for w and
// W the next and the previous in reading order, wrapping around.
func (t *Text) neighbor(from int, key string) (int, bool) {
	count := len(t.containers)

	switch key {
	case "w":
		return (from + 1) % count, true
	case "W":
		return (from + count - 1) % count, true
	case "h", "j", "k", "l":
	default:
		return 0, false
	}

	origin := t.containers[from].Bounds

	// First only the containers that share rows or columns with from, then
	// any that way.
	for _, sharing := range []bool{true, false} {
		best, bestScore := -1, 0

		for index, container := range t.containers {
			gap, side, ahead := facing(key, origin, container.Bounds)
			if index == from || !ahead || (sharing && side > 0) {
				continue
			}

			score := gap + halfDivisor*side
			if best < 0 || score < bestScore {
				best, bestScore = index, score
			}
		}

		if best >= 0 {
			return best, true
		}
	}

	// When bounds overlap, as when a line under one block widens it past
	// the block beside it, no container lies wholly that way. Then the
	// nearest one whose center lies that way is next.
	return t.centerToward(from, key)
}

// centerToward returns the container nearest from whose center lies the way
// key points from from's center.
func (t *Text) centerToward(from int, key string) (int, bool) {
	centerOf := func(bounds image.Rectangle) image.Point {
		return bounds.Min.Add(bounds.Size().Div(halfDivisor))
	}

	origin := centerOf(t.containers[from].Bounds)
	best, bestDistance := -1, 0

	for index, container := range t.containers {
		offset := centerOf(container.Bounds).Sub(origin)

		var ahead int

		switch key {
		case "h":
			ahead = -offset.X
		case "l":
			ahead = offset.X
		case "k":
			ahead = -offset.Y
		default:
			ahead = offset.Y
		}

		if index == from || ahead <= 0 {
			continue
		}

		distance := offset.X*offset.X + offset.Y*offset.Y
		if best < 0 || distance < bestDistance {
			best, bestDistance = index, distance
		}
	}

	return best, best >= 0
}

// facingTolerance is how far a container may reach back over the facing edge
// and still count as lying that way.
const facingTolerance = 4

// facing measures other from origin the way key points: the gap between the
// facing edges, the gap to the side between their spans, zero when they
// overlap, and whether other lies wholly that way.
func facing(key string, origin, other image.Rectangle) (int, int, bool) {
	sideX := max(max(origin.Min.X, other.Min.X)-min(origin.Max.X, other.Max.X), 0)
	sideY := max(max(origin.Min.Y, other.Min.Y)-min(origin.Max.Y, other.Max.Y), 0)

	switch key {
	case "h":
		return origin.Min.X - other.Max.X, sideY, other.Max.X <= origin.Min.X+facingTolerance
	case "l":
		return other.Min.X - origin.Max.X, sideY, other.Min.X >= origin.Max.X-facingTolerance
	case "k":
		return origin.Min.Y - other.Max.Y, sideX, other.Max.Y <= origin.Min.Y+facingTolerance
	default:
		return other.Min.Y - origin.Max.Y, sideX, other.Min.Y >= origin.Max.Y-facingTolerance
	}
}
