// Config mutation handlers: set, set without reload, reset, and
// in-memory-only set. Reads and status live in info.go; health and
// capability reporting in info_health.go.

package ipcctrl

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/config"
)

func (h *InfoHandler) handleConfigSet(ctx context.Context, cmd ipc.Command) ipc.Response {
	if len(cmd.Args) < minConfigSetArgs {
		return ipc.Response{
			Success: false,
			Message: "config-set requires key and value arguments",
			Code:    ipc.CodeInvalidInput,
		}
	}

	key := cmd.Args[0]
	value := cmd.Args[1]
	noReload := len(cmd.Args) > minConfigSetArgs && cmd.Args[minConfigSetArgs] == "--no-reload"

	if noReload {
		// Lightweight path: update in-memory config and persist without
		// disrupting active hotkeys or exiting the current mode. Useful
		// when setting multiple fields in sequence. Caller should run
		// "neru config reload" afterward to apply all changes.
		return h.handleConfigSetNoReload(ctx, key, value)
	}

	// Full path: delegate to the app-level callback for reconfiguration
	// (component updates, hotkey re-registration, etc.).
	if h.setConfigField != nil {
		err := h.setConfigField(ctx, key, value)
		if err != nil {
			return ipc.Response{
				Success: false,
				Message: err.Error(),
				Code:    ipc.CodeActionFailed,
			}
		}

		return ipc.Response{
			Success: true,
			Message: fmt.Sprintf("config %s set to %q", key, value),
			Code:    ipc.CodeOK,
		}
	}

	// Fallback (e.g. tests with no callback): update in-memory config only.
	return h.handleConfigSetInMemory(ctx, key, value)
}

// handleConfigSetNoReload updates the in-memory config and persists to the
// override file without triggering mode exit or hotkey re-registration.
func (h *InfoHandler) handleConfigSetNoReload(
	_ context.Context,
	key, value string,
) ipc.Response {
	cfg := h.configSnapshot()
	if cfg == nil {
		return h.configNotAvailableResponse()
	}

	newCfg, err := config.DeepCopyConfig(cfg)
	if err != nil {
		return ipc.Response{
			Success: false,
			Message: "failed to copy config: " + err.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	setErr := config.SetField(newCfg, key, value)
	if setErr != nil {
		return ipc.Response{
			Success: false,
			Message: setErr.Error(),
			Code:    ipc.CodeInvalidInput,
		}
	}

	// Skip Validate() here so interdependent fields (e.g. grid_cols + keys)
	// can be updated incrementally before a final "neru config reload".
	h.configService.Replace(newCfg)
	h.configMu.Lock()
	h.config = newCfg
	h.configMu.Unlock()

	// Persist to override file so changes survive restart.
	persistErr := h.configService.SaveOverrideField(key, value)
	if persistErr != nil {
		return ipc.Response{
			Success: false,
			Message: "config set but failed to persist: " + persistErr.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	return ipc.Response{
		Success: true,
		Message: fmt.Sprintf("config %s set to %q (reload required to apply)", key, value),
		Code:    ipc.CodeOK,
	}
}

// handleConfigReset removes a field from the override file, reverting it to
// the base config or default value on the next reload.  When --no-reload is
// present the in-memory config is left alone so callers can batch multiple
// resets before a final "neru config reload".
func (h *InfoHandler) handleConfigReset(ctx context.Context, cmd ipc.Command) ipc.Response {
	if len(cmd.Args) < 1 {
		return ipc.Response{
			Success: false,
			Message: "config-reset requires a key argument",
			Code:    ipc.CodeInvalidInput,
		}
	}

	key := cmd.Args[0]
	noReload := len(cmd.Args) > 1 && cmd.Args[1] == "--no-reload"

	if config.ConfigFieldType(key) == "unknown" {
		return ipc.Response{
			Success: false,
			Message: "unknown config field: " + key,
			Code:    ipc.CodeInvalidInput,
		}
	}

	removeErr := h.configService.RemoveOverrideField(key)
	if removeErr != nil {
		return ipc.Response{
			Success: false,
			Message: "failed to remove override: " + removeErr.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	if noReload {
		return ipc.Response{
			Success: true,
			Message: fmt.Sprintf("config %s reset (reload required to apply)", key),
			Code:    ipc.CodeOK,
		}
	}

	// Full reload to apply the reset immediately.
	return h.handleReloadConfig(ctx, ipc.Command{})
}

// handleConfigSetInMemory updates the in-memory config without persisting.
// This is the fallback path used when no app-level callback is registered
// (e.g. in tests).
func (h *InfoHandler) handleConfigSetInMemory(
	_ context.Context,
	key, value string,
) ipc.Response {
	cfg := h.configSnapshot()
	if cfg == nil {
		return h.configNotAvailableResponse()
	}

	newCfg, err := config.DeepCopyConfig(cfg)
	if err != nil {
		h.logger.Error("Failed to deep copy config", zap.Error(err))

		return ipc.Response{
			Success: false,
			Message: "failed to copy config: " + err.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	setErr := config.SetField(newCfg, key, value)
	if setErr != nil {
		return ipc.Response{
			Success: false,
			Message: setErr.Error(),
			Code:    ipc.CodeInvalidInput,
		}
	}

	updateErr := h.configService.Update(newCfg)
	if updateErr != nil {
		h.logger.Error("Failed to update config service", zap.Error(updateErr))

		return ipc.Response{
			Success: false,
			Message: "failed to apply config: " + updateErr.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	h.configMu.Lock()
	h.config = newCfg
	h.configMu.Unlock()

	return ipc.Response{
		Success: true,
		Message: fmt.Sprintf("config %s set to %q", key, value),
		Code:    ipc.CodeOK,
	}
}

// configNotAvailableResponse returns a standardized response for when the
// config is nil. Extracted as a helper to avoid repeating the string literal.
func (h *InfoHandler) configNotAvailableResponse() ipc.Response {
	return ipc.Response{
		Success: false,
		Message: "config not available",
		Code:    ipc.CodeActionFailed,
	}
}
