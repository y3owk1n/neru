package modes

import (
	"image"
	"time"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"github.com/y3owk1n/neru/internal/ui"
	"github.com/y3owk1n/neru/internal/ui/overlay"
	"go.uber.org/zap"
)

// Mode defines the interface that all navigation modes must implement.
// This provides a consistent API contract for mode activation, key handling,
// and cleanup operations.
type Mode interface {
	// Activate activates the mode with an optional pending action.
	Activate(action *string)

	// HandleKey processes a key press within the mode's context.
	HandleKey(key string)

	// Exit performs mode-specific cleanup and deactivation.
	Exit()

	// ModeType returns the domain mode type this implementation represents.
	ModeType() domain.Mode
}

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

	// Mode implementations
	modes map[domain.Mode]Mode

	// Screen bounds for coordinate conversion (grid and hints)
	screenBounds image.Rectangle

	enableEventTap    func()
	disableEventTap   func()
	refreshHotkeys    func()
	refreshHintsTimer *time.Timer
	refreshHintsCh    chan struct{} // Channel for dispatching timer callback to main thread
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
	enableEventTap func(),
	disableEventTap func(),
	refreshHotkeys func(),
) *Handler {
	// Initialize screen bounds for coordinate conversion
	screenBounds := bridge.ActiveScreenBounds()

	handler := &Handler{
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
		screenBounds:    screenBounds,
		enableEventTap:  enableEventTap,
		disableEventTap: disableEventTap,
		refreshHotkeys:  refreshHotkeys,
		refreshHintsCh:  make(chan struct{}, 1),
	}

	// Initialize mode implementations
	handler.modes = map[domain.Mode]Mode{
		domain.ModeHints:  NewHintsMode(handler),
		domain.ModeGrid:   NewGridMode(handler),
		domain.ModeScroll: NewScrollMode(handler),
	}

	return handler
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
