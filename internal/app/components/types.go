package components

import (
	"strings"

	"github.com/y3owk1n/neru/internal/app/components/action"
	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
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

// ScrollComponent encapsulates all scroll-related functionality.
type ScrollComponent struct {
	Overlay *scroll.Overlay
	Context *scroll.Context
	KeyMap  *scroll.KeyMap
}

// UpdateConfig updates the scroll component with new configuration.
func (s *ScrollComponent) UpdateConfig(cfg *config.Config, logger *zap.Logger) {
	if s.Overlay != nil {
		s.Overlay.UpdateConfig(cfg.Scroll)
	}

	s.KeyMap = scroll.NewKeyMap(cfg.Scroll.KeyBindings)
}

// ActionComponent encapsulates all action-related functionality.
type ActionComponent struct {
	Overlay *action.Overlay
}

// UpdateConfig updates the action component with new configuration.
func (a *ActionComponent) UpdateConfig(cfg *config.Config, _ *zap.Logger) {
	if a.Overlay != nil {
		a.Overlay.UpdateConfig(cfg.Action)
	}
}
