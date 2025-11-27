package app

import (
	"strings"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	derrors "github.com/y3owk1n/neru/internal/errors"
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
		overlayManager.WindowPtr(),
	)
	if hintOverlayErr != nil {
		return nil, derrors.Wrap(
			hintOverlayErr,
			derrors.CodeOverlayFailed,
			"failed to create hint overlay",
		)
	}

	component.Overlay = hintOverlay

	return component, nil
}

// getGridCharacters returns configured grid characters with fallbacks.
func getGridCharacters(config *config.Config, logger *zap.Logger) string {
	gridChars := config.Grid.Characters
	if strings.TrimSpace(gridChars) == "" {
		gridChars = domain.DefaultHintCharacters
		logger.Warn("No grid characters configured, using default: " + domain.DefaultHintCharacters)
	}

	return gridChars
}

// getSublayerKeys returns configured sublayer keys with fallbacks.
func getSublayerKeys(config *config.Config, gridChars string, logger *zap.Logger) string {
	keys := strings.TrimSpace(config.Grid.SublayerKeys)
	if keys == "" {
		keys = gridChars
	}

	if keys == "" {
		keys = domain.DefaultHintCharacters
		logger.Warn("No subgrid keys configured, using default: " + domain.DefaultHintCharacters)
	}

	return keys
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

		ctx := &grid.Context{}
		ctx.SetGridInstance(&gridInstance)
		component.Context = ctx

		return component
	}

	gridChars := getGridCharacters(config, logger)
	component.Style = grid.BuildStyle(config.Grid)
	gridOverlay := grid.NewOverlayWithWindow(config.Grid, logger, overlayManager.WindowPtr())

	var gridInstance *domainGrid.Grid

	keys := getSublayerKeys(config, gridChars, logger)

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

			overlayManager.UpdateGridMatches(component.Manager.CurrentInput())
		},
		func(cell *domainGrid.Cell) {
			overlayManager.ShowSubgrid(cell, component.Style)
		},
		logger,
	)

	ctx := &grid.Context{}
	ctx.SetGridInstance(&gridInstance)
	ctx.SetGridOverlay(&gridOverlay)
	component.Context = ctx

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
		overlayManager.WindowPtr(),
	)
	if scrollOverlayErr != nil {
		return nil, derrors.Wrap(
			scrollOverlayErr,
			derrors.CodeOverlayFailed,
			"failed to create scroll overlay",
		)
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
		overlayManager.WindowPtr(),
	)
	if actionOverlayErr != nil {
		return nil, derrors.Wrap(
			actionOverlayErr,
			derrors.CodeOverlayFailed,
			"failed to create action overlay",
		)
	}

	return &components.ActionComponent{
		Overlay: actionOverlay,
	}, nil
}
