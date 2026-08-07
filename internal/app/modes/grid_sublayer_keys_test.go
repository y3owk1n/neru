package modes

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// TestInitializeGridManager_AcceptsTheKeysTheOverlayDraws is the acceptance for
// #1269. The overlay decides which keys are *drawn* and the mode layer decides
// which keys are *accepted*, from separate reads of the configuration — the
// overlay is handed GridConfig alone, so it cannot even see the hint characters
// the rest of the chain fell back to. Four consumers inferred the blank case
// for themselves and nothing made their answers agree.
//
// So this asserts both sides against one value: grid.SubgridKeys over the
// resolved grid.sublayer_keys is what every overlay backend draws
// (render/grid/overlay_darwin.go, linux/overlay_shared_cgo.go,
// windows/overlay.go), and the manager built here is what accepts.
func TestInitializeGridManager_AcceptsTheKeysTheOverlayDraws(t *testing.T) {
	testCases := []struct {
		name           string
		sublayerKeys   string
		gridCharacters string
		hintCharacters string
	}{
		{
			name:           "keys the user configured",
			sublayerKeys:   "uiop",
			gridCharacters: gridLabelGridChars,
			hintCharacters: gridLabelHintChars,
		},
		{
			name:           "no keys configured, so the grid characters",
			gridCharacters: gridLabelGridChars,
			hintCharacters: gridLabelHintChars,
		},
		{
			name:           "no keys and no grid characters, so the hint characters",
			hintCharacters: gridLabelHintChars,
		},
		{
			name:           "more keys than the subgrid has cells",
			sublayerKeys:   "abcdefghijklmnop",
			gridCharacters: gridLabelGridChars,
			hintCharacters: gridLabelHintChars,
		},
		{
			// The floor. Nothing is configured anywhere, so the grid labels
			// itself a-z and the subgrid has to follow it there rather than be
			// drawn with nothing.
			name: "no keys, no grid characters and no hint characters",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Grid.Enabled = true
			cfg.Grid.SublayerKeys = testCase.sublayerKeys
			cfg.Grid.Characters = testCase.gridCharacters
			cfg.Hints.HintCharacters = testCase.hintCharacters
			cfg.ResolveDerived()

			// What the overlay is handed, read the way an overlay reads it.
			drawn := domainGrid.SubgridKeys(
				cfg.Grid.SublayerKeys,
				domain.SubgridRows*domain.SubgridCols,
			)
			if len(drawn) == 0 {
				t.Fatal("the overlay would draw a subgrid with no keys on it")
			}

			manager := openSubgridForKeys(t, cfg)

			for _, key := range drawn {
				if _, selected := manager.HandleInput(string(key)); !selected {
					t.Errorf("subgrid refused %q, which the overlay draws", string(key))
				}
			}

			for _, key := range notDrawn(drawn) {
				if _, selected := manager.HandleInput(string(key)); selected {
					t.Errorf("subgrid selected on %q, which the overlay never draws", string(key))
				}
			}
		})
	}
}

// openSubgridForKeys builds the grid mode's manager from cfg and types a cell
// coordinate into it, so the next key it is handed is a subgrid selection.
// Selecting inside a subgrid leaves it open, so one opening answers for every
// key the caller probes.
func openSubgridForKeys(t *testing.T, cfg *config.Config) *domainGrid.Manager {
	t.Helper()

	handler := newGridLabelHandler(cfg)

	gridInstance := handler.createGridInstance()
	handler.initializeGridManager(gridInstance)

	manager := handler.grid.Manager
	if manager == nil {
		t.Fatal("initializeGridManager left the handler without a manager")
	}

	cells := gridInstance.Cells()
	if len(cells) == 0 {
		t.Fatal("the grid has no cells to open a subgrid on")
	}

	for _, char := range cells[0].Coordinate() {
		manager.HandleInput(string(char))
	}

	return manager
}

// notDrawn returns the letters the overlay would not put on the subgrid, so a
// test can assert the manager refuses them rather than only that it accepts the
// rest — accepting everything would satisfy the first half on its own.
func notDrawn(drawn []rune) []rune {
	isDrawn := make(map[rune]bool, len(drawn))
	for _, key := range drawn {
		isDrawn[key] = true
	}

	var missing []rune

	for key := 'A'; key <= 'Z'; key++ {
		if !isDrawn[key] {
			missing = append(missing, key)
		}
	}

	return missing
}
