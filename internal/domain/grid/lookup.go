package grid

import (
	"image"
	"strings"
)

// AllCells returns all grid cells.
func (g *Grid) AllCells() []*Cell {
	return g.cells
}

// CellByCoordinate returns the cell for a given coordinate. (2, 3, or 4 characters).
func (g *Grid) CellByCoordinate(coordinate string) *Cell {
	coordinate = strings.ToUpper(coordinate)

	if g.index != nil {
		if cell, ok := g.index[coordinate]; ok {
			return cell
		}
	}

	for _, cell := range g.cells {
		if cell.Coordinate() == coordinate {
			return cell
		}
	}

	return nil
}

// CellForPoint returns the cell containing point, or nil when no cell does.
//
// Cells are emitted region by region and a region running off the right or
// bottom edge is clipped, so the cells do not always tile the whole bounds —
// a point inside Bounds can still land on no cell.
func (g *Grid) CellForPoint(point image.Point) *Cell {
	for _, cell := range g.cells {
		if point.In(cell.bounds) {
			return cell
		}
	}

	return nil
}

// HasCoordinatePrefix returns true if any coordinate starts with the given prefix.
func (g *Grid) HasCoordinatePrefix(prefix string) bool {
	prefix = strings.ToUpper(prefix)

	return g.prefixes[prefix]
}
