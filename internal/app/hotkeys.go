package app

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

const (
	globalHotkeyRepeatInitialDelay = 50 * time.Millisecond
	globalHotkeyRepeatInterval     = 50 * time.Millisecond
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

		a.logger.Debug(
			"Registering hotkey binding",
			zap.String("key", trimmedKey),
			zap.Int("action_count", len(actions)),
		)

		bindKey := config.CanonicalHotkeyForPlatform(trimmedKey)
		bindActions := actions

		var registerHotkeyErr error

		if releaseManager, ok := a.hotkeyManager.(HotkeyReleaseService); ok &&
			a.hotkeyActionsRepeatWhileHeld(bindActions) {
			_, registerHotkeyErr = releaseManager.RegisterWithRelease(
				bindKey,
				func() {
					a.startHotkeyRepeat(bindKey, bindActions)
				},
				func() {
					a.stopHotkeyRepeat(bindKey)
				},
			)
		} else {
			_, registerHotkeyErr = a.hotkeyManager.Register(bindKey, func() {
				a.dispatchHotkeyActionsAsync(bindKey, bindActions)
			})
		}

		if registerHotkeyErr != nil {
			a.logger.Error(
				"Failed to register hotkey binding",
				zap.String("key", trimmedKey),
				zap.Int("action_count", len(actions)),
				zap.Error(registerHotkeyErr),
			)

			continue
		}
	}
}

func (a *App) dispatchHotkeyActionsAsync(key string, actions []string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				a.logger.Error(
					"panic in hotkey handler",
					zap.Any("recover", r),
					zap.String("key", key),
				)
			}
		}()

		a.dispatchHotkeyActions(key, actions)
	}()
}

func (a *App) dispatchHotkeyActions(key string, actions []string) {
	for _, actionStr := range actions {
		trimmedAction := strings.TrimSpace(actionStr)
		if trimmedAction == "" {
			continue
		}

		executeHotkeyActionErr := a.executeHotkeyAction(key, trimmedAction)
		if executeHotkeyActionErr != nil {
			a.logger.Error("hotkey action failed",
				zap.String("key", key),
				zap.String("action", trimmedAction),
				zap.Error(executeHotkeyActionErr))
		}
	}
}

func (a *App) startHotkeyRepeat(key string, actions []string) {
	ctx, cancel := context.WithCancel(a.ctx)

	a.hotkeyRepeatMu.Lock()

	if a.hotkeyRepeatCancels == nil {
		a.hotkeyRepeatCancels = make(map[string]context.CancelFunc)
	}

	oldCancel := a.hotkeyRepeatCancels[key]
	if oldCancel != nil {
		delete(a.hotkeyRepeatCancels, key)
	}

	a.hotkeyRepeatCancels[key] = cancel
	a.hotkeyRepeatMu.Unlock()

	if oldCancel != nil {
		oldCancel()
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				a.logger.Error(
					"panic in repeating hotkey handler",
					zap.Any("recover", r),
					zap.String("key", key),
				)
			}
		}()

		a.dispatchHotkeyActions(key, actions)

		initialTimer := time.NewTimer(globalHotkeyRepeatInitialDelay)
		defer initialTimer.Stop()

		select {
		case <-ctx.Done():
			return
		case <-initialTimer.C:
		}

		ticker := time.NewTicker(globalHotkeyRepeatInterval)
		defer ticker.Stop()

		for {
			a.dispatchHotkeyActions(key, actions)

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (a *App) stopHotkeyRepeat(key string) {
	a.hotkeyRepeatMu.Lock()

	cancel := a.hotkeyRepeatCancels[key]
	if cancel != nil {
		delete(a.hotkeyRepeatCancels, key)
	}
	a.hotkeyRepeatMu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (a *App) stopAllHotkeyRepeats() {
	a.hotkeyRepeatMu.Lock()
	cancels := a.hotkeyRepeatCancels
	a.hotkeyRepeatCancels = nil
	a.hotkeyRepeatMu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

func (a *App) hotkeyActionsRepeatWhileHeld(actions []string) bool {
	if len(actions) != 1 {
		return false
	}

	parts := splitArgs(strings.TrimSpace(actions[0]))
	if len(parts) < 2 || parts[0] != "action" {
		return false
	}

	return action.IsHeldRepeatAction(action.Name(parts[1]))
}

func hotkeyModifiersFromKey(key string) action.Modifiers {
	var mods action.Modifiers

	for part := range strings.SplitSeq(config.NormalizeKeyForComparison(key), "+") {
		switch strings.TrimSpace(part) {
		case "cmd":
			mods |= action.ModCmd
		case "shift":
			mods |= action.ModShift
		case "alt":
			mods |= action.ModAlt
		case "ctrl":
			mods |= action.ModCtrl
		}
	}

	return mods
}

func splitArgs(input string) []string {
	var args []string

	var current strings.Builder

	inSingleQuote := false
	inDoubleQuote := false

	for _, char := range input {
		switch char {
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			} else {
				current.WriteRune(char)
			}
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			} else {
				current.WriteRune(char)
			}
		case ' ':
			if inSingleQuote || inDoubleQuote {
				current.WriteRune(char)
			} else if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// executeHotkeyAction executes a hotkey action, which can be either a shell command or an IPC command.
func (a *App) executeHotkeyAction(key, actionStr string) error {
	actionStr = strings.TrimSpace(actionStr)

	if actionStr == action.PrefixExec || strings.HasPrefix(actionStr, action.PrefixExec+" ") {
		return a.executeShellCommand(key, actionStr)
	}

	actionParts := splitArgs(actionStr)
	actionStr = actionParts[0]
	params := actionParts[1:]

	if a.modes != nil {
		switch actionStr {
		case domain.ModeString(domain.ModeHints),
			domain.ModeString(domain.ModeGrid),
			domain.ModeString(domain.ModeRecursiveGrid),
			domain.ModeString(domain.ModeScroll):
			hotkeyMods := hotkeyModifiersFromKey(key)
			a.modes.SuppressModifiersUntilReleased(hotkeyMods)
		}
	}

	ipcResponse := a.ipcController.HandleCommand(
		a.ctx,
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

	ctx, cancel := context.WithTimeout(a.ctx, domain.ShellCommandTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, "/bin/bash", "-lc", cmdString) //nolint:gosec

	commandOutput, commandErr := command.CombinedOutput()
	if commandErr != nil {
		a.logger.Error(
			"hotkey exec failed",
			zap.String("key", key),
			zap.Int("cmd_length", len(cmdString)),
			zap.Error(commandErr),
		)

		return derrors.Wrap(commandErr, derrors.CodeInternal, "hotkey exec failed")
	}

	a.logger.Debug(
		"hotkey exec completed",
		zap.String("key", key),
		zap.Int("cmd_length", len(cmdString)),
		zap.Int("output_bytes", len(commandOutput)),
	)

	return nil
}

// refreshHotkeysForAppOrCurrent manages hotkey registration based on Neru's enabled state
// and whether the currently focused application is excluded.
func (a *App) refreshHotkeysForAppOrCurrent(bundleID string) {
	if !a.appState.IsEnabled() {
		if a.appState.HotkeysRegistered() {
			a.logger.Debug("Neru disabled; unregistering hotkeys")
			a.stopAllHotkeyRepeats()
			a.hotkeyManager.UnregisterAll()
			a.appState.SetHotkeysRegistered(false)
		}

		return
	}

	if bundleID == "" {
		// Use ActionService to get focused bundle ID
		ctx := a.ctx

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
			a.logger.Debug("Focused app excluded; unregistering global hotkeys",
				zap.String("bundle_id", bundleID))
			a.stopAllHotkeyRepeats()
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
