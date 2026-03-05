package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	"go.uber.org/zap"
)

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

func (h *IPCControllerInfo) handleStatus(_ context.Context, _ ipc.Command) ipc.Response {
	configPath := h.ResolveConfigPath()

	h.configMu.RLock()
	cfg := h.config
	h.configMu.RUnlock()

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
	h.configMu.RLock()
	cfg := h.config
	h.configMu.RUnlock()

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

func (h *IPCControllerInfo) handleReloadConfig(_ context.Context, _ ipc.Command) ipc.Response {
	configPath := h.configService.GetConfigPath()

	err := h.configService.ReloadConfig(configPath)
	if err != nil {
		h.logger.Error("Failed to reload config", zap.Error(err))

		return ipc.Response{
			Success: false,
			Message: "failed to reload config: " + err.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	// Update the local config copy so subsequent IPC queries reflect the reload.
	h.UpdateConfig(h.configService.Get())

	return ipc.Response{
		Success: true,
		Message: "config reloaded successfully",
		Code:    ipc.CodeOK,
	}
}

func (h *IPCControllerInfo) handleHealth(ctx context.Context, _ ipc.Command) ipc.Response {
	// Get raw health status with errors
	rawHealthStatus := map[string]map[string]error{
		"hints":  h.hintService.Health(ctx),
		"grid":   h.gridService.Health(ctx),
		"action": h.actionService.Health(ctx),
		"scroll": h.scrollService.Health(ctx),
	}

	// Convert to serializable structure
	healthStatus := make(map[string]map[string]string)
	for service, checks := range rawHealthStatus {
		healthStatus[service] = make(map[string]string)
		for check, err := range checks {
			if err != nil {
				healthStatus[service][check] = err.Error()
			} else {
				healthStatus[service][check] = "ok"
			}
		}
	}

	// Check if any services have errors
	hasErrors := false
	for service, checks := range rawHealthStatus {
		for check, err := range checks {
			if err != nil {
				h.logger.Warn("Health check failed",
					zap.String("service", service),
					zap.String("check", check),
					zap.Error(err))

				hasErrors = true
			}
		}
	}

	healthJSON, err := json.Marshal(healthStatus)
	if err != nil {
		h.logger.Error("Failed to marshal health status", zap.Error(err))

		return ipc.Response{
			Success: false,
			Message: "failed to marshal health status: " + err.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	response := ipc.Response{
		Success: !hasErrors,
		Message: string(healthJSON),
		Code:    ipc.CodeOK,
	}

	if hasErrors {
		response.Code = ipc.CodeActionFailed
	}

	return response
}
