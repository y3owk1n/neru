package grid

import (
	"image"
	"math"
	"slices"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
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

	// MinCharactersLength is the fewest distinct characters a grid can be
	// labeled from — distinct, because repeats are dropped before the floor is
	// applied (newGridAlphabet).
	MinCharactersLength = 2

	// DefaultCharacters is the alphabet a grid is labeled from: a-z without
	// `o`, which is hard to tell from `0` at label size. It is both the
	// shipped default for grid.characters and grid.sublayer_keys (config
	// assigns them from this constant) and the set newGridAlphabet falls back
	// to when the configured one cannot label anything. Those were two
	// literals and they drifted — the fallback kept the `o` the default had
	// dropped — so they are one constant now, and a user who misconfigures
	// grid.characters cannot be shown a character the default excludes.
	DefaultCharacters = "abcdefghijklmnpqrstuvwxyz"

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

	// MaxKeyIndex is how many cells the subgrid every overlay draws has, and so
	// the index no key of it reaches. It is what those overlays cap their key
	// set at (SubgridKeys), which is why it is derived from the shipped
	// dimensions rather than written: the cells a backend draws and the keys it
	// draws on them have to be counted the same way, and a 9 written by hand is
	// a second answer waiting for the day the subgrid stops being 3x3.
	MaxKeyIndex = domain.SubgridRows * domain.SubgridCols

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

	// MinLabelLength is the shortest coordinate format supported by the grid.
	MinLabelLength = LabelLength2

	// DefaultMaxLabelLength preserves the grid's existing automatic 2–4 key
	// coordinate selection when no explicit limit is supplied.
	DefaultMaxLabelLength = LabelLength4

	// CountsCapacity is the capacity for counts.
	CountsCapacity = 5

	// PrefixLengthCheck is the check for prefix length.
	PrefixLengthCheck = 2
)

// Grid is a coordinate grid system for spatial navigation with optimized cell sizing.
type Grid struct {
	characters     string          // Characters used for coordinates (e.g., "asdfghjkl")
	rowChars       []rune          // Characters used for row labels
	colChars       []rune          // Characters used for column labels
	maxLabelLength int             // Longest coordinate the planner may choose
	bounds         image.Rectangle // Screen bounds
	cells          []*Cell         // All cells with uniform-length coordinates
	index          map[string]*Cell
	prefixes       map[string]bool // Set of all coordinate prefixes for fast lookup
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
	return NewGridWithOptions(Options{Characters: characters}, bounds, logger)
}

// NewGridWithLabels creates a grid with custom row and column labels.
// Empty rowLabels or colLabels are inferred from characters.
func NewGridWithLabels(
	characters, rowLabels, colLabels string,
	bounds image.Rectangle,
	logger *zap.Logger,
) *Grid {
	return NewGridWithOptions(Options{
		Characters: characters,
		RowLabels:  rowLabels,
		ColLabels:  colLabels,
	}, bounds, logger)
}

// Options are the label inputs that affect a grid's geometry.
type Options struct {
	Characters     string
	RowLabels      string
	ColLabels      string
	MaxLabelLength int
}

// NewGridWithOptions creates a grid from label options. MaxLabelLength limits
// the coarse coordinate to 2–4 keypresses; zero keeps the default limit of 4.
func NewGridWithOptions(options Options, bounds image.Rectangle, logger *zap.Logger) *Grid {
	// Constructors in this tree accept a nil logger and fall back to a no-op
	// rather than panicking on first use.
	if logger == nil {
		logger = zap.NewNop()
	}

	maxLabelLength := normalizeMaxLabelLength(options.MaxLabelLength)

	logger.Debug("Creating new grid",
		zap.String("characters", options.Characters),
		zap.String("rowLabels", options.RowLabels),
		zap.String("colLabels", options.ColLabels),
		zap.Int("max_label_length", maxLabelLength),
		zap.Int("bounds_width", bounds.Dx()),
		zap.Int("bounds_height", bounds.Dy()))

	alpha := newGridAlphabet(options.Characters, options.RowLabels, options.ColLabels)

	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	logger.Debug("Grid dimensions calculated",
		zap.Int("width", width),
		zap.Int("height", height))

	if gridCacheEnabled {
		if cells, ok := gridCache.get(
			alpha.characters,
			strings.ToUpper(options.RowLabels),
			strings.ToUpper(options.ColLabels),
			maxLabelLength,
			bounds,
		); ok {
			logger.Debug("Grid cache hit", zap.Int("cell_count", len(cells)))

			return newGridFromCells(alpha, maxLabelLength, bounds, cells)
		}

		logger.Debug("Grid cache miss")
	}

	if width <= 0 || height <= 0 {
		logger.Warn("Invalid grid bounds, creating minimal grid",
			zap.Int("width", width),
			zap.Int("height", height))

		return &Grid{
			characters:     alpha.characters,
			maxLabelLength: maxLabelLength,
			bounds:         bounds,
			cells:          []*Cell{},
			index:          make(map[string]*Cell),
			prefixes:       make(map[string]bool),
		}
	}

	// Clamp to a practical maximum to bound candidate-search work and prevent
	// overflow in area computation. Real displays are well under this.
	width = gridMin(width, MaxDisplayDimension)
	height = gridMin(height, MaxDisplayDimension)

	gridCols, gridRows, labelLength := planGridDimensions(
		width,
		height,
		alpha,
		maxLabelLength,
	)

	baseCellWidth := width / gridCols
	baseCellHeight := height / gridRows
	remainderWidth := width % gridCols
	remainderHeight := height % gridRows

	cells := generateCellsWithRegions(
		alpha.chars,
		alpha.rowChars,
		alpha.colChars,
		len(alpha.chars),
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
			alpha.characters,
			strings.ToUpper(options.RowLabels),
			strings.ToUpper(options.ColLabels),
			maxLabelLength,
			bounds,
			cells,
		)
		logger.Debug("Grid cache store", zap.Int("cell_count", len(cells)))
	}

	return newGridFromCells(alpha, maxLabelLength, bounds, cells)
}

// gridAlphabet is the normalized character sets a grid is labeled from.
type gridAlphabet struct {
	characters string // uppercase coordinate characters
	chars      []rune
	rowChars   []rune
	colChars   []rune
}

// newGridAlphabet uppercases the character sets, drops the characters they
// repeat, and falls back to DefaultCharacters when the coordinate set is left
// too small to label anything with.
//
// The repeats go here rather than at each of the three sets in turn because this
// is the one place all three are read from — a coordinate is built from chars,
// rowChars and colChars together (DistinctKeys says what a repeat costs).
//
// Dropping them can be what makes a set too small — "aa" is one character, not
// two — so the floor is applied to what is left, not to what was written.
func newGridAlphabet(characters, rowLabels, colLabels string) gridAlphabet {
	chars := DistinctKeys(characters)

	if len(chars) < MinCharactersLength {
		chars = DistinctKeys(DefaultCharacters)
	}

	rowChars := chars
	colChars := chars

	if rowLabels != "" {
		rowChars = DistinctKeys(rowLabels)
	}

	if colLabels != "" {
		colChars = DistinctKeys(colLabels)
	}

	return gridAlphabet{
		characters: string(chars),
		chars:      chars,
		rowChars:   rowChars,
		colChars:   colChars,
	}
}

// ResolveLabels answers the row and column labels a grid built from these
// arguments will actually use, so a caller can hold that answer instead of
// re-deriving it. Empty labels are inferred from characters, which is itself
// replaced by DefaultCharacters when it is empty or too short to label
// anything — that second step is why the answer is worth asking for rather than assuming
// "empty means characters".
//
// A label set that was written comes back upper-cased and otherwise as it was,
// repeats and all, even though the grid will drop those (newGridAlphabet). What
// the derivation settles is what the *user asked for*, and a repeat is the one
// part of that a reader still needs: config.warnGridKeySets reads these fields
// to tell the user a character was dropped, and it runs after the derivation, so
// dropping it here would leave nothing to report and the grid quietly labeled
// with a set nobody chose.
//
// Passing the result back into NewGridWithLabels builds the same grid as
// passing the empty strings did, which is what lets config.ResolveGridLabels
// settle the option at load time.
func ResolveLabels(characters, rowLabels, colLabels string) (string, string) {
	alpha := newGridAlphabet(characters, "", "")

	return settleLabels(rowLabels, alpha.chars), settleLabels(colLabels, alpha.chars)
}

// settleLabels is one label set's answer: what was written, upper-cased, or the
// characters the grid falls back on when nothing was.
func settleLabels(labels string, inferred []rune) string {
	if labels == "" {
		return string(inferred)
	}

	return strings.ToUpper(labels)
}

// ResolveCharacters answers the coordinate characters a grid built from this set
// will report ([Grid.Characters]): upper-cased, with repeats dropped, and
// replaced by DefaultCharacters when what is left is too short to label with.
//
// It is for the caller comparing a reloaded configuration against the grid it is
// already running — a set that differs from the live one only by a repeat or a
// letter's case is the same set, and treating it as a change rebuilds the grid
// and discards whatever coordinate the user was halfway through typing.
func ResolveCharacters(characters string) string {
	return newGridAlphabet(characters, "", "").characters
}

// newGridFromCells assembles a Grid and its lookup indexes from finished cells.
func newGridFromCells(
	alpha gridAlphabet,
	maxLabelLength int,
	bounds image.Rectangle,
	cells []*Cell,
) *Grid {
	index := make(map[string]*Cell, len(cells))
	for _, cell := range cells {
		index[cell.Coordinate()] = cell
	}

	return &Grid{
		characters:     alpha.characters,
		rowChars:       alpha.rowChars,
		colChars:       alpha.colChars,
		maxLabelLength: maxLabelLength,
		bounds:         bounds,
		cells:          cells,
		index:          index,
		prefixes:       buildPrefixIndex(cells),
	}
}

// planGridDimensions picks the column and row counts and the label length for a
// screen, keeping cells as square as possible and never planning more cells
// than the alphabet can label or the regions can reach.
func planGridDimensions(
	width, height int,
	alpha gridAlphabet,
	maxLabelLength int,
) (int, int, int) {
	numChars := len(alpha.chars)
	numRowChars := len(alpha.rowChars)
	numColChars := len(alpha.colChars)

	minCellSize, maxCellSize := calculateOptimalCellSizes(width, height)
	candidates := findValidGridConfigurations(width, height, minCellSize, maxCellSize)
	gridCols, gridRows := selectBestCandidate(candidates, width, height, minCellSize, maxCellSize)

	gridCols = gridMax(gridCols, MinGridCols)
	gridRows = gridMax(gridRows, MinGridRows)

	// Defensive: dimensions are screen-derived (< 300), the clamp prevents
	// allocation panics on absurd input.
	gridCols = gridMin(gridCols, MaxGridCols)
	gridRows = gridMin(gridRows, MaxGridRows)

	totalCells := gridRows * gridCols
	labelLength := calculateLabelLength(totalCells, numChars, numRowChars, numColChars)
	labelLength = gridMin(labelLength, maxLabelLength)

	var maxPossibleCells int
	switch labelLength {
	case LabelLength2:
		maxPossibleCells = numChars * numColChars
	case LabelLength3:
		maxPossibleCells = numChars * numColChars * numRowChars
	default:
		maxPossibleCells = numChars * numChars * numColChars * numRowChars
	}

	// Cap the grid to what the alphabet can label.
	if totalCells > maxPossibleCells {
		gridCols = gridMax(
			int(math.Sqrt(float64(maxPossibleCells)*float64(width)/float64(height))),
			1,
		)
		gridRows = gridMax(maxPossibleCells/gridCols, 1)
	}

	// Cells are emitted region by region and a clipped region forfeits its
	// unused capacity. A small character set can run out of regions mid-grid,
	// leaving screen area with no cell at all (a 4-character set on 1000x3000
	// lost the bottom ~430px). Shrink until the regions reach every cell; the
	// default 25-character alphabet comes through untouched.
	gridCols, gridRows = fitToAvailableRegions(
		gridCols, gridRows, numChars, numRowChars, numColChars, labelLength,
	)

	return gridCols, gridRows, labelLength
}

// normalizeMaxLabelLength gives the domain a safe zero value for callers that
// do not come through config validation, then bounds malformed values so the
// coordinate builder never receives a format it does not support.
func normalizeMaxLabelLength(maxLabelLength int) int {
	if maxLabelLength == 0 {
		return DefaultMaxLabelLength
	}

	return gridMin(gridMax(maxLabelLength, MinLabelLength), DefaultMaxLabelLength)
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

// MaxLabelLength returns the coarse-coordinate limit used to plan the grid.
func (g *Grid) MaxLabelLength() int {
	return g.maxLabelLength
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

// Cells returns all cells with uniform-length coordinates.
func (g *Grid) Cells() []*Cell {
	return g.cells
}

// Index returns the coordinate→cell index built at construction time.
// The returned map is shared internal state and must not be modified by callers.
func (g *Grid) Index() map[string]*Cell {
	return g.index
}
