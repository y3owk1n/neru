package modes

import (
	"context"
	"image"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/render"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/app/services/modeindicator"
	"github.com/y3owk1n/neru/internal/app/services/stickyindicator"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
)

// Mode defines the interface that all navigation modes must implement.
// This provides a consistent API contract for mode activation, key handling,
// and cleanup operations.
type Mode interface {
	// Activate enters the mode with the flags the activation carries. Every
	// flag the mode accepts is in there: nothing is copied out of it on the
	// way in, so a mode reads the same value the grammar produced.
	Activate(activation modecmd.Activation)

	// HandleKey processes a key press within the mode's context.
	HandleKey(key string)

	// Exit performs mode-specific cleanup and deactivation.
	Exit()

	// ModeType returns the domain mode type this implementation represents.
	ModeType() domain.Mode
}

// Handler is the locked outer shell of the mode handler: it owns the mutexes
// and the exported entry points, each of which takes mu and delegates to the
// embedded handlerState. Methods on Handler may lock; methods on state may not.
type Handler struct {
	handlerState

	// mu serializes access to handler state between the event tap callback thread
	// and timer goroutines (e.g., refreshHintsTimer). All public entry points
	// (HandleKeyPress, ActivateMode, ExitMode) and timer callbacks must hold this lock.
	mu sync.Mutex

	// moveMonitorMu serializes MoveMonitor invocations. Lock ordering is
	// always moveMonitorMu -> h.mu (MoveMonitor holds this while calling
	// refreshActiveModeOnNewScreen, which acquires h.mu via the
	// Refresh*ForScreenChange helpers). Never acquire in the reverse order.
	moveMonitorMu sync.Mutex
}

// state carries every field of the mode handler and every method that runs
// with Handler.mu already held. It deliberately has no mutex: a state method
// cannot lock, and it cannot reach the exported (locking) Handler surface, so
// "caller must hold the lock" is enforced by the compiler instead of by the
// old *Locked naming convention. Mode implementations receive a *state.
//
// The one escape hatch is outer, the back-reference to the owning Handler.
// It exists solely so state methods can schedule deferred work (timers,
// goroutines) whose callbacks must take the lock when they later fire. Never
// call anything on outer synchronously from a state method — that is the
// self-deadlock the split exists to prevent.
type handlerState struct {
	// outer is the owning Handler. Deferred callbacks only; see the type comment.
	outer *Handler

	config         *configpkg.Config
	themeProvider  configpkg.ThemeProvider
	system         ports.SystemPort
	logger         *zap.Logger
	appState       *state.AppState
	cursorState    *state.CursorState
	modifierState  *state.ModifierState
	overlayManager overlay.ManagerInterface
	renderer       *render.OverlayRenderer
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
	eventTap              ports.EventTapPort
	refreshHotkeys        func()
	executeActionSequence func(source string, steps []string)
	shutdown              func()
	refreshHintsTimer     *time.Timer
	modeSession           uint64
	hotkeyLastKey         string
	hotkeyLastKeyTime     int64

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
	Renderer       *render.OverlayRenderer

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
	// ExecuteActionSequence runs an ordered list of action steps. The daemon
	// owns the sequencing rules (bail handling, error reporting), so a binding
	// behaves the same whether it is dispatched from here or from anywhere
	// else. source names the caller (a bind key, "on-exit") for logging.
	ExecuteActionSequence func(source string, steps []string)
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

	handler := &Handler{}
	handler.handlerState = handlerState{
		outer:                  handler,
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
		executeActionSequence:  deps.ExecuteActionSequence,
		shutdown:               deps.Shutdown,
		textInput:              deps.TextInput,
		themeProvider:          deps.System,
		system:                 deps.System,
		cycleHintIndex:         -1,
	}

	// Initialize mode implementations. Modes run with the lock already held,
	// so they are built on the inner state and cannot re-enter the locked
	// surface.
	handler.modes = map[domain.Mode]Mode{
		domain.ModeHints:         NewHintsMode(&handler.handlerState),
		domain.ModeGrid:          NewGridMode(&handler.handlerState),
		domain.ModeScroll:        NewScrollMode(&handler.handlerState),
		domain.ModeRecursiveGrid: NewRecursiveGridMode(&handler.handlerState),
		domain.ModeMonitorSelect: NewMonitorSelectMode(&handler.handlerState),
	}

	return handler
}

// ActivateMode enters the mode an activation names, with every flag it was
// given.
//
// It is the handler's only activation entry point, and it takes the same
// Activation the grammar parses, the CLI builds and the configuration
// validator reads. Nothing is copied on the way in, so a flag a mode accepts
// cannot be lost between being read and being applied.
func (h *Handler) ActivateMode(activation modecmd.Activation) {
	h.mu.Lock()
	defer h.mu.Unlock()

	mode := activation.Mode

	// Toggle: if the mode is already active and --toggle was specified,
	// exit to idle instead of re-activating
	if activation.Toggle != nil && *activation.Toggle && h.appState.CurrentMode() == mode {
		h.exitMode()

		return
	}

	if mode == domain.ModeIdle {
		h.exitMode()

		return
	}

	modeImpl, exists := h.modes[mode]
	if !exists {
		h.logger.Warn("Unknown mode", zap.String("mode", domain.ModeString(mode)))

		return
	}

	// Normalize --on-exit for external (re-)activations. This method is the sole
	// entry point for user-driven activations (IPC, hotkeys, systray); internal
	// refreshes (repeat re-activation, space/screen change, cycle) bypass it and
	// call the activate* helpers directly with a nil onExit to preserve the
	// stored steps. An omitted --on-exit on a fresh external command must
	// clear any steps left over from a prior activation of the same mode
	// rather than inheriting them, so a later completed action does not run a
	// stale command. A nil slice reaching those helpers means "preserve"; the
	// non-nil empty slice substituted here means "clear", and it is a no-op at
	// dispatch time.
	if activation.OnExit == nil {
		activation.OnExit = []string{}
	}

	modeImpl.Activate(activation)
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

// setScreenBounds records the active screen bounds used for overlay coordinate
// conversion and informs the overlay backend of that screen's global origin.
// The grid, recursive-grid and hint overlays render in screen-local coordinates
// (origin 0,0); on backends whose overlay spans the whole desktop (Linux X11
// and Wayland) the origin lets them translate that content onto the correct
// monitor. It is a no-op where each screen owns an overlay window (macOS).
func (h *handlerState) setScreenBounds(bounds image.Rectangle) {
	h.screenBounds = bounds

	if h.overlayManager != nil {
		h.overlayManager.SetActiveScreenOrigin(bounds.Min)
	}
}

func (h *handlerState) focusedBundleID() string {
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

// stopHeldRepeat cancels any running held-key repeat goroutine.
func (h *handlerState) stopHeldRepeat() {
	if h.heldRepeatingCancel != nil {
		h.heldRepeatingCancel()
		h.heldRepeatingCancel = nil
	}

	h.heldRepeatingKey = ""
}
