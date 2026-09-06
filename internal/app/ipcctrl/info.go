package ipcctrl

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
)

// healthNotInitialized is the status string for components that were not initialized.
const healthNotInitialized = "not initialized"

// detailSuffix marks informational sibling keys in capabilitiesMap (e.g.
// "dark_mode_detection_detail"). Any key ending in this suffix carries
// free-form prose for the `neru doctor` metadata header and is deliberately
// excluded from the component health-row loop, which only understands the
// supported/stub/headless/ok vocabulary.
const detailSuffix = "_detail"

// minConfigSetArgs is the minimum number of arguments required by the
// config-set IPC handler (key and value).
const minConfigSetArgs = 2

// InfoHandler handles info and config-related IPC commands.
type InfoHandler struct {
	configService *loader.Service
	appState      *state.AppState
	config        *config.Config
	modes         *modes.Handler
	hintService   *services.HintService
	gridService   *services.GridService
	actionService *services.ActionService
	scrollService *services.ScrollService
	systemPort    ports.SystemPort
	eventTap      ports.EventTapPort
	ipcServer     ports.IPCPort
	reloadConfig  func(ctx context.Context, configPath string) error
	cursorSlots   *state.CursorSlots
	logger        *zap.Logger

	// configMu protects config from concurrent read/write.
	configMu sync.RWMutex

	// setConfigField is a callback invoked by handleConfigSet to apply a
	// config field change at the app level (reconfigure components, re-register
	// hotkeys, etc.). If nil only the in-memory config is updated.
	setConfigField func(ctx context.Context, key, value string) error
}

// InfoHandlerDeps collects everything NewInfoHandler needs.
//
// The reasoning matches Deps: the positional list had reached fourteen
// arguments, several nil at any given call site. Zero values are valid — a nil
// service means the commands that need it report it as unavailable, and
// EventTap and IPCServer are nil until initialization phase 8.
type InfoHandlerDeps struct {
	ConfigService *loader.Service
	AppState      *state.AppState
	Config        *config.Config
	Modes         *modes.Handler

	HintService   *services.HintService
	GridService   *services.GridService
	ActionService *services.ActionService
	ScrollService *services.ScrollService

	System    ports.SystemPort
	EventTap  ports.EventTapPort
	IPCServer ports.IPCPort

	// ReloadConfig performs a full app-level config reload.
	ReloadConfig func(ctx context.Context, configPath string) error
	// SetConfigField applies a runtime config field change.
	SetConfigField func(ctx context.Context, key, value string) error

	// CursorSlots is the store save_cursor_pos writes to, reported by status.
	CursorSlots *state.CursorSlots

	Logger *zap.Logger
}

// NewInfoHandler creates the info/config command handler.
func NewInfoHandler(deps InfoHandlerDeps) *InfoHandler {
	return &InfoHandler{
		configService:  deps.ConfigService,
		appState:       deps.AppState,
		config:         deps.Config,
		modes:          deps.Modes,
		hintService:    deps.HintService,
		gridService:    deps.GridService,
		actionService:  deps.ActionService,
		scrollService:  deps.ScrollService,
		systemPort:     deps.System,
		eventTap:       deps.EventTap,
		ipcServer:      deps.IPCServer,
		reloadConfig:   deps.ReloadConfig,
		setConfigField: deps.SetConfigField,
		cursorSlots:    deps.CursorSlots,
		logger:         deps.Logger,
	}
}

// RegisterHandlers registers info/config command handlers.
func (h *InfoHandler) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers[domain.CommandStatus] = h.handleStatus
	handlers[domain.CommandConfig] = h.handleConfig
	handlers[domain.CommandReloadConfig] = h.handleReloadConfig
	handlers[domain.CommandHealth] = h.handleHealth
	handlers[domain.CommandConfigSet] = h.handleConfigSet
}

// ResolveConfigPath determines the configuration file path for status reporting.
func (h *InfoHandler) ResolveConfigPath() string {
	configPath := h.configService.GetConfigPath()

	if configPath == "" {
		return "using default config"
	}

	_, err := os.Stat(configPath)
	if os.IsNotExist(err) {
		return "using default config"
	}

	// Convert to absolute path for display
	absPath, err := filepath.Abs(configPath)
	if err == nil {
		return absPath
	}

	return configPath
}

// UpdateConfig updates the stored config.
func (h *InfoHandler) UpdateConfig(cfg *config.Config) {
	h.configMu.Lock()
	defer h.configMu.Unlock()

	h.config = cfg
}

// configSnapshot returns the current config pointer under a read lock.
func (h *InfoHandler) configSnapshot() *config.Config {
	h.configMu.RLock()
	defer h.configMu.RUnlock()

	return h.config
}

func (h *InfoHandler) handleStatus(_ context.Context, _ ipc.Command) ipc.Response {
	configPath := h.ResolveConfigPath()

	cfg := h.configSnapshot()

	if cfg == nil {
		h.logger.Error("Config is nil in handleStatus")

		return h.configNotAvailableResponse()
	}

	status := map[string]any{
		"enabled":                h.appState.IsEnabled(),
		"mode":                   h.appState.ModeName(),
		"config":                 configPath,
		"hints_enabled":          cfg.Hints.Enabled,
		"grid_enabled":           cfg.Grid.Enabled,
		"recursive_grid_enabled": cfg.RecursiveGrid.Enabled,
		"capabilities":           capabilitiesMap(h.systemCapabilities()),
		"profile":                profileMap(platform.CurrentProfile()),

		// The runtime toggles. They are reported under the names the
		// corresponding toggle-* command reports, so a caller can set one with
		// --state and read back the field it just named.
		"scroll_inverted":         h.appState.IsScrollInverted(),
		"hidden_for_screen_share": h.appState.IsHiddenForScreenShare(),
		"cursor_follow_selection": h.cursorFollowSelection(),
		"saved_cursor_slots":      h.savedCursorSlots(),
	}

	return ipc.Response{
		Success: true,
		Message: "status retrieved successfully",
		Data:    status,
		Code:    ipc.CodeOK,
	}
}

// cursorFollowSelection reports the active mode's cursor-follow-selection
// preference, or nil when no mode carries one.
//
// It is null rather than false in that case because the two are different
// answers: false means a running mode is not following the selection, and null
// means there is nothing to follow it with. A caller that treated the absence
// as false would think it had read a state it can in fact only set once a mode
// is running.
func (h *InfoHandler) cursorFollowSelection() *bool {
	if h.modes == nil {
		return nil
	}

	enabled, ok := h.modes.CursorFollowSelection()
	if !ok {
		return nil
	}

	return &enabled
}

// savedCursorSlots reports the occupied cursor slots and the position each
// holds, as an object keyed by slot name.
//
// The points are re-keyed to lowercase x and y rather than encoded straight
// from image.Point, whose fields marshal as X and Y — the rest of this payload
// is snake_case, and the shape a script reads should not depend on how the
// position happens to be stored.
func (h *InfoHandler) savedCursorSlots() map[string]map[string]int {
	slots := map[string]map[string]int{}

	if h.cursorSlots == nil {
		return slots
	}

	for name, pos := range h.cursorSlots.Snapshot() {
		slots[name] = map[string]int{"x": pos.X, "y": pos.Y}
	}

	return slots
}

func (h *InfoHandler) handleConfig(ctx context.Context, cmd ipc.Command) ipc.Response {
	// Support sub-commands like "config set ..." from hotkey bindings.
	if len(cmd.Args) > 0 {
		switch cmd.Args[0] {
		case "set":
			return h.handleConfigSet(ctx, ipc.Command{
				Action: domain.CommandConfigSet,
				Args:   cmd.Args[1:],
			})
		case "reset":
			return h.handleConfigReset(ctx, ipc.Command{
				Action: "config-reset",
				Args:   cmd.Args[1:],
			})
		case "reload":
			return h.handleReloadConfig(ctx, ipc.Command{})
		default:
			return ipc.Response{
				Success: false,
				Message: "unknown config subcommand: " + cmd.Args[0],
				Code:    ipc.CodeInvalidInput,
			}
		}
	}

	// Default: dump the full config.
	cfg := h.configSnapshot()
	if cfg == nil {
		h.logger.Error("Config is nil in handleConfig")

		return h.configNotAvailableResponse()
	}

	return ipc.Response{
		Success: true,
		Data:    cfg,
		Code:    ipc.CodeOK,
	}
}

func (h *InfoHandler) handleReloadConfig(ctx context.Context, _ ipc.Command) ipc.Response {
	if h.reloadConfig == nil {
		h.logger.Error("Reload config callback is not set")

		return ipc.Response{
			Success: false,
			Message: "reload config not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	configPath := h.configService.GetConfigPath()

	err := h.reloadConfig(ctx, configPath)
	if err != nil {
		h.logger.Error("Failed to reload config", zap.Error(err))

		return ipc.Response{
			Success: false,
			Message: "failed to reload config: " + err.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	return ipc.Response{
		Success: true,
		Message: "config reloaded successfully",
		Code:    ipc.CodeOK,
	}
}
