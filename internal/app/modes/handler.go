package modes

import (
	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/application/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ui"
	"github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

// Handler encapsulates mode-specific logic and dependencies.
type Handler struct {
	Config         *config.Config
	Logger         *zap.Logger
	State          *state.AppState
	Cursor         *state.CursorState
	OverlayManager overlay.ManagerInterface
	Renderer       *ui.OverlayRenderer
	// New Services
	HintService   *services.HintService
	GridService   *services.GridService
	ActionService *services.ActionService
	ScrollService *services.ScrollService

	Hints  *components.HintsComponent
	Grid   *components.GridComponent
	Scroll *components.ScrollComponent
	Action *components.ActionComponent

	// Callbacks to App
	EnableEventTap  func()
	DisableEventTap func()
	RefreshHotkeys  func()
}

// NewHandler creates a new mode handler.
func NewHandler(
	cfg *config.Config,
	log *zap.Logger,
	st *state.AppState,
	cursor *state.CursorState,
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
		Config:          cfg,
		Logger:          log,
		State:           st,
		Cursor:          cursor,
		OverlayManager:  overlayManager,
		Renderer:        renderer,
		HintService:     hintService,
		GridService:     gridService,
		ActionService:   actionService,
		ScrollService:   scrollService,
		Hints:           hintsComponent,
		Grid:            grid,
		Scroll:          scroll,
		Action:          action,
		EnableEventTap:  enableEventTap,
		DisableEventTap: disableEventTap,
		RefreshHotkeys:  refreshHotkeys,
	}
}

// UpdateConfig updates the handler with new configuration.
func (h *Handler) UpdateConfig(cfg *config.Config) {
	h.Config = cfg
}
