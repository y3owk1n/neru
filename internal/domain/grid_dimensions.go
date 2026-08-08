package domain

// Grid subgrid dimensions.
const (
	SubgridRows = 3
	SubgridCols = 3
)

// GridDimensions is the shape of a rectangular grid: how many rows of cells it
// has and how many columns. It is the one thing every division of a rectangle
// into cells needs beyond the rectangle itself.
//
// It is a type rather than two int parameters because the domain divides
// rectangles into grids twice — grid.SubgridCells rounds each break, and
// recursivegrid.ComputeGridCells hands the spare pixels to the first cells —
// and the two used to spell the pair in opposite orders. Every caller passes
// equal counts today (the subgrid is a fixed 3x3, the recursive grid a
// configured square), so a transposed call compiled, drew the right cells and
// clicked the right point; the first non-square grid would have turned it into
// what looked like a rendering bug. Carrying the two counts together means
// there is no longer a pair to put in the wrong order, and each count is named
// at the point it is written down.
//
// It lives here rather than in either grid package because both of them divide
// rectangles and neither can import the other. This is the lowest layer with
// more than one caller — the rule ADR 0007 names.
type GridDimensions struct {
	// Rows is how many rows of cells the grid has.
	Rows int
	// Cols is how many columns of cells the grid has.
	Cols int
}

// SubgridDimensions is the shape of the subgrid a grid cell opens into.
//
// It is a function rather than a variable so that no caller can reshape the
// shipped subgrid for everyone else, and it exists so that the overlay backends
// ask for that shape by name instead of each pairing up the two constants
// themselves.
func SubgridDimensions() GridDimensions {
	return GridDimensions{Rows: SubgridRows, Cols: SubgridCols}
}

// CellCount is how many cells a grid of these dimensions has, which is also how
// many keys it needs.
func (d GridDimensions) CellCount() int {
	return d.Rows * d.Cols
}
