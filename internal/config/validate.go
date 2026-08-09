package config

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Validate validates the configuration, reporting only what stops it loading.
// It is the form every caller that can act on nothing else uses.
//
// It collects no warnings, so it has nothing to attribute and passes no written
// configuration: the zero [WrittenConfig] is the honest answer for a caller
// that never loaded a file.
func (c *Config) Validate() error {
	return c.ValidateWithWarnings(nil, WrittenConfig{})
}

// ValidateWithWarnings validates the configuration and collects, into warnings,
// the parts of it that load and will not do what they say. A nil sink discards
// them; see [Warnings] for why the two are told apart at all.
//
// The configuration validated is the one the daemon will run on, derived values
// and all. written is what the user wrote it from, for the warnings that have
// to name a line in a file rather than a field in a struct; see
// [WrittenConfig], whose zero value means there is none to consult.
func (c *Config) ValidateWithWarnings(warnings *Warnings, written WrittenConfig) error {
	if c == nil {
		return derrors.New(derrors.CodeInvalidConfig, "configuration cannot be nil")
	}

	err := c.ValidateGeneral()
	if err != nil {
		return err
	}

	err = c.ValidateTheme()
	if err != nil {
		return err
	}

	err = c.ValidateModes()
	if err != nil {
		return err
	}

	err = c.ValidateMonitorSelect()
	if err != nil {
		return err
	}

	err = c.ValidateModeIndicator()
	if err != nil {
		return err
	}

	err = c.ValidateHints(warnings)
	if err != nil {
		return err
	}

	err = c.ValidateLogging()
	if err != nil {
		return err
	}

	err = c.ValidateScroll()
	if err != nil {
		return err
	}

	err = c.ValidateAppConfigs()
	if err != nil {
		return err
	}

	// Validate global hotkey app configs
	err = validateHotkeysAppConfigs("app_configs", c.AppConfigs)
	if err != nil {
		return err
	}

	// Validate grid settings
	err = c.ValidateGrid(warnings, written)
	if err != nil {
		return err
	}

	// Validate recursive-grid settings
	err = c.ValidateRecursiveGrid()
	if err != nil {
		return err
	}

	err = c.ValidateVirtualPointer()
	if err != nil {
		return err
	}

	err = c.ValidateMouseAction()
	if err != nil {
		return err
	}

	// Validate sticky modifiers settings
	err = c.ValidateStickyModifiers()
	if err != nil {
		return err
	}

	// Validate smooth cursor settings
	err = c.ValidateSmoothCursor()
	if err != nil {
		return err
	}

	// Validate smooth scroll settings
	err = c.ValidateSmoothScroll()
	if err != nil {
		return err
	}

	// Validate held-key repeat settings
	err = c.ValidateHeldRepeat(warnings)
	if err != nil {
		return err
	}

	// Validate top-level hotkey bindings
	err = c.ValidateHotkeyBindings()
	if err != nil {
		return err
	}

	// Validate per-mode custom hotkeys
	err = c.ValidateHotkeys()
	if err != nil {
		return err
	}

	// The two whole-configuration walks close the ladder. Both go through
	// eachBindingAction, which reads every action string the configuration can
	// dispatch rather than one section of it, so both run after the validators
	// that own the tables it reads: a fault either reports then names a binding
	// already read for shape.
	//
	// Their order relative to each other carries nothing: neither reads what the
	// other establishes. That they come last does, and
	// TestTheBindingWalksCloseTheLadder in internal/architecture fails when one
	// stops being at the end.
	err = c.ValidateMacros()
	if err != nil {
		return err
	}

	return c.ValidateModeCommands(warnings)
}

// ValidateHotkeyBindings validates the top-level [hotkeys] key format and action strings.
func (c *Config) ValidateHotkeyBindings() error {
	// Check for duplicate normalized keys (mirrors checkHotkeysConflicts
	// for per-mode hotkeys). After merge, two keys that normalize identically
	// would cause ambiguous runtime behavior.
	seen := make(map[string]string, len(c.Hotkeys.Bindings))

	for key, actions := range c.Hotkeys.Bindings {
		fieldName := "hotkeys." + key
		if strings.TrimSpace(key) == "" {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"hotkeys contains an empty key",
			)
		}

		normalized := NormalizeKeyForComparison(key)
		if prev, ok := seen[normalized]; ok {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"hotkeys has duplicate bindings (%q and %q)",
				prev,
				key,
			)
		}

		seen[normalized] = key

		err := ValidateHotkey(key, fieldName)
		if err != nil {
			return err
		}

		if len(actions) == 0 {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s cannot have an empty action list",
				fieldName,
			)
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

// ValidateGeneral validates general settings.
func (c *Config) ValidateGeneral() error {
	if c.General.KBLayoutToUse != "" && strings.TrimSpace(c.General.KBLayoutToUse) == "" {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"general.kb_layout_to_use cannot be whitespace-only",
		)
	}

	for index, key := range c.General.PassthroughUnboundedKeysBlacklist {
		fieldName := fmt.Sprintf("general.passthrough_unbounded_keys_blacklist[%d]", index)

		err := ValidateHotkey(key, fieldName)
		if err != nil {
			return err
		}

		if !HasPassthroughModifier(key) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s must include Cmd, Ctrl, Alt, or Option: %s",
				fieldName,
				key,
			)
		}
	}

	if c.General.ExecShell == "" {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"general.exec_shell cannot be empty",
		)
	}

	if !filepath.IsAbs(c.General.ExecShell) {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"general.exec_shell must be an absolute path (got: %q)",
			c.General.ExecShell,
		)
	}

	if len(c.General.ExecShellArgs) == 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"general.exec_shell_args cannot be empty",
		)
	}

	return nil
}

// ValidateModeIndicator validates the mode indicator configuration.
func (c *Config) ValidateModeIndicator() error {
	if c.ModeIndicator.UI.FontSize < 1 || c.ModeIndicator.UI.FontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"mode_indicator.ui.font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	err := validateMinValue(c.ModeIndicator.UI.BorderWidth, 0, "mode_indicator.ui.border_width")
	if err != nil {
		return err
	}

	err = validateMinValue(c.ModeIndicator.UI.PaddingX, -1, "mode_indicator.ui.padding_x")
	if err != nil {
		return err
	}

	err = validateMinValue(c.ModeIndicator.UI.PaddingY, -1, "mode_indicator.ui.padding_y")
	if err != nil {
		return err
	}

	err = validateMinValue(c.ModeIndicator.UI.BorderRadius, -1, "mode_indicator.ui.border_radius")
	if err != nil {
		return err
	}

	err = validateColors([]colorField{
		{c.ModeIndicator.UI.BackgroundColor, "mode_indicator.ui.background_color"},
		{c.ModeIndicator.UI.TextColor, "mode_indicator.ui.text_color"},
		{c.ModeIndicator.UI.BorderColor, "mode_indicator.ui.border_color"},
	})
	if err != nil {
		return err
	}

	// Validate per-mode color overrides (only when non-empty).
	modes := []struct {
		cfg  ModeIndicatorModeConfig
		name string
	}{
		{c.ModeIndicator.Scroll, ModeNameScroll},
		{c.ModeIndicator.Hints, ModeNameHints},
		{c.ModeIndicator.Grid, ModeNameGrid},
		{c.ModeIndicator.RecursiveGrid, ModeNameRecursiveGrid},
	}

	for _, mode := range modes {
		err = validateColors([]colorField{
			{mode.cfg.BackgroundColor, "mode_indicator." + mode.name + ".background_color"},
			{mode.cfg.TextColor, "mode_indicator." + mode.name + ".text_color"},
			{mode.cfg.BorderColor, "mode_indicator." + mode.name + ".border_color"},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// ValidateModes validates that at least one mode is enabled.
func (c *Config) ValidateModes() error {
	if !c.Hints.Enabled && !c.Grid.Enabled && !c.RecursiveGrid.Enabled {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"at least one mode must be enabled: hints.enabled, grid.enabled, or recursive_grid.enabled",
		)
	}

	return nil
}

// ValidateLogging validates the logging configuration.
func (c *Config) ValidateLogging() error {
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.Logging.LogLevel] {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"log_level must be one of: debug, info, warn, error",
		)
	}

	return nil
}

// validateMinValue validates that a value is at least the minimum.
func validateMinValue(value int, minimum int, fieldName string) error {
	if value < minimum {
		return derrors.New(
			derrors.CodeInvalidConfig,
			fieldName+" must be at least "+strconv.Itoa(minimum),
		)
	}

	return nil
}

// ValidateScroll validates the scroll configuration.
func (c *Config) ValidateScroll() error {
	err := validateMinValue(c.Scroll.ScrollStep, 1, "scroll.scroll_step")
	if err != nil {
		return err
	}

	err = validateMinValue(c.Scroll.ScrollStepHalf, 1, "scroll.scroll_step_half")
	if err != nil {
		return err
	}

	err = validateMinValue(c.Scroll.ScrollStepFull, 1, "scroll.scroll_step_full")
	if err != nil {
		return err
	}

	err = validateScrollAppConfigs("scroll", c.Scroll.AppConfigs)
	if err != nil {
		return err
	}

	return nil
}
