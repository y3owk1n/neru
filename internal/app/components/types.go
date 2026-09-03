package components

import (
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
//
// It runs under the mode handler's `h.mu` (`modes.Handler.UpdateConfig`),
// because the manager it writes is the one a keystroke reads under that same
// lock and carries none of its own (#1277). So it must stay pure: no overlay,
// service or port call belongs here.
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
				// Each side asked of the grid rather than compared as written,
				// which is the same rule one step further on: the grid drops a
				// character a set repeats, so a set that gained only a repeat is
				// the set already in use, and comparing the strings would rebuild
				// on every reload for the rest of that config's life.
				characters := cfg.GridCharacters()
				newCharacters := domainGrid.ResolveCharacters(characters)
				newRowLabels := string(domainGrid.DistinctKeys(cfg.Grid.RowLabels))
				newColLabels := string(domainGrid.DistinctKeys(cfg.Grid.ColLabels))

				charactersChanged := newCharacters != oldGrid.Characters()
				rowLabelsChanged := newRowLabels != oldGrid.RowLabels()
				colLabelsChanged := newColLabels != oldGrid.ColLabels()
				maxLabelLengthChanged := cfg.Grid.MaxLabelLength != oldGrid.MaxLabelLength()

				if charactersChanged || rowLabelsChanged || colLabelsChanged ||
					maxLabelLengthChanged {
					logger.Debug("Recreating grid due to config changes",
						zap.Bool("charactersChanged", charactersChanged),
						zap.Bool("rowLabelsChanged", rowLabelsChanged),
						zap.Bool("colLabelsChanged", colLabelsChanged),
						zap.Bool("maxLabelLengthChanged", maxLabelLengthChanged))
					newGrid := domainGrid.NewGridWithOptions(
						cfg.GridOptions(),
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

// UpdateConfig updates the scroll component with new configuration. There is
// nothing to derive today; it is called anyway so that whatever lands here
// lands under the mode handler's `h.mu` (`modes.Handler.UpdateConfig`) by
// construction rather than by someone noticing, the way the grid's did not
// (#1277). The same rule applies: stay pure — no overlay, service or port call.
func (s *ScrollComponent) UpdateConfig(_ *config.Config, _ *zap.Logger) {
}

// RecursiveGridComponent encapsulates all recursive-grid-related functionality.
type RecursiveGridComponent struct {
	Manager *domainRecursiveGrid.Manager
	Context *recursivegrid.Context
}
