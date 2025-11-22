package app

import (
	"fmt"
	"strings"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/features/action"
	"github.com/y3owk1n/neru/internal/features/grid"
	"github.com/y3owk1n/neru/internal/features/hints"
	"github.com/y3owk1n/neru/internal/features/scroll"
	"github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

// createHintsComponent initializes the hints component based on configuration.
func createHintsComponent(
	cfg *config.Config,
	log *zap.Logger,
	overlayManager *overlay.Manager,
) (*components.HintsComponent, error) {
	component := &components.HintsComponent{}

	// Ensure hint characters are configured
	hintChars := cfg.Hints.HintCharacters
	if strings.TrimSpace(hintChars) == "" {
		hintChars = domain.DefaultHintCharacters
		log.Warn("No hint characters configured, using default: " + domain.DefaultHintCharacters)
	}

	// Always initialize generator to prevent nil pointer dereferences
	component.Generator = hints.NewGenerator(hintChars)

	// Only initialize full component if hints are enabled
	if !cfg.Hints.Enabled {
		return component, nil
	}

	component.Style = hints.BuildStyle(cfg.Hints)
	component.Manager = hints.NewManager(func(hs []*hints.Hint) {
		if component.Overlay == nil {
			return
		}
		err := component.Overlay.DrawHintsWithStyle(hs, component.Style)
		if err != nil {
			log.Error("Failed to redraw hints", zap.Error(err))
		}
	}, log)
	component.Router = hints.NewRouter(component.Manager, log)
	component.Context = &hints.Context{}

	hintOverlay, err := hints.NewOverlayWithWindow(cfg.Hints, log, overlayManager.GetWindowPtr())
	if err != nil {
		return nil, fmt.Errorf("failed to create hint overlay: %w", err)
	}
	component.Overlay = hintOverlay

	return component, nil
}

// createGridComponent initializes the grid component based on configuration.
func createGridComponent(
	cfg *config.Config,
	log *zap.Logger,
	overlayManager *overlay.Manager,
) *components.GridComponent {
	component := &components.GridComponent{}

	// Initialize minimal context even when disabled
	if !cfg.Grid.Enabled {
		var gridInstance *grid.Grid
		component.Context = &grid.Context{
			GridInstance: &gridInstance,
		}
		return component
	}

	// Ensure grid characters are configured
	gridChars := cfg.Grid.Characters
	if strings.TrimSpace(gridChars) == "" {
		gridChars = domain.DefaultHintCharacters
		log.Warn("No grid characters configured, using default: " + domain.DefaultHintCharacters)
	}

	component.Style = grid.BuildStyle(cfg.Grid)
	gridOverlay := grid.NewOverlayWithWindow(cfg.Grid, log, overlayManager.GetWindowPtr())
	var gridInstance *grid.Grid

	// Determine sublayer keys with fallback chain
	keys := strings.TrimSpace(cfg.Grid.SublayerKeys)
	if keys == "" {
		keys = gridChars
	}
	if keys == "" {
		keys = domain.DefaultHintCharacters
		log.Warn("No subgrid keys configured, using default: " + domain.DefaultHintCharacters)
	}

	// Create grid manager with callbacks
	component.Manager = grid.NewManager(
		nil,
		domain.SubgridRows,
		domain.SubgridCols,
		keys,
		func(_ bool) {
			if gridInstance == nil {
				return
			}
			gridOverlay.UpdateMatches(component.Manager.GetInput())
		},
		func(cell *grid.Cell) {
			gridOverlay.ShowSubgrid(cell, component.Style)
		},
		log,
	)

	component.Context = &grid.Context{
		GridInstance: &gridInstance,
		GridOverlay:  &gridOverlay,
	}

	return component
}

// createScrollComponent initializes the scroll component.
func createScrollComponent(
	cfg *config.Config,
	log *zap.Logger,
	overlayManager *overlay.Manager,
) (*components.ScrollComponent, error) {
	scrollOverlay, err := scroll.NewOverlayWithWindow(
		cfg.Scroll,
		log,
		overlayManager.GetWindowPtr(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create scroll overlay: %w", err)
	}

	return &components.ScrollComponent{
		Overlay: scrollOverlay,
		Context: &scroll.Context{},
	}, nil
}

// createActionComponent initializes the action component.
func createActionComponent(
	cfg *config.Config,
	log *zap.Logger,
	overlayManager *overlay.Manager,
) (*components.ActionComponent, error) {
	actionOverlay, err := action.NewOverlayWithWindow(
		cfg.Action,
		log,
		overlayManager.GetWindowPtr(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create action overlay: %w", err)
	}

	return &components.ActionComponent{
		Overlay: actionOverlay,
	}, nil
}
