package modes

import (
	"image"
	"sync"
	"time"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/app/services/modeindicator"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
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
	// mu serializes access to Handler state between the event tap callback thread
	// and timer goroutines (e.g., refreshHintsTimer). All public entry points
	// (HandleKeyPress, ActivateMode, ExitMode) and timer callbacks must hold this lock.
	mu sync.Mutex

	config         *config.Config
	logger         *zap.Logger
	appState       *state.AppState
	cursorState    *state.CursorState
	overlayManager overlay.ManagerInterface
	renderer       *ui.OverlayRenderer
	// New Services
	hintService          *services.HintService
	gridService          *services.GridService
	actionService        *services.ActionService
	scrollService        *services.ScrollService
	modeIndicatorService *modeindicator.Service

	hints         *components.HintsComponent
	grid          *components.GridComponent
	scroll        *components.ScrollComponent
	recursiveGrid *components.RecursiveGridComponent

	// Mode implementations
	modes map[domain.Mode]Mode

	// Screen bounds for coordinate conversion (grid and hints)
	screenBounds image.Rectangle

	enableEventTap    func()
	disableEventTap   func()
	refreshHotkeys    func()
	refreshHintsTimer *time.Timer

	// Scroll mode polling
	scrollTicker *time.Ticker
	scrollStopCh chan struct{}
	scrollDoneCh chan struct{}
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
	modeIndicatorService *modeindicator.Service,
	hintsComponent *components.HintsComponent,
	grid *components.GridComponent,
	scroll *components.ScrollComponent,
	recursiveGridComponent *components.RecursiveGridComponent,
	enableEventTap func(),
	disableEventTap func(),
	refreshHotkeys func(),
) *Handler {
	// Initialize screen bounds for coordinate conversion
	screenBounds := bridge.ActiveScreenBounds()

	handler := &Handler{
		config:               config,
		logger:               logger,
		appState:             appState,
		cursorState:          cursorState,
		overlayManager:       overlayManager,
		renderer:             renderer,
		hintService:          hintService,
		gridService:          gridService,
		actionService:        actionService,
		scrollService:        scrollService,
		modeIndicatorService: modeIndicatorService,
		hints:                hintsComponent,
		grid:                 grid,
		scroll:               scroll,
		recursiveGrid:        recursiveGridComponent,
		screenBounds:         screenBounds,
		enableEventTap:       enableEventTap,
		disableEventTap:      disableEventTap,
		refreshHotkeys:       refreshHotkeys,
	}

	// Initialize mode implementations
	handler.modes = map[domain.Mode]Mode{
		domain.ModeHints:         NewHintsMode(handler),
		domain.ModeGrid:          NewGridMode(handler),
		domain.ModeScroll:        NewScrollMode(handler),
		domain.ModeRecursiveGrid: NewRecursiveGridMode(handler),
	}

	return handler
}

// RefreshHintsForScreenChange updates the hint collection under the handler
// mutex so that the onUpdate callback can safely read h.screenBounds and
// write to h.overlayManager. Called from the screen-change goroutine in
// lifecycle.go.
func (h *Handler) RefreshHintsForScreenChange(hintCollection *domainHint.Collection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// Re-read screen bounds under the lock so the onUpdate callback
	// uses coordinates that match the resized overlay.
	h.screenBounds = bridge.ActiveScreenBounds()
	h.hints.Context.SetHints(hintCollection)
}

// UpdateConfig updates the handler with new configuration.
func (h *Handler) UpdateConfig(config *config.Config) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.config = config

	if h.renderer != nil {
		h.renderer.UpdateConfig(
			hints.BuildStyle(config.Hints),
			grid.BuildStyle(config.Grid),
			recursivegrid.BuildStyle(config.RecursiveGrid),
		)
	}
}
