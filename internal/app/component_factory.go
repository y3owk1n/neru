package app

import (
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

// ComponentFactory assembles the per-mode components: the domain state a mode
// keeps and the callbacks it drives the overlay through.
//
// It builds no overlay of its own. The render components are constructed by
// the overlay, on the surface only the overlay knows about, and arrive here
// already built.
type ComponentFactory struct {
	config *config.Config
	logger *zap.Logger
	// overlayPort is how the callbacks this factory builds reach the screen.
	// They are grid's incremental updates, which stay plain calls by ADR 0003.
	overlayPort ports.OverlayPort
	// rendered are the overlay's own render components. A nil entry is an
	// overlay this session will not draw.
	rendered overlay.Components
}

// NewComponentFactory creates a new component factory.
func NewComponentFactory(
	config *config.Config,
	logger *zap.Logger,
	overlayPort ports.OverlayPort,
	rendered overlay.Components,
) *ComponentFactory {
	return &ComponentFactory{
		config:      config,
		logger:      logger,
		overlayPort: overlayPort,
		rendered:    rendered,
	}
}

// CreateHintsComponent creates the hints component.
//
// Hints are the one mode whose component is left empty when the mode is
// disabled: nothing reads its context, and the overlay builds no hints overlay
// either.
func (f *ComponentFactory) CreateHintsComponent() *components.HintsComponent {
	if !f.config.Hints.Enabled {
		return &components.HintsComponent{}
	}

	return &components.HintsComponent{
		Overlay: f.rendered.Hints,
		Context: &hints.Context{},
	}
}

// CreateGridComponent creates the grid component. It is built in full even
// when grid mode is disabled, because a config reload enables the mode without
// rebuilding the component.
func (f *ComponentFactory) CreateGridComponent() *components.GridComponent {
	ctx := &grid.Context{}

	var gridInstance *domainGrid.Grid

	ctx.SetGridInstance(&gridInstance)

	component := &components.GridComponent{
		Overlay: f.rendered.Grid,
		Context: ctx,
	}

	gridChars := f.getGridCharacters()
	subKeys := f.getSublayerKeys(gridChars)

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

			f.overlayPort.UpdateGridMatches(component.Manager.CurrentInput())
		},
		func(cell *domainGrid.Cell) {
			f.overlayPort.ShowGridSubgrid(cell)
		},
		f.logger,
	)

	return component
}

// CreateScrollComponent creates the scroll component. It owns scroll context
// and key mappings only; the visual mode indicator is an overlay of its own.
func (f *ComponentFactory) CreateScrollComponent() *components.ScrollComponent {
	return &components.ScrollComponent{
		Context: &scroll.Context{},
	}
}

// CreateModeIndicatorComponent creates the shared mode indicator component.
func (f *ComponentFactory) CreateModeIndicatorComponent() *components.ModeIndicatorComponent {
	return &components.ModeIndicatorComponent{
		Overlay: f.rendered.ModeIndicator,
	}
}

// CreateStickyIndicatorComponent creates the sticky modifiers indicator component.
func (f *ComponentFactory) CreateStickyIndicatorComponent() *components.StickyIndicatorComponent {
	return &components.StickyIndicatorComponent{
		Overlay: f.rendered.StickyModifiers,
	}
}

// CreateRecursiveGridComponent creates the recursive-grid component.
func (f *ComponentFactory) CreateRecursiveGridComponent() *components.RecursiveGridComponent {
	return &components.RecursiveGridComponent{
		Overlay: f.rendered.RecursiveGrid,
		Context: &recursivegrid.Context{},
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
