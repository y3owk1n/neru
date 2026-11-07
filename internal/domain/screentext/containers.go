package screentext

import (
	"image"
	"slices"
)

const (
	// containerGapColumns is the widest gap, in characters, between two
	// words on one line of the same container. A wider one is a gutter.
	containerGapColumns = 4
	// containerGapLines is the tallest gap, in the taller word's heights,
	// between two stacked words of the same container. It spans a paragraph
	// break, and a wider one is space between blocks.
	containerGapLines = 2
	// backgroundTolerance is how far apart two backgrounds may be, in any
	// channel, and still be one.
	backgroundTolerance = 0x18
	// channelMask keeps one 8-bit channel of a 0xRRGGBB color.
	channelMask = 0xFF
)

// findContainers groups words into containers, in reading order. Two words
// belong together when they sit on the same background and are near: on one
// line with a gap no wider than a few characters, or stacked, no further
// apart side to side than an indent, with a gap no taller than a paragraph
// break. Groups whose
// bounds overlap are one container, since their text would interleave.
func findContainers(words []Word, cell int) [][]Word {
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

	for a := range words {
		for b := a + 1; b < len(words); b++ {
			if sameBackground(words[a].Background, words[b].Background) &&
				near(words[a], words[b], cell) {
				parent[root(a)] = root(b)
			}
		}
	}

	byRoot := map[int][]Word{}
	for index, word := range words {
		byRoot[root(index)] = append(byRoot[root(index)], word)
	}

	groups := make([][]Word, 0, len(byRoot))
	for _, group := range byRoot {
		groups = append(groups, group)
	}

	groups = mergeOverlapping(groups)

	slices.SortFunc(groups, func(a, b []Word) int {
		boundsA, boundsB := boundsOf(a), boundsOf(b)
		if boundsA.Min.Y != boundsB.Min.Y {
			return boundsA.Min.Y - boundsB.Min.Y
		}

		return boundsA.Min.X - boundsB.Min.X
	})

	return groups
}

// near reports whether two words sit close enough to read as one block.
func near(a, b Word, cell int) bool {
	boxA, boxB := a.Bounds, b.Bounds
	overlapY := min(boxA.Max.Y, boxB.Max.Y) - max(boxA.Min.Y, boxB.Min.Y)
	gapX := max(boxA.Min.X, boxB.Min.X) - min(boxA.Max.X, boxB.Max.X)
	gapY := -overlapY

	if overlapY*halfDivisor >= min(boxA.Dy(), boxB.Dy()) {
		return gapX <= containerGapColumns*cell
	}

	return gapX <= containerGapColumns*cell && gapY <= containerGapLines*max(boxA.Dy(), boxB.Dy())
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

// mergeOverlapping joins groups whose bounds overlap until none do.
func mergeOverlapping(groups [][]Word) [][]Word {
	for merged := true; merged; {
		merged = false

		for a := 0; a < len(groups) && !merged; a++ {
			for b := a + 1; b < len(groups); b++ {
				if boundsOf(groups[a]).Overlaps(boundsOf(groups[b])) {
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

	return 0, false
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
