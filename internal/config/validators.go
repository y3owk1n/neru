package config

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/y3owk1n/neru/internal/core/domain/action"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

var validModifiers = map[string]bool{
	"Cmd":         true,
	"Ctrl":        true,
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

func isValidModifier(mod string) bool {
	return validModifiers[mod]
}

type colorField struct {
	color     Color
	fieldName string
}

func validateColors(fields []colorField) error {
	for _, field := range fields {
		err := field.color.Validate(field.fieldName)
		if err != nil {
			return err
		}
	}

	return nil
}

func validateThemePalette(name string, palette ThemePalette) error {
	fields := []struct {
		value     string
		fieldName string
	}{
		{value: palette.Surface, fieldName: name + ".surface"},
		{value: palette.Accent, fieldName: name + ".accent"},
		{value: palette.AccentAlt, fieldName: name + ".accent_alt"},
		{value: palette.OnAccentAlt, fieldName: name + ".on_accent_alt"},
		{value: palette.Text, fieldName: name + ".text"},
	}

	for _, field := range fields {
		err := ValidateSolidColor(field.value, field.fieldName)
		if err != nil {
			return err
		}
	}

	return nil
}

// ValidateTheme validates the top-level theme palette configuration.
func (c *Config) ValidateTheme() error {
	err := validateThemePalette("theme.light", c.Theme.Light)
	if err != nil {
		return err
	}

	return validateThemePalette("theme.dark", c.Theme.Dark)
}

// ValidateHints validates the hints configuration.
func (c *Config) ValidateHints() error {
	if c.Hints.Enabled && len(c.Hints.ClickableRoles) == 0 {
		return derrors.New(derrors.CodeInvalidConfig,
			"hints.clickable_roles cannot be empty when hints are enabled")
	}

	if strings.TrimSpace(c.Hints.HintCharacters) == "" {
		return derrors.New(derrors.CodeInvalidConfig, "hint_characters cannot be empty")
	}

	if len(c.Hints.HintCharacters) < MinCharactersLength {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hint_characters must contain at least 2 characters",
		)
	}

	for _, r := range c.Hints.HintCharacters {
		if r > unicode.MaxASCII {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"hint_characters can only contain ASCII characters",
			)
		}
	}

	err := validateColors([]colorField{
		{c.Hints.UI.BackgroundColor, "hints.ui.background_color"},
		{c.Hints.UI.TextColor, "hints.ui.text_color"},
		{c.Hints.UI.MatchedTextColor, "hints.ui.matched_text_color"},
		{c.Hints.UI.BorderColor, "hints.ui.border_color"},
	})
	if err != nil {
		return err
	}

	if c.Hints.UI.FontSize < 6 || c.Hints.UI.FontSize > 72 {
		return derrors.New(derrors.CodeInvalidConfig, "hints.ui.font_size must be between 6 and 72")
	}

	err = validateMinValue(c.Hints.UI.BorderRadius, -1, "hints.ui.border_radius")
	if err != nil {
		return err
	}

	err = validateMinValue(c.Hints.UI.PaddingX, -1, "hints.ui.padding_x")
	if err != nil {
		return err
	}

	err = validateMinValue(c.Hints.UI.PaddingY, -1, "hints.ui.padding_y")
	if err != nil {
		return err
	}

	if c.Hints.UI.BorderWidth < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "hints.ui.border_width must be non-negative")
	}

	err = validateMinValue(c.Hints.MaxDepth, 0, "hints.max_depth")
	if err != nil {
		return err
	}

	err = validateMinValue(c.Hints.ParallelThreshold, 1, "hints.parallel_threshold")
	if err != nil {
		return err
	}

	return nil
}

// ValidateAppConfigs validates per-app hint configuration.
func (c *Config) ValidateAppConfigs() error {
	seen := make(map[string]struct{}, len(c.Hints.AppConfigs))
	for idx, appConfig := range c.Hints.AppConfigs {
		if strings.TrimSpace(appConfig.BundleID) == "" {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"hints.app_configs[%d].bundle_id cannot be empty",
				idx,
			)
		}

		if _, ok := seen[appConfig.BundleID]; ok {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"duplicate hints.app_configs bundle_id: %s",
				appConfig.BundleID,
			)
		}

		seen[appConfig.BundleID] = struct{}{}

		err := validateHotkeyTable(
			fmt.Sprintf("hints.app_configs[%d].hotkeys", idx),
			appConfig.Hotkeys,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// ValidateGrid validates the grid configuration.
func (c *Config) ValidateGrid() error {
	if !c.Grid.Enabled {
		return nil
	}

	if strings.TrimSpace(c.Grid.Characters) == "" {
		return derrors.New(derrors.CodeInvalidConfig, "grid.characters cannot be empty")
	}

	for _, r := range c.Grid.Characters {
		if r > unicode.MaxASCII {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"grid.characters can only contain ASCII characters",
			)
		}
	}

	if c.Grid.SublayerKeys != "" {
		for _, r := range c.Grid.SublayerKeys {
			if r > unicode.MaxASCII {
				return derrors.New(
					derrors.CodeInvalidConfig,
					"grid.sublayer_keys can only contain ASCII characters",
				)
			}
		}
	}

	err := validateColors([]colorField{
		{c.Grid.UI.BackgroundColor, "grid.ui.background_color"},
		{c.Grid.UI.TextColor, "grid.ui.text_color"},
		{c.Grid.UI.MatchedTextColor, "grid.ui.matched_text_color"},
		{c.Grid.UI.MatchedBackgroundColor, "grid.ui.matched_background_color"},
		{c.Grid.UI.MatchedBorderColor, "grid.ui.matched_border_color"},
		{c.Grid.UI.BorderColor, "grid.ui.border_color"},
	})
	if err != nil {
		return err
	}

	if c.Grid.UI.FontSize < 6 || c.Grid.UI.FontSize > 72 {
		return derrors.New(derrors.CodeInvalidConfig, "grid.ui.font_size must be between 6 and 72")
	}

	if c.Grid.UI.BorderWidth < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "grid.ui.border_width must be non-negative")
	}

	return nil
}

// ValidateStickyModifiers validates sticky modifier settings.
func (c *Config) ValidateStickyModifiers() error {
	if !c.StickyModifiers.Enabled {
		return nil
	}

	if c.StickyModifiers.TapMaxDuration < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"sticky_modifiers.tap_max_duration must be >= 0",
		)
	}

	return validateColors([]colorField{
		{c.StickyModifiers.UI.BackgroundColor, "sticky_modifiers.ui.background_color"},
		{c.StickyModifiers.UI.TextColor, "sticky_modifiers.ui.text_color"},
		{c.StickyModifiers.UI.BorderColor, "sticky_modifiers.ui.border_color"},
	})
}

// ValidateSmoothCursor validates smooth cursor settings.
func (c *Config) ValidateSmoothCursor() error {
	if c.SmoothCursor.Steps < 1 {
		return derrors.New(derrors.CodeInvalidConfig, "smooth_cursor.steps must be >= 1")
	}

	if c.SmoothCursor.MaxDuration < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "smooth_cursor.max_duration must be >= 0")
	}

	if c.SmoothCursor.DurationPerPixel < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"smooth_cursor.duration_per_pixel must be >= 0",
		)
	}

	return nil
}

// ValidateHotkeys validates per-mode hotkey syntax and actions.
func (c *Config) ValidateHotkeys() error {
	modeHotkeys := []struct {
		modeName string
		table    map[string]StringOrStringArray
	}{
		{modeNameHints, c.Hints.Hotkeys},
		{modeNameGrid, c.Grid.Hotkeys},
		{modeNameRecursiveGrid, c.RecursiveGrid.Hotkeys},
		{modeNameScroll, c.Scroll.Hotkeys},
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
	modes := []struct {
		modeName string
		table    map[string]StringOrStringArray
	}{
		{modeNameHints, c.Hints.Hotkeys},
		{modeNameGrid, c.Grid.Hotkeys},
		{modeNameRecursiveGrid, c.RecursiveGrid.Hotkeys},
		{modeNameScroll, c.Scroll.Hotkeys},
	}

	for _, mode := range modes {
		err := checkHotkeyConflicts(mode.modeName+".hotkeys", mode.table)
		if err != nil {
			return err
		}
	}

	for idx, appConfig := range c.Hints.AppConfigs {
		err := checkHotkeyConflicts(
			fmt.Sprintf(
				"hints.hotkeys merged with hints.app_configs[%d] (%s)",
				idx,
				appConfig.BundleID,
			),
			c.HotkeysForModeAndApp(modeNameHints, appConfig.BundleID),
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

// ValidateColor validates a single hex color value (#RGB/#RRGGBB/#AARRGGBB).
// It uses the pre-compiled colorRegex from Color.
func ValidateColor(color, fieldName string) error {
	if color == "" {
		return nil
	}

	if !colorRegex.MatchString(color) {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s has invalid color format: %s",
			fieldName,
			color,
		)
	}

	return nil
}

// ValidateRecursiveGrid validates recursive grid configuration.
func (c *Config) ValidateRecursiveGrid() error {
	if !c.RecursiveGrid.Enabled {
		return nil
	}

	if c.RecursiveGrid.GridCols < DefaultRecursiveGridMinGridCols {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"recursive_grid.grid_cols must be >= %d",
			DefaultRecursiveGridMinGridCols,
		)
	}

	if c.RecursiveGrid.GridRows < DefaultRecursiveGridMinGridRows {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"recursive_grid.grid_rows must be >= %d",
			DefaultRecursiveGridMinGridRows,
		)
	}

	if c.RecursiveGrid.MaxDepth < 1 {
		return derrors.New(derrors.CodeInvalidConfig, "recursive_grid.max_depth must be >= 1")
	}

	if c.RecursiveGrid.AnimationDurationMS < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.animation_duration_ms must be non-negative",
		)
	}

	expectedKeys := c.RecursiveGrid.GridCols * c.RecursiveGrid.GridRows
	if utf8.RuneCountInString(c.RecursiveGrid.Keys) != expectedKeys {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"recursive_grid.keys must have %d characters",
			expectedKeys,
		)
	}

	for _, layer := range c.RecursiveGrid.Layers {
		if layer.Depth < 0 {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"recursive_grid.layers.depth must be >= 0",
			)
		}

		if layer.GridCols < DefaultRecursiveGridMinGridCols ||
			layer.GridRows < DefaultRecursiveGridMinGridRows {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"recursive_grid.layers grid dimensions must be >= 2",
			)
		}

		if utf8.RuneCountInString(layer.Keys) != layer.GridCols*layer.GridRows {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"recursive_grid.layers depth %d keys length mismatch",
				layer.Depth,
			)
		}
	}

	err := validateColors([]colorField{
		{c.RecursiveGrid.UI.LineColor, "recursive_grid.ui.line_color"},
		{c.RecursiveGrid.UI.HighlightColor, "recursive_grid.ui.highlight_color"},
		{c.RecursiveGrid.UI.TextColor, "recursive_grid.ui.text_color"},
		{c.RecursiveGrid.UI.LabelBackgroundColor, "recursive_grid.ui.label_background_color"},
		{c.RecursiveGrid.UI.SubKeyPreviewTextColor, "recursive_grid.ui.sub_key_preview_text_color"},
	})
	if err != nil {
		return err
	}

	if c.RecursiveGrid.UI.LineWidth < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.ui.line_width must be non-negative",
		)
	}

	if c.RecursiveGrid.UI.FontSize < 1 {
		return derrors.New(derrors.CodeInvalidConfig, "recursive_grid.ui.font_size must be >= 1")
	}

	return nil
}

// ValidateVirtualPointer validates virtual pointer configuration.
func (c *Config) ValidateVirtualPointer() error {
	if !c.VirtualPointer.Enabled {
		return nil
	}

	err := validateColors([]colorField{
		{c.VirtualPointer.UI.Color, "virtual_pointer.ui.color"},
	})
	if err != nil {
		return err
	}

	if c.VirtualPointer.UI.Size < 1 {
		return derrors.New(derrors.CodeInvalidConfig, "virtual_pointer.ui.size must be >= 1")
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
		if action.IsKnownName(action.Name(name)) {
			return nil
		}

		return derrors.Newf(derrors.CodeInvalidConfig, "unknown action subcommand: %s", name)
	}

	// Mode commands may include flags (e.g. "hints --action left_click").
	// Split on space and validate the first word as a known root/mode command.
	cmd := strings.Fields(trimmed)[0]

	switch cmd {
	case "idle", "hints", "grid", "scroll", "recursive_grid",
		"toggle-screen-share", "toggle-cursor-follow-selection":
		return nil
	default:
		return derrors.Newf(derrors.CodeInvalidConfig, "unknown command: %s", trimmed)
	}
}
