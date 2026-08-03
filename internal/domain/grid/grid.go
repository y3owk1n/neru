package grid

import (
	"image"
	"math"
	"slices"
	"strings"

	"go.uber.org/zap"
)

const (
	// SmallScreenArea is the threshold for small screen area.
	SmallScreenArea = 1500000
	// MediumScreenArea is the threshold for medium screen area.
	MediumScreenArea = 2500000
	// LargeScreenArea is the threshold for large screen area.
	LargeScreenArea = 4000000

	// ExtremeAspectRatioHigh is the high threshold for extreme aspect ratios.
	ExtremeAspectRatioHigh = 2.5
	// ExtremeAspectRatioLow is the low threshold for extreme aspect ratios.
	ExtremeAspectRatioLow = 0.4
	// AspectRatioAdjustment is the adjustment factor for extreme aspect ratios.
	AspectRatioAdjustment = 1.2

	// MinCharactersLength is the minimum length for characters.
	MinCharactersLength = 2

	// MinGridCols is the minimum number of grid columns.
	MinGridCols = 2

	// MinGridRows is the minimum number of grid rows.
	MinGridRows = 2

	// MaxGridCols is the maximum number of grid columns.
	// This prevents allocation-size panics from oversized dimensions.
	MaxGridCols = 10000

	// MaxGridRows is the maximum number of grid rows.
	// This prevents allocation-size panics from oversized dimensions.
	MaxGridRows = 10000

	// MaxDisplayDimension caps the width and height in pixels to prevent
	// excessive work in candidate generation and overflow in area computation.
	// Real display bounds are well under this value (typical max is ~30000).
	MaxDisplayDimension = 50000

	// MaxKeyIndex is the maximum key index.
	MaxKeyIndex = 9

	// RoundingFactor is the factor for rounding.
	RoundingFactor = 0.5

	// CenterDivisor is the divisor for center calculation.
	CenterDivisor = 2

	// ScoreWeight is the weight for scoring.
	ScoreWeight = 0.1

	// StringBuilderGrow2 is the growth for string builder.
	StringBuilderGrow2 = 2

	// StringBuilderGrow3 is the growth for string builder.
	StringBuilderGrow3 = 3

	// StringBuilderGrow4 is the growth for string builder.
	StringBuilderGrow4 = 4

	// LabelLength2 is the label length 2.
	LabelLength2 = 2

	// LabelLength3 is the label length 3.
	LabelLength3 = 3

	// LabelLength4 is the label length 4.
	LabelLength4 = 4

	// CountsCapacity is the capacity for counts.
	CountsCapacity = 5

	// PrefixLengthCheck is the check for prefix length.
	PrefixLengthCheck = 2
)

// Grid is a coordinate grid system for spatial navigation with optimized cell sizing.
type Grid struct {
	characters string          // Characters used for coordinates (e.g., "asdfghjkl")
	rowChars   []rune          // Characters used for row labels
	colChars   []rune          // Characters used for column labels
	bounds     image.Rectangle // Screen bounds
	cells      []*Cell         // All cells with 3-char coordinates
	index      map[string]*Cell
	prefixes   map[string]bool // Set of all coordinate prefixes for fast lookup
}

// Cell is a grid cell containing coordinate, bounds, and center point information.
type Cell struct {
	coordinate string          // 3-character coordinate (e.g., "AAA", "ABC")
	bounds     image.Rectangle // Cell bounds
	center     image.Point     // Center point
}

// Coordinate returns the 3-character coordinate.
func (c *Cell) Coordinate() string {
	return c.coordinate
}

// Bounds returns the cell bounds.
func (c *Cell) Bounds() image.Rectangle {
	return c.bounds
}

// Center returns the center point.
func (c *Cell) Center() image.Point {
	return c.center
}

// NewGrid creates a grid whose cell size is chosen from the screen area, so a
// small display gets finer cells than a large one. See the ScreenArea constants
// for the thresholds.
//
// Labels are grouped into regions named by their first character: within region
// A the cells run AAA, ABA, ACA left to right, then AAB, ABB, ACB on the next
// row, and regions themselves fill left to right and then wrap. That is what
// lets a user read "C**" as "somewhere in region C".
//
// Empty rowLabels or colLabels are inferred from characters.
func NewGrid(characters string, bounds image.Rectangle, logger *zap.Logger) *Grid {
	return NewGridWithLabels(characters, "", "", bounds, logger)
}

// NewGridWithLabels creates a grid with custom row and column labels.
// If rowLabels or colLabels are empty, they will be inferred from characters.
func NewGridWithLabels(
	characters, rowLabels, colLabels string,
	bounds image.Rectangle,
	logger *zap.Logger,
) *Grid {
	// Constructors in this tree accept a nil logger and fall back to a no-op
	// rather than panicking on first use. Without this the daemon dies with a
	// nil dereference on any path that builds a grid before logging is wired.
	if logger == nil {
		logger = zap.NewNop()
	}

	logger.Debug("Creating new grid",
		zap.String("characters", characters),
		zap.String("rowLabels", rowLabels),
		zap.String("colLabels", colLabels),
		zap.Int("bounds_width", bounds.Dx()),
		zap.Int("bounds_height", bounds.Dy()))

	if characters == "" {
		characters = "abcdefghijklmnopqrstuvwxyz"
	}
	// Cache uppercase conversion once at the start
	uppercaseChars := strings.ToUpper(characters)
	chars := []rune(uppercaseChars)
	numChars := len(chars)

	// Ensure we have valid characters
	if numChars < MinCharactersLength {
		uppercaseChars = strings.ToUpper("abcdefghijklmnopqrstuvwxyz")
		chars = []rune(uppercaseChars)
		numChars = len(chars)
	}

	// Prepare row and column labels
	rowChars := chars
	colChars := chars

	if rowLabels != "" {
		rowChars = []rune(strings.ToUpper(rowLabels))
	}

	if colLabels != "" {
		colChars = []rune(strings.ToUpper(colLabels))
	}

	numRowChars := len(rowChars)
	numColChars := len(colChars)

	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	logger.Debug("Grid dimensions calculated",
		zap.Int("width", width),
		zap.Int("height", height))

	if gridCacheEnabled {
		if cells, ok := gridCache.get(
			uppercaseChars,
			strings.ToUpper(rowLabels),
			strings.ToUpper(colLabels),
			bounds,
		); ok {
			logger.Debug("Grid cache hit",
				zap.Int("cell_count", len(cells)))

			// Pre-allocate index map with exact capacity
			index := make(map[string]*Cell, len(cells))
			for _, cell := range cells {
				index[cell.Coordinate()] = cell
			}

			// Build prefix index for fast prefix matching
			prefixes := buildPrefixIndex(cells)

			return &Grid{
				characters: uppercaseChars,
				rowChars:   rowChars,
				colChars:   colChars,
				bounds:     bounds,
				cells:      cells,
				index:      index,
				prefixes:   prefixes,
			}
		}

		logger.Debug("Grid cache miss")
	}

	if width <= 0 || height <= 0 {
		logger.Warn("Invalid grid bounds, creating minimal grid",
			zap.Int("width", width),
			zap.Int("height", height))

		return &Grid{
			characters: uppercaseChars,
			bounds:     bounds,
			cells:      []*Cell{},
			index:      make(map[string]*Cell),
			prefixes:   make(map[string]bool),
		}
	}

	// Clamp display dimensions to a practical maximum to bound candidate search
	// work and prevent overflow in area computation. Real display bounds are
	// typically under 30000 pixels per side; this clamp is purely defensive.
	if width > MaxDisplayDimension {
		width = MaxDisplayDimension
	}

	if height > MaxDisplayDimension {
		height = MaxDisplayDimension
	}

	// Automatically determine optimal cell size constraints based on screen characteristics
	minCellSize, maxCellSize := calculateOptimalCellSizes(width, height)

	// Find all valid grid configurations and pick the one with best aspect ratio match
	// This ensures cells are as square as possible for intuitive navigation
	candidates := findValidGridConfigurations(width, height, minCellSize, maxCellSize)

	// Pick the candidate with the best (lowest) score
	gridCols, gridRows := selectBestCandidate(candidates, width, height, minCellSize, maxCellSize)

	// Safety check: ensure we always have at least a 2x2 grid
	if gridCols < MinGridCols {
		gridCols = 2
	}

	if gridRows < MinGridRows {
		gridRows = 2
	}

	// Clamp grid dimensions to a practical maximum to prevent allocation panics.
	// gridCols and gridRows are screen-derived (< 300), so this is defensive.
	if gridCols > MaxGridCols {
		gridCols = MaxGridCols
	}

	if gridRows > MaxGridRows {
		gridRows = MaxGridRows
	}

	// No overflow guard is needed: both dimensions are clamped to constants
	// above, so the product is at most MaxGridCols*MaxGridRows.
	totalCells := gridRows * gridCols

	// Determine optimal label length based on total cells and available characters
	labelLength := calculateLabelLength(totalCells, numChars, numRowChars, numColChars)

	// Calculate maximum possible cells we can label based on label length
	var maxPossibleCells int
	switch labelLength {
	case LabelLength2:
		maxPossibleCells = numChars * numColChars
	case LabelLength3:
		maxPossibleCells = numChars * numColChars * numRowChars
	default:
		maxPossibleCells = numChars * numChars * numColChars * numRowChars
	}

	// Cap totalCells to what we can actually label
	if totalCells > maxPossibleCells {
		// Calculate grid dimensions that fit within maxPossibleCells
		gridCols = gridMax(
			int(math.Sqrt(float64(maxPossibleCells)*float64(width)/float64(height))),
			1,
		)
		gridRows = gridMax(maxPossibleCells/gridCols, 1)
		// Update totalCells to match the actual grid dimensions
		totalCells = gridRows * gridCols //nolint:ineffassign,staticcheck,wastedassign // totalCells is used later in calculateLabelLength
	}

	// Cells are emitted region by region, and a region that runs off the right
	// or bottom edge is clipped — its unused capacity is forfeited. With a large
	// character set there are far more region prefixes than the grid needs, so
	// that waste is invisible. With a small one the regions run out mid-grid and
	// generateCellsWithRegions stops early, leaving screen area with no cell at
	// all: a 4-character set on 1000x3000 lost the bottom ~430px entirely.
	//
	// Shrink the grid until the regions can actually reach every cell. This only
	// binds for small character sets; the default 25-character alphabet has
	// hundreds of spare prefixes and comes through untouched.
	gridCols, gridRows = fitToAvailableRegions(
		gridCols, gridRows, numChars, numRowChars, numColChars, labelLength,
	)

	// Calculate base cell sizes and remainders
	baseCellWidth := width / gridCols
	baseCellHeight := height / gridRows
	remainderWidth := width % gridCols
	remainderHeight := height % gridRows

	// Generate cells with spatial region logic
	cells := generateCellsWithRegions(
		chars,
		rowChars,
		colChars,
		numChars,
		gridCols,
		gridRows,
		labelLength,
		bounds,
		baseCellWidth,
		baseCellHeight,
		remainderWidth,
		remainderHeight,
		logger,
	)

	logger.Debug("Grid created successfully",
		zap.Int("cell_count", len(cells)),
		zap.Int("grid_cols", gridCols),
		zap.Int("grid_rows", gridRows),
		zap.Int("label_length", labelLength))

	if gridCacheEnabled {
		gridCache.put(
			uppercaseChars,
			strings.ToUpper(rowLabels),
			strings.ToUpper(colLabels),
			bounds,
			cells,
		)
		logger.Debug("Grid cache store",
			zap.Int("cell_count", len(cells)))
	}

	// Pre-allocate index map with exact capacity
	index := make(map[string]*Cell, len(cells))
	for _, cell := range cells {
		index[cell.Coordinate()] = cell
	}

	// Build prefix index for fast prefix matching
	prefixes := buildPrefixIndex(cells)

	return &Grid{
		characters: uppercaseChars,
		rowChars:   rowChars,
		colChars:   colChars,
		bounds:     bounds,
		cells:      cells,
		index:      index,
		prefixes:   prefixes,
	}
}

// Characters returns the characters used for coordinates.
func (g *Grid) Characters() string {
	return g.characters
}

// RowLabels returns the row labels used for coordinates.
func (g *Grid) RowLabels() string {
	return string(g.rowChars)
}

// ColLabels returns the column labels used for coordinates.
func (g *Grid) ColLabels() string {
	return string(g.colChars)
}

// ValidCharacters returns all characters that can appear in grid coordinates.
func (g *Grid) ValidCharacters() string {
	// If no custom labels, return the main characters
	if len(g.rowChars) == 0 && len(g.colChars) == 0 {
		return g.characters
	}

	charSet := make(map[rune]bool)
	for _, r := range g.characters {
		charSet[r] = true
	}

	for _, r := range g.rowChars {
		charSet[r] = true
	}

	for _, r := range g.colChars {
		charSet[r] = true
	}

	result := make([]rune, 0, len(charSet))
	for r := range charSet {
		result = append(result, r)
	}

	slices.Sort(result)

	return string(result)
}

// Bounds returns the screen bounds.
func (g *Grid) Bounds() image.Rectangle {
	return g.bounds
}

// Cells returns all cells with 3-char coordinates.
func (g *Grid) Cells() []*Cell {
	return g.cells
}

// Index returns the coordinate→cell index built at construction time.
// The returned map is shared internal state and must not be modified by callers.
func (g *Grid) Index() map[string]*Cell {
	return g.index
}
