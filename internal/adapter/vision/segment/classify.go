package segment

import (
	"image"
	"math"
)

// The size rules, in logical pixels. They describe desktop UI, not a tuning
// surface: each was checked against rendered light and dark scenes with known
// clickable boxes.
const (
	// maxControlHeight is the tallest frame that is one control; a taller one
	// is a card or panel whose children are the targets.
	maxControlHeight = 64
	// maxTargetWidth is the widest target; anything wider is layout.
	maxTargetWidth = 650
	// maxPictureSide bounds a picture that is one target (an avatar, a
	// thumbnail). Bigger pictures are content, and only suppress what is on them.
	maxPictureSide = 5 * maxControlHeight
	// minPictureSide and pictureFill describe a picture: dense ink, unlike text,
	// which leaves a third of its box empty or more.
	minPictureSide = 16
	pictureFill    = 0.85
	// minFrameSide is the smallest box that can be a frame rather than a glyph.
	minFrameSide = 10
	// frameCover is the share of each side a frame's ink covers.
	frameCover = 0.7
	// lineArtFill is the ink density under which a big blob is crossing
	// rules (a table grid, a divider meeting a border) rather than a glyph.
	lineArtFill = 0.1
	// lineLength and lineThickness describe a separator.
	lineLength    = 12
	lineThickness = 2
	// minTargetArea drops specks.
	minTargetArea = 12
	// markGap is how far a dot, accent or underline may sit from its glyph.
	markGap = 2
	// wordGap is the widest gap between glyphs of one label, in glyph heights.
	wordGap = 0.4
	// inkCos is the least cosine between two glyphs' ink directions that still
	// reads as one color.
	inkCos = 0.97
	// inkRatio is the least ratio of two glyphs' ink lengths that still reads
	// as one color, which tells white text from gray.
	inkRatio = 0.5
	// bgMatch is the largest background difference between glyphs of one label.
	bgMatch = 40
	// smallGlyph is the height under which a glyph's ink is too antialiased
	// to compare, and smallCount the ink count under which a glyph is a mark.
	smallGlyph = 6
	smallCount = 12
	// shellChild: a child at least 1/shellChild of its holder's height makes
	// the holder a shell even when it is not a frame.
	shellChild = 4
	// coincidentSlack and coincidentShare bound how far apart two nested
	// targets' centers may be and still be one thing: a few pixels, or a
	// fifth of the outer target's shorter side.
	coincidentSlack = 3
	coincidentShare = 5
	// gridCell is the bucket size of the spatial index, in frame pixels.
	gridCell = 32
)

// limits are the size rules in frame pixels.
type limits struct {
	maxControlHeight, maxTargetWidth, maxPictureSide, minPictureSide int
	minFrameSide, lineLength, lineThickness, minTargetArea, markGap  int
	smallGlyph                                                       int
}

//nolint:varnamelen // blob indices and box sides read as the geometry writes them
func newLimits(scale float64) limits {
	px := func(v float64) int { return max(int(math.Round(v*scale)), 1) }

	return limits{
		maxControlHeight: px(maxControlHeight),
		maxTargetWidth:   px(maxTargetWidth),
		maxPictureSide:   px(maxPictureSide),
		minPictureSide:   px(minPictureSide),
		minFrameSide:     px(minFrameSide),
		lineLength:       px(lineLength),
		lineThickness:    px(lineThickness),
		minTargetArea:    max(int(math.Round(minTargetArea*scale*scale)), 1),
		markGap:          px(markGap),
		smallGlyph:       px(smallGlyph),
	}
}

type kind uint8

const (
	glyph kind = iota
	line
	picture
	control
	container
)

// classify sorts blobs into lines, pictures, controls, containers and glyphs,
// groups glyphs into labels, and returns the targets in frame pixels:
// controls, small pictures and labels. Anything inside a control or a picture
// is part of it and gets no target of its own.
//
//nolint:varnamelen // blob indices and box sides read as the geometry writes them
func classify(blobs []blob, lim limits) []image.Rectangle {
	kinds := make([]kind, len(blobs))

	for i := range blobs {
		b := &blobs[i]
		w, h := b.width(), b.height()

		switch {
		case (h <= lim.lineThickness && w >= lim.lineLength) ||
			(w <= lim.lineThickness && h >= lim.lineLength):
			kinds[i] = line
		case max(w, h) >= 3*lim.lineLength && !isFrame(b, lim) &&
			float64(b.count) < lineArtFill*float64(w*h):
			kinds[i] = line
		case w >= lim.minPictureSide && h >= lim.minPictureSide &&
			float64(b.count) >= pictureFill*float64(w*h):
			kinds[i] = picture
		}
	}

	holder := holders(blobs, kinds)

	// A shell holding anything is a control when it is control-sized and a
	// container when it is not. A shell is a frame, or any outline holding a
	// child of real size, which is what a round key or an icon tile around its
	// glyph is. A blob holding only a mark is a glyph cluster enclosing a stray
	// dot, and stays a glyph.
	for i, j := range holder {
		if j < 0 || kinds[j] != glyph {
			continue
		}

		child, shell := &blobs[i], &blobs[j]

		substantial := child.count > smallCount && child.height()*shellChild >= shell.height()
		if !substantial && !isFrame(shell, lim) {
			continue
		}

		kinds[j] = container
		if fits(shell, lim) {
			kinds[j] = control
		}
	}

	// scope is the nearest enclosing frame or picture, the surface a blob sits
	// on. Glyphs only group with glyphs on the same surface.
	scope := make([]int, len(blobs))
	for i := range blobs {
		j := holder[i]
		for j >= 0 && kinds[j] == glyph && !isFrame(&blobs[j], lim) {
			j = holder[j]
		}

		scope[i] = j
	}

	inPicture := func(i int) bool {
		for j := scope[i]; j >= 0; j = scope[j] {
			if kinds[j] == picture {
				return true
			}
		}

		return false
	}

	glyphs := make([]bool, len(blobs))
	for i := range blobs {
		glyphs[i] = kinds[i] == glyph && !isFrame(&blobs[i], lim) && fits(&blobs[i], lim) &&
			!inPicture(i)
	}

	dropSeparators(blobs, glyphs, lim)

	label := groupLabels(blobs, glyphs, scope, lim)

	// A control holding two or more separate things is a group of them (a
	// segmented control, a tab strip) rather than one target. Its words are
	// one thing when they read as one caption ("Save changes"); frames of its
	// own are each one thing.
	wordBoxes := map[int]map[int]image.Rectangle{}
	frameParts := map[int]int{}

	for i := range blobs {
		j := scope[i]
		if j < 0 || kinds[j] != control || kinds[i] == line {
			continue
		}

		if !glyphs[i] {
			frameParts[j]++

			continue
		}

		if wordBoxes[j] == nil {
			wordBoxes[j] = map[int]image.Rectangle{}
		}

		if box, ok := wordBoxes[j][label[i]]; ok {
			wordBoxes[j][label[i]] = box.Union(blobs[i].rect())
		} else {
			wordBoxes[j][label[i]] = blobs[i].rect()
		}
	}

	for j := range blobs {
		if kinds[j] == control &&
			frameParts[j]+captions(wordBoxes[j]) >= 2 {
			kinds[j] = container
		}
	}

	absorbed := func(i int) bool {
		for j := scope[i]; j >= 0; j = scope[j] {
			if kinds[j] == control || kinds[j] == picture {
				return true
			}
		}

		return false
	}

	targets := make([]image.Rectangle, 0, len(blobs)/4) //nolint:mnd // a first guess
	boxes := map[int]image.Rectangle{}

	for i := range blobs {
		b := &blobs[i]
		if absorbed(i) {
			continue
		}

		switch {
		case glyphs[i]:
			if box, ok := boxes[label[i]]; ok {
				boxes[label[i]] = box.Union(b.rect())
			} else {
				boxes[label[i]] = b.rect()
			}
		case kinds[i] == control,
			kinds[i] == glyph && isFrame(b, lim) && fits(b, lim),
			kinds[i] == picture && b.height() <= lim.maxPictureSide && b.width() <= lim.maxPictureSide:
			targets = append(targets, b.rect())
		}
	}

	for _, box := range boxes {
		if box.Dx()*box.Dy() < lim.minTargetArea || box.Dx() > lim.maxTargetWidth {
			continue
		}

		targets = append(targets, box)
	}

	return dropCoincident(targets)
}

// dropCoincident removes a target lying inside another with nearly the same
// center: a ring and the glyph in it, found twice. Both would click the same
// spot, so only the outer one keeps a hint.
//
//nolint:varnamelen // target indices read as the geometry writes them
func dropCoincident(targets []image.Rectangle) []image.Rectangle {
	center := func(r image.Rectangle) image.Point {
		return image.Pt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2) //nolint:mnd // the midpoint
	}

	cells := map[image.Point][]int{}
	for i, t := range targets {
		c := center(t)
		key := image.Pt(c.X/gridCell, c.Y/gridCell)
		cells[key] = append(cells[key], i)
	}

	dropped := make([]bool, len(targets))

	for i, outer := range targets {
		oc := center(outer)
		slack := max(coincidentSlack, min(outer.Dx(), outer.Dy())/coincidentShare)

		for cy := (oc.Y - slack) / gridCell; cy <= (oc.Y+slack)/gridCell; cy++ {
			for cx := (oc.X - slack) / gridCell; cx <= (oc.X+slack)/gridCell; cx++ {
				for _, j := range cells[image.Pt(cx, cy)] {
					inner := targets[j]
					if j == i || dropped[j] || inner == outer || !inner.In(outer) {
						continue
					}

					ic := center(inner)
					if abs(ic.X-oc.X) <= slack && abs(ic.Y-oc.Y) <= slack {
						dropped[j] = true
					}
				}
			}
		}
	}

	kept := targets[:0]
	for i, t := range targets {
		if !dropped[i] {
			kept = append(kept, t)
		}
	}

	return kept
}

// captions counts the captions a control's words make. Words on one line no
// further apart than half their height, a space, are one caption, the way a
// button reads "Save changes", while tabs in a strip and links in a bar stand
// further apart than that.
//
//nolint:varnamelen // word indices read as the geometry writes them
func captions(words map[int]image.Rectangle) int {
	boxes := make([]image.Rectangle, 0, len(words))
	for _, box := range words {
		boxes = append(boxes, box)
	}

	parent := make([]int, len(boxes))
	for i := range parent {
		parent[i] = i
	}

	find := func(a int) int {
		for parent[a] != a {
			a = parent[a]
		}

		return a
	}

	for i, a := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			b := boxes[j]
			overlap := min(a.Max.Y, b.Max.Y) - max(a.Min.Y, b.Min.Y)
			gap := max(a.Min.X, b.Min.X) - min(a.Max.X, b.Max.X)

			if overlap*2 >= min(a.Dy(), b.Dy()) && gap*2 <= max(a.Dy(), b.Dy()) {
				parent[find(j)] = find(i)
			}
		}
	}

	count := 0

	for i := range boxes {
		if find(i) == i {
			count++
		}
	}

	return count
}

// fits reports whether a blob is small enough to be one target.
func fits(b *blob, lim limits) bool {
	return b.height() <= lim.maxControlHeight && b.width() <= lim.maxTargetWidth
}

// isFrame reports a closed outline: a border or a fill's edge covering at
// least frameCover of every side of its box. Words cover their baseline, and
// only their ascenders reach the top.
//
//nolint:varnamelen // blob indices and box sides read as the geometry writes them
func isFrame(b *blob, lim limits) bool {
	w, h := b.width(), b.height()
	if w < lim.minFrameSide || h < lim.minFrameSide {
		return false
	}

	covered := func(ink, length int) bool { return float64(ink) >= frameCover*float64(length) }

	return covered(max(b.top[0], b.top[1]), w) && covered(max(b.bottom[0], b.bottom[1]), w) &&
		covered(b.leftRows, h) && covered(b.rightRows, h)
}

// holders returns, for each blob, the smallest other blob whose box strictly
// encloses it, or -1. Lines hold nothing.
//
//nolint:varnamelen // blob indices and box sides read as the geometry writes them
func holders(blobs []blob, kinds []kind) []int {
	index := newGrid(blobs, func(*blob) int { return 0 })
	holder := make([]int, len(blobs))

	for i := range blobs {
		b := &blobs[i]
		holder[i] = -1
		best := math.MaxInt
		ownArea := b.width() * b.height()

		for _, j := range index.at(b.centerX(), b.centerY()) {
			o := &blobs[j]
			if int(j) == i || kinds[j] == line || !o.encloses(b) {
				continue
			}

			if area := o.width() * o.height(); area > ownArea && area < best {
				best, holder[i] = area, int(j)
			}
		}
	}

	return holder
}

// dropSeparators takes thin vertical strokes standing apart between two words
// on their line, the "|" between links in "hide | 93 comments", out of the
// glyphs, so they never join two labels into one. A stroke inside a word ("l",
// "I", "1") sits a pixel from its neighbors, and one with no word on either
// side is a label of its own, so both stay glyphs.
//
//nolint:varnamelen // blob indices and box sides read as the geometry writes them
func dropSeparators(blobs []blob, glyphs []bool, lim limits) {
	index := newGrid(blobs, func(*blob) int { return lim.markGap + 1 })

	for i := range blobs {
		b := &blobs[i]
		if !glyphs[i] || b.width() > lim.lineThickness+1 ||
			b.height() < lim.smallGlyph+lim.markGap {
			continue
		}

		// Words on both sides within the stroke's own height, none touching.
		reach := b.height()
		touching, left, right := false, false, false

		index.each(b.minX-reach, b.minY, b.maxX+reach, b.maxY, func(j int32) {
			o := &blobs[j]
			if int(j) == i || !glyphs[j] || o.maxY < b.minY || o.minY > b.maxY {
				return
			}

			gap := max(o.minX, b.minX) - min(o.maxX, b.maxX) - 1

			switch {
			case gap <= lim.markGap:
				touching = true
			case gap <= reach && o.maxX < b.minX:
				left = true
			case gap <= reach && o.minX > b.maxX:
				right = true
			}
		})

		if !touching && left && right {
			glyphs[i] = false
		}
	}
}

// groupLabels unions glyphs on the same surface that read as one label, and
// returns each blob's label: the index of one member, shared by the group.
//
//nolint:varnamelen // blob indices and box sides read as the geometry writes them
func groupLabels(blobs []blob, glyphs []bool, scope []int, lim limits) []int {
	parent := make([]int32, len(blobs))
	for i := range parent {
		parent[i] = int32(i)
	}

	find := func(a int32) int32 {
		for parent[a] != a {
			parent[a] = parent[parent[a]]
			a = parent[a]
		}

		return a
	}

	reach := func(b *blob) int { return int(wordGap*float64(b.height())) + lim.markGap }
	index := newGrid(blobs, reach)

	for i := range blobs {
		if !glyphs[i] {
			continue
		}

		a := &blobs[i]
		pad := reach(a)

		index.each(a.minX-pad, a.minY-pad, a.maxX+pad, a.maxY+pad, func(j int32) {
			if int(j) <= i || !glyphs[j] || scope[j] != scope[i] || find(int32(i)) == find(j) {
				return
			}

			if sameLabel(a, &blobs[j], lim) {
				parent[find(j)] = find(int32(i))
			}
		})
	}

	label := make([]int, len(blobs))
	for i := range label {
		label[i] = int(find(int32(i)))
	}

	return label
}

// sameLabel reports whether two sibling glyphs read as one label: a mark
// (dot, accent, underline) stacked on a glyph, or neighbors on one line closer
// than wordGap heights in the same ink on the same background. A blob never
// groups with one that encloses it.
//
//nolint:varnamelen // blob indices and box sides read as the geometry writes them
func sameLabel(a, b *blob, lim limits) bool {
	if a.encloses(b) || b.encloses(a) {
		return false
	}

	gapX := max(a.minX, b.minX) - min(a.maxX, b.maxX) - 1
	gapY := max(a.minY, b.minY) - min(a.maxY, b.maxY) - 1
	lo, hi := min(a.height(), b.height()), max(a.height(), b.height())

	mark := min(a.count, b.count) <= smallCount || lo <= lim.lineThickness+1 || lo*3 <= hi
	if gapX < 0 && gapY <= lim.markGap && mark {
		return true
	}

	// Same line: overlapping by at least half the shorter height.
	if gapY >= 0 || -gapY*2 < lo {
		return false
	}

	// The mask marks the pixel before each step, so a blob's box starts one
	// pixel early: the ink gap is one wider, and the ink one shorter, than the
	// boxes say. Words are told apart on the ink.
	inkGap, inkHeight := gapX+1, hi-1
	if float64(inkGap) > max(wordGap*float64(inkHeight), float64(lim.markGap)) ||
		distance(a.bg, b.bg) > bgMatch {
		return false
	}

	return lo < lim.smallGlyph || sameInk(a, b)
}

// sameInk compares the direction of each glyph's ink away from its background.
// Antialiasing mixes ink with background linearly, which shortens that vector
// without turning it, so a thin stroke and a thick one in one color agree.
// Neutral inks share a direction, so for glyphs big enough to hold a fully
// covered pixel the lengths must agree too, which tells white from gray.
//
//nolint:varnamelen // blob indices and box sides read as the geometry writes them
func sameInk(a, b *blob) bool {
	var dot, na, nb float64

	for k := range a.ink {
		va, vb := float64(a.ink[k]-a.bg[k]), float64(b.ink[k]-b.bg[k])
		dot += va * vb
		na += va * va
		nb += vb * vb
	}

	if dot <= 0 || dot*dot < inkCos*inkCos*na*nb {
		return false
	}

	if a.count > smallCount && b.count > smallCount {
		return min(na, nb) >= inkRatio*inkRatio*max(na, nb)
	}

	return true
}

// grid buckets blob indices by the gridCell squares their padded boxes cover.
type grid struct {
	cols, rows int
	cells      [][]int32
}

//nolint:varnamelen // blob indices and box sides read as the geometry writes them
func newGrid(blobs []blob, pad func(*blob) int) *grid {
	maxX, maxY := 0, 0
	for i := range blobs {
		maxX, maxY = max(maxX, blobs[i].maxX), max(maxY, blobs[i].maxY)
	}

	g := &grid{cols: maxX/gridCell + 1, rows: maxY/gridCell + 1}
	g.cells = make([][]int32, g.cols*g.rows)

	for i := range blobs {
		b := &blobs[i]
		p := pad(b)

		x0, y0, x1, y1 := g.span(b.minX-p, b.minY-p, b.maxX+p, b.maxY+p)
		for cy := y0; cy <= y1; cy++ {
			for cx := x0; cx <= x1; cx++ {
				g.cells[cy*g.cols+cx] = append(g.cells[cy*g.cols+cx], int32(i))
			}
		}
	}

	return g
}

// span clamps a pixel box to the cells it covers, as the first and last
// column and row.
func (g *grid) span(minX, minY, maxX, maxY int) (int, int, int, int) {
	clamp := func(v, n int) int { return min(max(v/gridCell, 0), n-1) }

	return clamp(max(minX, 0), g.cols), clamp(max(minY, 0), g.rows),
		clamp(max(maxX, 0), g.cols), clamp(max(maxY, 0), g.rows)
}

// at returns the blobs bucketed in the cell holding a pixel.
func (g *grid) at(x, y int) []int32 {
	x0, y0, _, _ := g.span(x, y, x, y)

	return g.cells[y0*g.cols+x0]
}

// each calls fn for every blob bucketed in the cells a pixel box covers. A blob
// in several of them is visited once per cell, so fn must tolerate repeats.
func (g *grid) each(minX, minY, maxX, maxY int, fn func(int32)) {
	x0, y0, x1, y1 := g.span(minX, minY, maxX, maxY)
	for cy := y0; cy <= y1; cy++ {
		for cx := x0; cx <= x1; cx++ {
			for _, j := range g.cells[cy*g.cols+cx] {
				fn(j)
			}
		}
	}
}
