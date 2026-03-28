package modes

import (
	"context"
	"image"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/app/services/modeindicator"
	"github.com/y3owk1n/neru/internal/app/services/stickyindicator"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
	"github.com/y3owk1n/neru/internal/ui"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

// Mode defines the interface that all navigation modes must implement.
// This provides a consistent API contract for mode activation, key handling,
// and cleanup operations.
type Mode interface {
	// Activate activates the mode with an optional pending action.
	// When repeat is true the mode re-activates after performing the action
	// instead of exiting.
	Activate(opts ModeActivationOptions)

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

	config         *configpkg.Config
	themeProvider  configpkg.ThemeProvider
	system         ports.SystemPort
	logger         *zap.Logger
	appState       *state.AppState
	cursorState    *state.CursorState
	modifierState  *state.ModifierState
	overlayManager overlay.ManagerInterface
	renderer       *ui.OverlayRenderer
	// New Services
	hintService            *services.HintService
	gridService            *services.GridService
	actionService          *services.ActionService
	scrollService          *services.ScrollService
	modeIndicatorService   *modeindicator.Service
	stickyIndicatorService *stickyindicator.Service

	hints         *components.HintsComponent
	grid          *components.GridComponent
	scroll        *components.ScrollComponent
	recursiveGrid *components.RecursiveGridComponent

	// Mode implementations
	modes map[domain.Mode]Mode

	// Screen bounds for coordinate conversion (grid and hints)
	screenBounds image.Rectangle

	enableEventTap             func()
	disableEventTap            func()
	setModifierPassthrough     func(enabled bool, blacklist []string)
	setInterceptedModifierKeys func(keys []string)
	setPassthroughCallback     func(cb func())
	setStickyModifierToggle    func(enabled bool)
	postModifierEvent          func(modifier string, isDown bool)
	refreshHotkeys             func()
	executeHotkeyAction        func(key, actionStr string) error
	refreshHintsTimer          *time.Timer
	modeSession                uint64
	hotkeyLastKey              string
	hotkeyLastKeyTime          int64

	// Pending modifier key for tap detection (down/up without intervening keys)
	pendingModifierKeys    map[string]time.Time
	pendingModifierTimers  map[string]*time.Timer // debounce timers for delayed sticky toggle
	modifierDetectionArmed bool                   // true once all modifiers have been released after mode entry
	lastRegularKeyTime     time.Time              // timestamp of the last non-modifier key press
	debounceNotify         chan struct{}          // test-only: signaled when a debounce callback completes

	// Indicator polling (shared by all modes)
	indicatorTicker *time.Ticker
	indicatorStopCh chan struct{}
	indicatorDoneCh chan struct{}
}

// NewHandler creates a new mode handler.
func NewHandler(
	config *configpkg.Config,
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
	stickyIndicatorService *stickyindicator.Service,
	hintsComponent *components.HintsComponent,
	grid *components.GridComponent,
	scroll *components.ScrollComponent,
	recursiveGridComponent *components.RecursiveGridComponent,
	enableEventTap func(),
	disableEventTap func(),
	setModifierPassthrough func(enabled bool, blacklist []string),
	setInterceptedModifierKeys func(keys []string),
	setPassthroughCallback func(cb func()),
	setStickyModifierToggle func(enabled bool),
	postModifierEvent func(modifier string, isDown bool),
	refreshHotkeys func(),
	executeHotkeyAction func(key, actionStr string) error,
	systemPort ports.SystemPort,
) *Handler {
	// Initialize screen bounds for coordinate conversion.
	// Use a background context since this runs during startup.
	// CodeNotSupported is expected on non-darwin platforms and is silently ignored;
	// any other error is logged as a warning.
	var screenBounds image.Rectangle

	if systemPort != nil {
		var boundsErr error

		screenBounds, boundsErr = systemPort.ScreenBounds(context.Background())
		if boundsErr != nil && !derrors.IsNotSupported(boundsErr) {
			logger.Warn("Failed to get initial screen bounds", zap.Error(boundsErr))
		}
	}

	handler := &Handler{
		config:                     config,
		logger:                     logger,
		appState:                   appState,
		cursorState:                cursorState,
		modifierState:              state.NewModifierState(),
		overlayManager:             overlayManager,
		renderer:                   renderer,
		hintService:                hintService,
		gridService:                gridService,
		actionService:              actionService,
		scrollService:              scrollService,
		modeIndicatorService:       modeIndicatorService,
		stickyIndicatorService:     stickyIndicatorService,
		hints:                      hintsComponent,
		grid:                       grid,
		scroll:                     scroll,
		recursiveGrid:              recursiveGridComponent,
		screenBounds:               screenBounds,
		enableEventTap:             enableEventTap,
		disableEventTap:            disableEventTap,
		setModifierPassthrough:     setModifierPassthrough,
		setInterceptedModifierKeys: setInterceptedModifierKeys,
		setPassthroughCallback:     setPassthroughCallback,
		setStickyModifierToggle:    setStickyModifierToggle,
		postModifierEvent:          postModifierEvent,
		refreshHotkeys:             refreshHotkeys,
		executeHotkeyAction:        executeHotkeyAction,
		themeProvider:              systemPort,
		system:                     systemPort,
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
//
// Returns true if the refresh was performed, false if the mode was exited
// concurrently (TOCTOU guard).
func (h *Handler) RefreshHintsForScreenChange(hintCollection *domainHint.Collection) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Re-check mode under the lock to close the TOCTOU window between the
	// snapshot in processScreenChange and the actual work here.
	if h.appState.CurrentMode() != domain.ModeHints {
		h.logger.Debug("Skipping hint screen-change refresh: mode exited concurrently")

		return false
	}
	// Re-read screen bounds under the lock so the onUpdate callback
	// uses coordinates that match the resized overlay.
	if h.system != nil {
		b, err := h.system.ScreenBounds(context.Background())
		if err == nil {
			h.screenBounds = b
		} else if !derrors.IsNotSupported(err) {
			h.logger.Warn("Failed to refresh screen bounds after screen change", zap.Error(err))
		}
	}

	h.hints.Context.SetHints(hintCollection)

	return true
}

// RefreshGridForScreenChange regenerates the grid with updated screen bounds
// under the handler mutex. The user's current input is reset because old cell
// coordinates are invalid on the new screen. Called from the screen-change
// handler in lifecycle.go when ModeGrid is active.
//
// Returns true if the refresh was performed, false if the mode was exited
// concurrently (TOCTOU guard) or the draw failed.
func (h *Handler) RefreshGridForScreenChange() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Re-check mode under the lock to close the TOCTOU window between the
	// snapshot in processScreenChange and the actual work here.
	if h.appState.CurrentMode() != domain.ModeGrid {
		h.logger.Debug("Skipping grid screen-change refresh: mode exited concurrently")

		return false
	}

	// Regenerate the grid with updated screen bounds.
	// createGridInstance also updates h.screenBounds and sets the grid on the context.
	gridInstance := h.createGridInstance()

	currentInput := ""

	if h.grid.Manager != nil {
		// Sync the Manager's internal grid reference so subsequent key presses
		// use the new grid's geometry for cell matching (fixes stale-bounds bug).
		h.grid.Manager.UpdateGrid(gridInstance)

		// Reset input state because old cell coordinates/bounds are invalid on
		// the new screen, and any in-progress subgrid selection would reference
		// a stale cell.
		h.grid.Manager.Reset()
	}

	drawGridErr := h.renderer.DrawGrid(gridInstance, currentInput)
	if drawGridErr != nil {
		h.logger.Error("Failed to refresh grid after screen change", zap.Error(drawGridErr))

		return false
	}

	return true
}

// RefreshRecursiveGridForScreenChange remaps the recursive-grid manager's
// bounds to the new screen dimensions, preserving the user's current depth
// and selection progress. Called from the screen-change handler in
// lifecycle.go when ModeRecursiveGrid is active.
//
// Returns true if the refresh was performed, false if the mode was exited
// concurrently (TOCTOU guard — the caller snapshots the mode without holding
// h.mu, so a concurrent ExitMode could have transitioned to Idle by the time
// we acquire the lock here).
func (h *Handler) RefreshRecursiveGridForScreenChange() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Re-check mode under the lock to close the TOCTOU window between the
	// snapshot in processScreenChange and the actual work here.
	if h.appState.CurrentMode() != domain.ModeRecursiveGrid {
		h.logger.Debug("Skipping recursive-grid screen-change refresh: mode exited concurrently")

		return false
	}

	// Re-read screen bounds under the lock so the overlay uses coordinates
	// that match the resized window.
	if h.system != nil {
		b, err := h.system.ScreenBounds(context.Background())
		if err == nil {
			h.screenBounds = b
		} else if !derrors.IsNotSupported(err) {
			h.logger.Warn("Failed to refresh screen bounds for recursive grid", zap.Error(err))
		}
	}

	normalizedBounds := coordinates.NormalizeToLocalCoordinates(h.screenBounds)

	if h.recursiveGrid != nil && h.recursiveGrid.Manager != nil {
		// Proportionally remap all bounds (history + currentBounds) so the
		// user's zoomed-in region maps to the equivalent area on the new screen.
		h.recursiveGrid.Manager.CurrentGrid().RemapToNewBounds(normalizedBounds)
	} else {
		// No existing manager — fall back to full initialization.
		h.initializeRecursiveGridManager(normalizedBounds)
	}

	// Redraw the overlay with the remapped grid.
	h.updateRecursiveGridOverlay()

	return true
}

// RefreshHintsForThemeChange redraws the hints overlay with updated styles
// after a system theme change. Only performs the redraw if ModeHints is
// currently active.
//
// Returns true if a redraw was performed.
func (h *Handler) RefreshHintsForThemeChange() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() != domain.ModeHints {
		return false
	}

	hintCollection := h.hints.Context.Hints()
	if hintCollection == nil {
		return false
	}

	// Convert domain hints to overlay hints for rendering
	filteredHints := hintCollection.All()
	overlayHints := make([]*hints.Hint, len(filteredHints))
	screenBounds := h.screenBounds

	for index, hint := range filteredHints {
		// Convert screen-absolute coordinates to overlay-local coordinates
		localPos := image.Point{
			X: hint.Position().X - screenBounds.Min.X,
			Y: hint.Position().Y - screenBounds.Min.Y,
		}
		overlayHints[index] = hints.NewHint(
			hint.Label(),
			localPos,
			hint.Element().Bounds().Size(),
			hint.MatchedPrefix(),
		)
	}

	drawHintsErr := h.renderer.DrawHints(overlayHints)
	if drawHintsErr != nil {
		h.logger.Error("Failed to refresh hints after theme change", zap.Error(drawHintsErr))

		return false
	}

	return true
}

// RefreshGridForThemeChange redraws the grid overlay with updated styles
// after a system theme change. Only performs the redraw if ModeGrid is
// currently active.
//
// Returns true if a redraw was performed.
func (h *Handler) RefreshGridForThemeChange() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() != domain.ModeGrid {
		return false
	}

	gridInstancePtr := h.grid.Context.GridInstance()
	if gridInstancePtr == nil || *gridInstancePtr == nil {
		return false
	}

	gridInstance := *gridInstancePtr

	currentInput := ""
	if h.grid.Manager != nil {
		currentInput = h.grid.Manager.CurrentInput()
	}

	drawGridErr := h.renderer.DrawGrid(gridInstance, currentInput)
	if drawGridErr != nil {
		h.logger.Error("Failed to refresh grid after theme change", zap.Error(drawGridErr))

		return false
	}

	return true
}

// RefreshRecursiveGridForThemeChange redraws the recursive-grid overlay with
// updated styles after a system theme change. Only performs the redraw if
// ModeRecursiveGrid is currently active.
//
// Returns true if a redraw was performed.
func (h *Handler) RefreshRecursiveGridForThemeChange() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() != domain.ModeRecursiveGrid {
		return false
	}

	h.updateRecursiveGridOverlay()

	return true
}

// UpdateConfig updates the handler with new configuration.
func (h *Handler) UpdateConfig(config *configpkg.Config) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.config = config

	if h.renderer != nil {
		h.renderer.UpdateConfig(
			hints.BuildStyle(config.Hints, h.themeProvider),
			grid.BuildStyle(config.Grid, h.themeProvider),
			recursivegrid.BuildStyle(config.RecursiveGrid, h.themeProvider),
		)
	}

	h.syncModifierPassthrough(h.appState.CurrentMode())
}

// ResetCurrentMode resets current mode input state without exiting.
func (h *Handler) ResetCurrentMode() {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch h.appState.CurrentMode() {
	case domain.ModeGrid:
		if h.grid != nil && h.grid.Manager != nil {
			h.grid.Manager.Reset()

			gridInstancePtr := h.grid.Context.GridInstance()
			if gridInstancePtr != nil && *gridInstancePtr != nil {
				err := h.renderer.DrawGrid(
					*gridInstancePtr,
					h.grid.Manager.CurrentInput(),
				)
				if err != nil {
					h.logger.Error("Failed to redraw grid after reset", zap.Error(err))
				}
			}
		}
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid != nil && h.recursiveGrid.Manager != nil {
			h.recursiveGrid.Manager.Reset()
			h.updateRecursiveGridOverlay()

			center := h.recursiveGrid.Manager.CurrentCenter()

			absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(center, h.screenBounds)
			if h.recursiveGrid.Context != nil {
				h.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)

				if !h.recursiveGrid.Context.CursorFollowSelection() {
					return
				}
			}

			err := h.actionService.MoveCursorToPoint(
				context.Background(),
				absoluteCenter,
			)
			if err != nil {
				h.logger.Error("Failed to move cursor after recursive-grid reset", zap.Error(err))
			}
		}
	case domain.ModeIdle, domain.ModeHints, domain.ModeScroll:
		// no-op
	}
}

// BackspaceCurrentMode performs mode-aware backspace behavior without exiting.
func (h *Handler) BackspaceCurrentMode() {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch h.appState.CurrentMode() {
	case domain.ModeHints:
		if h.hints != nil && h.hints.Context != nil && h.hints.Context.Manager() != nil {
			h.hints.Context.Manager().HandleBackspace()
		}
	case domain.ModeGrid:
		if h.grid != nil && h.grid.Manager != nil {
			h.grid.Manager.HandleBackspace()
		}
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid != nil && h.recursiveGrid.Manager != nil &&
			h.recursiveGrid.Manager.Backtrack() {
			h.updateRecursiveGridOverlay()
			center := h.recursiveGrid.Manager.CurrentCenter()

			absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(center, h.screenBounds)

			if h.recursiveGrid.Context != nil {
				h.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)

				if !h.recursiveGrid.Context.CursorFollowSelection() {
					return
				}
			}

			err := h.actionService.MoveCursorToPoint(
				context.Background(),
				absoluteCenter,
			)
			if err != nil {
				h.logger.Error(
					"Failed to move cursor after recursive-grid backspace",
					zap.Error(err),
				)
			}
		}
	case domain.ModeIdle, domain.ModeScroll:
		// no-op
	}
}

func (h *Handler) focusedBundleID() string {
	if h.actionService == nil {
		return ""
	}

	bundleID, err := h.actionService.FocusedAppBundleID(context.Background())
	if err != nil {
		h.logger.Debug("Failed to get focused app bundle ID for mode hotkeys", zap.Error(err))

		return ""
	}

	return bundleID
}
