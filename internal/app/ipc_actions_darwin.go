//go:build darwin

package app

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/infra/accessibility"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

func (h *IPCControllerActions) handleFocusWindowAction(
	ctx context.Context,
	parsed parsedActionArgs,
) ipc.Response {
	if parsed.hasX || parsed.hasY || parsed.hasDX || parsed.hasDY ||
		parsed.hasCenter || parsed.hasWindow || parsed.useSelection ||
		parsed.useBare || parsed.hasMonitorName || parsed.usePrevious ||
		parsed.modifierStr != "" || parsed.hasSteps {
		return ipc.Response{
			Success: false,
			Message: "focus_window does not support these flags",
			Code:    ipc.CodeInvalidInput,
		}
	}

	h.logger.Debug("Cycling window focus via IPC",
		zap.Bool("backward", parsed.useBackward),
	)

	windows, err := accessibility.AllFocusableWindowsOnActiveSpace()
	if err != nil {
		h.logger.Error("Failed to get focusable windows", zap.Error(err))

		return ipc.Response{
			Success: false,
			Message: "failed to get focusable windows: " + err.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	if len(windows) == 0 {
		return ipc.Response{
			Success: false,
			Message: "no focusable windows found on the active space",
			Code:    ipc.CodeActionFailed,
		}
	}

	defer accessibility.ReleaseAll(windows)

	// Find the currently focused window index using FrontmostWindow
	// (which uses kAXFocusedWindowAttribute on the app element) rather than
	// per-window IsFocused(), since kAXFocusedAttribute on window elements
	// is unreliable across applications.
	frontmost := accessibility.FrontmostWindow()

	var currentIndex int
	if frontmost != nil {
		for i, w := range windows {
			if w.Equal(frontmost) {
				currentIndex = i

				break
			}
		}

		frontmost.Release()
	}

	// Default to the first window if none is detected as focused.
	var targetIndex int
	if parsed.useBackward {
		targetIndex = currentIndex - 1
		if targetIndex < 0 {
			targetIndex = len(windows) - 1
		}
	} else {
		targetIndex = currentIndex + 1
		if targetIndex >= len(windows) {
			targetIndex = 0
		}
	}

	h.logger.Debug("Focusing window",
		zap.Int("currentIndex", currentIndex),
		zap.Int("targetIndex", targetIndex),
		zap.Int("totalWindows", len(windows)),
	)

	activateErr := windows[targetIndex].ActivateWindow()
	if activateErr != nil {
		h.logger.Error("Failed to activate window", zap.Error(activateErr))

		return ipc.Response{
			Success: false,
			Message: "failed to activate window: " + activateErr.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	return ipc.Response{
		Success: true,
		Message: "focus_window performed",
		Code:    ipc.CodeOK,
	}
}
