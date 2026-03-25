package app

import (
	"context"
	"os/exec"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

// actionsReferenceDisabledMode reports whether any action in the list
// activates a mode that is currently disabled in the configuration.
// This ensures that multi-action bindings like ["exec echo test", "hints"]
// are skipped entirely when hints is disabled, rather than only checking
// the first action.
func actionsReferenceDisabledMode(actions []string, cfg *config.Config) bool {
	hintsStr := domain.ModeString(domain.ModeHints)
	gridStr := domain.ModeString(domain.ModeGrid)

	recursiveGridStr := domain.ModeString(domain.ModeRecursiveGrid)
	for _, actionStr := range actions {
		trimmed := strings.TrimSpace(actionStr)
		if trimmed == "" {
			continue
		}

		mode := strings.Split(trimmed, " ")[0]
		switch {
		case mode == hintsStr && !cfg.Hints.Enabled:
			return true
		case mode == gridStr && !cfg.Grid.Enabled:
			return true
		case mode == recursiveGridStr && !cfg.RecursiveGrid.Enabled:
			return true
		}
	}

	return false
}

// registerHotkeys registers all global hotkeys defined in the configuration.
func (a *App) registerHotkeys() {
	cfg := a.configSnapshot()

	for key, actions := range cfg.Hotkeys.Bindings {
		trimmedKey := strings.TrimSpace(key)

		if trimmedKey == "" || len(actions) == 0 {
			continue
		}

		if actionsReferenceDisabledMode(actions, cfg) {
			continue
		}

		a.logger.Info(
			"Registering hotkey binding",
			zap.String("key", trimmedKey),
			zap.Strings("actions", actions),
		)

		bindKey := trimmedKey
		bindActions := actions

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

				for _, actionStr := range bindActions {
					trimmedAction := strings.TrimSpace(actionStr)
					if trimmedAction == "" {
						continue
					}

					executeHotkeyActionErr := a.executeHotkeyAction(bindKey, trimmedAction)
					if executeHotkeyActionErr != nil {
						a.logger.Error("hotkey action failed",
							zap.String("key", bindKey),
							zap.String("action", trimmedAction),
							zap.Error(executeHotkeyActionErr))
					}
				}
			}()
		})
		if registerHotkeyErr != nil {
			a.logger.Error(
				"Failed to register hotkey binding",
				zap.String("key", trimmedKey),
				zap.Strings("actions", actions),
				zap.Error(registerHotkeyErr),
			)

			continue
		}
	}
}

// executeHotkeyAction executes a hotkey action, which can be either a shell command or an IPC command.
func (a *App) executeHotkeyAction(key, actionStr string) error {
	actionStr = strings.TrimSpace(actionStr)

	if actionStr == action.PrefixExec || strings.HasPrefix(actionStr, action.PrefixExec+" ") {
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
	cmdString := strings.TrimSpace(strings.TrimPrefix(actionStr, action.PrefixExec))
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

	cfg := a.configSnapshot()

	if cfg.IsAppExcluded(bundleID) {
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
