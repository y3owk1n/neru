package grid_test

import (
	"image"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/logger"
	"github.com/y3owk1n/neru/internal/domain/grid"
)

const (
	testCharacters = "ABC"
	// allLetters is a 26-character input, not the set a grid falls back to —
	// that one is grid.DefaultCharacters, which leaves `o` out.
	allLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

func TestGrid_Initialization(t *testing.T) {
	log := logger.Get()
	tests := []struct {
		name      string
		chars     string
		bounds    image.Rectangle
		wantCells int
	}{
		{
			name:      "standard 1080p",
			chars:     allLetters,
			bounds:    image.Rect(0, 0, 1920, 1080),
			wantCells: 26 * 26, // 2 chars depth
		},
		{
			name:      "small screen",
			chars:     "ABC",
			bounds:    image.Rect(0, 0, 100, 100),
			wantCells: 3 * 3, // 2 chars depth
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			grid := grid.NewGrid(testCase.chars, testCase.bounds, log)
			if len(grid.AllCells()) == 0 {
				t.Error("Expected cells to be generated")
			}
		})
	}
}

func TestGrid_CellByCoordinate(t *testing.T) {
	logger := logger.Get()
	grid := grid.NewGrid("ABC", image.Rect(0, 0, 300, 300), logger)

	cells := grid.AllCells()
	if len(cells) == 0 {
		t.Fatal("Expected cells to be generated")
	}

	validCoordinate := cells[0].Coordinate()

	tests := []struct {
		name  string
		coord string
		want  bool // exists
	}{
		{"valid " + validCoordinate, validCoordinate, true},
		{"invalid ZZZ", "ZZZ", false},
		{"lowercase coordinate", strings.ToLower(validCoordinate), true},
		{"empty coordinate", "", false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cell := grid.CellByCoordinate(testCase.coord)
			if (cell != nil) != testCase.want {
				t.Errorf(
					"CellByCoordinate(%q) exists = %v, want %v",
					testCase.coord,
					cell != nil,
					testCase.want,
				)
			}
		})
	}
}

func TestCell_Methods(t *testing.T) {
	logger := logger.Get()
	grid := grid.NewGrid("ABC", image.Rect(0, 0, 300, 300), logger)

	cells := grid.AllCells()
	if len(cells) == 0 {
		t.Fatal("Expected cells to be generated")
	}

	cell := cells[0]

	// Test that methods return non-zero values
	if cell.Coordinate() == "" {
		t.Error("Coordinate() returned empty string")
	}

	bounds := cell.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Errorf("Bounds() returned invalid bounds: %v", bounds)
	}

	center := cell.Center()
	if center.X < 0 || center.Y < 0 {
		t.Errorf("Center() returned invalid center: %v", center)
	}

	// Test that center is within bounds
	if !center.In(bounds) {
		t.Errorf("Center %v is not within bounds %v", center, bounds)
	}
}

func TestGrid_Getters(t *testing.T) {
	logger := logger.Get()
	bounds := image.Rect(0, 0, 1920, 1080)
	gridInstance := grid.NewGrid(testCharacters, bounds, logger)

	if gridInstance.Characters() != testCharacters {
		t.Errorf("Characters() = %q, want %q", gridInstance.Characters(), testCharacters)
	}

	if gridInstance.Bounds() != bounds {
		t.Errorf("Bounds() = %v, want %v", gridInstance.Bounds(), bounds)
	}

	cells := gridInstance.Cells()
	if len(cells) == 0 {
		t.Error("Cells() returned empty slice")
	}

	index := gridInstance.Index()
	if len(index) != len(cells) {
		t.Errorf("Index() length = %d, want %d", len(index), len(cells))
	}

	allCells := gridInstance.AllCells()
	if len(allCells) != len(cells) {
		t.Errorf("AllCells() length = %d, want %d", len(allCells), len(cells))
	}
}

func TestCalculateOptimalGrid(t *testing.T) {
	tests := []struct {
		name       string
		characters string
		wantRows   int
		wantCols   int
	}{
		{"normal characters", testCharacters, 3, 3},
		{"empty string", "", 9, 9},
		{"single character", "A", 9, 9},
		{"long string", allLetters, 26, 26},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			rows, cols := grid.CalculateOptimalGrid(testCase.characters)
			if rows != testCase.wantRows || cols != testCase.wantCols {
				t.Errorf("CalculateOptimalGrid(%q) = (%d, %d), want (%d, %d)",
					testCase.characters, rows, cols, testCase.wantRows, testCase.wantCols)
			}
		})
	}
}

func TestGrid_InvalidBounds(t *testing.T) {
	logger := logger.Get()

	// Test with zero width
	gridInstance := grid.NewGrid("ABC", image.Rect(0, 0, 0, 100), logger)
	if len(gridInstance.Cells()) != 0 {
		t.Error("Expected empty cells for zero width")
	}

	// Test with zero height
	gridInstance = grid.NewGrid("ABC", image.Rect(0, 0, 100, 0), logger)
	if len(gridInstance.Cells()) != 0 {
		t.Error("Expected empty cells for zero height")
	}

	// Test with negative dimensions (image.Rect normalizes this to positive dimensions)
	// So this actually creates a valid grid. Let's test with truly invalid bounds
	gridInstance = grid.NewGrid("ABC", image.Rect(100, 100, 100, 100), logger) // Zero width/height
	if len(gridInstance.Cells()) != 0 {
		t.Errorf(
			"Expected empty cells for zero dimensions, got %d cells",
			len(gridInstance.Cells()),
		)
	}
}

func TestGrid_EmptyCharacters(t *testing.T) {
	logger := logger.Get()
	bounds := image.Rect(0, 0, 300, 300)
	fallback := strings.ToUpper(grid.DefaultCharacters)

	// Empty characters should default to the fallback alphabet
	gridInstance := grid.NewGrid("", bounds, logger)
	if gridInstance.Characters() != fallback {
		t.Errorf(
			"Empty characters should default to alphabet, got %q",
			gridInstance.Characters(),
		)
	}

	// Single character should also default
	gridInstance = grid.NewGrid("A", bounds, logger)
	if gridInstance.Characters() != fallback {
		t.Errorf(
			"Single character should default to alphabet, got %q",
			gridInstance.Characters(),
		)
	}
}

// TestGrid_RepeatedCharactersLabelOnce pins what a coordinate set that lists a
// character twice builds: the grid the distinct characters build. A repeat used
// to widen the alphabet the labeling pass counted on, which drew coordinates
// twice over and left every cell past the first one carrying them unreachable —
// `characters = "aab"` put 16 different cells under `AAAA`.
func TestGrid_RepeatedCharactersLabelOnce(t *testing.T) {
	log := logger.Get()
	bounds := image.Rect(0, 0, 1000, 800)

	repeated := grid.NewGrid("aab", bounds, log)
	distinct := grid.NewGrid("ab", bounds, log)

	if repeated.Characters() != distinct.Characters() {
		t.Errorf(
			"Characters() = %q, want %q — the repeat is not a character",
			repeated.Characters(), distinct.Characters(),
		)
	}

	if len(repeated.Cells()) != len(distinct.Cells()) {
		t.Errorf(
			"a grid built from %q has %d cells, one built from %q has %d",
			"aab", len(repeated.Cells()), "ab", len(distinct.Cells()),
		)
	}
}

// TestGrid_RepeatsCanLeaveTooFewCharacters covers the floor the dedupe lands on:
// a set that is only repeats has one distinct character, which cannot label a
// grid at all, so it falls back like any other set too short to label with.
func TestGrid_RepeatsCanLeaveTooFewCharacters(t *testing.T) {
	gridInstance := grid.NewGrid("aA", image.Rect(0, 0, 300, 300), logger.Get())

	if want := strings.ToUpper(grid.DefaultCharacters); gridInstance.Characters() != want {
		t.Errorf("Characters() = %q, want the fallback %q", gridInstance.Characters(), want)
	}
}

func TestGrid_WithCustomLabels(t *testing.T) {
	logger := logger.Get()
	bounds := image.Rect(0, 0, 300, 300)

	gridInstance := grid.NewGridWithLabels(testCharacters, "123", "XYZ", bounds, logger)

	if gridInstance.Characters() != testCharacters {
		t.Errorf("Characters() = %q, want %q", gridInstance.Characters(), testCharacters)
	}

	validChars := gridInstance.ValidCharacters()

	expectedChars := "ABC123XYZ"
	for _, r := range expectedChars {
		if !strings.ContainsRune(validChars, r) {
			t.Errorf("ValidCharacters() missing %c, got %q", r, validChars)
		}
	}

	cells := gridInstance.Cells()
	if len(cells) == 0 {
		t.Fatal("No cells generated")
	}

	foundLabels := make(map[string]bool)
	for _, cell := range cells {
		if foundLabels[cell.Coordinate()] {
			t.Errorf("Duplicate coordinate found: %s", cell.Coordinate())
		}

		foundLabels[cell.Coordinate()] = true
	}

	// Should have unique coordinates for all cells
	if len(foundLabels) != len(cells) {
		t.Errorf("Expected %d unique coordinates, got %d", len(cells), len(foundLabels))
	}
}

func TestGrid_CustomLabelsWithSymbols(t *testing.T) {
	logger := logger.Get()
	bounds := image.Rect(0, 0, 500, 500)

	characters := "AOEUIDHTNSPYFGKXBM"
	rowLabels := "',.PYFGCRL/AOEUIDHTNS-;QJKXBMWVZ="
	colLabels := "AOEUIDHTNS"

	gridInstance := grid.NewGridWithLabels(characters, rowLabels, colLabels, bounds, logger)

	validChars := gridInstance.ValidCharacters()

	expectedSymbols := "',./-;=QJKXBMWVZPYFGCRL"
	for _, r := range expectedSymbols {
		if !strings.ContainsRune(validChars, r) {
			t.Errorf("ValidCharacters() missing symbol %c, got %q", r, validChars)
		}
	}

	cells := gridInstance.Cells()

	coordMap := make(map[string]bool)
	for _, cell := range cells {
		coord := cell.Coordinate()
		if coordMap[coord] {
			t.Errorf("Duplicate coordinate: %s", coord)
		}

		coordMap[coord] = true

		// Verify coordinate uses valid characters
		for _, r := range coord {
			if !strings.ContainsRune(validChars, r) {
				t.Errorf("Coordinate %s contains invalid character %c", coord, r)
			}
		}
	}
}

func TestGrid_BackwardCompatibility(t *testing.T) {
	logger := logger.Get()
	bounds := image.Rect(0, 0, 300, 300)

	// Test that old NewGrid still works (empty row/col labels)
	gridInstance := grid.NewGrid(testCharacters, bounds, logger)

	if gridInstance.Characters() != testCharacters {
		t.Errorf("Characters() = %q, want %q", gridInstance.Characters(), testCharacters)
	}

	// ValidCharacters should be same as Characters when no custom labels
	if gridInstance.ValidCharacters() != gridInstance.Characters() {
		t.Errorf(
			"ValidCharacters() = %q, want %q",
			gridInstance.ValidCharacters(),
			gridInstance.Characters(),
		)
	}

	// Should have unique coordinates
	cells := gridInstance.Cells()

	coordMap := make(map[string]bool)
	for _, cell := range cells {
		if coordMap[cell.Coordinate()] {
			t.Errorf("Duplicate coordinate: %s", cell.Coordinate())
		}

		coordMap[cell.Coordinate()] = true
	}
}

func TestGrid_HasCoordinatePrefix(t *testing.T) {
	logger := logger.Get()
	bounds := image.Rect(0, 0, 300, 300)

	grid1 := grid.NewGrid(testCharacters, bounds, logger)

	cells := grid1.Cells()
	if len(cells) == 0 {
		t.Fatal("Grid should have cells")
	}

	sampleCoord := cells[0].Coordinate()
	if len(sampleCoord) < 2 {
		t.Fatalf("Expected coordinate length >= 2, got %d", len(sampleCoord))
	}

	for i := 1; i <= len(sampleCoord); i++ {
		prefix := sampleCoord[:i]
		if !grid1.HasCoordinatePrefix(prefix) {
			t.Errorf(
				"HasCoordinatePrefix(%q) should be true for coordinate %q",
				prefix,
				sampleCoord,
			)
		}
	}

	if grid1.HasCoordinatePrefix("INVALID") {
		t.Error("HasCoordinatePrefix should return false for invalid prefix")
	}

	grid2 := grid.NewGrid(testCharacters, bounds, logger)

	for i := 1; i <= len(sampleCoord); i++ {
		prefix := sampleCoord[:i]
		if !grid2.HasCoordinatePrefix(prefix) {
			t.Errorf(
				"HasCoordinatePrefix(%q) should be true on cached grid for coordinate %q",
				prefix,
				sampleCoord,
			)
		}
	}

	if grid2.HasCoordinatePrefix("INVALID") {
		t.Error("HasCoordinatePrefix should return false for invalid prefix on cached grid")
	}
}
