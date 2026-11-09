package screentext

import (
	"image"
	"slices"
)

const (
	// sameLevelGapPercent is how wide a gap has to be, as a share of the
	// widest gap across a block, to split it at the same level. Narrower
	// gaps split the parts in turn, so a footer far below a column is a
	// branch of its own, and the column's sections are branches of the
	// column.
	sameLevelGapPercent = 60
	// percent is the scale sameLevelGapPercent is on.
	percent = 100
)

// treeNode is a container in the tree: a group of words, or a block of the
// containers inside it.
type treeNode struct {
	parent   int
	group    int // the group a leaf holds, or -1 for a block
	children []int
	bounds   image.Rectangle
}

// buildTree arranges groups into a tree, root first. It cuts the screen
// apart at the widest gaps that run all the way across it, side to side or
// top to bottom, and cuts each part the same way, until a part is one group
// or no gap runs across it. A part no gap crosses, such as one whose groups'
// bounds overlap, keeps its groups as children in reading order.
func buildTree(groups [][]Word) []treeNode {
	bounds := make([]image.Rectangle, len(groups))
	all := make([]int, len(groups))

	for index, group := range groups {
		bounds[index] = boundsOf(group)
		all[index] = index
	}

	var nodes []treeNode

	var build func(members []int, parent int) int

	build = func(members []int, parent int) int {
		index := len(nodes)

		if len(members) == 1 {
			nodes = append(
				nodes,
				treeNode{parent: parent, group: members[0], bounds: bounds[members[0]]},
			)

			return index
		}

		span := bounds[members[0]]
		for _, member := range members[1:] {
			span = span.Union(bounds[member])
		}

		nodes = append(nodes, treeNode{parent: parent, group: -1, bounds: span})

		parts := cutAcross(members, bounds)
		if parts == nil {
			parts = make([][]int, 0, len(members))
			for _, member := range readingOrder(members, bounds) {
				parts = append(parts, []int{member})
			}
		}

		for _, part := range parts {
			child := build(part, index)
			nodes[index].children = append(nodes[index].children, child)
		}

		return index
	}

	if len(groups) > 0 {
		build(all, -1)
	}

	return nodes
}

// cutAcross splits members at the gaps that run all the way across them, on
// whichever axis has the widest one, keeping the parts in reading order. It
// returns nil when no gap runs across.
func cutAcross(members []int, bounds []image.Rectangle) [][]int {
	rows, rowGap := cutAlong(members, bounds, func(r image.Rectangle) (int, int) {
		return r.Min.Y, r.Max.Y
	})
	columns, columnGap := cutAlong(members, bounds, func(r image.Rectangle) (int, int) {
		return r.Min.X, r.Max.X
	})

	switch {
	case rowGap == 0 && columnGap == 0:
		return nil
	case columnGap > rowGap:
		return columns
	default:
		return rows
	}
}

// cutAlong splits members at the gaps between their spans along one axis
// that are within sameLevelGapPercent of the widest, and returns the parts in
// order and the widest gap, zero when none.
func cutAlong(
	members []int,
	bounds []image.Rectangle,
	span func(image.Rectangle) (int, int),
) ([][]int, int) {
	sorted := slices.Clone(members)
	slices.SortStableFunc(sorted, func(a, b int) int {
		startA, _ := span(bounds[a])
		startB, _ := span(bounds[b])

		return startA - startB
	})

	gaps := make([]int, len(sorted))
	widest := 0

	_, reach := span(bounds[sorted[0]])

	for index := 1; index < len(sorted); index++ {
		start, end := span(bounds[sorted[index]])
		if start > reach {
			gaps[index] = start - reach
			widest = max(widest, gaps[index])
		}

		reach = max(reach, end)
	}

	if widest == 0 {
		return nil, 0
	}

	parts := [][]int{{sorted[0]}}

	for index := 1; index < len(sorted); index++ {
		if gaps[index]*percent >= widest*sameLevelGapPercent {
			parts = append(parts, nil)
		}

		parts[len(parts)-1] = append(parts[len(parts)-1], sorted[index])
	}

	return parts, widest
}

// readingOrder sorts members top to bottom, then left to right.
func readingOrder(members []int, bounds []image.Rectangle) []int {
	sorted := slices.Clone(members)
	slices.SortStableFunc(sorted, func(a, b int) int {
		if bounds[a].Min.Y != bounds[b].Min.Y {
			return bounds[a].Min.Y - bounds[b].Min.Y
		}

		return bounds[a].Min.X - bounds[b].Min.X
	})

	return sorted
}

// leavesInOrder lists the tree's leaves, by index into nodes, depth first,
// which is the order they are laid out in.
func leavesInOrder(nodes []treeNode) []int {
	var order []int

	var walk func(index int)

	walk = func(index int) {
		if nodes[index].group >= 0 {
			order = append(order, index)

			return
		}

		for _, child := range nodes[index].children {
			walk(child)
		}
	}

	if len(nodes) > 0 {
		walk(0)
	}

	return order
}
