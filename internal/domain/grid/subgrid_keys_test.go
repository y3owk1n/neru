package grid_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/logger"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/grid"
)

// TestSubgridKeys pins the answer to "which keys does a subgrid have?". It used
// to be answered once per overlay backend and once more by the manager, each
// with its own trimming, its own case and its own cap, and nothing made the set
// that was drawn and the set that was accepted the same set.
func TestSubgridKeys(t *testing.T) {
	testCases := []struct {
		name string
		keys string
		want string
	}{
		{
			name: "keys are the case they are drawn in",
			keys: "asdfghjkl",
			want: "ASDFGHJKL",
		},
		{
			name: "surrounding whitespace is not a key",
			keys: "  asdf  ",
			want: "ASDF",
		},
		{
			name: "keys past the last cell are dropped",
			keys: "abcdefghijklmnop",
			want: "ABCDEFGHI",
		},
		{
			name: "fewer keys than cells label the cells there are keys for",
			keys: "abc",
			want: "ABC",
		},
		{
			name: "no keys is a subgrid with nothing on it",
			keys: "",
			want: "",
		},
		{
			name: "a key listed twice is one key",
			keys: "aabcdefgh",
			want: "ABCDEFGH",
		},
		{
			name: "a repeat is dropped whichever case it is written in",
			keys: "aAbB",
			want: "AB",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := string(
				grid.SubgridKeys(testCase.keys, grid.MaxKeyIndex),
			); got != testCase.want {
				t.Errorf("SubgridKeys(%q) = %q, want %q", testCase.keys, got, testCase.want)
			}
		})
	}
}

// TestSubgridKeys_CapAgreesWithTheCellsThereAre keeps the two ends of the cap
// together. Every overlay caps its key set at MaxKeyIndex and then draws those
// keys on the rectangles SubgridCells hands back; if the constant and the
// division ever disagree, a backend draws a label with no cell under it or a
// cell with no label on it. Both are answers to "how big is the subgrid?", and
// this is where they have to be one answer.
func TestSubgridKeys_CapAgreesWithTheCellsThereAre(t *testing.T) {
	divided := grid.SubgridCells(
		image.Rect(0, 0, 300, 150),
		domain.SubgridDimensions(),
	)

	if grid.MaxKeyIndex != len(divided) {
		t.Errorf(
			"overlays cap the key set at %d but the subgrid divides into %d cells",
			grid.MaxKeyIndex, len(divided),
		)
	}
}

// TestManager_AcceptsExactlyTheKeysTheSubgridIsDrawnWith is the acceptance the
// overlays rest on: the manager selects for every key SubgridKeys hands back,
// and for no other key. An overlay draws that set, so a key the manager takes
// and the overlay never drew — or the other way round — is a subgrid that lies
// about what it will do.
func TestManager_AcceptsExactlyTheKeysTheSubgridIsDrawnWith(t *testing.T) {
	// More keys than cells, so the cap is exercised rather than assumed.
	const configured = "asdfghjklzxc"

	drawn := grid.SubgridKeys(configured, grid.MaxKeyIndex)
	if len(drawn) != domain.SubgridRows*domain.SubgridCols {
		t.Fatalf("SubgridKeys(%q) drew %d keys, want %d", configured, len(drawn),
			domain.SubgridRows*domain.SubgridCols)
	}

	for _, key := range drawn {
		manager, _ := newSubgridKeyManager(t, configured)

		if _, selected := manager.HandleInput(string(key)); !selected {
			t.Errorf("subgrid refused %q, which it is drawn with", string(key))
		}
	}

	for _, key := range "ZXC" {
		manager, _ := newSubgridKeyManager(t, configured)

		if _, selected := manager.HandleInput(string(key)); selected {
			t.Errorf("subgrid selected on %q, which no cell is drawn with", string(key))
		}
	}
}

// TestManager_EveryDrawnSubgridKeySelectsItsOwnPoint is what a duplicated key
// costs. Selection resolves a key by its first index, so a key listed twice used
// to draw a second label over a second cell and then send both presses to the
// first one: nine labels, eight reachable points, and no way to tell which of
// the two was dead.
func TestManager_EveryDrawnSubgridKeySelectsItsOwnPoint(t *testing.T) {
	// Nine keys, one of them written twice, so the set fills the subgrid only if
	// the repeat counts as a key.
	const configured = "aabcdefgh"

	drawn := grid.SubgridKeys(configured, grid.MaxKeyIndex)

	points := make(map[image.Point]rune, len(drawn))

	for _, key := range drawn {
		manager, _ := newSubgridKeyManager(t, configured)

		point, selected := manager.HandleInput(string(key))
		if !selected {
			t.Errorf("subgrid refused %q, which it is drawn with", string(key))

			continue
		}

		if previous, shared := points[point]; shared {
			t.Errorf("keys %q and %q both select %v", string(previous), string(key), point)
		}

		points[point] = key
	}
}

// newSubgridKeyManager returns a manager sitting in an open subgrid, so the
// next key it is handed is a subgrid selection, along with the bounds of the
// cell that subgrid was opened on — the rectangle an overlay would have been
// handed to draw it in.
func newSubgridKeyManager(t *testing.T, subKeys string) (*grid.Manager, image.Rectangle) {
	t.Helper()

	return newSubgridManager(t, domain.SubgridDimensions(), subKeys)
}

// newSubgridManager is newSubgridKeyManager for a subgrid of any shape: it
// drives the manager into a cell's subgrid and hands back the manager and the
// rectangle that subgrid covers.
func newSubgridManager(
	t *testing.T,
	dims domain.GridDimensions,
	subKeys string,
) (*grid.Manager, image.Rectangle) {
	t.Helper()

	log := logger.Get()
	testGrid := grid.NewGrid("abcdefghijklmnopqrstuvwxyz", image.Rect(0, 0, 1000, 800), log)

	var opened image.Rectangle

	manager := grid.NewManager(
		testGrid,
		dims,
		subKeys,
		func(bool) {},
		func(cell *grid.Cell) { opened = cell.Bounds() },
		log,
	)

	cell := testGrid.CellForPoint(image.Point{X: 500, Y: 400})
	if cell == nil {
		t.Fatal("the test grid has no cell at its center")
	}

	for _, char := range cell.Coordinate() {
		manager.HandleInput(string(char))
	}

	if opened.Empty() {
		t.Fatal("the manager did not open a subgrid on any cell")
	}

	return manager, opened
}
