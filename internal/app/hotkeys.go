package app

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/infra/ipc"
	"go.uber.org/zap"
)

// registerHotkeys registers all global hotkeys defined in the configuration.
func (a *App) registerHotkeys() {
	// Note: Escape key for exiting modes is hardcoded in handleKeyPress, not registered as global hotkey

	for k, v := range a.config.Hotkeys.Bindings {
		key := strings.TrimSpace(k)
		action := strings.TrimSpace(v)
		if key == "" || action == "" {
			continue
		}
		mode := action
		if parts := strings.Split(action, " "); len(parts) > 0 {
			mode = parts[0]
		}
		if mode == domain.GetModeString(domain.ModeHints) && !a.config.Hints.Enabled {
			continue
		}
		if mode == domain.GetModeString(domain.ModeGrid) && !a.config.Grid.Enabled {
			continue
		}

		a.logger.Info(
			"Registering hotkey binding",
			zap.String("key", key),
			zap.String("action", action),
		)

		bindKey := key
		bindAction := action

		var err error
		_, err = a.hotkeyManager.Register(bindKey, func() {
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

				err := a.executeHotkeyAction(bindKey, bindAction)
				if err != nil {
					a.logger.Error("hotkey action failed",
						zap.String("key", bindKey),
						zap.String("action", bindAction),
						zap.Error(err))
				}
			}()
		})
		if err != nil {
			a.logger.Error(
				"Failed to register hotkey binding",
				zap.String("key", key),
				zap.String("action", action),
				zap.Error(err),
			)
			continue
		}
	}
}

// executeHotkeyAction executes a hotkey action, which can be either a shell command or an IPC command.
func (a *App) executeHotkeyAction(key, action string) error {
	if strings.HasPrefix(action, domain.ActionPrefixExec) {
		return a.executeShellCommand(key, action)
	}

	actionParts := strings.Split(action, " ")
	action = actionParts[0]
	params := actionParts[1:]

	resp := a.handleIPCCommand(context.Background(), ipc.Command{Action: action, Args: params})
	if !resp.Success {
		return errors.New(resp.Message)
	}

	a.logger.Info("hotkey action executed", zap.String("key", key), zap.String("action", action))
	return nil
}

// executeShellCommand executes a shell command triggered by a hotkey.
func (a *App) executeShellCommand(key, action string) error {
	cmdStr := strings.TrimSpace(strings.TrimPrefix(action, "exec"))
	if cmdStr == "" {
		a.logger.Error("hotkey exec has empty command", zap.String("key", key))
		return errors.New("empty command")
	}

	a.logger.Debug(
		"Executing shell command from hotkey",
		zap.String("key", key),
		zap.String("cmd", cmdStr),
	)
	ctx, cancel := context.WithTimeout(context.Background(), domain.ShellCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/bash", "-lc", cmdStr) //nolint:gosec
	out, err := cmd.CombinedOutput()
	if err != nil {
		a.logger.Error(
			"hotkey exec failed",
			zap.String("key", key),
			zap.String("cmd", cmdStr),
			zap.ByteString("output", out),
			zap.Error(err),
		)
		return fmt.Errorf("hotkey exec failed: %w", err)
	}

	a.logger.Info(
		"hotkey exec completed",
		zap.String("key", key),
		zap.String("cmd", cmdStr),
		zap.ByteString("output", out),
	)
	return nil
}

// refreshHotkeysForAppOrCurrent manages hotkey registration based on Neru's enabled state
// and whether the currently focused application is excluded.
func (a *App) refreshHotkeysForAppOrCurrent(bundleID string) {
	if !a.state.IsEnabled() {
		if a.state.HotkeysRegistered() {
			a.logger.Debug("Neru disabled; unregistering hotkeys")
			a.hotkeyManager.UnregisterAll()
			a.state.SetHotkeysRegistered(false)
		}
		return
	}

	if bundleID == "" {
		// Use ActionService to get focused bundle ID
		ctx := context.Background()
		var err error
		bundleID, err = a.actionService.GetFocusedAppBundleID(ctx)
		if err != nil {
			a.logger.Warn("Failed to get focused app bundle ID", zap.Error(err))
			return
		}
	}

	if a.config.IsAppExcluded(bundleID) {
		if a.state.HotkeysRegistered() {
			a.logger.Info("Focused app excluded; unregistering global hotkeys",
				zap.String("bundle_id", bundleID))
			a.hotkeyManager.UnregisterAll()
			a.state.SetHotkeysRegistered(false)
		}
		return
	}

	if !a.state.HotkeysRegistered() {
		a.registerHotkeys()
		a.state.SetHotkeysRegistered(true)
		a.logger.Debug("Hotkeys registered")
	}
}
