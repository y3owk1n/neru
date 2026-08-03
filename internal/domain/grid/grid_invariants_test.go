package grid_test

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain/grid"
)

// gridSizes covers ordinary displays plus the extreme aspect ratios the cell
// sizing has a special branch for, and sizes that do not divide evenly by the
// cell count so the remainder-distribution path is exercised.
func gridSizes() []image.Rectangle {
	return []image.Rectangle{
		image.Rect(0, 0, 1920, 1080),
		image.Rect(0, 0, 1366, 768),
		image.Rect(0, 0, 3840, 2160),
		image.Rect(0, 0, 1080, 1920),    // portrait
		image.Rect(0, 0, 5120, 1440),    // ultrawide
		image.Rect(0, 0, 1000, 3000),    // extremely tall
		image.Rect(0, 0, 1001, 733),     // prime-ish, forces remainders
		image.Rect(100, 200, 1300, 950), // non-zero origin (secondary display)
	}
}

func gridAlphabets() []string {
	return []string{"ab", "abcd", "asdfghjkl", "abcdefghijklmnopqrstuvwxyz"}
}

func newTestGrid(bounds image.Rectangle, characters string) *grid.Grid {
	return grid.NewGrid(characters, bounds, zap.NewNop())
}

func gridCaseName(bounds image.Rectangle, characters string) string {
	return fmt.Sprintf("%dx%d@%d,%d/chars=%d",
		bounds.Dx(), bounds.Dy(), bounds.Min.X, bounds.Min.Y, len(characters))
}

// TestGrid_CellsStayWithinBounds is the most basic geometric requirement: a
// cell outside the screen cannot be clicked, and a cell that pokes past the
// edge would place its center off-screen.
//
// The grid is built by a geometry pass (splitting the screen into rows and
// columns, distributing the leftover pixels) and a labeling pass (assigning a
// coordinate to each cell). Both are arithmetic-heavy, and a single off-by-one
// in either still produces a plausible-looking grid — cells exist, coordinates
// look right — while leaving dead pixels the user cannot click, overlapping
// cells that shadow each other, or duplicate coordinates.
//
// These tests assert the structural invariants that must hold for every grid,
// rather than pinning specific numbers for one screen size.
func TestGrid_CellsStayWithinBounds(t *testing.T) {
	for _, bounds := range gridSizes() {
		for _, characters := range gridAlphabets() {
			t.Run(gridCaseName(bounds, characters), func(t *testing.T) {
				testGrid := newTestGrid(bounds, characters)

				cells := testGrid.Cells()
				if len(cells) == 0 {
					t.Fatal("grid produced no cells")
				}

				for _, cell := range cells {
					cellBounds := cell.Bounds()

					if !cellBounds.In(bounds) {
						t.Errorf("cell %q has bounds %v, outside the grid bounds %v",
							cell.Coordinate(), cellBounds, bounds)
					}

					if cellBounds.Empty() {
						t.Errorf("cell %q has empty bounds %v", cell.Coordinate(), cellBounds)
					}

					// The center is the point a click is dispatched to, so it
					// has to be inside the cell it belongs to.
					if center := cell.Center(); !center.In(cellBounds) {
						t.Errorf("cell %q center %v lies outside its own bounds %v",
							cell.Coordinate(), center, cellBounds)
					}
				}
			})
		}
	}
}

// TestGrid_CellsTileTheBoundsExactly is the invariant that catches the leftover
// pixel distribution. Rows and columns rarely divide the screen evenly, so the
// remainder is spread one pixel at a time across the leading cells. If that
// arithmetic drifts, adjacent cells either overlap (two labels claim the same
// pixel) or leave a seam the user cannot target — neither of which changes the
// cell count, so only a coverage check like this notices.
func TestGrid_CellsTileTheBoundsExactly(t *testing.T) {
	for _, bounds := range gridSizes() {
		for _, characters := range gridAlphabets() {
			t.Run(gridCaseName(bounds, characters), func(t *testing.T) {
				cells := newTestGrid(bounds, characters).Cells()

				coveredArea := assertCellsAbut(t, cells)

				// Non-overlapping cells that abut in both directions and cover
				// the full area can only be an exact tiling.
				if wantArea := bounds.Dx() * bounds.Dy(); coveredArea != wantArea {
					t.Errorf(
						"cells cover %d square pixels, want %d (the grid must tile its bounds exactly)",
						coveredArea,
						wantArea,
					)
				}
			})
		}
	}
}

// TestGrid_CellsNeverOverlapOrLeaveSeams is the weaker structural companion to
// the tiling check: whatever cells exist must line up exactly, and the grid must
// start at the bounds origin. Kept separate because it holds even for grids that
// legitimately cannot cover the screen (a character set too small to address it).
func TestGrid_CellsNeverOverlapOrLeaveSeams(t *testing.T) {
	for _, bounds := range gridSizes() {
		for _, characters := range gridAlphabets() {
			t.Run(gridCaseName(bounds, characters), func(t *testing.T) {
				cells := newTestGrid(bounds, characters).Cells()

				coveredArea := assertCellsAbut(t, cells)

				if coveredArea <= 0 {
					t.Fatal("grid covers no area at all")
				}

				// Cells must start at the origin of the bounds; an offset grid
				// would push the whole layout off the top-left of the screen.
				minX, minY := cells[0].Bounds().Min.X, cells[0].Bounds().Min.Y
				for _, cell := range cells {
					minX = min(minX, cell.Bounds().Min.X)
					minY = min(minY, cell.Bounds().Min.Y)
				}

				if minX != bounds.Min.X || minY != bounds.Min.Y {
					t.Errorf("grid starts at (%d,%d), want the bounds origin (%d,%d)",
						minX, minY, bounds.Min.X, bounds.Min.Y)
				}
			})
		}
	}
}

// assertCellsAbut checks that cells sharing a row abut horizontally and cells
// sharing a column abut vertically, and returns the total area they cover.
func assertCellsAbut(t *testing.T, cells []*grid.Cell) int {
	t.Helper()

	if len(cells) == 0 {
		t.Fatal("grid produced no cells")
	}

	rows := make(map[int][]image.Rectangle)
	columns := make(map[int][]image.Rectangle)

	coveredArea := 0

	for _, cell := range cells {
		cellBounds := cell.Bounds()
		rows[cellBounds.Min.Y] = append(rows[cellBounds.Min.Y], cellBounds)
		columns[cellBounds.Min.X] = append(columns[cellBounds.Min.X], cellBounds)
		coveredArea += cellBounds.Dx() * cellBounds.Dy()
	}

	assertStripsAbut(t, rows, true)
	assertStripsAbut(t, columns, false)

	return coveredArea
}

// assertStripsAbut checks that the rectangles in each strip, sorted along the
// varying axis, touch edge-to-edge with no gap and no overlap.
func assertStripsAbut(t *testing.T, strips map[int][]image.Rectangle, horizontal bool) {
	t.Helper()

	for key, strip := range strips {
		sorted := make([]image.Rectangle, len(strip))
		copy(sorted, strip)

		// Insertion sort along the varying axis; strips are short.
		for i := 1; i < len(sorted); i++ {
			for j := i; j > 0 && startOf(sorted[j], horizontal) < startOf(sorted[j-1], horizontal); j-- {
				sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
			}
		}

		for idx := 1; idx < len(sorted); idx++ {
			prevEnd := endOf(sorted[idx-1], horizontal)
			currStart := startOf(sorted[idx], horizontal)

			if prevEnd != currStart {
				axis := "row"
				if !horizontal {
					axis = "column"
				}

				t.Errorf(
					"%s at %d: cell %v ends at %d but the next starts at %d (gap or overlap of %d px)",
					axis,
					key,
					sorted[idx-1],
					prevEnd,
					currStart,
					currStart-prevEnd,
				)

				break
			}
		}
	}
}

func startOf(rect image.Rectangle, horizontal bool) int {
	if horizontal {
		return rect.Min.X
	}

	return rect.Min.Y
}

func endOf(rect image.Rectangle, horizontal bool) int {
	if horizontal {
		return rect.Max.X
	}

	return rect.Max.Y
}

// TestGrid_CoordinatesAreUniqueAndAddressable checks the labeling pass. Two
// cells sharing a coordinate makes one of them permanently unreachable, and a
// coordinate that does not resolve back to its own cell would send the click
// somewhere else entirely.
func TestGrid_CoordinatesAreUniqueAndAddressable(t *testing.T) {
	for _, bounds := range gridSizes() {
		for _, characters := range gridAlphabets() {
			t.Run(gridCaseName(bounds, characters), func(t *testing.T) {
				testGrid := newTestGrid(bounds, characters)

				cells := testGrid.Cells()
				seen := make(map[string]image.Rectangle, len(cells))

				valid := testGrid.ValidCharacters()

				for _, cell := range cells {
					coordinate := cell.Coordinate()

					if coordinate == "" {
						t.Errorf("cell at %v has an empty coordinate", cell.Bounds())

						continue
					}

					if previous, duplicate := seen[coordinate]; duplicate {
						t.Errorf("coordinate %q is used by both %v and %v",
							coordinate, previous, cell.Bounds())
					}

					seen[coordinate] = cell.Bounds()

					// Every character in a coordinate must be typeable, i.e.
					// reported by ValidCharacters — that set is what the input
					// validator accepts, so a coordinate outside it could
					// never be entered.
					for _, char := range coordinate {
						if !strings.ContainsRune(valid, char) {
							t.Errorf(
								"coordinate %q contains %q, which is not in ValidCharacters(%q)",
								coordinate, string(char), valid,
							)
						}
					}

					// The coordinate must resolve back to this exact cell.
					resolved := testGrid.CellByCoordinate(coordinate)
					if resolved == nil {
						t.Errorf("CellByCoordinate(%q) returned nil", coordinate)

						continue
					}

					if resolved != cell {
						t.Errorf("CellByCoordinate(%q) resolved to the cell at %v, want %v",
							coordinate, resolved.Bounds(), cell.Bounds())
					}
				}

				if len(testGrid.Index()) != len(cells) {
					t.Errorf("index holds %d entries for %d cells",
						len(testGrid.Index()), len(cells))
				}
			})
		}
	}
}

// TestGrid_CoordinateLengthIsUniform pins that every coordinate in a grid has
// the same length. The input handler completes a selection once the user has
// typed labelLength characters, so a grid mixing 2- and 3-character coordinates
// would fire on the wrong cell partway through a longer label.
func TestGrid_CoordinateLengthIsUniform(t *testing.T) {
	for _, bounds := range gridSizes() {
		for _, characters := range gridAlphabets() {
			t.Run(gridCaseName(bounds, characters), func(t *testing.T) {
				testGrid := newTestGrid(bounds, characters)

				cells := testGrid.Cells()
				if len(cells) == 0 {
					t.Fatal("grid produced no cells")
				}

				want := len(cells[0].Coordinate())

				if want < grid.LabelLength2 || want > grid.LabelLength4 {
					t.Errorf("coordinate length %d is outside the supported range %d..%d",
						want, grid.LabelLength2, grid.LabelLength4)
				}

				for _, cell := range cells {
					if got := len(cell.Coordinate()); got != want {
						t.Errorf("cell %q has a %d-character coordinate, want %d for every cell",
							cell.Coordinate(), got, want)
					}
				}

				// The label must be long enough to address every cell: with
				// numChars characters, a label of length L can name at most
				// numChars^L cells.
				capacity := 1
				for range want {
					capacity *= len(characters)
				}

				if len(cells) > capacity {
					t.Errorf(
						"grid has %d cells but a %d-character label over %d characters addresses only %d",
						len(cells),
						want,
						len(characters),
						capacity,
					)
				}
			})
		}
	}
}

// TestGrid_NilLoggerIsAccepted pins the constructor convention every other
// constructor in this tree follows: a nil logger falls back to a no-op instead
// of panicking. This is reachable in production from any path that builds a
// grid before logging is wired, and the panic takes the daemon down.
func TestGrid_NilLoggerIsAccepted(t *testing.T) {
	testGrid := grid.NewGrid("abc", image.Rect(0, 0, 400, 300), nil)
	if testGrid == nil {
		t.Fatal("NewGrid with a nil logger returned nil")
	}

	if len(testGrid.Cells()) == 0 {
		t.Error("NewGrid with a nil logger produced no cells")
	}

	withLabels := grid.NewGridWithLabels("abc", "ab", "bc", image.Rect(0, 0, 400, 300), nil)
	if withLabels == nil {
		t.Fatal("NewGridWithLabels with a nil logger returned nil")
	}

	if len(withLabels.Cells()) == 0 {
		t.Error("NewGridWithLabels with a nil logger produced no cells")
	}
}

// TestGrid_HasCoordinatePrefixMatchesRealPrefixes checks the prefix index the
// input handler consults on every keystroke to decide whether the partial input
// can still lead anywhere. A false negative rejects a valid keystroke; a false
// positive leaves the user in a dead end.
func TestGrid_HasCoordinatePrefixMatchesRealPrefixes(t *testing.T) {
	testGrid := newTestGrid(image.Rect(0, 0, 1920, 1080), "asdfghjkl")

	cells := testGrid.Cells()
	if len(cells) == 0 {
		t.Fatal("grid produced no cells")
	}

	// Every genuine prefix of every coordinate must be reported.
	realPrefixes := make(map[string]bool)

	for _, cell := range cells {
		coordinate := cell.Coordinate()
		for idx := 1; idx <= len(coordinate); idx++ {
			realPrefixes[coordinate[:idx]] = true
		}
	}

	for prefix := range realPrefixes {
		if !testGrid.HasCoordinatePrefix(prefix) {
			t.Errorf("HasCoordinatePrefix(%q) = false, but a coordinate starts with it", prefix)
		}
	}

	// And a prefix no coordinate starts with must be rejected. "ZZ" cannot
	// occur: Z is not in the alphabet above.
	for _, absent := range []string{"Z", "ZZ", "ZZZ"} {
		if testGrid.HasCoordinatePrefix(absent) {
			t.Errorf("HasCoordinatePrefix(%q) = true, but no coordinate starts with it", absent)
		}
	}
}
