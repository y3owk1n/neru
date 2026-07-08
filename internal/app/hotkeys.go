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

		// When the backend can report key releases, register every hotkey through
		// the release path and decide press-by-press whether to repeat. The
		// repeat-vs-once choice depends on the effective binding (the per-mode
		// override when the active mode binds this key, otherwise the global
		// binding), which is only known at press time — so it cannot be made here,
		// at registration, from the global action alone.
		if releaseManager, ok := a.hotkeyManager.(HotkeyReleaseService); ok {
			_, registerHotkeyErr = releaseManager.RegisterWithRelease(
				bindKey,
				func() {
					a.dispatchModeAwareHeldHotkey(bindKey, bindActions)
				},
				func() {
					a.stopHotkeyRepeat(bindKey)
				},
			)
		} else {
			// Backend without release events (currently Windows and Linux): held-key
			// repeat is not possible, so a single mode-aware dispatch is the whole
			// behavior.
			_, registerHotkeyErr = a.hotkeyManager.Register(bindKey, func() {
				a.dispatchModeAwareHotkeyAsync(bindKey, bindActions)
			})
		}

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

// dispatchModeAwareHotkeyAsync dispatches a global hotkey once, applying any
// per-mode binding for the same key while a navigation mode is active.
//
// Global hotkeys fire through an always-on, per-hotkey event tap that is
// separate from the event tap serving per-mode hotkeys. When a mode is active
// and the pressed key is bound both globally and by that mode, the global tap
// runs the global action and consumes the event before the mode tap can apply
// the mode-specific binding. Resolving the mode binding here makes the more
// specific per-mode binding win. Falls back to the global actions when the
// active mode does not bind the key (or when idle).
//
// This is the single-dispatch path used by hotkey backends that cannot report
// key releases (and therefore cannot repeat while held). Backends that can
// report releases use dispatchModeAwareHeldHotkey, which applies the same
// override resolution and additionally repeats held-repeatable actions.
func (a *App) dispatchModeAwareHotkeyAsync(key string, globalActions []string) {
	actions := globalActions

	if a.modes != nil {
		if overrideActions, ok := a.modes.ModeHotkeyOverride(key); ok {
			actions = overrideActions
		}
	}

	a.dispatchHotkeyActionsAsync(key, actions)
}

// dispatchModeAwareHeldHotkey handles a global hotkey press on backends that
// report key releases. It resolves the effective binding (the per-mode override
// when the active mode binds the key, otherwise the global binding) and
// dispatches it, repeating while held only when that effective binding is a
// single held-repeatable action and held-key repeat is enabled. The matching
// release callback (stopHotkeyRepeat) cancels any repeat this started; it is a
// no-op when nothing was started.
func (a *App) dispatchModeAwareHeldHotkey(key string, globalActions []string) {
	var (
		override    []string
		hasOverride bool
	)

	if a.modes != nil {
		override, hasOverride = a.modes.ModeHotkeyOverride(key)
	}

	actions, repeat := a.effectiveHeldHotkey(hasOverride, override, globalActions, a.configSnapshot())
	if repeat {
		a.startHotkeyRepeat(key, actions)

		return
	}

	a.dispatchHotkeyActionsAsync(key, actions)
}

// effectiveHeldHotkey resolves which actions a global hotkey press should run
// and whether they should repeat while held. The per-mode override wins over
// the global binding when present, and the repeat decision is then made from
// the resolved actions — not from the global binding — so a per-mode override
// takes precedence on the held-repeat path too.
func (a *App) effectiveHeldHotkey(
	hasOverride bool,
	override, globalActions []string,
	cfg *config.Config,
) ([]string, bool) {
	actions := globalActions
	if hasOverride {
		actions = override
	}

	return actions, a.hotkeyActionsRepeatWhileHeld(actions, cfg)
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
			if derrors.IsCode(executeHotkeyActionErr, derrors.CodeChainBail) {
				return
			}

			a.logger.Error("hotkey action failed",
				zap.String("key", key),
				zap.String("action", trimmedAction),
				zap.Error(executeHotkeyActionErr))
		}
	}
}

func (a *App) startHotkeyRepeat(key string, actions []string) {
	cfg := a.configSnapshot().HeldRepeat
	if !cfg.Enabled {
		return
	}

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

		initialTimer := time.NewTimer(time.Duration(cfg.InitialDelay) * time.Millisecond)
		defer initialTimer.Stop()

		select {
		case <-ctx.Done():
			return
		case <-initialTimer.C:
		}

		ticker := time.NewTicker(time.Duration(cfg.Interval) * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.dispatchHotkeyActions(key, actions)
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

func (a *App) hotkeyActionsRepeatWhileHeld(actions []string, cfg *config.Config) bool {
	if !cfg.HeldRepeat.Enabled {
		return false
	}

	if len(actions) != 1 {
		return false
	}

	parts := splitArgs(strings.TrimSpace(actions[0]))
	if len(parts) < 2 || parts[0] != actionCmd {
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
			domain.ModeString(domain.ModeScroll),
			domain.ModeString(domain.ModeMonitorSelect):
			hotkeyMods := hotkeyModifiersFromKey(key)
			a.modes.SuppressModifiersUntilReleased(hotkeyMods)
		}
	}

	ipcResponse := a.ipcController.HandleCommand(
		a.ctx,
		ipc.Command{Action: actionStr, Args: params},
	)
	if !ipcResponse.Success {
		if ipcResponse.Code == ipc.CodeChainBail {
			return derrors.New(derrors.CodeChainBail, ipcResponse.Message)
		}

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

	cfg := a.configSnapshot()
	shell := cfg.General.ExecShell
	shellArgs := cfg.General.ExecShellArgs

	args := make([]string, 0, len(shellArgs)+1)
	args = append(args, shellArgs...)
	args = append(args, cmdString)

	command := exec.CommandContext(ctx, shell, args...) //nolint:gosec

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
	a.hotkeyRegistrationMu.Lock()
	defer a.hotkeyRegistrationMu.Unlock()

	if !a.appState.IsEnabled() {
		if a.appState.HotkeysRegistered() {
			a.logger.Debug("Neru disabled; unregistering hotkeys")
			a.stopAllHotkeyRepeats()
			a.hotkeyManager.UnregisterAll()
			a.appState.SetHotkeysRegistered(false)
		}

		return
	}

	cfg := a.configSnapshot()

	if bundleID == "" {
		// Use ActionService to get focused bundle ID
		ctx := a.ctx

		var bundleIDErr error

		bundleID, bundleIDErr = a.actionService.FocusedAppBundleID(ctx)
		if bundleIDErr != nil {
			// Fail open: when the focused app can't be determined (always the
			// case on Linux Wayland, which has no focus-query API), fall
			// through with an empty bundle ID so global hotkeys still register.
			// Failing closed would permanently disable Neru on those platforms.
			// The next focus event re-evaluates per-app exclusion on platforms
			// that support it. When exclusions are configured but cannot be
			// enforced, warn so the user knows; otherwise this is routine.
			logFn := a.logger.Debug
			if len(cfg.General.ExcludedApps) > 0 {
				logFn = a.logger.Warn
			}

			logFn(
				"Focused app unknown; registering global hotkeys without per-app exclusion (configured excluded_apps are not enforced)",
				zap.Int("excluded_apps", len(cfg.General.ExcludedApps)),
				zap.Error(bundleIDErr),
			)

			bundleID = ""
		}
	}

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
