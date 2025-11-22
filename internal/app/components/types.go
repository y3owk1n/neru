package components

import (
	"strings"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/features/action"
	"github.com/y3owk1n/neru/internal/features/grid"
	"github.com/y3owk1n/neru/internal/features/hints"
	"github.com/y3owk1n/neru/internal/features/scroll"
	"go.uber.org/zap"
)

// HintsComponent encapsulates all hints-related functionality.
type HintsComponent struct {
	Generator *hints.Generator
	Overlay   *hints.Overlay
	Manager   *hints.Manager
	Router    *hints.Router
	Context   *hints.Context
	Style     hints.StyleMode
}

// UpdateConfig updates the hints component with new configuration.
func (h *HintsComponent) UpdateConfig(cfg *config.Config, _ *zap.Logger) {
	if h.Overlay != nil && cfg.Hints.Enabled {
		h.Style = hints.BuildStyle(cfg.Hints)
		h.Overlay.UpdateConfig(cfg.Hints)
	}
	// Update generator characters if they changed
	if h.Generator != nil && cfg.Hints.HintCharacters != "" {
		h.Generator.UpdateCharacters(cfg.Hints.HintCharacters)
	}
}

// GridComponent encapsulates all grid-related functionality.
type GridComponent struct {
	Manager *grid.Manager
	Router  *grid.Router
	Context *grid.Context
	Style   grid.Style
}

// UpdateConfig updates the grid component with new configuration.
func (g *GridComponent) UpdateConfig(cfg *config.Config, logger *zap.Logger) {
	if cfg.Grid.Enabled {
		g.Style = grid.BuildStyle(cfg.Grid)
		if g.Context != nil && g.Context.GetGridOverlay() != nil {
			(*g.Context.GetGridOverlay()).UpdateConfig(cfg.Grid)
		}

		if g.Manager != nil {
			// Recreate grid if characters changed
			oldGrid := g.Manager.GetGrid()
			if oldGrid != nil && cfg.Grid.Characters != "" &&
				strings.ToUpper(cfg.Grid.Characters) != oldGrid.GetCharacters() {
				logger.Debug("Recreating grid with new characters")
				newGrid := grid.NewGrid(cfg.Grid.Characters, oldGrid.GetBounds(), logger)
				g.Manager.UpdateGrid(newGrid)
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

// ScrollComponent encapsulates all scroll-related functionality.
type ScrollComponent struct {
	Overlay *scroll.Overlay
	Context *scroll.Context
}

// UpdateConfig updates the scroll component with new configuration.
func (s *ScrollComponent) UpdateConfig(cfg *config.Config, _ *zap.Logger) {
	if s.Overlay != nil {
		s.Overlay.UpdateConfig(cfg.Scroll)
	}
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
