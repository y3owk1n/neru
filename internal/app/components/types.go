package components

import (
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	domainRecursiveGrid "github.com/y3owk1n/neru/internal/domain/recursivegrid"
)

// HintsComponent encapsulates all hints-related functionality.
//
// It names no overlay at all: what a hints session keeps is state, and the
// surface it ends up on is reached through ports.OverlayPort.
type HintsComponent struct {
	Context *hints.Context
}

// GridComponent encapsulates all grid-related functionality.
type GridComponent struct {
	Manager *domainGrid.Manager
	Router  *domainGrid.Router
	Context *grid.Context
}

// UpdateConfig rebuilds the grid's domain state — the grid itself and its
// subgrid keys — when the configuration that defines them changes. Appearance
// is not its business; the overlay resolves that.
func (g *GridComponent) UpdateConfig(cfg *config.Config, logger *zap.Logger) {
	if cfg.Grid.Enabled {
		if g.Manager != nil {
			// Recreate grid if characters or labels changed
			oldGrid := g.Manager.Grid()
			if oldGrid != nil && cfg.Grid.Characters != "" {
				// Empty row/column labels in config mean "infer from
				// characters", which is what NewGridWithLabels does when it
				// builds the grid. Resolve them the same way before comparing:
				// the grid stores the inferred labels, so comparing those
				// against a bare "" would never match and every config reload
				// would rebuild the grid — discarding in-flight grid input —
				// even when nothing about the labels changed.
				newCharacters := strings.ToUpper(cfg.Grid.Characters)

				newRowLabels := newCharacters
				if cfg.Grid.RowLabels != "" {
					newRowLabels = strings.ToUpper(cfg.Grid.RowLabels)
				}

				newColLabels := newCharacters
				if cfg.Grid.ColLabels != "" {
					newColLabels = strings.ToUpper(cfg.Grid.ColLabels)
				}

				charactersChanged := newCharacters != oldGrid.Characters()
				rowLabelsChanged := newRowLabels != oldGrid.RowLabels()
				colLabelsChanged := newColLabels != oldGrid.ColLabels()

				if charactersChanged || rowLabelsChanged || colLabelsChanged {
					logger.Debug("Recreating grid due to config changes",
						zap.Bool("charactersChanged", charactersChanged),
						zap.Bool("rowLabelsChanged", rowLabelsChanged),
						zap.Bool("colLabelsChanged", colLabelsChanged))
					newGrid := domainGrid.NewGridWithLabels(
						cfg.Grid.Characters,
						cfg.Grid.RowLabels,
						cfg.Grid.ColLabels,
						oldGrid.Bounds(),
						logger,
					)
					g.Manager.UpdateGrid(newGrid)
				}
			}

			// Update manager subgrid keys if they changed
			subKeys := cfg.Grid.SublayerKeys
			if subKeys == "" {
				subKeys = cfg.Grid.Characters
			}

			g.Manager.UpdateSubKeys(subKeys)
		}
	}
}

// ScrollComponent encapsulates scroll key mapping and state (no overlay).
type ScrollComponent struct {
	Context *scroll.Context
}

// UpdateConfig updates the scroll component with new configuration.
func (s *ScrollComponent) UpdateConfig(_ *config.Config, _ *zap.Logger) {
}

// RecursiveGridComponent encapsulates all recursive-grid-related functionality.
type RecursiveGridComponent struct {
	Manager *domainRecursiveGrid.Manager
	Context *recursivegrid.Context
}
