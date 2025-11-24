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
	config *config.Config,
	logger *zap.Logger,
	overlayManager OverlayManager,
) (*components.HintsComponent, error) {
	component := &components.HintsComponent{}

	// Only initialize component if hints are enabled
	if !config.Hints.Enabled {
		return component, nil
	}

	component.Style = hints.BuildStyle(config.Hints)
	component.Context = &hints.Context{}

	hintOverlay, hintOverlayErr := hints.NewOverlayWithWindow(
		config.Hints,
		logger,
		overlayManager.GetWindowPtr(),
	)
	if hintOverlayErr != nil {
		return nil, fmt.Errorf("failed to create hint overlay: %w", hintOverlayErr)
	}

	component.Overlay = hintOverlay

	return component, nil
}

// createGridComponent initializes the grid component based on configuration.
func createGridComponent(
	config *config.Config,
	logger *zap.Logger,
	overlayManager OverlayManager,
) *components.GridComponent {
	component := &components.GridComponent{}

	// Initialize minimal context even when disabled
	if !config.Grid.Enabled {
		var gridInstance *domainGrid.Grid

		component.Context = &grid.Context{
			GridInstance: &gridInstance,
		}

		return component
	}

	// Ensure grid characters are configured
	gridChars := config.Grid.Characters
	if strings.TrimSpace(gridChars) == "" {
		gridChars = domain.DefaultHintCharacters
		logger.Warn("No grid characters configured, using default: " + domain.DefaultHintCharacters)
	}

	component.Style = grid.BuildStyle(config.Grid)
	gridOverlay := grid.NewOverlayWithWindow(config.Grid, logger, overlayManager.GetWindowPtr())

	var gridInstance *domainGrid.Grid

	// Determine sublayer keys with fallback chain
	keys := strings.TrimSpace(config.Grid.SublayerKeys)
	if keys == "" {
		keys = gridChars
	}

	if keys == "" {
		keys = domain.DefaultHintCharacters
		logger.Warn("No subgrid keys configured, using default: " + domain.DefaultHintCharacters)
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
		logger,
	)

	component.Context = &grid.Context{
		GridInstance: &gridInstance,
		GridOverlay:  &gridOverlay,
	}

	return component
}

// createScrollComponent initializes the scroll component.
func createScrollComponent(
	config *config.Config,
	logger *zap.Logger,
	overlayManager OverlayManager,
) (*components.ScrollComponent, error) {
	scrollOverlay, scrollOverlayErr := scroll.NewOverlayWithWindow(
		config.Scroll,
		logger,
		overlayManager.GetWindowPtr(),
	)
	if scrollOverlayErr != nil {
		return nil, fmt.Errorf("failed to create scroll overlay: %w", scrollOverlayErr)
	}

	return &components.ScrollComponent{
		Overlay: scrollOverlay,
		Context: &scroll.Context{},
	}, nil
}

// createActionComponent initializes the action component.
func createActionComponent(
	config *config.Config,
	logger *zap.Logger,
	overlayManager OverlayManager,
) (*components.ActionComponent, error) {
	actionOverlay, actionOverlayErr := action.NewOverlayWithWindow(
		config.Action,
		logger,
		overlayManager.GetWindowPtr(),
	)
	if actionOverlayErr != nil {
		return nil, fmt.Errorf("failed to create action overlay: %w", actionOverlayErr)
	}

	return &components.ActionComponent{
		Overlay: actionOverlay,
	}, nil
}
