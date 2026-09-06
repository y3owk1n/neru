package keybinding

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/heldrepeat"
	"github.com/y3owk1n/neru/internal/app/sequence"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/ports"
)

// ActionsReferenceDisabledMode reports whether any action in the list activates
// a mode that is disabled in the config. It checks every action, not just the
// first, so ["exec echo test", "hints"] is skipped entirely when hints is off.
func ActionsReferenceDisabledMode(actions []string, cfg *config.Config) bool {
	hintsStr := domain.ModeString(domain.ModeHints)
	gridStr := domain.ModeString(domain.ModeGrid)

	recursiveGridStr := domain.ModeString(domain.ModeRecursiveGrid)

	return anyBindingStep(actions, cfg, func(step string) bool {
		switch mode := commandOf(step); {
		case mode == hintsStr && !cfg.Hints.Enabled:
			return true
		case mode == gridStr && !cfg.Grid.Enabled:
			return true
		case mode == recursiveGridStr && !cfg.RecursiveGrid.Enabled:
			return true
		default:
			return false
		}
	})
}

// registerHotkeys registers global hotkeys for the specified app bundle ID.
// When bundleID is non-empty and per-app hotkey overrides are configured, the
// app-specific bindings are used instead of the default [hotkeys] bindings.
func (b *Binder) registerHotkeys(bundleID string) {
	cfg := b.settings()

	bindings := cfg.Hotkeys.Bindings
	if bundleID != "" && cfg.HasGlobalAppHotkeyOverrides() {
		bindings = cfg.GlobalHotkeysForApp(bundleID)
	}

	// What the backend actually took, which is not what was asked for: a chord
	// another process already owns is refused, logged and skipped below. The taps
	// are told the difference because they hand a registered chord back to the
	// mechanism that owns it, and one nobody owns has to be dispatched instead
	// (see Deps.PublishRegisteredHotkeys).
	registered := make([]string, 0, len(bindings))

	defer func() {
		if b.publishRegistered != nil {
			b.publishRegistered(registered)
		}
	}()

	for key, actions := range bindings {
		trimmedKey := strings.TrimSpace(key)

		if trimmedKey == "" || len(actions) == 0 {
			continue
		}

		if ActionsReferenceDisabledMode(actions, cfg) {
			continue
		}

		b.logger.Debug(
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
		if releaseManager, ok := b.hotkeyManager.(ports.HotkeyReleaseRegistrar); ok {
			_, registerHotkeyErr = releaseManager.RegisterWithRelease(
				bindKey,
				func() {
					b.dispatchModeAwareHeldHotkey(bindKey, bindActions)
				},
				func() {
					b.stopHotkeyRepeat(bindKey)
				},
			)
		} else {
			// Backend without release events: held-key repeat is not possible, so
			// a single mode-aware dispatch is the whole behavior. Every shipped
			// manager reports releases; this is the fallback the port's optional
			// extension rules require.
			_, registerHotkeyErr = b.hotkeyManager.Register(bindKey, func() {
				b.dispatchModeAwareHotkeyAsync(bindKey, bindActions)
			})
		}

		if registerHotkeyErr != nil {
			b.logger.Error(
				"Failed to register hotkey binding",
				zap.String("key", trimmedKey),
				zap.Strings("actions", actions),
				zap.Error(registerHotkeyErr),
			)

			continue
		}

		registered = append(registered, bindKey)
	}
}

// dispatchModeAwareHotkeyAsync dispatches a global hotkey once, letting a
// per-mode binding for the same key win while a mode is active — the global
// tap consumes the event before the mode tap can see it, so the override is
// resolved here. Falls back to the global actions when the mode does not bind
// the key. This is the path for backends that cannot report key releases;
// ones that can use dispatchModeAwareHeldHotkey, which also repeats.
func (b *Binder) dispatchModeAwareHotkeyAsync(key string, globalActions []string) {
	actions := globalActions

	if b.modes != nil {
		if overrideActions, ok := b.modes.ModeHotkeyOverride(key); ok {
			actions = overrideActions
		}

		// Suppress hotkey modifiers synchronously so the per-mode event tap
		// sees them as suppressed before the debounce timer can fire.
		if actionsContainModeSwitch(actions, b.settings()) {
			b.modes.SuppressModifiersForHotkey(ModifiersFromKey(key))
		}
	}

	b.dispatchHotkeyActionsAsync(key, actions)
}

// dispatchModeAwareHeldHotkey handles a global hotkey press on backends that
// report key releases. It resolves the effective binding (the per-mode override
// when the active mode binds the key, otherwise the global binding) and
// dispatches it, repeating while held only when that effective binding is a
// single held-repeatable action and held-key repeat is enabled. The matching
// release callback (stopHotkeyRepeat) cancels any repeat this started; it is a
// no-op when nothing was started.
func (b *Binder) dispatchModeAwareHeldHotkey(key string, globalActions []string) {
	var (
		override    []string
		hasOverride bool
	)

	if b.modes != nil {
		override, hasOverride = b.modes.ModeHotkeyOverride(key)
	}

	actions, repeat := b.effectiveHeldHotkey(
		hasOverride,
		override,
		globalActions,
		b.settings(),
	)

	// Suppress hotkey modifiers synchronously so the per-mode event tap
	// sees them as suppressed before the debounce timer can fire.
	// This must happen before the async dispatch or startHotkeyRepeat.
	if b.modes != nil && actionsContainModeSwitch(actions, b.settings()) {
		b.modes.SuppressModifiersForHotkey(ModifiersFromKey(key))
	}

	if repeat && b.startMotion(key, actions) {
		return
	}

	if repeat {
		b.startHotkeyRepeat(key, actions)

		return
	}

	b.dispatchHotkeyActionsAsync(key, actions)
}

// startMotion presses key into the glide's held set when its binding
// qualifies, reporting whether it did. The release callback drops it again.
func (b *Binder) startMotion(key string, actions []string) bool {
	if b.motion == nil {
		return false
	}

	dir, step, ok := b.settings().HeldRepeat.HeldMotion(actions)
	if !ok {
		return false
	}

	b.motion.Press(key, dir, step)

	return true
}

// effectiveHeldHotkey resolves which actions a global hotkey press should run
// and whether they should repeat while held. The per-mode override wins over
// the global binding when present, and the repeat decision is then made from
// the resolved actions — not from the global binding — so a per-mode override
// takes precedence on the held-repeat path too.
func (b *Binder) effectiveHeldHotkey(
	hasOverride bool,
	override, globalActions []string,
	cfg *config.Config,
) ([]string, bool) {
	actions := globalActions
	if hasOverride {
		actions = override
	}

	return actions, b.hotkeyActionsRepeatWhileHeld(actions, cfg)
}

func (b *Binder) dispatchHotkeyActionsAsync(key string, actions []string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				b.logger.Error(
					"panic in hotkey handler",
					zap.Any("recover", r),
					zap.String("key", key),
				)
			}
		}()

		b.runActionSequence(key, actions)
	}()
}

func (b *Binder) startHotkeyRepeat(key string, actions []string) {
	cfg := b.settings().HeldRepeat
	if !cfg.Enabled {
		return
	}

	ctx, cancel := context.WithCancel(b.context())

	b.hotkeyRepeatMu.Lock()

	if b.hotkeyRepeatCancels == nil {
		b.hotkeyRepeatCancels = make(map[string]context.CancelFunc)
	}

	oldCancel := b.hotkeyRepeatCancels[key]
	if oldCancel != nil {
		delete(b.hotkeyRepeatCancels, key)
	}

	b.hotkeyRepeatCancels[key] = cancel
	b.hotkeyRepeatMu.Unlock()

	if oldCancel != nil {
		oldCancel()
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				b.logger.Error(
					"panic in repeating hotkey handler",
					zap.Any("recover", r),
					zap.String("key", key),
				)
			}
		}()

		b.runActionSequence(key, actions)

		heldrepeat.Run(ctx, cfg, actions, func(tickActions []string) {
			b.runActionSequence(key, tickActions)
		})
	}()
}

func (b *Binder) stopHotkeyRepeat(key string) {
	b.motion.Release(key)

	b.hotkeyRepeatMu.Lock()

	cancel := b.hotkeyRepeatCancels[key]
	if cancel != nil {
		delete(b.hotkeyRepeatCancels, key)
	}
	b.hotkeyRepeatMu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (b *Binder) stopAllHotkeyRepeats() {
	b.motion.ReleaseAll()

	b.hotkeyRepeatMu.Lock()
	cancels := b.hotkeyRepeatCancels
	b.hotkeyRepeatCancels = nil
	b.hotkeyRepeatMu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

func (b *Binder) hotkeyActionsRepeatWhileHeld(actions []string, cfg *config.Config) bool {
	if !cfg.HeldRepeat.Enabled {
		return false
	}

	return action.IsHeldRepeatAction(action.Name(config.HeldRepeatActionName(actions)))
}

// ModifiersFromKey reads the modifier part of a hotkey specification, so a
// mode opened by that hotkey can suppress the modifiers still physically held.
func ModifiersFromKey(key string) action.Modifiers {
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

// actionsContainModeSwitch reports whether any action in the list is a
// mode-switch action (hints, grid, recursive_grid, scroll, monitor_select).
func actionsContainModeSwitch(actions []string, cfg *config.Config) bool {
	return anyBindingStep(actions, cfg, func(step string) bool {
		// The action format is "<mode_name>" with optional args, so split
		// to get just the mode name for comparison.
		switch commandOf(step) {
		case domain.ModeString(domain.ModeHints),
			domain.ModeString(domain.ModeGrid),
			domain.ModeString(domain.ModeRecursiveGrid),
			domain.ModeString(domain.ModeScroll),
			domain.ModeString(domain.ModeMonitorSelect):
			return true
		default:
			return false
		}
	})
}

// commandOf returns the command word of an action string, ignoring its flags.
// It returns "" for a blank action.
func commandOf(actionStr string) string {
	fields := strings.Fields(strings.TrimSpace(actionStr))
	if len(fields) == 0 {
		return ""
	}

	return fields[0]
}

// maxInspectedSteps bounds how many steps a binding inspection will look at.
//
// Expansion follows nested sequences, and a binding whose macros fan out into
// each other multiplies at every level. That is a config nobody writes on
// purpose, but the inspection runs on the key-press path, so it is bounded
// rather than trusted: past the budget the answer is "no mode named here",
// the same conclusion drawn for anything else that cannot be expanded.
const maxInspectedSteps = 256

// anyBindingStep reports whether pred holds for any step the binding will run,
// looking inside the sequence constructs ("run", "macro") as deep as the
// executor will follow them.
//
// It walks rather than expanding into a slice, so a match short-circuits and
// nothing accumulates in memory — both callers only ever need the first hit.
func anyBindingStep(actions []string, cfg *config.Config, pred func(step string) bool) bool {
	budget := maxInspectedSteps

	return anyBindingStepAtDepth(actions, cfg, 0, &budget, pred)
}

// anyBindingStepAtDepth walks the steps of one sequence, recursing into nested
// ones until sequence.MaxDepth. Past that depth the executor refuses to start
// the sequence, so its steps never run and the construct is left unexpanded.
func anyBindingStepAtDepth(
	actions []string,
	cfg *config.Config,
	depth int,
	budget *int,
	pred func(step string) bool,
) bool {
	for _, actionStr := range actions {
		trimmed := strings.TrimSpace(actionStr)
		if trimmed == "" {
			continue
		}

		if *budget <= 0 {
			return false
		}

		*budget--

		nested := nestedSteps(trimmed, cfg, depth)
		if nested == nil {
			if pred(trimmed) {
				return true
			}

			continue
		}

		if anyBindingStepAtDepth(nested, cfg, depth+1, budget, pred) {
			return true
		}
	}

	return false
}

// nestedSteps returns the steps a "run" or "macro" step carries, or nil when
// the step is an ordinary action or cannot be expanded — an unknown macro, or
// nesting the executor would refuse anyway.
func nestedSteps(step string, cfg *config.Config, depth int) []string {
	if depth >= sequence.MaxDepth {
		return nil
	}

	// splitArgs applies the same quoting rules the executor uses, so the steps
	// seen here are the steps that will run.
	switch commandOf(step) {
	case domain.CommandRun:
		return splitArgs(step)[1:]
	case config.MacroCommand:
		if cfg == nil {
			return nil
		}

		name, args, _ := config.ParseMacroCall(step)

		body, defined := cfg.Macros[name]
		if !defined {
			return nil
		}

		return config.ExpandMacroSteps(body, args)
	default:
		return nil
	}
}

// splitArgs tokenises an action step. The rules belong to the step grammar
// rather than to the hotkey layer, so they live in the config package, which
// also needs them to read macro calls during validation.
func splitArgs(input string) []string {
	return config.SplitStepArgs(input)
}

// refreshHotkeysForAppOrCurrent manages hotkey registration based on Neru's enabled state
// and whether the currently focused application is excluded.
func (b *Binder) refreshHotkeysForAppOrCurrent(bundleID string) {
	b.hotkeyRegistrationMu.Lock()
	defer b.hotkeyRegistrationMu.Unlock()

	if !b.appState.IsEnabled() {
		if b.appState.HotkeysRegistered() {
			b.logger.Debug("Neru disabled; unregistering hotkeys")
			b.stopAllHotkeyRepeats()
			b.hotkeyManager.UnregisterAll()
			b.appState.SetHotkeysRegistered(false)
		}

		return
	}

	cfg := b.settings()

	if bundleID == "" {
		// Use ActionService to get focused bundle ID
		ctx := b.context()

		var bundleIDErr error

		bundleID, bundleIDErr = b.actionService.FocusedAppBundleID(ctx)
		if bundleIDErr != nil {
			// Fail open: when the focused app can't be determined (always the
			// case on Linux Wayland, which has no focus-query API), fall
			// through with an empty bundle ID so global hotkeys still register.
			// Failing closed would permanently disable Neru on those platforms.
			// The next focus event re-evaluates per-app exclusion on platforms
			// that support it. When exclusions are configured but cannot be
			// enforced, warn so the user knows; otherwise this is routine.
			logFn := b.logger.Debug
			if len(cfg.General.ExcludedApps) > 0 {
				logFn = b.logger.Warn
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
		if b.appState.HotkeysRegistered() {
			b.logger.Debug("Focused app excluded; unregistering global hotkeys",
				zap.String("bundle_id", bundleID))
			b.stopAllHotkeyRepeats()
			b.hotkeyManager.UnregisterAll()
			b.appState.SetHotkeysRegistered(false)
		}

		return
	}

	if !b.appState.HotkeysRegistered() {
		b.registerHotkeys(bundleID)
		b.appState.SetHotkeysRegistered(true)
		b.logger.Debug("Hotkeys registered",
			zap.String("bundle_id", bundleID))
	} else if bundleID != b.currentHotkeyBundleID && cfg.HasGlobalAppHotkeyOverrides() {
		// Focus changed to a different app with possibly different hotkey
		// bindings. Re-register with the new app's bindings.
		b.stopAllHotkeyRepeats()
		b.hotkeyManager.UnregisterAll()
		b.registerHotkeys(bundleID)
		b.logger.Debug("Hotkeys re-registered for app",
			zap.String("bundle_id", bundleID))
	}

	// Track which bundle ID's bindings are currently registered.
	b.currentHotkeyBundleID = bundleID
}
