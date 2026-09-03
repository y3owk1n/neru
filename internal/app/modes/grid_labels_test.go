package modes

import (
	"context"
	"image"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// gridLabelHintChars is the hint alphabet these fixtures fall back to,
// gridLabelHintLabels the labels a grid built from it carries, and
// gridLabelGridChars the characters a grid is built from when it has its own.
const (
	gridLabelHintChars  = "qwerty"
	gridLabelHintLabels = "QWERTY"
	gridLabelGridChars  = "asdfghjkl"
)

// TestCreateGridInstance_UsesTheResolvedLabels is the invariant the config-side
// resolution rests on: the labels the config carries have to be the labels the
// grid is built with. They are resolved from the characters a grid is built
// from, so a call site that reached for a different character set would draw a
// grid the configuration does not describe.
func TestCreateGridInstance_UsesTheResolvedLabels(t *testing.T) {
	testCases := []struct {
		name           string
		gridCharacters string
		hintCharacters string
		rowLabels      string
		colLabels      string
		wantCharacters string
	}{
		{
			name:           "labels inferred from the grid characters",
			gridCharacters: "asdf",
			hintCharacters: gridLabelHintChars,
			wantCharacters: "ASDF",
		},
		{
			name:           "labels the user configured",
			gridCharacters: "asdf",
			hintCharacters: gridLabelHintChars,
			rowLabels:      "xy",
			colLabels:      "zw",
			wantCharacters: "ASDF",
		},
		{
			name:           "labels inferred through the hint-characters fallback",
			gridCharacters: "",
			hintCharacters: gridLabelHintChars,
			wantCharacters: gridLabelHintLabels,
		},
		{
			name:           "labels inferred from a character set too short to label with",
			gridCharacters: "a",
			hintCharacters: gridLabelHintChars,
			wantCharacters: strings.ToUpper(domainGrid.DefaultCharacters),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Grid.Enabled = true
			cfg.Grid.Characters = testCase.gridCharacters
			cfg.Hints.HintCharacters = testCase.hintCharacters
			cfg.Grid.RowLabels = testCase.rowLabels
			cfg.Grid.ColLabels = testCase.colLabels
			cfg.ResolveGridLabels()

			handler := newGridLabelHandler(cfg)

			gridInstance := handler.createGridInstance()

			if got := gridInstance.RowLabels(); got != cfg.Grid.RowLabels {
				t.Errorf("grid RowLabels() = %q, want config's %q", got, cfg.Grid.RowLabels)
			}

			if got := gridInstance.ColLabels(); got != cfg.Grid.ColLabels {
				t.Errorf("grid ColLabels() = %q, want config's %q", got, cfg.Grid.ColLabels)
			}

			if got := gridInstance.Characters(); got != testCase.wantCharacters {
				t.Errorf("grid Characters() = %q, want %q", got, testCase.wantCharacters)
			}
		})
	}
}

// TestCreateGridInstance_UsesMaxLabelLength pins the mode-to-domain wiring for
// the two-key coarse selection option.
func TestCreateGridInstance_UsesMaxLabelLength(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Grid.Enabled = true
	cfg.Grid.Characters = "abcd"
	cfg.Grid.MaxLabelLength = 2
	cfg.ResolveGridLabels()

	gridInstance := newGridLabelHandler(cfg).createGridInstance()
	if got := gridInstance.MaxLabelLength(); got != 2 {
		t.Fatalf("grid MaxLabelLength() = %d, want 2", got)
	}

	for _, cell := range gridInstance.Cells() {
		if got := len(cell.Coordinate()); got > 2 {
			t.Fatalf("coordinate %q has length %d, want at most 2", cell.Coordinate(), got)
		}
	}
}

// TestInitializeGridManager_FallbackGridUsesTheResolvedLabels covers the
// defensive branch that builds its own grid when handed none. It reached for
// grid.characters directly while the path above reached for the hint
// characters when those were blank, so the two disagreed about what the grid
// was labeled from — invisible while the labels were inferred at draw time
// from whichever set was passed, and a wrong grid once they are resolved once.
func TestInitializeGridManager_FallbackGridUsesTheResolvedLabels(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Grid.Enabled = true
	cfg.Grid.Characters = ""
	cfg.Hints.HintCharacters = gridLabelHintChars
	cfg.Grid.RowLabels = ""
	cfg.Grid.ColLabels = ""
	cfg.ResolveGridLabels()

	handler := newGridLabelHandler(cfg)

	handler.initializeGridManager(nil)

	built := handler.grid.Manager.Grid()
	if built == nil {
		t.Fatal("initializeGridManager(nil) left the manager without a grid")
	}

	if got := built.RowLabels(); got != cfg.Grid.RowLabels {
		t.Errorf("fallback grid RowLabels() = %q, want config's %q", got, cfg.Grid.RowLabels)
	}

	if got := built.ColLabels(); got != cfg.Grid.ColLabels {
		t.Errorf("fallback grid ColLabels() = %q, want config's %q", got, cfg.Grid.ColLabels)
	}

	if got := built.Characters(); got != gridLabelHintLabels {
		t.Errorf(
			"fallback grid Characters() = %q, want %q — the same set the labels were "+
				"resolved from",
			got, gridLabelHintLabels,
		)
	}
}

// newGridLabelHandler builds the least handler these tests need. The screen
// has to be a real one: a grid with no area short-circuits to a minimal grid
// that carries no labels at all, which would pass these assertions vacuously.
func newGridLabelHandler(cfg *config.Config) *Handler {
	var gridInstance *domainGrid.Grid

	gridContext := &gridcomponent.Context{}
	gridContext.SetGridInstance(&gridInstance)

	screen := image.Rect(0, 0, 1920, 1080)

	return newHandlerWithState(handlerState{
		config: cfg,
		logger: zap.NewNop(),
		grid:   &components.GridComponent{Context: gridContext},
		system: &portmocks.MockSystemPort{
			ScreenBoundsFunc: func(_ context.Context) (image.Rectangle, error) {
				return screen, nil
			},
		},
		overlayPort:  &portmocks.MockOverlayPort{},
		screenBounds: screen,
	})
}
