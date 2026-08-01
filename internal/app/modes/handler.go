package modes

import (
	"context"
	"image"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/app/services/modeindicator"
	"github.com/y3owk1n/neru/internal/app/services/stickyindicator"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/overlay"
	"github.com/y3owk1n/neru/internal/core/infra/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/core/infra/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/core/infra/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/core/ports"
	"github.com/y3owk1n/neru/internal/ui"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
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

	// eventTap is nil until the app's phase 8 calls SetEventTap; every use
	// goes through the nil-guarded helpers in eventtap.go.
	eventTap            ports.EventTapPort
	refreshHotkeys      func()
	executeHotkeyAction func(key, actionStr string) error
	shutdown            func()
	refreshHintsTimer   *time.Timer
	modeSession         uint64
	hotkeyLastKey       string
	hotkeyLastKeyTime   int64

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

	// Base context for Handler methods. Injected by the App via NewHandler so
	// all Handler operations observe app-level cancellation.
	ctx context.Context //nolint:containedctx

	// heldRepeatingKey tracks which key is currently held for custom repeat.
	// When non-empty, macOS native key-down events for this key are suppressed
	// and a custom goroutine drives the repeat at heldRepeatInterval.
	heldRepeatingKey    string
	heldRepeatingCancel context.CancelFunc
}

// HandlerDeps collects everything NewHandler needs.
//
// It is a struct rather than a positional list because the list had reached
// twenty-nine arguments, seven of which were closures that together formed an
// open-coded ports.EventTapPort. Those seven are gone: the handler holds the
// port itself, injected after construction via SetEventTap because the event
// tap does not exist yet when the handler is built.
type HandlerDeps struct {
	Ctx context.Context //nolint:containedctx // root context, matches Handler.ctx

	Config *configpkg.Config
	Logger *zap.Logger

	AppState    *state.AppState
	CursorState *state.CursorState

	OverlayManager overlay.ManagerInterface
	Renderer       *ui.OverlayRenderer

	HintService            *services.HintService
	GridService            *services.GridService
	ActionService          *services.ActionService
	ScrollService          *services.ScrollService
	ModeIndicatorService   *modeindicator.Service
	StickyIndicatorService *stickyindicator.Service

	HintsComponent         *components.HintsComponent
	GridComponent          *components.GridComponent
	ScrollComponent        *components.ScrollComponent
	RecursiveGridComponent *components.RecursiveGridComponent

	// RefreshHotkeys re-registers hotkeys for the focused app.
	RefreshHotkeys func()
	// ExecuteHotkeyAction runs a hotkey's action string.
	ExecuteHotkeyAction func(key, actionStr string) error
	// Shutdown quits the daemon.
	Shutdown func()

	TextInput ports.TextInputPort
	System    ports.SystemPort
}

// NewHandler creates a mode handler from deps.
func NewHandler(deps HandlerDeps) *Handler {
	logger := deps.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	logger = logger.Named("modes")

	// Initialize screen bounds for coordinate conversion.
	// Use a background context since this runs during startup.
	// CodeNotSupported is expected on non-darwin platforms and is silently ignored;
	// any other error is logged as a warning.
	var screenBounds image.Rectangle

	if deps.System != nil {
		var boundsErr error

		screenBounds, boundsErr = deps.System.ScreenBounds(context.Background())
		if boundsErr != nil && !derrors.IsNotSupported(boundsErr) {
			logger.Warn("Failed to get initial screen bounds", zap.Error(boundsErr))
		}
	}

	handler := &Handler{
		ctx:                    deps.Ctx,
		config:                 deps.Config,
		logger:                 logger,
		appState:               deps.AppState,
		cursorState:            deps.CursorState,
		modifierState:          state.NewModifierState(),
		overlayManager:         deps.OverlayManager,
		renderer:               deps.Renderer,
		hintService:            deps.HintService,
		gridService:            deps.GridService,
		actionService:          deps.ActionService,
		scrollService:          deps.ScrollService,
		modeIndicatorService:   deps.ModeIndicatorService,
		stickyIndicatorService: deps.StickyIndicatorService,
		hints:                  deps.HintsComponent,
		grid:                   deps.GridComponent,
		scroll:                 deps.ScrollComponent,
		recursiveGrid:          deps.RecursiveGridComponent,
		screenBounds:           screenBounds,
		refreshHotkeys:         deps.RefreshHotkeys,
		executeHotkeyAction:    deps.ExecuteHotkeyAction,
		shutdown:               deps.Shutdown,
		textInput:              deps.TextInput,
		themeProvider:          deps.System,
		system:                 deps.System,
		cycleHintIndex:         -1,
	}

	// Initialize mode implementations
	handler.modes = map[domain.Mode]Mode{
		domain.ModeHints:         NewHintsMode(handler),
		domain.ModeGrid:          NewGridMode(handler),
		domain.ModeScroll:        NewScrollMode(handler),
		domain.ModeRecursiveGrid: NewRecursiveGridMode(handler),
		domain.ModeMonitorSelect: NewMonitorSelectMode(handler),
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
			h.setScreenBounds(b)
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
			h.setScreenBounds(b)
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
				nil, // preserve the stored --on-exit action across re-activation
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

// setScreenBounds records the active screen bounds used for overlay coordinate
// conversion and informs the overlay backend of that screen's global origin.
// The grid, recursive-grid and hint overlays render in screen-local coordinates
// (origin 0,0); on backends whose overlay spans the whole desktop (Linux X11
// and Wayland) the origin lets them translate that content onto the correct
// monitor. It is a no-op where each screen owns an overlay window (macOS).
func (h *Handler) setScreenBounds(bounds image.Rectangle) {
	h.screenBounds = bounds

	if h.overlayManager != nil {
		h.overlayManager.SetActiveScreenOrigin(bounds.Min)
	}
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
			if h.hasEventTap() {
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

	if h.hintSearchEventTapDisabled && h.hasEventTap() &&
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
