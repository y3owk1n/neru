package components

import (
	"strings"

	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	domainRecursiveGrid "github.com/y3owk1n/neru/internal/core/domain/recursivegrid"
	"go.uber.org/zap"
)

// HintsComponent encapsulates all hints-related functionality.
type HintsComponent struct {
	Overlay *hints.Overlay
	Context *hints.Context
	Style   hints.StyleMode
}

// UpdateConfig updates the hints component with new configuration.
func (h *HintsComponent) UpdateConfig(cfg *config.Config, _ *zap.Logger) {
	if h.Overlay != nil && cfg.Hints.Enabled {
		h.Style = hints.BuildStyle(cfg.Hints)
		h.Overlay.UpdateConfig(cfg.Hints)
	}
}

// GridComponent encapsulates all grid-related functionality.
type GridComponent struct {
	Manager *domainGrid.Manager
	Router  *domainGrid.Router
	Context *grid.Context
	Style   grid.Style
}

// UpdateConfig updates the grid component with new configuration.
func (g *GridComponent) UpdateConfig(config *config.Config, logger *zap.Logger) {
	if config.Grid.Enabled {
		g.Style = grid.BuildStyle(config.Grid)
		if g.Context != nil && g.Context.GridOverlay() != nil {
			(*g.Context.GridOverlay()).SetConfig(config.Grid)
		}

		if g.Manager != nil {
			// Recreate grid if characters or labels changed
			oldGrid := g.Manager.Grid()
			if oldGrid != nil && config.Grid.Characters != "" {
				charactersChanged := strings.ToUpper(config.Grid.Characters) != oldGrid.Characters()
				rowLabelsChanged := strings.ToUpper(config.Grid.RowLabels) != oldGrid.RowLabels()
				colLabelsChanged := strings.ToUpper(config.Grid.ColLabels) != oldGrid.ColLabels()

				if charactersChanged || rowLabelsChanged || colLabelsChanged {
					logger.Debug("Recreating grid due to config changes",
						zap.Bool("charactersChanged", charactersChanged),
						zap.Bool("rowLabelsChanged", rowLabelsChanged),
						zap.Bool("colLabelsChanged", colLabelsChanged))
					newGrid := domainGrid.NewGridWithLabels(
						config.Grid.Characters,
						config.Grid.RowLabels,
						config.Grid.ColLabels,
						oldGrid.Bounds(),
						logger,
					)
					g.Manager.UpdateGrid(newGrid)
				}
			}

			// Update manager subgrid keys if they changed
			subKeys := config.Grid.SublayerKeys
			if subKeys == "" {
				subKeys = config.Grid.Characters
			}

			g.Manager.UpdateSubKeys(subKeys)
		}
	}
}

// ScrollComponent encapsulates scroll key mapping and state (no overlay).
type ScrollComponent struct {
	Context *scroll.Context
	KeyMap  *scroll.KeyMap
}

// UpdateConfig updates the scroll component with new configuration.
func (s *ScrollComponent) UpdateConfig(cfg *config.Config, logger *zap.Logger) {
	s.KeyMap = scroll.NewKeyMap(cfg.Scroll.KeyBindings)
}

// ModeIndicatorComponent encapsulates the shared mode indicator overlay.
type ModeIndicatorComponent struct {
	Overlay *scroll.Overlay
}

// UpdateConfig updates the mode indicator component with new configuration.
func (m *ModeIndicatorComponent) UpdateConfig(cfg *config.Config, _ *zap.Logger) {
	if m.Overlay != nil {
		m.Overlay.UpdateConfig(cfg.Scroll, cfg.ModeIndicator)
	}
}

// RecursiveGridComponent encapsulates all recursive-grid-related functionality.
type RecursiveGridComponent struct {
	Manager *domainRecursiveGrid.Manager
	Overlay *recursivegrid.Overlay
	Context *recursivegrid.Context
	Style   recursivegrid.Style
}

// UpdateConfig updates the recursive-grid component with new configuration.
func (q *RecursiveGridComponent) UpdateConfig(cfg *config.Config, _ *zap.Logger) {
	if cfg.RecursiveGrid.Enabled {
		q.Style = recursivegrid.BuildStyle(cfg.RecursiveGrid)
		if q.Overlay != nil {
			q.Overlay.SetConfig(cfg.RecursiveGrid)
		}
	}
}
