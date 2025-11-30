package app

import (
	"strings"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/components/action"
	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"go.uber.org/zap"
)

// ComponentFactory provides standardized component creation patterns.
type ComponentFactory struct {
	config         *config.Config
	logger         *zap.Logger
	overlayManager OverlayManager
}

// NewComponentFactory creates a new component factory.
func NewComponentFactory(
	config *config.Config,
	logger *zap.Logger,
	overlayManager OverlayManager,
) *ComponentFactory {
	return &ComponentFactory{
		config:         config,
		logger:         logger,
		overlayManager: overlayManager,
	}
}

// ComponentCreationOptions defines options for component creation.
type ComponentCreationOptions struct {
	SkipIfDisabled bool   // Skip creation if component is disabled in config
	Required       bool   // Component is required (return error if creation fails)
	OverlayType    string // Type of overlay to create ("hints", "grid", "action", "scroll")
}

// CreateHintsComponent creates a hints component with standardized error handling.
func (f *ComponentFactory) CreateHintsComponent(
	opts ComponentCreationOptions,
) (*components.HintsComponent, error) {
	component := &components.HintsComponent{}

	// Check if component should be skipped
	if opts.SkipIfDisabled && !f.config.Hints.Enabled {
		return component, nil
	}

	// Build style
	component.Style = hints.BuildStyle(f.config.Hints)
	component.Context = &hints.Context{}

	// Create overlay
	if opts.OverlayType != "" {
		hintOverlay, err := f.createOverlay("hints", f.config.Hints)
		if err != nil {
			if opts.Required {
				return nil, derrors.Wrap(
					err,
					derrors.CodeOverlayFailed,
					"failed to create hints overlay",
				)
			}

			f.logger.Warn(
				"Failed to create hints overlay, continuing without overlay",
				zap.Error(err),
			)
		} else {
			if overlay, ok := hintOverlay.(*hints.Overlay); ok {
				component.Overlay = overlay
			} else {
				f.logger.Error("Unexpected overlay type for hints", zap.Any("overlay", hintOverlay))
			}
		}
	}

	return component, nil
}

// CreateGridComponent creates a grid component with standardized error handling.
func (f *ComponentFactory) CreateGridComponent(
	opts ComponentCreationOptions,
) (*components.GridComponent, error) {
	component := &components.GridComponent{}

	// Initialize minimal context even when disabled
	ctx := &grid.Context{}

	var gridInstance *domainGrid.Grid
	ctx.SetGridInstance(&gridInstance)
	component.Context = ctx

	// Check if component should be skipped
	if opts.SkipIfDisabled && !f.config.Grid.Enabled {
		return component, nil
	}

	// Build style and configuration
	component.Style = grid.BuildStyle(f.config.Grid)
	gridChars := f.getGridCharacters()
	subKeys := f.getSublayerKeys(gridChars)

	// Create overlay
	if opts.OverlayType != "" {
		gridOverlay := f.createGridOverlay()
		ctx.SetGridOverlay(&gridOverlay)
	}

	// Create grid manager with callbacks
	component.Manager = domainGrid.NewManager(
		nil,
		domain.SubgridRows,
		domain.SubgridCols,
		subKeys,
		func(_ bool) {
			if gridInstance == nil || (*gridInstance).Characters() == "" {
				return
			}

			f.overlayManager.UpdateGridMatches(component.Manager.CurrentInput())
		},
		func(cell *domainGrid.Cell) {
			f.overlayManager.ShowSubgrid(cell, component.Style)
		},
		f.logger,
	)

	return component, nil
}

// CreateScrollComponent creates a scroll component with standardized error handling.
func (f *ComponentFactory) CreateScrollComponent(
	opts ComponentCreationOptions,
) (*components.ScrollComponent, error) {
	// Create overlay
	var scrollOverlay *scroll.Overlay
	if opts.OverlayType != "" {
		overlay, err := f.createOverlay("scroll", f.config.Scroll)
		if err != nil {
			if opts.Required {
				return nil, derrors.Wrap(
					err,
					derrors.CodeOverlayFailed,
					"failed to create scroll overlay",
				)
			}

			f.logger.Warn(
				"Failed to create scroll overlay, continuing without overlay",
				zap.Error(err),
			)
		} else {
			if scrollOverlayTyped, ok := overlay.(*scroll.Overlay); ok {
				scrollOverlay = scrollOverlayTyped
			} else {
				f.logger.Error("Unexpected overlay type for scroll", zap.Any("overlay", overlay))
			}
		}
	}

	return &components.ScrollComponent{
		Overlay: scrollOverlay,
		Context: &scroll.Context{},
	}, nil
}

// CreateActionComponent creates an action component with standardized error handling.
func (f *ComponentFactory) CreateActionComponent(
	opts ComponentCreationOptions,
) (*components.ActionComponent, error) {
	// Create overlay
	var actionOverlay *action.Overlay
	if opts.OverlayType != "" {
		overlay, err := f.createOverlay("action", f.config.Action)
		if err != nil {
			if opts.Required {
				return nil, derrors.Wrap(
					err,
					derrors.CodeOverlayFailed,
					"failed to create action overlay",
				)
			}

			f.logger.Warn(
				"Failed to create action overlay, continuing without overlay",
				zap.Error(err),
			)
		} else {
			if actionOverlayTyped, ok := overlay.(*action.Overlay); ok {
				actionOverlay = actionOverlayTyped
			} else {
				f.logger.Error("Unexpected overlay type for action", zap.Any("overlay", overlay))
			}
		}
	}

	return &components.ActionComponent{
		Overlay: actionOverlay,
	}, nil
}

// Helper methods

func (f *ComponentFactory) createOverlay(overlayType string, cfg any) (any, error) {
	switch overlayType {
	case "hints":
		hintsConfig, ok := cfg.(config.HintsConfig)
		if !ok {
			return nil, derrors.New(derrors.CodeInvalidInput, "invalid hints config type")
		}

		return hints.NewOverlayWithWindow(hintsConfig, f.logger, f.overlayManager.WindowPtr())
	case "grid":
		return f.createGridOverlay(), nil
	case "action":
		actionConfig, ok := cfg.(config.ActionConfig)
		if !ok {
			return nil, derrors.New(derrors.CodeInvalidInput, "invalid action config type")
		}

		return action.NewOverlayWithWindow(actionConfig, f.logger, f.overlayManager.WindowPtr())
	case "scroll":
		scrollConfig, ok := cfg.(config.ScrollConfig)
		if !ok {
			return nil, derrors.New(derrors.CodeInvalidInput, "invalid scroll config type")
		}

		return scroll.NewOverlayWithWindow(scrollConfig, f.logger, f.overlayManager.WindowPtr())
	default:
		return nil, derrors.New(derrors.CodeInvalidInput, "unknown overlay type: "+overlayType)
	}
}

func (f *ComponentFactory) createGridOverlay() *grid.Overlay {
	return grid.NewOverlayWithWindow(f.config.Grid, f.logger, f.overlayManager.WindowPtr())
}

func (f *ComponentFactory) getGridCharacters() string {
	gridChars := f.config.Grid.Characters
	if strings.TrimSpace(gridChars) == "" {
		gridChars = domain.DefaultHintCharacters
		f.logger.Warn(
			"No grid characters configured, using default: " + domain.DefaultHintCharacters,
		)
	}

	return gridChars
}

func (f *ComponentFactory) getSublayerKeys(gridChars string) string {
	keys := strings.TrimSpace(f.config.Grid.SublayerKeys)
	if keys == "" {
		keys = gridChars
	}

	if keys == "" {
		keys = domain.DefaultHintCharacters
		f.logger.Warn("No subgrid keys configured, using default: " + domain.DefaultHintCharacters)
	}

	return keys
}
