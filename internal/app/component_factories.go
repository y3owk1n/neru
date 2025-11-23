package app

import (
	"fmt"
	"strings"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/features/action"
	"github.com/y3owk1n/neru/internal/features/grid"
	"github.com/y3owk1n/neru/internal/features/hints"
	"github.com/y3owk1n/neru/internal/features/scroll"
	"go.uber.org/zap"
)

// createHintsComponent initializes the hints component based on configuration.
func createHintsComponent(
	cfg *config.Config,
	log *zap.Logger,
	overlayManager OverlayManager,
) (*components.HintsComponent, error) {
	component := &components.HintsComponent{}

	// Only initialize component if hints are enabled
	if !cfg.Hints.Enabled {
		return component, nil
	}

	component.Style = hints.BuildStyle(cfg.Hints)
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
	overlayManager OverlayManager,
) *components.GridComponent {
	component := &components.GridComponent{}

	// Initialize minimal context even when disabled
	if !cfg.Grid.Enabled {
		var gridInstance *domainGrid.Grid
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
	var gridInstance *domainGrid.Grid

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
	component.Manager = domainGrid.NewManager(
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
		func(cell *domainGrid.Cell) {
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
	overlayManager OverlayManager,
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
	overlayManager OverlayManager,
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
