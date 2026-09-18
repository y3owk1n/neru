package ipcctrl

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
)

// Controller handles IPC command routing and execution.
type Controller struct {
	// Services
	HintService   *services.HintService
	GridService   *services.GridService
	ActionService *services.ActionService
	ScrollService *services.ScrollService
	ConfigService *loader.Service

	// State
	AppState *state.AppState

	// Infrastructure
	Logger    *zap.Logger
	System    ports.SystemPort
	Overlay   ports.OverlayPort
	EventTap  ports.EventTapPort
	IPCServer ports.IPCPort
	KeyFeed   ports.KeyFeedPort

	// Mode management
	Modes *modes.Handler

	// Reload callback for full app-level config reload
	ReloadConfig func(ctx context.Context, configPath string) error

	// SetConfigField callback for runtime config field changes with full
	// app-level reconfiguration (component updates, hotkey re-registration, etc.).
	// If nil, the config is only updated in-memory.
	SetConfigField func(ctx context.Context, key, value string) error

	// ExecuteSequence runs an action sequence. If nil, the "run" command
	// reports that sequencing is unavailable.
	ExecuteSequence sequenceRunner

	// SetEnabled pauses or resumes the application. See Deps.SetEnabled.
	SetEnabled func(enabled bool)

	// ExecuteMacro runs a named macro. If nil, the "macro" command reports
	// that macros are unavailable.
	ExecuteMacro macroRunner

	// Info handler for config updates
	infoHandler *InfoHandler

	Handlers map[string]func(context.Context, ipc.Command) ipc.Response
}

// Deps collects everything New needs.
//
// It is a struct rather than a positional parameter list because the list had
// grown to fourteen arguments, most of them nil at any given call site — which
// made every new port a mechanical edit across seven test files and made the
// nils impossible to read. Zero values are valid: a nil service simply means
// the corresponding commands report that it is unavailable, and EventTap and
// IPCServer are legitimately nil until initialization phase 8 fills them in via
// SetInfrastructure.
type Deps struct {
	// Services
	HintService   *services.HintService
	GridService   *services.GridService
	ActionService *services.ActionService
	ScrollService *services.ScrollService
	ConfigService *loader.Service

	// State
	AppState *state.AppState
	Config   *config.Config

	// Mode management
	Modes *modes.Handler

	// Infrastructure ports
	System ports.SystemPort
	// Overlay answers the live overlay capability for status and health. Nil
	// leaves the system port's preset in place.
	Overlay   ports.OverlayPort
	EventTap  ports.EventTapPort
	IPCServer ports.IPCPort
	KeyFeed   ports.KeyFeedPort

	// ReloadConfig performs a full app-level config reload.
	ReloadConfig func(ctx context.Context, configPath string) error

	// ExecuteSequence runs an action sequence on behalf of the "run" command.
	ExecuteSequence sequenceRunner

	// ExecuteMacro runs a named macro on behalf of the "macro" command.
	ExecuteMacro macroRunner

	// SetEnabled pauses or resumes the application for "stop" and "start".
	// It flips the flag, exits the active mode and unregisters or restores the
	// global hotkeys. When nil, the hotkeys wait for the binder's next refresh.
	SetEnabled func(enabled bool)

	Logger *zap.Logger
}

// New creates a new IPC controller with the given dependencies.
func New(deps Deps) *Controller {
	logger := deps.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	ipcController := &Controller{
		HintService:     deps.HintService,
		GridService:     deps.GridService,
		ActionService:   deps.ActionService,
		ScrollService:   deps.ScrollService,
		ConfigService:   deps.ConfigService,
		AppState:        deps.AppState,
		Modes:           deps.Modes,
		System:          deps.System,
		Overlay:         deps.Overlay,
		EventTap:        deps.EventTap,
		IPCServer:       deps.IPCServer,
		KeyFeed:         deps.KeyFeed,
		ReloadConfig:    deps.ReloadConfig,
		ExecuteSequence: deps.ExecuteSequence,
		ExecuteMacro:    deps.ExecuteMacro,
		SetEnabled:      deps.SetEnabled,
		Logger:          logger.Named("ipc.controller"),
		Handlers:        make(map[string]func(context.Context, ipc.Command) ipc.Response),
	}

	// Register command handlers
	ipcController.registerHandlers(deps.Config)

	return ipcController
}

// HandleCommand routes an IPC command to the appropriate handler.
func (c *Controller) HandleCommand(ctx context.Context, command ipc.Command) ipc.Response {
	c.Logger.Debug(
		"Handling IPC command",
		zap.String("action", command.Action),
	)

	handler, ok := c.Handlers[command.Action]
	if !ok {
		return ipc.Response{
			Success: false,
			Message: "unknown command: " + command.Action,
			Code:    ipc.CodeUnknownCommand,
		}
	}

	// `neru stop` promises that every mode and action is off until `neru
	// start`. The CLI and every step a hotkey, macro or --on-exit runs all
	// arrive here, so this one check enforces that.
	if !c.AppState.IsEnabled() && !answeredWhilePaused[command.Action] {
		return ipc.Response{
			Success: false,
			Message: "neru is stopped; run `neru start` first",
			Code:    ipc.CodeNotRunning,
		}
	}

	return handler(ctx, command)
}

// answeredWhilePaused lists what `neru stop` does not switch off: lifecycle,
// status and configuration, and the preference toggles. Anything else is
// refused while paused.
var answeredWhilePaused = map[string]bool{
	domain.CommandPing:                        true,
	domain.CommandStart:                       true,
	domain.CommandStop:                        true,
	domain.CommandStatus:                      true,
	domain.CommandConfig:                      true,
	domain.CommandReloadConfig:                true,
	domain.CommandHealth:                      true,
	domain.CommandConfigSet:                   true,
	domain.CommandToggleScreenShare:           true,
	domain.CommandToggleScrollInvert:          true,
	domain.CommandToggleCursorFollowSelection: true,
	domain.ModeString(domain.ModeIdle):        true,
}

// UpdateConfig updates the stored config.
func (c *Controller) UpdateConfig(cfg *config.Config) {
	if c.infoHandler != nil {
		c.infoHandler.UpdateConfig(cfg)
	}
}

// SetConfigFieldCallback sets the callback for runtime config field changes
// and propagates it to the info handler. Must be called after construction
// (e.g. from initializeIPCController) since the constructor's registerHandlers
// runs before the callback can be set.
func (c *Controller) SetConfigFieldCallback(
	cb func(ctx context.Context, key, value string) error,
) {
	c.SetConfigField = cb
	if c.infoHandler != nil {
		c.infoHandler.setConfigField = cb
	}
}

// SetInfrastructure updates the infrastructure references on the controller
// and its info handler. This is called after event tap and IPC server are
// initialized (Phase 8), since the IPC controller is created earlier (Phase 7).
func (c *Controller) SetInfrastructure(eventTap ports.EventTapPort, ipcServer ports.IPCPort) {
	c.EventTap = eventTap

	c.IPCServer = ipcServer
	if c.infoHandler != nil {
		c.infoHandler.eventTap = eventTap
		c.infoHandler.ipcServer = ipcServer
	}
}

// setEnabled calls Deps.SetEnabled. When none was wired it flips the flag and
// exits the mode itself.
func (c *Controller) setEnabled(enabled bool) {
	if c.SetEnabled != nil {
		c.SetEnabled(enabled)

		return
	}

	c.AppState.SetEnabled(enabled)

	if !enabled && c.Modes != nil {
		c.Modes.ExitMode()
	}
}

// registerHandlers registers all command handlers by delegating to sub-controllers.
func (c *Controller) registerHandlers(cfg *config.Config) {
	// Initialize handler components
	lifecycleHandler := NewLifecycleHandler(c.AppState, c.Modes, c.setEnabled, c.Logger)
	modesHandler := NewModesHandler(c.Modes, c.Logger)
	// The slots are IPC-session state with no dependencies, so the controller
	// owns them: the actions handler writes them and the info handler reports
	// them, and nothing outside this controller needs to reach them.
	cursorSlots := state.NewCursorSlots()

	actionsHandler := NewActionsHandler(
		c.ActionService,
		c.ScrollService,
		c.Modes,
		c.AppState,
		c.KeyFeed,
		cursorSlots,
		c.Logger,
	)
	// SetConfigField stays zero here; SetConfigFieldCallback fills it in after
	// construction, since the callback needs the fully built app.
	c.infoHandler = NewInfoHandler(InfoHandlerDeps{
		ConfigService: c.ConfigService,
		AppState:      c.AppState,
		Config:        cfg,
		Modes:         c.Modes,
		HintService:   c.HintService,
		GridService:   c.GridService,
		ActionService: c.ActionService,
		ScrollService: c.ScrollService,
		System:        c.System,
		Overlay:       c.Overlay,
		EventTap:      c.EventTap,
		IPCServer:     c.IPCServer,
		ReloadConfig:  c.ReloadConfig,
		CursorSlots:   cursorSlots,
		Logger:        c.Logger,
	})

	lifecycleHandler.RegisterHandlers(c.Handlers)
	modesHandler.RegisterHandlers(c.Handlers)
	actionsHandler.RegisterHandlers(c.Handlers)

	c.infoHandler.RegisterHandlers(c.Handlers)

	// Register overlay handler
	overlayHandler := NewOverlayHandler(c.AppState, c.Logger)
	overlayHandler.RegisterHandlers(c.Handlers)

	// Register scroll handler
	scrollHandler := NewScrollHandler(c.AppState, c.ScrollService, c.Logger)
	scrollHandler.RegisterHandlers(c.Handlers)

	// Register action sequence handler
	sequenceHandler := NewSequenceHandler(c.ExecuteSequence, c.ExecuteMacro, c.Logger)
	sequenceHandler.RegisterHandlers(c.Handlers)
}
