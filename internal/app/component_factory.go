package app

import (
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/modeindicator"
	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/app/components/stickyindicator"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// ComponentFactory provides standardized component creation patterns.
type ComponentFactory struct {
	config         *config.Config
	logger         *zap.Logger
	overlayManager OverlayManager
	themeProvider  config.ThemeProvider
}

// NewComponentFactory creates a new component factory.
func NewComponentFactory(
	config *config.Config,
	logger *zap.Logger,
	overlayManager OverlayManager,
	themeProvider config.ThemeProvider,
) *ComponentFactory {
	return &ComponentFactory{
		config:         config,
		logger:         logger,
		overlayManager: overlayManager,
		themeProvider:  themeProvider,
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
	component := &components.HintsComponent{
		Theme: f.themeProvider,
	}

	// Check if component should be skipped
	if opts.SkipIfDisabled && !f.config.Hints.Enabled {
		return component, nil
	}

	// Build style
	component.Style = hints.BuildStyle(f.config.Hints, f.themeProvider)
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
		} else if hintOverlay != nil {
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
	component := &components.GridComponent{
		Theme: f.themeProvider,
	}

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
	component.Style = grid.BuildStyle(f.config.Grid, f.themeProvider)
	gridChars := f.getGridCharacters()
	subKeys := f.getSublayerKeys(gridChars)

	// Create overlay
	if opts.OverlayType != "" {
		overlay, err := f.createOverlay("grid", f.config.Grid)
		if err != nil {
			if opts.Required {
				return nil, derrors.Wrap(
					err,
					derrors.CodeOverlayFailed,
					"failed to create grid overlay",
				)
			}

			f.logger.Warn(
				"Failed to create grid overlay, continuing without overlay",
				zap.Error(err),
			)
		} else if overlay != nil {
			if gridOverlayTyped, ok := overlay.(*grid.Overlay); ok {
				component.Overlay = gridOverlayTyped
			} else {
				f.logger.Error("Unexpected overlay type for grid", zap.Any("overlay", overlay))
			}
		}
	}

	// Create grid manager with callbacks
	component.Manager = domainGrid.NewManager(
		nil,
		domain.SubgridRows,
		domain.SubgridCols,
		subKeys,
		func(_ bool) {
			instancePtr := ctx.GridInstance()
			if instancePtr == nil || *instancePtr == nil || (*instancePtr).Characters() == "" {
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
// This component now only owns scroll context and key mappings; the visual mode
// indicator overlay is managed separately.
func (f *ComponentFactory) CreateScrollComponent(
	opts ComponentCreationOptions,
) (*components.ScrollComponent, error) {
	_ = opts

	return &components.ScrollComponent{
		Context: &scroll.Context{},
	}, nil
}

// CreateModeIndicatorComponent creates the shared mode indicator overlay component.
func (f *ComponentFactory) CreateModeIndicatorComponent(
	opts ComponentCreationOptions,
) (*components.ModeIndicatorComponent, error) {
	var indicatorOverlay *modeindicator.Overlay
	if opts.OverlayType != "" {
		overlay, err := f.createOverlay("mode_indicator", f.config.ModeIndicator)
		if err != nil {
			if opts.Required {
				return nil, derrors.Wrap(
					err,
					derrors.CodeOverlayFailed,
					"failed to create mode indicator overlay",
				)
			}

			f.logger.Warn(
				"Failed to create mode indicator overlay, continuing without overlay",
				zap.Error(err),
			)
		} else if overlay != nil {
			if typed, ok := overlay.(*modeindicator.Overlay); ok {
				indicatorOverlay = typed
			} else {
				f.logger.Error(
					"Unexpected overlay type for mode indicator",
					zap.Any("overlay", overlay),
				)
			}
		}
	}

	return &components.ModeIndicatorComponent{
		Overlay: indicatorOverlay,
	}, nil
}

// CreateStickyIndicatorComponent creates the sticky modifiers indicator overlay component.
func (f *ComponentFactory) CreateStickyIndicatorComponent(
	opts ComponentCreationOptions,
) (*components.StickyIndicatorComponent, error) {
	var stickyOverlay *stickyindicator.Overlay
	if opts.OverlayType != "" {
		overlay, err := f.createOverlay("sticky_modifiers", f.config.StickyModifiers.UI)
		if err != nil {
			if opts.Required {
				return nil, derrors.Wrap(
					err,
					derrors.CodeOverlayFailed,
					"failed to create sticky indicator overlay",
				)
			}

			f.logger.Warn(
				"Failed to create sticky indicator overlay, continuing without overlay",
				zap.Error(err),
			)
		} else if overlay != nil {
			if typed, ok := overlay.(*stickyindicator.Overlay); ok {
				stickyOverlay = typed
			} else {
				f.logger.Error(
					"Unexpected overlay type for sticky indicator",
					zap.Any("overlay", overlay),
				)
			}
		}
	}

	return &components.StickyIndicatorComponent{
		Overlay: stickyOverlay,
	}, nil
}

// CreateRecursiveGridComponent creates a recursive-grid component with standardized error handling.
func (f *ComponentFactory) CreateRecursiveGridComponent(
	opts ComponentCreationOptions,
) (*components.RecursiveGridComponent, error) {
	// Create overlay
	var recursiveGridOverlay *recursivegrid.Overlay
	if opts.OverlayType != "" {
		overlay, err := f.createOverlay("recursive_grid", f.config.RecursiveGrid)
		if err != nil {
			if opts.Required {
				return nil, derrors.Wrap(
					err,
					derrors.CodeOverlayFailed,
					"failed to create recursive_grid overlay",
				)
			}

			f.logger.Warn(
				"Failed to create recursive_grid overlay, continuing without overlay",
				zap.Error(err),
			)
		} else if overlay != nil {
			if recursiveGridOverlayTyped, ok := overlay.(*recursivegrid.Overlay); ok {
				recursiveGridOverlay = recursiveGridOverlayTyped
			} else {
				f.logger.Error(
					"Unexpected overlay type for recursive_grid",
					zap.Any("overlay", overlay),
				)
			}
		}
	}

	return &components.RecursiveGridComponent{
		Overlay: recursiveGridOverlay,
		Context: &recursivegrid.Context{},
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

		// When no real overlay window exists (e.g. in tests with a no-op overlay
		// manager), return nil rather than creating an overlay with a nil C window
		// handle, which would crash on any CGo call.
		if f.overlayManager.WindowPtr() == nil {
			return nil, nil //nolint:nilnil
		}

		return hints.NewOverlayWithWindow(hintsConfig, f.logger, f.overlayManager.WindowPtr())
	case "grid":
		gridConfig, ok := cfg.(config.GridConfig)
		if !ok {
			return nil, derrors.New(derrors.CodeInvalidInput, "invalid grid config type")
		}

		if f.overlayManager.WindowPtr() == nil {
			return nil, nil //nolint:nilnil
		}

		return grid.NewOverlayWithWindow(gridConfig, f.logger, f.overlayManager.WindowPtr()), nil
	case "mode_indicator":
		indicatorConfig, ok := cfg.(config.ModeIndicatorConfig)
		if !ok {
			return nil, derrors.New(derrors.CodeInvalidInput, "invalid mode indicator config type")
		}

		// Mode indicator creates its own dedicated window (not the shared manager
		// window) so it doesn't conflict with hints/grid content. The nil-window
		// check here serves as a proxy for detecting headless/test environments
		// where no native windows should be created at all.
		if f.overlayManager.WindowPtr() == nil {
			return nil, nil //nolint:nilnil
		}

		return modeindicator.NewOverlay(
			indicatorConfig,
			f.themeProvider,
			f.logger,
		)
	case "recursive_grid":
		recursiveGridConfig, ok := cfg.(config.RecursiveGridConfig)
		if !ok {
			return nil, derrors.New(derrors.CodeInvalidInput, "invalid recursive_grid config type")
		}

		if f.overlayManager.WindowPtr() == nil {
			return nil, nil //nolint:nilnil
		}

		return recursivegrid.NewOverlayWithWindow(
			recursiveGridConfig,
			f.logger,
			f.overlayManager.WindowPtr(),
		), nil
	case "sticky_modifiers":
		uiConfig, ok := cfg.(config.StickyModifiersUI)
		if !ok {
			return nil, derrors.New(
				derrors.CodeInvalidInput,
				"invalid sticky modifiers config type",
			)
		}

		if f.overlayManager.WindowPtr() == nil {
			return nil, nil //nolint:nilnil
		}

		return stickyindicator.NewOverlay(
			uiConfig,
			f.themeProvider,
			f.logger,
		)
	default:
		return nil, derrors.New(derrors.CodeInvalidInput, "unknown overlay type: "+overlayType)
	}
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
