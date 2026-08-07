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
				// The config arrives with its labels already resolved to the
				// ones in use (config.ResolveGridLabels), and the grid stores
				// the same resolved form, so this is a plain comparison. It
				// used to infer the labels here, which is the only place that
				// rule existed — and getting it wrong rebuilt the grid on every
				// reload, discarding the user's in-flight coordinate input.
				characters := cfg.GridCharacters()
				newCharacters := strings.ToUpper(characters)
				newRowLabels := cfg.Grid.RowLabels
				newColLabels := cfg.Grid.ColLabels

				charactersChanged := newCharacters != oldGrid.Characters()
				rowLabelsChanged := newRowLabels != oldGrid.RowLabels()
				colLabelsChanged := newColLabels != oldGrid.ColLabels()

				if charactersChanged || rowLabelsChanged || colLabelsChanged {
					logger.Debug("Recreating grid due to config changes",
						zap.Bool("charactersChanged", charactersChanged),
						zap.Bool("rowLabelsChanged", rowLabelsChanged),
						zap.Bool("colLabelsChanged", colLabelsChanged))
					newGrid := domainGrid.NewGridWithLabels(
						characters,
						cfg.Grid.RowLabels,
						cfg.Grid.ColLabels,
						oldGrid.Bounds(),
						logger,
					)
					g.Manager.UpdateGrid(newGrid)
				}
			}

			// Update manager subgrid keys if they changed. They arrive resolved
			// to the ones the overlay draws (config.ResolveSublayerKeys), so
			// there is nothing to infer here.
			g.Manager.UpdateSubKeys(cfg.Grid.SublayerKeys)
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
