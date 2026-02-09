package app

import (
	"context"
	"os/exec"
	"strings"

	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	"go.uber.org/zap"
)

// registerHotkeys registers all global hotkeys defined in the configuration.
func (a *App) registerHotkeys() {
	for key, value := range a.config.Hotkeys.Bindings {
		trimmedKey := strings.TrimSpace(key)

		action := strings.TrimSpace(value)
		if trimmedKey == "" || action == "" {
			continue
		}

		mode := action
		if parts := strings.Split(action, " "); len(parts) > 0 {
			mode = parts[0]
		}

		if mode == domain.ModeString(domain.ModeHints) && !a.config.Hints.Enabled {
			continue
		}

		if mode == domain.ModeString(domain.ModeGrid) && !a.config.Grid.Enabled {
			continue
		}

		a.logger.Info(
			"Registering hotkey binding",
			zap.String("key", trimmedKey),
			zap.String("action", action),
		)

		bindKey := trimmedKey
		bindAction := action

		var registerHotkeyErr error

		_, registerHotkeyErr = a.hotkeyManager.Register(bindKey, func() {
			// Run handler in separate goroutine so the hotkey callback returns quickly.
			go func() {
				defer func() {
					if r := recover(); r != nil {
						a.logger.Error(
							"panic in hotkey handler",
							zap.Any("recover", r),
							zap.String("key", bindKey),
						)
					}
				}()

				executeHotkeyActionErr := a.executeHotkeyAction(bindKey, bindAction)
				if executeHotkeyActionErr != nil {
					a.logger.Error("hotkey action failed",
						zap.String("key", bindKey),
						zap.String("action", bindAction),
						zap.Error(executeHotkeyActionErr))
				}
			}()
		})
		if registerHotkeyErr != nil {
			a.logger.Error(
				"Failed to register hotkey binding",
				zap.String("key", trimmedKey),
				zap.String("action", action),
				zap.Error(registerHotkeyErr),
			)

			continue
		}
	}
}

// executeHotkeyAction executes a hotkey action, which can be either a shell command or an IPC command.
func (a *App) executeHotkeyAction(key, actionStr string) error {
	if strings.HasPrefix(actionStr, action.PrefixExec) {
		return a.executeShellCommand(key, actionStr)
	}

	actionParts := strings.Split(actionStr, " ")
	actionStr = actionParts[0]
	params := actionParts[1:]

	ipcResponse := a.ipcController.HandleCommand(
		context.Background(),
		ipc.Command{Action: actionStr, Args: params},
	)
	if !ipcResponse.Success {
		return derrors.New(derrors.CodeIPCFailed, ipcResponse.Message)
	}

	a.logger.Debug(
		"hotkey action executed",
		zap.String("key", key),
		zap.String("action", actionStr),
	)

	return nil
}

// executeShellCommand executes a shell command triggered by a hotkey.
func (a *App) executeShellCommand(key, actionStr string) error {
	cmdString := strings.TrimSpace(strings.TrimPrefix(actionStr, "exec"))
	if cmdString == "" {
		a.logger.Error("hotkey exec has empty command", zap.String("key", key))

		return derrors.New(derrors.CodeInvalidInput, "empty command")
	}

	a.logger.Debug(
		"Executing shell command from hotkey",
		zap.String("key", key),
		zap.String("cmd", cmdString),
	)

	ctx, cancel := context.WithTimeout(context.Background(), domain.ShellCommandTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, "/bin/bash", "-lc", cmdString) //nolint:gosec

	commandOutput, commandErr := command.CombinedOutput()
	if commandErr != nil {
		a.logger.Error(
			"hotkey exec failed",
			zap.String("key", key),
			zap.String("cmd", cmdString),
			zap.ByteString("output", commandOutput),
			zap.Error(commandErr),
		)

		return derrors.Wrap(commandErr, derrors.CodeInternal, "hotkey exec failed")
	}

	a.logger.Info(
		"hotkey exec completed",
		zap.String("key", key),
		zap.String("cmd", cmdString),
		zap.ByteString("output", commandOutput),
	)

	return nil
}

// refreshHotkeysForAppOrCurrent manages hotkey registration based on Neru's enabled state
// and whether the currently focused application is excluded.
func (a *App) refreshHotkeysForAppOrCurrent(bundleID string) {
	if !a.appState.IsEnabled() {
		if a.appState.HotkeysRegistered() {
			a.logger.Debug("Neru disabled; unregistering hotkeys")
			a.hotkeyManager.UnregisterAll()
			a.appState.SetHotkeysRegistered(false)
		}

		return
	}

	if bundleID == "" {
		// Use ActionService to get focused bundle ID
		ctx := context.Background()

		var bundleIDErr error

		bundleID, bundleIDErr = a.actionService.FocusedAppBundleID(ctx)
		if bundleIDErr != nil {
			a.logger.Warn("Failed to get focused app bundle ID", zap.Error(bundleIDErr))

			return
		}
	}

	if a.config.IsAppExcluded(bundleID) {
		if a.appState.HotkeysRegistered() {
			a.logger.Info("Focused app excluded; unregistering global hotkeys",
				zap.String("bundle_id", bundleID))
			a.hotkeyManager.UnregisterAll()
			a.appState.SetHotkeysRegistered(false)
		}

		return
	}

	if !a.appState.HotkeysRegistered() {
		a.registerHotkeys()
		a.appState.SetHotkeysRegistered(true)
		a.logger.Debug("Hotkeys registered")
	}
}
