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
	"github.com/y3owk1n/neru/internal/core/domain/action"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/axobserver"
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
	monitorSelect *monitorSelectSession

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
	shutdown                   func()
	refreshHintsTimer          *time.Timer
	modeSession                uint64
	hotkeyLastKey              string
	hotkeyLastKeyTime          int64

	textInput                  ports.TextInputPort
	hintSearchTextInputActive  bool
	hintSearchEventTapDisabled bool

	// Pending modifier taps waiting to be committed after a short "no follow-up"
	// window. A regular key press cancels all pending taps.
	pendingModifierKeys   map[action.Modifiers]time.Time
	pendingModifierTimers map[action.Modifiers]*time.Timer
	heldModifiers         action.Modifiers
	usedInChordModifiers  action.Modifiers
	suppressedModifiers   action.Modifiers
	suppressedUntil       time.Time
	modifierFreshPress    map[action.Modifiers]bool
	debounceNotify        chan struct{} // test-only: signaled when a debounce callback completes

	// moveMonitorMu serializes MoveMonitor invocations. Lock ordering is
	// always moveMonitorMu -> h.mu (MoveMonitor holds this while calling
	// refreshActiveModeOnNewScreen, which acquires h.mu via the
	// Refresh*ForScreenChange helpers). Never acquire in the reverse order.
	moveMonitorMu sync.Mutex

	// Indicator polling (shared by all modes)
	indicatorTicker *time.Ticker
	indicatorStopCh chan struct{}
	indicatorDoneCh chan struct{}

	// systemCursorHidden tracks whether hide_cursor (or hints virtual pointer) is active.
	systemCursorHidden bool

	// lastCursorRehideTime records the last time RehideSystemCursor was called
	// to avoid excessive re-hide calls in the polling loop.
	lastCursorRehideTime time.Time

	// Cycle hint state
	cycleHintIndex int

	// Auto-refresh: observerMgr arms AX observers on the focused app while a
	// hints session runs with hints.auto_refresh enabled. An observed change
	// feeds the debounce state below, which coalesces a burst of changes into a
	// single-flight refresh (leading edge fires immediately, mid-burst changes
	// collapse into one trailing scan, bounded by autoRefreshMaxWait).
	//
	// The debounce state is guarded by the leaf autoRefreshMu, which is never
	// held while acquiring the mode lock (h.mu); the observer callback takes only
	// autoRefreshMu, so it can never deadlock a teardown that holds h.mu while
	// joining the observer thread. hintRefreshFiring marks the debounced fire so
	// a transient empty scan keeps the session alive and the debounce gate is
	// skipped on re-entry; it is guarded by h.mu, not autoRefreshMu.
	observerMgr *axobserver.Manager

	autoRefreshMu          sync.Mutex
	autoRefreshTimer       *time.Timer
	autoRefreshBurstOpen   bool
	autoRefreshScanPending bool
	autoRefreshBurstStart  time.Time
	autoRefreshDebounce    time.Duration
	autoRefreshMaxWait     time.Duration
	autoRefreshOnFire      func() // test seam; nil in production
	hintRefreshFiring      bool

	// Settle backoff: after an observer-driven scan, keep re-scanning at a
	// widening interval until the hint set stops changing, so web content that
	// renders with no AX notification is still caught. The interval resets dense
	// whenever the set changes; the scan-count and window ceilings do not, so a
	// continuously-changing page still winds down. Guarded by autoRefreshMu, except
	// lastAppliedFingerprint which is only touched on the scan path under h.mu +
	// autoRefreshMu. See auto_refresh.go.
	autoRefreshSettling    bool
	settleInterval         time.Duration
	settleStableAtCap      int
	settleScanCount        int
	settleStart            time.Time
	lastAppliedFingerprint uint64

	// Base context for Handler methods. Injected by the App via NewHandler so
	// all Handler operations observe app-level cancellation.
	ctx context.Context //nolint:containedctx

	// heldRepeatingKey tracks which key is currently held for custom repeat.
	// When non-empty, macOS native key-down events for this key are suppressed
	// and a custom goroutine drives the repeat at heldRepeatInterval.
	heldRepeatingKey    string
	heldRepeatingCancel context.CancelFunc
}

// NewHandler creates a new mode handler.
func NewHandler(
	ctx context.Context,
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
	shutdown func(),
	textInput ports.TextInputPort,
	systemPort ports.SystemPort,
) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}

	logger = logger.Named("modes")

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
		ctx:                        ctx,
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
		shutdown:                   shutdown,
		textInput:                  textInput,
		themeProvider:              systemPort,
		system:                     systemPort,
		cycleHintIndex:             -1,
	}

	// Initialize mode implementations
	handler.modes = map[domain.Mode]Mode{
		domain.ModeHints:         NewHintsMode(handler),
		domain.ModeGrid:          NewGridMode(handler),
		domain.ModeScroll:        NewScrollMode(handler),
		domain.ModeRecursiveGrid: NewRecursiveGridMode(handler),
		domain.ModeMonitorSelect: NewMonitorSelectMode(handler),
	}

	// Auto-refresh observer plumbing. An observed UI change notifies the debounce
	// state, which coalesces a burst into a single-flight refresh. Nothing is
	// armed until a hints session runs with hints.auto_refresh enabled.
	handler.autoRefreshDebounce = defaultAutoRefreshDebounce
	handler.autoRefreshMaxWait = defaultAutoRefreshDebounce * autoRefreshMaxWaitFactor
	handler.observerMgr = axobserver.New(func(_ int) {
		handler.onObserverChange()
	}, handler.logger)

	return handler
}

// RefreshHintsForScreenChange updates the hint collection under the handler
// mutex so that the onUpdate callback can safely read h.screenBounds and
// write to h.overlayManager. Called from the screen-change goroutine in
// lifecycle.go.
//
// Returns true if the refresh was performed, false if the mode was exited
// concurrently (TOCTOU guard).
func (h *Handler) RefreshHintsForScreenChange(
	ctx context.Context,
	hintService *services.HintService,
) bool {
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
		b, err := h.system.ScreenBounds(ctx)
		if err == nil {
			h.screenBounds = b
		} else if !derrors.IsNotSupported(err) {
			h.logger.Warn("Failed to refresh screen bounds after screen change", zap.Error(err))
		}
	}

	// Escape any active IME search session before refreshing hints on the new
	// screen. The old IME session is bound to the previous screen and loses
	// focus during the space transition, causing subsequent keystrokes to be
	// forwarded to the frontmost app instead.
	if h.hints != nil && h.hints.Context != nil && h.hints.Context.SearchActive() {
		h.cancelHintSearch()
	}

	// Get current filter options from context
	filterRoles := h.hints.Context.FilterRoles()
	filterTextContains := h.hints.Context.FilterTextContains()
	strategyOverride := h.hints.Context.StrategyOverride()
	labelDirectionOverride := h.hints.Context.LabelDirectionOverride()

	// Generate hints with filters preserved; SetHints below performs the
	// single redraw after active-screen filtering.
	splitWordOverride := false
	if h.hints != nil && h.hints.Context != nil {
		splitWordOverride = h.hints.Context.SplitWord()
	}

	domainHints, showHintsErr := hintService.GenerateHints(
		ctx,
		filterRoles,
		filterTextContains,
		"",
		strategyOverride,
		labelDirectionOverride,
		splitWordOverride,
	)
	if showHintsErr != nil {
		h.logger.Error("Failed to refresh hints after screen change", zap.Error(showHintsErr))
		h.exitModeLocked()

		return false
	}

	if len(domainHints) == 0 {
		h.logger.Debug("No hints after screen change refresh")
		h.exitModeLocked()

		return false
	}

	allHints := domainHints

	filtered := filterHintsForScreen(allHints, h.screenBounds)
	if len(filtered) == 0 {
		h.logger.Debug("No hints on active screen after filter; skipping refresh")
		h.exitModeLocked()

		return false
	}

	setHintsErr := h.hints.Context.SetHints(
		domainHint.NewCollection(filtered),
	)
	if setHintsErr != nil {
		h.logger.Error("Failed to refresh hints for screen change", zap.Error(setHintsErr))

		return false
	}

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

	// Clear stale selection — old coordinates are invalid on the new screen.
	h.grid.Context.ClearSelectionPoint()

	drawGridErr := h.renderer.DrawGrid(gridInstance, currentInput)
	if drawGridErr != nil {
		h.logger.Error("Failed to refresh grid after screen change", zap.Error(drawGridErr))

		return false
	}

	// Ensure the virtual pointer is hidden (DrawGrid may clear cursorIndicatorVisible
	// via NeruClearOverlay, but we explicitly hide it for consistency).
	h.refreshGridVirtualPointerLocked()

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
		b, err := h.system.ScreenBounds(h.ctx)
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

	// Clear stale selection — old coordinates are invalid on the new screen.
	if h.recursiveGrid != nil && h.recursiveGrid.Context != nil {
		h.recursiveGrid.Context.ClearSelectionPoint()
	}

	// Redraw the overlay with the remapped grid.
	h.updateRecursiveGridOverlay()
	h.refreshRecursiveGridVirtualPointerLocked()

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

	drawHintsErr := h.overlayManager.DrawHintsWithStyle(
		overlayHints,
		h.currentHintStyleLocked(),
	)
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

	h.refreshGridVirtualPointerLocked()

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
	h.refreshRecursiveGridVirtualPointerLocked()

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

			// Clear stale selection — input was reset so no cell is selected.
			h.grid.Context.ClearSelectionPoint()

			gridInstancePtr := h.grid.Context.GridInstance()
			if gridInstancePtr != nil && *gridInstancePtr != nil {
				err := h.renderer.DrawGrid(
					*gridInstancePtr,
					h.grid.Manager.CurrentInput(),
				)
				if err != nil {
					h.logger.Error("Failed to redraw grid after reset", zap.Error(err))
				}

				h.refreshGridVirtualPointerLocked()
			}
		}
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid != nil && h.recursiveGrid.Manager != nil {
			h.recursiveGrid.Manager.Reset()

			center := h.recursiveGrid.Manager.CurrentCenter()

			absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(center, h.screenBounds)
			if h.recursiveGrid.Context != nil {
				h.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)
			}

			h.updateRecursiveGridOverlay()

			if h.recursiveGrid.Context != nil {
				if !h.recursiveGrid.Context.CursorFollowSelection() {
					h.refreshRecursiveGridVirtualPointerLocked()

					return
				}
			}

			err := h.actionService.MoveCursorToPoint(
				h.ctx,
				absoluteCenter,
			)
			if err != nil {
				h.logger.Error("Failed to move cursor after recursive-grid reset", zap.Error(err))
			}
		}
	case domain.ModeMonitorSelect:
		if h.monitorSelect != nil {
			h.monitorSelect.input = ""
			h.monitorSelect.selectedIndex = 0
			h.redrawMonitorSelectLocked()
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
			backspaceErr := h.hints.Context.Manager().HandleBackspace()
			if backspaceErr != nil {
				h.logger.Error("Hint backspace failed", zap.Error(backspaceErr))
			}
		}

		h.cycleHintIndex = -1
	case domain.ModeGrid:
		if h.grid != nil && h.grid.Manager != nil {
			h.grid.Manager.HandleBackspace()
		}
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid != nil && h.recursiveGrid.Manager != nil &&
			h.recursiveGrid.Manager.Backtrack() {
			center := h.recursiveGrid.Manager.CurrentCenter()

			absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(center, h.screenBounds)

			if h.recursiveGrid.Context != nil {
				h.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)
			}

			h.updateRecursiveGridOverlay()

			if h.recursiveGrid.Context != nil {
				if !h.recursiveGrid.Context.CursorFollowSelection() {
					h.refreshRecursiveGridVirtualPointerLocked()

					return
				}
			}

			err := h.actionService.MoveCursorToPoint(
				h.ctx,
				absoluteCenter,
			)
			if err != nil {
				h.logger.Error(
					"Failed to move cursor after recursive-grid backspace",
					zap.Error(err),
				)
			}
		}
	case domain.ModeMonitorSelect:
		if h.monitorSelect != nil {
			h.monitorSelect.Backspace()
			h.redrawMonitorSelectLocked()
		}
	case domain.ModeIdle, domain.ModeScroll:
		// no-op
	}
}

// StartHintSearch activates text filtering for hints mode.
func (h *Handler) StartHintSearch() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.startHintSearchLocked()
}

// CycleHint cycles through visible hints in hints mode, selecting the next or previous one.
// When executeAction is true, any pending action is performed on the selected hint
// (used by search confirmation). When false, only the cursor moves (used by the
// cycle_hint IPC action so users can browse results without triggering clicks).
func (h *Handler) CycleHint(ctx context.Context, backward bool, executeAction bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() != domain.ModeHints {
		return derrors.New(derrors.CodeInvalidInput, "cycle_hint requires hints mode")
	}

	if h.hints == nil || h.hints.Context == nil {
		return derrors.New(derrors.CodeActionFailed, "hints component not available")
	}

	manager := h.hints.Context.Manager()
	if manager == nil {
		return derrors.New(derrors.CodeActionFailed, "hints manager not available")
	}

	filteredHints := manager.FilteredHints()
	if len(filteredHints) == 0 {
		filteredHints = h.hints.Context.Hints().All()
	}

	if len(filteredHints) == 0 {
		return derrors.New(derrors.CodeActionFailed, "no hints available")
	}

	if h.cycleHintIndex >= len(filteredHints) {
		h.cycleHintIndex = len(filteredHints) - 1
	}

	switch {
	case h.cycleHintIndex < 0:
		h.cycleHintIndex = 0
		if backward {
			h.cycleHintIndex = len(filteredHints) - 1
		}
	default:
		if backward {
			if h.cycleHintIndex > 0 {
				h.cycleHintIndex--
			} else {
				h.cycleHintIndex = len(filteredHints) - 1
			}
		} else {
			if h.cycleHintIndex < len(filteredHints)-1 {
				h.cycleHintIndex++
			} else {
				h.cycleHintIndex = 0
			}
		}
	}

	selectedHint := filteredHints[h.cycleHintIndex]

	center := selectedHint.Element().Center()

	moveErr := h.actionService.MoveCursorToPoint(ctx, center)
	if moveErr != nil {
		h.logger.Error("Failed to move cursor during cycle_hint", zap.Error(moveErr))

		return derrors.New(derrors.CodeActionFailed, "failed to move cursor: "+moveErr.Error())
	}

	pendingAction := h.hints.Context.PendingAction()

	pendingModifier := h.hints.Context.PendingModifier()
	if pendingAction != nil && executeAction {
		repeat := h.hints.Context.Repeat()
		cursorFollowSelection := h.hints.Context.CursorFollowSelection()
		filterRoles := h.hints.Context.FilterRoles()
		filterTextContains := h.hints.Context.FilterTextContains()
		startWithSearch := h.hints.Context.StartWithSearch()
		strategyOverride := h.hints.Context.StrategyOverride()
		labelDirectionOverride := h.hints.Context.LabelDirectionOverride()
		splitWord := h.hints.Context.SplitWord()

		h.executeActionAtPoint(pendingAction, pendingModifier, center, repeat, func() {
			h.activateHintModeInternal(
				nil,
				nil,
				nil,
				filterRoles,
				filterTextContains,
				&startWithSearch,
				nil,
				&strategyOverride,
				&labelDirectionOverride,
				&splitWord,
			)

			// Restore state so subsequent cycles continue to execute the action
			// Guard: only restore if repeat was originally set (mode is still hints).
			if repeat && h.appState.CurrentMode() == domain.ModeHints &&
				h.hints != nil && h.hints.Context != nil {
				h.hints.Context.SetPendingAction(pendingAction)
				h.hints.Context.SetPendingModifier(pendingModifier)
				h.hints.Context.SetRepeat(true)
				h.hints.Context.SetCursorFollowSelection(cursorFollowSelection)
				h.hints.Context.SetFilterRoles(filterRoles)
				h.hints.Context.SetFilterTextContains(filterTextContains)
				h.hints.Context.SetStartWithSearch(startWithSearch)
				h.hints.Context.SetStrategyOverride(strategyOverride)
				h.hints.Context.SetLabelDirectionOverride(labelDirectionOverride)
				h.hints.Context.SetSplitWord(splitWord)
			}
		})
	}

	return nil
}

func (h *Handler) startHintSearchLocked() error {
	if h.appState.CurrentMode() != domain.ModeHints {
		return derrors.New(derrors.CodeInvalidInput, "search_hints requires hints mode")
	}

	if h.hints == nil || h.hints.Context == nil {
		return derrors.New(derrors.CodeActionFailed, "hints component not available")
	}

	if h.hints.Context.SourceHints() == nil {
		return derrors.New(derrors.CodeActionFailed, "hints not available")
	}

	h.stopHintSearchTextInputLocked(true)
	h.hints.Context.SetSearchQuery("")
	h.hints.Context.SetSearchActive(true)

	if h.hints.Context.HideOnEmptySearch() {
		// When hide-on-empty-search is active, hide all hints initially.
		// Hints will appear as the user types a query.
		setHintsErr := h.hints.Context.ClearVisibleHints()
		if setHintsErr != nil {
			return setHintsErr
		}
	} else {
		setHintsErr := h.hints.Context.SetVisibleHints(
			h.hints.Context.SourceHints(),
		)
		if setHintsErr != nil {
			return setHintsErr
		}
	}

	h.cycleHintIndex = -1
	h.drawHintSearchInput()

	if h.textInput != nil {
		searchFrame := h.searchInputFrame()
		position := searchFrame.Position()
		height := estimatedSearchInputHeight(h.config.Hints.SearchInputUI)
		textInputFrame := ports.TextInputFrame{
			X:      position.X,
			Y:      position.Y,
			Width:  searchFrame.Width(),
			Height: height,
		}

		started, _ := h.textInput.StartHintSearchSession(
			h.ctx,
			ports.TextInputCallbacks{
				OnQueryChanged: func(query string) {
					h.mu.Lock()
					defer h.mu.Unlock()

					if h.appState.CurrentMode() != domain.ModeHints || h.hints == nil ||
						h.hints.Context == nil {
						return
					}

					if !h.hints.Context.SearchActive() {
						return
					}

					h.hints.Context.SetSearchQuery(query)
					h.applyHintSearchFilter()
				},
				OnConfirm: func() {
					h.mu.Lock()
					defer h.mu.Unlock()

					if h.appState.CurrentMode() != domain.ModeHints {
						return
					}

					h.confirmHintSearch()
				},
				OnCancel: func() {
					h.mu.Lock()
					defer h.mu.Unlock()

					if h.appState.CurrentMode() != domain.ModeHints {
						return
					}

					h.cancelHintSearch()
				},
			},
			textInputFrame,
		)

		if started {
			h.hintSearchTextInputActive = true
			if h.disableEventTap != nil {
				h.disableEventTap()
				h.hintSearchEventTapDisabled = true
			}
		}
	}

	return nil
}

func (h *Handler) stopHintSearchTextInputLocked(keepEventTapDisabled bool) {
	if h.hintSearchTextInputActive && h.textInput != nil {
		// Use Background context since this may be called during cleanup,
		// after h.ctx has already been canceled.
		_ = h.textInput.StopHintSearchSession(context.Background())
	}

	h.hintSearchTextInputActive = false

	if h.hintSearchEventTapDisabled && h.enableEventTap != nil &&
		h.appState.CurrentMode() == domain.ModeHints && !keepEventTapDisabled {
		h.enableEventTap()
		h.hintSearchEventTapDisabled = false
	}
}

func (h *Handler) focusedBundleID() string {
	if h.actionService == nil {
		return ""
	}

	bundleID, err := h.actionService.FocusedAppBundleID(h.ctx)
	if err != nil {
		h.logger.Debug("Failed to get focused app bundle ID for mode hotkeys", zap.Error(err))

		return ""
	}

	return bundleID
}

// stopHeldRepeatLocked cancels any running held-key repeat goroutine.
// Caller must hold h.mu.
func (h *Handler) stopHeldRepeatLocked() {
	if h.heldRepeatingCancel != nil {
		h.heldRepeatingCancel()
		h.heldRepeatingCancel = nil
	}

	h.heldRepeatingKey = ""
}
