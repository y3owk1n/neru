package modes

import (
	"context"
	"image"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/heldmotion"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/app/services/modeindicator"
	"github.com/y3owk1n/neru/internal/app/services/stickyindicator"
	"github.com/y3owk1n/neru/internal/app/services/virtualpointer"
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

	// RefreshForMonitorMove puts the mode's Frame back on screen against
	// targetBounds after the cursor has been warped to another display. The
	// frame was taken off screen before the warp, so this is a transition onto
	// the display the cursor landed on rather than a repaint of the one it
	// left.
	//
	// The caller holds h.mu across the whole dispatch, so the mode it selected
	// is still the active one here and an implementation must not re-check.
	// Why it is core rather than an optional extension, and why the re-checks
	// are gone, is ADR 0004.
	RefreshForMonitorMove(ctx context.Context, targetBounds image.Rectangle)
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
	// always moveMonitorMu -> h.mu: MoveMonitor holds this while clearing the
	// frame and again while refreshActiveModeForMonitorMove dispatches the
	// refresh, each of which takes h.mu for the length of its own call. Never
	// acquire in the reverse order.
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

	config        *configpkg.Config
	system        ports.SystemPort
	logger        *zap.Logger
	appState      *state.AppState
	cursorState   *state.CursorState
	modifierState *state.ModifierState
	// overlayPort is the one way a mode reaches the screen: it hands over a
	// Frame describing what should be on it and never sequences a transition
	// itself. Since #1213 it is the only overlay reference this package has:
	// the Linux keyboard grab goes through this same port rather than through
	// the overlay package's singleton (indicator_polling.go).
	//
	// Its type is overlaySurface, not ports.OverlayPort: the calls the handler
	// must never make are not on it, so making one is a compile error rather
	// than a rule to remember (overlay.go).
	overlayPort overlaySurface
	// hintsFrameOnScreen records whether this activation has already put the
	// hints Frame on screen. The hint manager's update callback fires on
	// activation and again on every narrowing keystroke; only the first needs
	// the overlay shown and switched, and paying for a window show per
	// keystroke is the latency regression AGENTS.md forbids.
	//
	// It may only be cleared in a locked section that also invalidates the hint
	// manager's pending update generation, or a debounce timer firing after the
	// clear performs a window transition for an activation that is already
	// gone. The two sites that clear it both satisfy that (`hints.go`,
	// `hintdraw.go`).
	hintsFrameOnScreen bool
	// New Services
	hintService            *services.HintService
	gridService            *services.GridService
	actionService          *services.ActionService
	scrollService          *services.ScrollService
	modeIndicatorService   *modeindicator.Service
	stickyIndicatorService *stickyindicator.Service
	virtualPointerService  *virtualpointer.Service

	hints         *components.HintsComponent
	grid          *components.GridComponent
	scroll        *components.ScrollComponent
	recursiveGrid *components.RecursiveGridComponent
	monitorSelect *monitorSelectSession

	// Mode implementations
	modes map[domain.Mode]Mode

	// Screen bounds for coordinate conversion (grid and hints)
	screenBounds image.Rectangle

	// focusedApp is the cell the application watcher publishes the focused app
	// into, and the keymap fields below are what the handler settles from it.
	// Both belong to keymap.go, which is where the rules about them are
	// written; the cell is lock-free on purpose and must stay that way.
	focusedApp *focusedAppCell
	// keymap is the mode's own bindings in force and globalHotkeys is the global
	// [hotkeys] table it falls back to for a chord it does not bind, filtered to
	// modifier chords. The two settle together and from the same inputs, so a
	// keystroke that consults both still asks the platform nothing.
	//
	// keymapSettledFor is what they were settled for, and keymapSettled says they
	// have been settled at all — two zero keymaps are a legitimate answer (a mode
	// that binds nothing, with no global chords), so neither can double as "not
	// settled yet".
	keymap           configpkg.Keymap
	globalHotkeys    configpkg.Keymap
	keymapSettledFor keymapInputs
	keymapSettled    bool

	// eventTap is nil until the app's phase 8 calls SetEventTap; every use
	// goes through the nil-guarded helpers in eventtap.go.
	eventTap              ports.EventTapPort
	refreshHotkeys        func()
	executeActionSequence func(source string, steps []string)
	shutdown              func()
	refreshHintsTimer     *time.Timer
	modeSession           uint64
	// customModeName is the declared mode a ModeCustom session is in, and
	// empty outside one. It is what tells one declaration from another
	// wherever the enum alone cannot: the keymap, the indicator, the frame.
	customModeName    string
	hotkeyLastKey     string
	hotkeyLastKeyTime int64

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

	// systemCursorHidden tracks whether the hide_cursor action is holding the
	// system cursor hidden. It is also what gates the standalone
	// cursor-following virtual pointer that stands in for it; the in-frame
	// pointers the grid modes draw are unrelated and no mode sets this.
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

	// motion is this handler's namespace in the held-key glide controller;
	// nil (or a nil-safe empty group) when the app runs without one.
	motion *heldmotion.Group
	// fedPress is set for the duration of a key fed over IPC, which has no
	// release event and so must take the discrete path, never the glide.
	fedPress bool
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

	// OverlayPort is what the modes draw through. It is narrowed to
	// overlaySurface here rather than taken as ports.OverlayPort: a caller
	// passes the full port and the handler keeps only the part it may call
	// (overlay.go).
	OverlayPort overlaySurface

	HintService            *services.HintService
	GridService            *services.GridService
	ActionService          *services.ActionService
	ScrollService          *services.ScrollService
	ModeIndicatorService   *modeindicator.Service
	StickyIndicatorService *stickyindicator.Service
	VirtualPointerService  *virtualpointer.Service

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

	// Motion is the handler's namespace in the held-key glide controller.
	Motion *heldmotion.Group
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
		overlayPort:            deps.OverlayPort,
		hintService:            deps.HintService,
		gridService:            deps.GridService,
		actionService:          deps.ActionService,
		scrollService:          deps.ScrollService,
		modeIndicatorService:   deps.ModeIndicatorService,
		stickyIndicatorService: deps.StickyIndicatorService,
		virtualPointerService:  deps.VirtualPointerService,
		hints:                  deps.HintsComponent,
		grid:                   deps.GridComponent,
		scroll:                 deps.ScrollComponent,
		recursiveGrid:          deps.RecursiveGridComponent,
		screenBounds:           screenBounds,
		refreshHotkeys:         deps.RefreshHotkeys,
		executeActionSequence:  deps.ExecuteActionSequence,
		shutdown:               deps.Shutdown,
		textInput:              deps.TextInput,
		motion:                 deps.Motion,
		system:                 deps.System,
		focusedApp:             &focusedAppCell{},
		cycleHintIndex:         -1,
	}

	fillIndicatorServices(&handler.handlerState, logger)

	handler.modes = newModes(&handler.handlerState)

	return handler
}

// newModes builds the map the handler dispatches every mode operation through.
// Modes run with the lock already held, so they are built on the inner state
// and cannot re-enter the locked surface.
//
// Registering a mode here is what puts it in front of TestModeExtensionMatrix,
// which then fails until the mode states which optional extensions it carries
// (extensions.go).
func newModes(state *handlerState) map[domain.Mode]Mode {
	return map[domain.Mode]Mode{
		domain.ModeHints:         NewHintsMode(state),
		domain.ModeGrid:          NewGridMode(state),
		domain.ModeScroll:        NewScrollMode(state),
		domain.ModeRecursiveGrid: NewRecursiveGridMode(state),
		domain.ModeMonitorSelect: NewMonitorSelectMode(state),
		domain.ModeCustom:        NewCustomMode(state),
	}
}

// fillIndicatorServices gives the handler a service for every indicator,
// substituting a portless one for anything the caller left out.
//
// Mode logic then drives an indicator unconditionally: whether the indicator
// was ever constructed — a disabled one, a headless backend — is answered
// inside the service instead of by a pointer test at each call site.
//
// Only a test builds a handler without one, so a substitution in a real
// session is a wiring mistake, and it says so rather than leaving an
// indicator silently dead.
func fillIndicatorServices(state *handlerState, logger *zap.Logger) {
	if state.modeIndicatorService == nil {
		logIndicatorSubstitution(logger, ports.ModeIndicator)

		state.modeIndicatorService = modeindicator.NewService(state.system, nil)
	}

	if state.stickyIndicatorService == nil {
		logIndicatorSubstitution(logger, ports.StickyModifiersIndicator)

		state.stickyIndicatorService = stickyindicator.NewService(state.system, nil)
	}

	if state.virtualPointerService == nil {
		logIndicatorSubstitution(logger, ports.VirtualPointerIndicator)

		state.virtualPointerService = virtualpointer.NewService(state.system, nil)
	}
}

// logIndicatorSubstitution reports an indicator the handler was not given a
// service for. It names the indicator and nothing it would draw.
func logIndicatorSubstitution(logger *zap.Logger, indicator ports.Indicator) {
	if logger == nil {
		return
	}

	logger.Debug(
		"No service for indicator; it will not be drawn",
		zap.String("indicator", indicator.String()),
	)
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
	// exit to idle instead of re-activating. Every declared mode shares one
	// enum value, so there "already active" also means the same name: toggling
	// one declared mode from inside another switches rather than exits.
	if activation.Toggle != nil && *activation.Toggle && h.appState.CurrentMode() == mode &&
		(mode != domain.ModeCustom || h.customModeName == activation.Name) {
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

// UpdateConfig updates the handler with new configuration. Overlay appearance
// is not part of it: the overlay resolves its own Style and the handler reads
// the result.
func (h *Handler) UpdateConfig(config *configpkg.Config) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.config = config

	h.updateComponentConfigs(config)

	// A declared mode the new configuration no longer declares has no keymap
	// to settle and no way out but this one, so the session ends here rather
	// than leaving the keyboard captured by a mode that answers nothing.
	if h.appState.CurrentMode() == domain.ModeCustom {
		if _, declared := config.Modes[h.customModeName]; !declared {
			h.logger.Info("Custom mode exited: the reloaded configuration no longer declares it",
				zap.String("mode", h.customModeName))
			h.exitMode()
		}
	}

	// Replacing the configuration replaces what is bound, and settling it here
	// rather than on the next keystroke is what keeps the keystroke path unable
	// to ask the platform anything (ADR 0005).
	h.settledKeymap()

	h.syncModifierPassthrough(h.appState.CurrentMode())
}

// updateComponentConfigs rebuilds the domain state the mode components derive
// from configuration. Overlay appearance is not here: it reaches the render
// components through the overlay's own Style notification.
//
// It runs here rather than on the app's reconfigure path because the state it
// rebuilds is the state a keystroke reads. The grid manager is the one that
// matters and it carries no lock of its own: the handler assigns it on
// activation and reads it on every key, both under `h.mu`, so a reload writing
// it from the app layer was a plain data race on live mode state (#1277).
func (h *handlerState) updateComponentConfigs(config *configpkg.Config) {
	if h.grid != nil {
		h.grid.UpdateConfig(config, h.logger)
	}

	if h.scroll != nil {
		h.scroll.UpdateConfig(config, h.logger)
	}
}

// setScreenBounds records the active screen bounds used for overlay coordinate
// conversion and names that display to the overlay. The grid, recursive-grid
// and hint surfaces are drawn in screen-local coordinates (origin 0,0), so a
// backend whose surface spans the whole desktop needs the screen to place that
// content on the right monitor; where each screen owns a window it is ignored.
func (h *handlerState) setScreenBounds(bounds image.Rectangle) {
	h.screenBounds = bounds

	if h.overlayPort != nil {
		h.overlayPort.SetActiveScreen(bounds)
	}
}

// stopHeldRepeat cancels any running held-key repeat goroutine.
func (h *handlerState) stopHeldRepeat() {
	if h.heldRepeatingCancel != nil {
		h.heldRepeatingCancel()
		h.heldRepeatingCancel = nil
	}

	h.heldRepeatingKey = ""
}
