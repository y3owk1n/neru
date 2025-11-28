package modes

import (
	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/ui"
	"github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

// Handler encapsulates mode-specific logic and dependencies.
type Handler struct {
	config         *config.Config
	logger         *zap.Logger
	appState       *state.AppState
	cursorState    *state.CursorState
	overlayManager overlay.ManagerInterface
	renderer       *ui.OverlayRenderer
	// New Services
	hintService   *services.HintService
	gridService   *services.GridService
	actionService *services.ActionService
	scrollService *services.ScrollService

	hints  *components.HintsComponent
	grid   *components.GridComponent
	scroll *components.ScrollComponent
	action *components.ActionComponent

	enableEventTap  func()
	disableEventTap func()
	refreshHotkeys  func()
}

// NewHandler creates a new mode handler.
func NewHandler(
	config *config.Config,
	logger *zap.Logger,
	appState *state.AppState,
	cursorState *state.CursorState,
	overlayManager overlay.ManagerInterface,
	renderer *ui.OverlayRenderer,
	hintService *services.HintService,
	gridService *services.GridService,
	actionService *services.ActionService,
	scrollService *services.ScrollService,
	hintsComponent *components.HintsComponent,
	grid *components.GridComponent,
	scroll *components.ScrollComponent,
	action *components.ActionComponent,
	enableEventTap func(),
	disableEventTap func(),
	refreshHotkeys func(),
) *Handler {
	return &Handler{
		config:          config,
		logger:          logger,
		appState:        appState,
		cursorState:     cursorState,
		overlayManager:  overlayManager,
		renderer:        renderer,
		hintService:     hintService,
		gridService:     gridService,
		actionService:   actionService,
		scrollService:   scrollService,
		hints:           hintsComponent,
		grid:            grid,
		scroll:          scroll,
		action:          action,
		enableEventTap:  enableEventTap,
		disableEventTap: disableEventTap,
		refreshHotkeys:  refreshHotkeys,
	}
}

// UpdateConfig updates the handler with new configuration.
func (h *Handler) UpdateConfig(config *config.Config) {
	h.config = config
	if h.renderer != nil {
		h.renderer.UpdateConfig(
			hints.BuildStyle(config.Hints),
			grid.BuildStyle(config.Grid),
		)
	}
}
