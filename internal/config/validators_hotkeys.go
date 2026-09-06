package config

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

var validModifiers = map[string]bool{
	"Primary":     true,
	"Cmd":         true,
	"Command":     true,
	"Super":       true,
	"Meta":        true,
	"Ctrl":        true,
	"Control":     true,
	"Alt":         true,
	"Shift":       true,
	"Option":      true,
	"RightCmd":    true,
	"RightCtrl":   true,
	"RightAlt":    true,
	"RightOption": true,
	"RightShift":  true,
	"LeftCmd":     true,
	"LeftCtrl":    true,
	"LeftAlt":     true,
	"LeftOption":  true,
	"LeftShift":   true,
}

const minModifierComboParts = 2

// builtInModeCount sizes the tables that list one entry per built-in mode
// before the declared modes are appended.
const builtInModeCount = 5

func isValidModifier(mod string) bool {
	return validModifiers[mod]
}

// ValidateHotkeys validates per-mode hotkey syntax and actions.
func (c *Config) ValidateHotkeys() error {
	type modeTable struct {
		modeName string
		table    map[string]StringOrStringArray
	}

	modeHotkeys := make([]modeTable, 0, builtInModeCount+len(c.Modes))
	modeHotkeys = append(modeHotkeys,
		modeTable{ModeNameHints, c.Hints.Hotkeys},
		modeTable{ModeNameGrid, c.Grid.Hotkeys},
		modeTable{ModeNameRecursiveGrid, c.RecursiveGrid.Hotkeys},
		modeTable{ModeNameScroll, c.Scroll.Hotkeys},
		// monitor_select binds keys like the other modes and dispatches them
		// through the same executor, so its table is checked like theirs.
		modeTable{ModeNameMonitorSelect, c.MonitorSelect.Hotkeys},
	)

	for name, mode := range c.Modes {
		modeHotkeys = append(modeHotkeys, modeTable{customModeField(name), mode.Hotkeys})
	}

	for _, mode := range modeHotkeys {
		err := validateHotkeyTable(mode.modeName+".hotkeys", mode.table)
		if err != nil {
			return err
		}
	}

	return c.checkHotkeysConflicts()
}

func validateHotkeyTable(fieldPrefix string, table map[string]StringOrStringArray) error {
	for key, actions := range table {
		fieldName := fmt.Sprintf("%s.%s", fieldPrefix, key)

		err := ValidateHotkey(key, fieldName)
		if err != nil {
			return err
		}

		if len(actions) == 0 {
			return derrors.Newf(derrors.CodeInvalidConfig, "%s cannot be empty", fieldName)
		}

		if len(actions) == 1 && actions[0] == DisabledSentinel {
			continue
		}

		for actionIndex, actionStr := range actions {
			trimmed := strings.TrimSpace(actionStr)
			if trimmed == "" {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s[%d] cannot be empty",
					fieldName,
					actionIndex,
				)
			}

			err := validateHotkeyActionString(trimmed)
			if err != nil {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s[%d]: %v",
					fieldName,
					actionIndex,
					err,
				)
			}
		}
	}

	return nil
}

func (c *Config) checkHotkeysConflicts() error {
	type modeTable struct {
		modeName string
		table    map[string]StringOrStringArray
	}

	modes := make([]modeTable, 0, builtInModeCount+len(c.Modes))
	modes = append(modes,
		modeTable{ModeNameHints, c.Hints.Hotkeys},
		modeTable{ModeNameGrid, c.Grid.Hotkeys},
		modeTable{ModeNameRecursiveGrid, c.RecursiveGrid.Hotkeys},
		modeTable{ModeNameScroll, c.Scroll.Hotkeys},
	)

	for name, mode := range c.Modes {
		modes = append(modes, modeTable{customModeField(name), mode.Hotkeys})
	}

	for _, mode := range modes {
		err := checkHotkeyConflicts(mode.modeName+".hotkeys", mode.table)
		if err != nil {
			return err
		}
	}

	for name, mode := range c.Modes {
		for idx, appConfig := range mode.AppConfigs {
			err := checkHotkeyConflicts(
				fmt.Sprintf(
					"%s.hotkeys merged with %s.app_configs[%d] (%s)",
					customModeField(name),
					customModeField(name),
					idx,
					appConfig.BundleID,
				),
				c.HotkeysForModeAndApp(name, appConfig.BundleID),
			)
			if err != nil {
				return err
			}
		}
	}

	for idx, appConfig := range c.Hints.AppConfigs {
		err := checkHotkeyConflicts(
			fmt.Sprintf(
				"hints.hotkeys merged with hints.app_configs[%d] (%s)",
				idx,
				appConfig.BundleID,
			),
			c.HotkeysForModeAndApp(ModeNameHints, appConfig.BundleID),
		)
		if err != nil {
			return err
		}
	}

	for idx, appConfig := range c.Scroll.AppConfigs {
		err := checkHotkeyConflicts(
			fmt.Sprintf(
				"scroll.hotkeys merged with scroll.app_configs[%d] (%s)",
				idx,
				appConfig.BundleID,
			),
			c.HotkeysForModeAndApp(ModeNameScroll, appConfig.BundleID),
		)
		if err != nil {
			return err
		}
	}

	// Check merged global hotkeys for each [[app_configs]] entry
	for idx, appConfig := range c.AppConfigs {
		merged := c.GlobalHotkeysForApp(appConfig.BundleID)
		if merged == nil {
			continue
		}
		// Convert to StringOrStringArray for conflict checking
		table := make(map[string]StringOrStringArray, len(merged))
		for k, v := range merged {
			table[k] = StringOrStringArray(v)
		}

		err := checkHotkeyConflicts(
			fmt.Sprintf(
				"hotkeys merged with app_configs[%d] (%s)",
				idx,
				appConfig.BundleID,
			),
			table,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func checkHotkeyConflicts(fieldPrefix string, table map[string]StringOrStringArray) error {
	seen := map[string]string{}
	for key := range table {
		normalized := NormalizeKeyForComparison(key)
		if prev, ok := seen[normalized]; ok {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s has duplicate bindings (%q and %q)",
				fieldPrefix,
				prev,
				key,
			)
		}

		seen[normalized] = key
	}

	// Check prefix conflicts: a single-character binding shadows any
	// two-letter sequence that starts with the same character, because
	// at runtime Phase 2 (direct match) fires before Phase 3 (sequence
	// start), making the sequence silently unreachable.
	//
	// We use the original key (not normalized) to identify sequences,
	// matching the ValidateHotkey logic: a sequence is exactly 2 ASCII
	// letters in the original form. Named keys like "Up" normalize to
	// "up" which passes IsAllLetters, but they are not sequences.
	for key := range table {
		normalized := NormalizeKeyForComparison(key)
		if len(normalized) != 1 {
			continue
		}

		for seqKey := range table {
			if len(seqKey) != 2 || !IsAllLetters(seqKey) || IsValidNamedKey(seqKey) {
				continue
			}

			normalizedSeq := NormalizeKeyForComparison(seqKey)
			if strings.HasPrefix(normalizedSeq, normalized) {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s has a prefix conflict: single-key binding %q shadows sequence %q; the single key is always matched first at runtime, so the sequence can never fire",
					fieldPrefix,
					key,
					seqKey,
				)
			}
		}
	}

	return nil
}

// ValidateHotkey validates hotkey format (single key, named key, modifier combo, or 2-letter sequence).
func ValidateHotkey(hotkey, fieldName string) error {
	if strings.TrimSpace(hotkey) == "" {
		return derrors.Newf(derrors.CodeInvalidConfig, "%s cannot be empty", fieldName)
	}

	// Accept Vim-like 2-letter sequences.
	if len(hotkey) == 2 && IsAllLetters(hotkey) {
		return nil
	}

	if strings.Contains(hotkey, "+") {
		return validateModifierCombo(hotkey, fieldName)
	}

	if IsValidNamedKey(hotkey) {
		return nil
	}

	if len(hotkey) == 1 {
		r, _ := utf8.DecodeRuneInString(hotkey)
		if r > unicode.MaxASCII {
			return derrors.Newf(derrors.CodeInvalidConfig, "%s must be ASCII", fieldName)
		}

		return nil
	}

	return derrors.Newf(
		derrors.CodeInvalidConfig,
		"%s has invalid key format: %s",
		fieldName,
		hotkey,
	)
}

func validateModifierCombo(key, fieldName string) error {
	parts := strings.Split(key, "+")
	if len(parts) < minModifierComboParts {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s has invalid modifier combo: %s",
			fieldName,
			key,
		)
	}

	for i := range len(parts) - 1 {
		mod := strings.TrimSpace(parts[i])
		if !isValidModifier(mod) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s has invalid modifier '%s'",
				fieldName,
				mod,
			)
		}
	}

	last := strings.TrimSpace(parts[len(parts)-1])
	if last == "" {
		return derrors.Newf(derrors.CodeInvalidConfig, "%s has empty key in combo", fieldName)
	}

	if !IsValidNamedKey(last) && len(last) != 1 {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s has invalid key '%s' in modifier combo",
			fieldName,
			last,
		)
	}

	return nil
}

// validateActionChain validates a comma-separated action chain such as
// "left_click,left_click". The runtime executes chains at a single target
// point, so only mouse button actions are accepted (see handleActionChain).
func validateActionChain(chain string) error {
	for name := range strings.SplitSeq(chain, ",") {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"empty action in comma-separated list: %s",
				chain,
			)
		}

		if !action.IsKnownName(action.Name(trimmed)) {
			return derrors.Newf(derrors.CodeInvalidConfig, "unknown action subcommand: %s", trimmed)
		}

		// Known names without an executable type (sleep, feed, reset, ...) are
		// not chainable either, so a ToType failure here is a chain violation
		// rather than an unknown name.
		actionType, err := action.Name(trimmed).ToType()
		if err != nil || !actionType.IsMouseButton() {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s cannot be used in an action chain; only mouse button actions are allowed",
				trimmed,
			)
		}
	}

	return nil
}

func validateHotkeyActionString(actionStr string) error {
	trimmed := strings.TrimSpace(actionStr)
	if trimmed == "" {
		return derrors.New(derrors.CodeInvalidConfig, "action cannot be empty")
	}

	if strings.HasPrefix(trimmed, action.PrefixExec+" ") {
		return nil
	}

	if after, ok := strings.CutPrefix(trimmed, "action "); ok {
		args := strings.Fields(strings.TrimSpace(after))
		if len(args) == 0 {
			return derrors.New(derrors.CodeInvalidConfig, "missing action subcommand")
		}

		name := args[0]
		if strings.Contains(name, ",") {
			return validateActionChain(name)
		}

		if action.IsKnownName(action.Name(name)) {
			return nil
		}

		return derrors.Newf(derrors.CodeInvalidConfig, "unknown action subcommand: %s", name)
	}

	// Mode commands may include flags (e.g. "hints --action left_click"). Only
	// the command word is judged here: the flags after it are read by
	// ValidateModeCommands, which weighs what it finds rather than refusing all
	// of it.
	cmd := strings.Fields(trimmed)[0]

	if _, isMode := modecmd.LookupMode(cmd); isMode {
		return nil
	}

	switch cmd {
	case "toggle-screen-share", CmdToggleCursorFollowSelection,
		"toggle-scroll-invert",
		"config",
		// "run" takes its own steps as arguments. Each one is validated when
		// the sequence executes, not here: the steps arrive as a single quoted
		// string per step, so splitting them apart correctly is the runtime's
		// job, not the validator's.
		CmdRun,
		// "macro" is checked against the [macros] table by ValidateMacros,
		// which needs the whole config rather than one action string.
		CmdMacro:
		return nil
	default:
		return derrors.Newf(derrors.CodeInvalidConfig, "unknown command: %s", trimmed)
	}
}
