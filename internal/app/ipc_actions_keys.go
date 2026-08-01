package app

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

func (h *IPCControllerActions) handleFeedAction(
	ctx context.Context,
	args []string,
) ipc.Response {
	if len(args) == 0 {
		return ipc.Response{
			Success: false,
			Message: "feed requires at least one key (e.g., action feed o, action feed ctrl+c)",
			Code:    ipc.CodeInvalidInput,
		}
	}

	// Check for --mode flag passed via IPC (from hotkey action strings).
	// When coming from the CLI, cobra strips the flag before it reaches this handler.
	if args[0] == "--mode" {
		return h.handleFeedToModeAction(args[1:])
	}

	keys := make([]string, 0, len(args))
	for _, arg := range args {
		key := strings.TrimSpace(arg)
		if key == "" {
			return ipc.Response{
				Success: false,
				Message: "feed keys cannot be empty",
				Code:    ipc.CodeInvalidInput,
			}
		}

		keys = append(keys, key)
	}

	h.logger.Debug("Feeding keys via IPC", zap.Int("key_count", len(keys)))

	for index, key := range keys {
		err := h.keyFeed.Feed(ctx, key)
		if err != nil {
			code := ipc.CodeActionFailed
			if derrors.IsCode(err, derrors.CodeInvalidInput) {
				code = ipc.CodeInvalidInput
			}

			h.logger.Error("Failed to feed key",
				zap.Error(err),
				zap.String("key", key),
				zap.Int("index", index))

			return ipc.Response{
				Success: false,
				Message: fmt.Sprintf("failed to feed key %s: %s", key, err.Error()),
				Code:    code,
			}
		}
	}

	return ipc.Response{
		Success: true,
		Message: "feed performed",
		Code:    ipc.CodeOK,
	}
}

func (h *IPCControllerActions) handleFeedToModeAction(args []string) ipc.Response {
	if len(args) == 0 {
		return ipc.Response{
			Success: false,
			Message: "feed-mode requires at least one key (e.g., action feed --mode o, action feed --mode ctrl+c)",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if h.modesHandler == nil {
		return ipc.Response{
			Success: false,
			Message: msgModesHandlerNotAvailable,
			Code:    ipc.CodeActionFailed,
		}
	}

	keys := make([]string, 0, len(args))
	for _, arg := range args {
		key := strings.TrimSpace(arg)
		if key == "" {
			return ipc.Response{
				Success: false,
				Message: "feed-mode keys cannot be empty",
				Code:    ipc.CodeInvalidInput,
			}
		}

		keys = append(keys, key)
	}

	h.logger.Debug("Feeding keys to mode system", zap.Int("key_count", len(keys)))

	for _, key := range keys {
		normalized := config.CanonicalHotkeyForPlatform(key)
		if normalized == "" {
			return ipc.Response{
				Success: false,
				Message: fmt.Sprintf("invalid key: %q", key),
				Code:    ipc.CodeInvalidInput,
			}
		}

		h.modesHandler.HandleFedKeyPress(normalized)
	}

	return ipc.Response{
		Success: true,
		Message: "feed-mode performed",
		Code:    ipc.CodeOK,
	}
}

func (h *IPCControllerActions) handleBackspaceAction(parsed parsedActionArgs) ipc.Response {
	if hasUnsupportedFlags(parsed) {
		return ipc.Response{
			Success: false,
			Message: "backspace does not support action flags",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if h.modesHandler == nil {
		return ipc.Response{
			Success: false,
			Message: msgModesHandlerNotAvailable,
			Code:    ipc.CodeActionFailed,
		}
	}

	h.modesHandler.BackspaceCurrentMode()

	return ipc.Response{Success: true, Message: "mode backspace", Code: ipc.CodeOK}
}

// handleCycleHintAction cycles through visible hints in hints mode.
func (h *IPCControllerActions) handleCycleHintAction(
	ctx context.Context,
	parsed parsedActionArgs,
) ipc.Response {
	if parsed.hasX || parsed.hasY || parsed.hasDX || parsed.hasDY ||
		parsed.hasCenter || parsed.hasWindow || parsed.useSelection ||
		parsed.useBare || parsed.hasMonitorName || parsed.usePrevious ||
		parsed.modifierStr != "" {
		return ipc.Response{
			Success: false,
			Message: "cycle_hint does not support these flags",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if h.modesHandler == nil {
		return ipc.Response{
			Success: false,
			Message: msgModesHandlerNotAvailable,
			Code:    ipc.CodeActionFailed,
		}
	}

	h.logger.Debug("Cycling hints via IPC",
		zap.Bool("backward", parsed.useBackward),
	)

	err := h.modesHandler.CycleHint(ctx, parsed.useBackward, false)
	if err != nil {
		h.logger.Error("Failed to cycle hints", zap.Error(err))

		return ipc.Response{
			Success: false,
			Message: "failed to cycle hints: " + err.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	return ipc.Response{
		Success: true,
		Message: "cycle_hint performed",
		Code:    ipc.CodeOK,
	}
}

// handleSearchHintsAction activates text search in hints mode.
func (h *IPCControllerActions) handleSearchHintsAction(parsed parsedActionArgs) ipc.Response {
	if hasUnsupportedFlags(parsed) || parsed.useBackward || parsed.hasWindow {
		return ipc.Response{
			Success: false,
			Message: "search_hints does not support action flags",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if h.modesHandler == nil {
		return ipc.Response{
			Success: false,
			Message: msgModesHandlerNotAvailable,
			Code:    ipc.CodeActionFailed,
		}
	}

	err := h.modesHandler.StartHintSearch()
	if err != nil {
		h.logger.Error("Failed to start hint search", zap.Error(err))

		return ipc.Response{
			Success: false,
			Message: "failed to start hint search: " + err.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	return ipc.Response{
		Success: true,
		Message: "search_hints activated",
		Code:    ipc.CodeOK,
	}
}
