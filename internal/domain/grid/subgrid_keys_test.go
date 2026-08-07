package grid_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/logger"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/grid"
)

// subgridCells is the size of the subgrid these cases label — the one every
// caller in the repo draws, since all three surfaces and the manager are 3x3.
const subgridCells = domain.SubgridRows * domain.SubgridCols

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
			if got := string(grid.SubgridKeys(testCase.keys, subgridCells)); got != testCase.want {
				t.Errorf("SubgridKeys(%q) = %q, want %q", testCase.keys, got, testCase.want)
			}
		})
	}
}

// TestSubgridKeys_CapAgreesWithTheSelectionBound keeps the two ends of the cap
// together. SubgridKeys drops keys past the caller's last cell, and
// handleSubgridSelection separately refuses any key index at or past
// MaxKeyIndex; if that bound and the cell count ever disagree, a key the
// overlay drew is a key the manager refuses — the drawn/accepted split this
// replaced, reappearing inside the domain.
func TestSubgridKeys_CapAgreesWithTheSelectionBound(t *testing.T) {
	if grid.MaxKeyIndex != subgridCells {
		t.Errorf(
			"selection refuses key indexes at or past %d but the subgrid has %d cells",
			grid.MaxKeyIndex, subgridCells,
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

	drawn := grid.SubgridKeys(configured, subgridCells)
	if len(drawn) != domain.SubgridRows*domain.SubgridCols {
		t.Fatalf("SubgridKeys(%q) drew %d keys, want %d", configured, len(drawn),
			domain.SubgridRows*domain.SubgridCols)
	}

	for _, key := range drawn {
		manager := newSubgridKeyManager(t, configured)

		if _, selected := manager.HandleInput(string(key)); !selected {
			t.Errorf("subgrid refused %q, which it is drawn with", string(key))
		}
	}

	for _, key := range "ZXC" {
		manager := newSubgridKeyManager(t, configured)

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

	drawn := grid.SubgridKeys(configured, subgridCells)

	points := make(map[image.Point]rune, len(drawn))

	for _, key := range drawn {
		manager := newSubgridKeyManager(t, configured)

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
// next key it is handed is a subgrid selection.
func newSubgridKeyManager(t *testing.T, subKeys string) *grid.Manager {
	t.Helper()

	log := logger.Get()
	testGrid := grid.NewGrid("abcdefghijklmnopqrstuvwxyz", image.Rect(0, 0, 1000, 800), log)

	manager := grid.NewManager(
		testGrid,
		domain.SubgridRows,
		domain.SubgridCols,
		subKeys,
		func(bool) {},
		func(*grid.Cell) {},
		log,
	)

	cell := testGrid.CellForPoint(image.Point{X: 500, Y: 400})
	if cell == nil {
		t.Fatal("the test grid has no cell at its center")
	}

	for _, char := range cell.Coordinate() {
		manager.HandleInput(string(char))
	}

	return manager
}
