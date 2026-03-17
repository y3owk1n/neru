package app

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// healthNotInitialized is the status string for components that were not initialized.
const healthNotInitialized = "not initialized"

// IPCControllerInfo handles info and config-related IPC commands.
type IPCControllerInfo struct {
	configService *config.Service
	appState      *state.AppState
	config        *config.Config
	modes         *modes.Handler
	hintService   *services.HintService
	gridService   *services.GridService
	actionService *services.ActionService
	scrollService *services.ScrollService
	eventTap      ports.EventTapPort
	ipcServer     ports.IPCPort
	reloadConfig  func(ctx context.Context, configPath string) error
	logger        *zap.Logger

	// configMu protects config from concurrent read/write.
	configMu sync.RWMutex
}

// NewIPCControllerInfo creates a new info/config command handler.
func NewIPCControllerInfo(
	configService *config.Service,
	appState *state.AppState,
	config *config.Config,
	modes *modes.Handler,
	hintService *services.HintService,
	gridService *services.GridService,
	actionService *services.ActionService,
	scrollService *services.ScrollService,
	eventTap ports.EventTapPort,
	ipcServer ports.IPCPort,
	reloadConfig func(ctx context.Context, configPath string) error,
	logger *zap.Logger,
) *IPCControllerInfo {
	return &IPCControllerInfo{
		configService: configService,
		appState:      appState,
		config:        config,
		modes:         modes,
		hintService:   hintService,
		gridService:   gridService,
		actionService: actionService,
		scrollService: scrollService,
		eventTap:      eventTap,
		ipcServer:     ipcServer,
		reloadConfig:  reloadConfig,
		logger:        logger,
	}
}

// RegisterHandlers registers info/config command handlers.
func (h *IPCControllerInfo) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers[domain.CommandStatus] = h.handleStatus
	handlers[domain.CommandConfig] = h.handleConfig
	handlers[domain.CommandReloadConfig] = h.handleReloadConfig
	handlers[domain.CommandHealth] = h.handleHealth
}

// ResolveConfigPath determines the configuration file path for status reporting.
func (h *IPCControllerInfo) ResolveConfigPath() string {
	configPath := h.configService.GetConfigPath()

	if configPath == "" {
		return "using default config"
	}

	// Check if the config file actually exists
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
func (h *IPCControllerInfo) UpdateConfig(cfg *config.Config) {
	h.configMu.Lock()
	defer h.configMu.Unlock()

	h.config = cfg
}

// configSnapshot returns the current config pointer under a read lock.
func (h *IPCControllerInfo) configSnapshot() *config.Config {
	h.configMu.RLock()
	defer h.configMu.RUnlock()

	return h.config
}

func (h *IPCControllerInfo) handleStatus(_ context.Context, _ ipc.Command) ipc.Response {
	configPath := h.ResolveConfigPath()

	cfg := h.configSnapshot()

	if cfg == nil {
		h.logger.Error("Config is nil in handleStatus")

		return ipc.Response{
			Success: false,
			Message: "config not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	status := map[string]any{
		"enabled":                h.appState.IsEnabled(),
		"mode":                   domain.ModeString(h.appState.CurrentMode()),
		"config":                 configPath,
		"hints_enabled":          cfg.Hints.Enabled,
		"grid_enabled":           cfg.Grid.Enabled,
		"recursive_grid_enabled": cfg.RecursiveGrid.Enabled,
	}

	return ipc.Response{
		Success: true,
		Message: "status retrieved successfully",
		Data:    status,
		Code:    ipc.CodeOK,
	}
}

func (h *IPCControllerInfo) handleConfig(_ context.Context, _ ipc.Command) ipc.Response {
	cfg := h.configSnapshot()

	if cfg == nil {
		h.logger.Error("Config is nil in handleConfig")

		return ipc.Response{
			Success: false,
			Message: "config not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	return ipc.Response{
		Success: true,
		Data:    cfg,
		Code:    ipc.CodeOK,
	}
}

func (h *IPCControllerInfo) handleReloadConfig(ctx context.Context, _ ipc.Command) ipc.Response {
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

func (h *IPCControllerInfo) handleHealth(ctx context.Context, _ ipc.Command) ipc.Response {
	hasErrors := false
	// --- component checks ---------------------------------------------------
	components := make(map[string]string)
	// Event tap — only enabled during active modes (hints/grid/scroll),
	// so "disabled" in idle mode is expected and healthy.
	if h.eventTap != nil {
		if h.eventTap.IsEnabled() {
			components["event_tap"] = "ok (active)"
		} else {
			components["event_tap"] = "ok (idle)"
		}
	} else {
		components["event_tap"] = healthNotInitialized
		hasErrors = true
	}
	// IPC server (implicitly healthy since we're responding, but verify)
	if h.ipcServer != nil {
		if h.ipcServer.IsRunning() {
			components["ipc_server"] = "ok"
		} else {
			components["ipc_server"] = "not running"
			hasErrors = true
		}
	} else {
		components["ipc_server"] = healthNotInitialized
		hasErrors = true
	}
	// Config
	cfg := h.configSnapshot()
	if cfg != nil {
		validateErr := cfg.Validate()
		if validateErr != nil {
			components["config"] = validateErr.Error()
			hasErrors = true
		} else {
			components["config"] = "ok"
		}
	} else {
		components["config"] = "not loaded"
		hasErrors = true
	}
	// Service health checks (accessibility + overlay per service)
	serviceChecks := map[string]map[string]error{}
	if h.hintService != nil {
		serviceChecks["hints"] = h.hintService.Health(ctx)
	} else {
		components["hints"] = healthNotInitialized
		hasErrors = true
	}

	if h.gridService != nil {
		serviceChecks["grid"] = h.gridService.Health(ctx)
	} else {
		components["grid"] = healthNotInitialized
		hasErrors = true
	}

	if h.actionService != nil {
		serviceChecks["action"] = h.actionService.Health(ctx)
	} else {
		components["action"] = healthNotInitialized
		hasErrors = true
	}

	if h.scrollService != nil {
		serviceChecks["scroll"] = h.scrollService.Health(ctx)
	} else {
		components["scroll"] = healthNotInitialized
		hasErrors = true
	}
	// Flatten service sub-checks into components map
	for service, checks := range serviceChecks {
		for check, err := range checks {
			key := service + "." + check

			if err != nil {
				components[key] = err.Error()
				hasErrors = true

				h.logger.Warn("Health check failed",
					zap.String("service", service),
					zap.String("check", check),
					zap.Error(err))
			} else {
				components[key] = "ok"
			}
		}
	}

	// --- metadata -----------------------------------------------------------
	configPath := h.ResolveConfigPath()

	mode := ""
	if h.appState != nil {
		mode = domain.ModeString(h.appState.CurrentMode())
	}

	data := map[string]any{
		"version":    ipc.BuildVersion(),
		"config":     configPath,
		"mode":       mode,
		"components": components,
	}

	response := ipc.Response{
		Success: !hasErrors,
		Data:    data,
		Code:    ipc.CodeOK,
	}

	if hasErrors {
		response.Code = ipc.CodeActionFailed
	}

	return response
}
