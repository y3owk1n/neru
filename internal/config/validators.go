package config

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/y3owk1n/neru/internal/core/domain/action"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

func validateAutoExitActions(actions []string, fieldName string) error {
	for _, actionName := range actions {
		if !action.IsDirectKeyBindingName(action.Name(actionName)) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s contains unknown action '%s' (valid: %s)",
				fieldName,
				actionName,
				action.DirectKeyBindingNamesString(),
			)
		}
	}

	return nil
}

var validModifiers = map[string]bool{
	"Cmd":    true,
	"Ctrl":   true,
	"Alt":    true,
	"Shift":  true,
	"Option": true,
}

func isValidModifier(mod string) bool {
	return validModifiers[mod]
}

// validateBackspaceKeyFormat validates that a backspace_key value is a recognized key format.
// Valid formats: empty (default backspace/delete), single character, named key (from validNamedKeys),
// or a modifier combo (e.g. "Ctrl+H").
// The fieldName parameter is used in error messages.
func validateBackspaceKeyFormat(key, fieldName string) error {
	if key == "" {
		return nil // empty means use default backspace/delete
	}
	// Named key (e.g. "backspace", "delete", "Tab", "F1")
	if IsValidNamedKey(key) {
		return nil
	}
	// Modifier combo (e.g. "Ctrl+H")
	if strings.Contains(key, "+") {
		return validateModifierCombo(key, fieldName)
	}
	// Single character
	if len(key) == 1 {
		return nil
	}

	return derrors.Newf(
		derrors.CodeInvalidConfig,
		"%s = '%s' is invalid; must be a single character, a named key (e.g. 'backspace', 'Tab', 'F1'), or a modifier combo (e.g. 'Ctrl+H')",
		fieldName,
		key,
	)
}

// colorField represents a color configuration field to validate.
type colorField struct {
	value     string
	fieldName string
}

// validateColors batch validates multiple color fields.
func validateColors(fields []colorField) error {
	for _, field := range fields {
		err := ValidateColor(field.value, field.fieldName)
		if err != nil {
			return err
		}
	}

	return nil
}

// ValidateHints validates the hints configuration.
func (c *Config) ValidateHints() error {
	if c.Hints.Enabled {
		if len(c.Hints.ClickableRoles) == 0 {
			return derrors.New(derrors.CodeInvalidConfig,
				"hints.clickable_roles cannot be empty when hints are enabled")
		}
	}

	err := validateAutoExitActions(
		c.Hints.AutoExitActions,
		"hints.auto_exit_actions",
	)
	if err != nil {
		return err
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

	// Validate backspace key format
	err = validateBackspaceKeyFormat(c.Hints.BackspaceKey, "hints.backspace_key")
	if err != nil {
		return err
	}

	// Validate backspace key doesn't conflict with hint characters
	err = checkBackspaceKeyCharConflict(
		c.Hints.BackspaceKey,
		c.Hints.HintCharacters,
		"hints.backspace_key",
		"hints.hint_characters",
		"hint selection",
	)
	if err != nil {
		return err
	}

	err = validateColors([]colorField{
		{c.Hints.BackgroundColorLight, "hints.background_color_light"},
		{c.Hints.BackgroundColorDark, "hints.background_color_dark"},
		{c.Hints.TextColorLight, "hints.text_color_light"},
		{c.Hints.TextColorDark, "hints.text_color_dark"},
		{c.Hints.MatchedTextColorLight, "hints.matched_text_color_light"},
		{c.Hints.MatchedTextColorDark, "hints.matched_text_color_dark"},
		{c.Hints.BorderColorLight, "hints.border_color_light"},
		{c.Hints.BorderColorDark, "hints.border_color_dark"},
	})
	if err != nil {
		return err
	}

	if c.Hints.FontSize < 6 || c.Hints.FontSize > 72 {
		return derrors.New(derrors.CodeInvalidConfig, "hints.font_size must be between 6 and 72")
	}

	err = validateMinValue(c.Hints.BorderRadius, -1, "hints.border_radius")
	if err != nil {
		return err
	}

	err = validateMinValue(c.Hints.PaddingX, -1, "hints.padding_x")
	if err != nil {
		return err
	}

	err = validateMinValue(c.Hints.PaddingY, -1, "hints.padding_y")
	if err != nil {
		return err
	}

	if c.Hints.BorderWidth < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "hints.border_width must be non-negative")
	}

	if c.Hints.MouseActionRefreshDelay < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.mouse_action_refresh_delay must be non-negative",
		)
	}

	err = validateMinValue(c.Hints.MaxDepth, 0, "hints.max_depth")
	if err != nil {
		return err
	}

	err = validateMinValue(c.Hints.ParallelThreshold, 1, "hints.parallel_threshold")
	if err != nil {
		return err
	}

	if c.Hints.MouseActionRefreshDelay > MaxMouseActionRefreshDelay {
		return derrors.New(
			derrors.CodeInvalidConfig,
			fmt.Sprintf(
				"hints.mouse_action_refresh_delay must be at most %d (10 seconds)",
				MaxMouseActionRefreshDelay,
			),
		)
	}

	for _, role := range c.Hints.ClickableRoles {
		if strings.TrimSpace(role) == "" {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"hints.clickable_roles cannot contain empty values",
			)
		}
	}

	for _, bundle := range c.Hints.AdditionalAXSupport.AdditionalElectronBundles {
		if strings.TrimSpace(bundle) == "" {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"hints.electron_support.additional_electron_bundles cannot contain empty values",
			)
		}
	}

	for _, bundle := range c.Hints.AdditionalAXSupport.AdditionalChromiumBundles {
		if strings.TrimSpace(bundle) == "" {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"hints.electron_support.additional_chromium_bundles cannot contain empty values",
			)
		}
	}

	for _, bundle := range c.Hints.AdditionalAXSupport.AdditionalFirefoxBundles {
		if strings.TrimSpace(bundle) == "" {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"hints.electron_support.additional_firefox_bundles cannot contain empty values",
			)
		}
	}

	return nil
}

// ValidateAppConfigs validates the app configurations.
func (c *Config) ValidateAppConfigs() error {
	var validateErr error

	for index, appConfig := range c.Hints.AppConfigs {
		if strings.TrimSpace(appConfig.BundleID) == "" {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"hints.app_configs[%d].bundle_id cannot be empty",
				index,
			)
		}

		for _, role := range appConfig.AdditionalClickable {
			if strings.TrimSpace(role) == "" {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"hints.app_configs[%d].additional_clickable_roles cannot contain empty values",
					index,
				)
			}
		}

		if appConfig.MouseActionRefreshDelay != nil && *appConfig.MouseActionRefreshDelay < 0 {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"hints.app_configs[%d].mouse_action_refresh_delay must be non-negative",
				index,
			)
		}

		if appConfig.MouseActionRefreshDelay != nil &&
			*appConfig.MouseActionRefreshDelay > MaxMouseActionRefreshDelay {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"hints.app_configs[%d].mouse_action_refresh_delay must be at most %d (10 seconds)",
				index,
				MaxMouseActionRefreshDelay,
			)
		}
	}

	// Validate hotkey bindings once, regardless of app configs
	for key, value := range c.Hotkeys.Bindings {
		if strings.TrimSpace(key) == "" {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"hotkeys.bindings contains an empty key",
			)
		}

		validateErr = ValidateHotkey(key, "hotkeys.bindings")
		if validateErr != nil {
			return validateErr
		}

		if strings.TrimSpace(value) == "" {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"hotkeys.bindings[%s] cannot be empty",
				key,
			)
		}
	}

	return nil
}

// ValidateGrid validates the grid configuration.
func (c *Config) ValidateGrid() error {
	if strings.TrimSpace(c.Grid.Characters) == "" {
		return derrors.New(derrors.CodeInvalidConfig, "grid.characters cannot be empty")
	}

	if len(c.Grid.Characters) < MinCharactersLength {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"grid.characters must contain at least 2 characters",
		)
	}

	resetKey := c.Grid.ResetKey
	if resetKey == "" {
		resetKey = " "
	}

	// Validate reset key format: single character, named key, or modifier combo
	switch {
	case strings.Contains(resetKey, "+"):
		// Validate modifier combo (e.g. "Ctrl+R")
		err := validateResetKeyCombo(resetKey, "grid.reset_key")
		if err != nil {
			return err
		}

		// Even modifier-combo reset keys must not conflict with the configured backspace key
		if gridBackspaceKey := c.Grid.BackspaceKey; gridBackspaceKey != "" &&
			NormalizeKeyForComparison(resetKey) == NormalizeKeyForComparison(gridBackspaceKey) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"grid.reset_key cannot be the same as grid.backspace_key ('%s')",
				gridBackspaceKey,
			)
		}

		// Modifier combos don't conflict with grid characters, so no further checks needed
	case IsValidNamedKey(resetKey):
		// Named key reset (e.g. "Home", "F1", "Tab")
		// Named keys don't conflict with grid character sets, but check backspace conflict
		gridBackspaceKey := c.Grid.BackspaceKey
		if gridBackspaceKey == "" {
			normalizedResetKey := NormalizeKeyForComparison(resetKey)
			if normalizedResetKey == KeyNameDelete {
				return derrors.New(
					derrors.CodeInvalidConfig,
					"grid.reset_key cannot be 'backspace' or 'delete'; these keys are reserved for input correction",
				)
			}
		} else {
			normalizedResetKey := NormalizeKeyForComparison(resetKey)
			normalizedBackspaceKey := NormalizeKeyForComparison(gridBackspaceKey)

			if normalizedResetKey == normalizedBackspaceKey {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"grid.reset_key cannot be the same as grid.backspace_key ('%s')",
					gridBackspaceKey,
				)
			}
		}
	default:
		// Validate single character reset key
		if len(resetKey) != 1 {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"grid.reset_key must be a single character, a named key (e.g. 'Home', 'F1'), or a modifier combo (e.g. 'Ctrl+R')",
			)
		}

		// Validate reset key is ASCII
		if rune(resetKey[0]) > unicode.MaxASCII {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"grid.reset_key must be an ASCII character",
			)
		}

		// Reset key cannot match the configured backspace key (they would conflict)
		gridBackspaceKey := c.Grid.BackspaceKey
		if gridBackspaceKey == "" {
			// Default: backspace and delete are reserved for input correction.
			// NormalizeKeyForComparison maps all backspace/delete variants to KeyNameDelete.
			normalizedResetKey := NormalizeKeyForComparison(resetKey)
			if normalizedResetKey == KeyNameDelete {
				return derrors.New(
					derrors.CodeInvalidConfig,
					"grid.reset_key cannot be 'backspace' or 'delete'; these keys are reserved for input correction",
				)
			}
		} else {
			normalizedResetKey := NormalizeKeyForComparison(resetKey)

			normalizedBackspaceKey := NormalizeKeyForComparison(gridBackspaceKey)
			if normalizedResetKey == normalizedBackspaceKey {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"grid.reset_key cannot be the same as grid.backspace_key ('%s')",
					gridBackspaceKey,
				)
			}
		}

		// Single-character reset key cannot be in grid characters
		if strings.ContainsRune(
			strings.ToLower(c.Grid.Characters),
			rune(strings.ToLower(resetKey)[0]),
		) {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"grid.characters cannot contain '"+resetKey+"' as it is reserved for reset",
			)
		}
	}

	for _, r := range c.Grid.Characters {
		if r > unicode.MaxASCII {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"grid.characters can only contain ASCII characters",
			)
		}
	}

	// Check for duplicate characters (case-insensitive)
	if duplicates := findDuplicateChars(c.Grid.Characters); len(duplicates) > 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			fmt.Sprintf("grid.characters contains duplicate characters: %v", duplicates),
		)
	}

	// Validate row labels if provided
	if c.Grid.RowLabels != "" {
		if len(c.Grid.RowLabels) < MinCharactersLength {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"grid.row_labels must contain at least 2 characters if specified",
			)
		}

		// Only check for reset key conflict if reset key is a single character
		if len(resetKey) == 1 &&
			strings.ContainsRune(
				strings.ToLower(c.Grid.RowLabels),
				rune(strings.ToLower(resetKey)[0]),
			) {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"grid.row_labels cannot contain '"+resetKey+"' as it is reserved for reset",
			)
		}

		for _, r := range c.Grid.RowLabels {
			if r > unicode.MaxASCII {
				return derrors.New(
					derrors.CodeInvalidConfig,
					"grid.row_labels can only contain ASCII characters",
				)
			}
		}

		// Check for duplicate characters in row_labels
		if duplicates := findDuplicateChars(c.Grid.RowLabels); len(duplicates) > 0 {
			return derrors.New(
				derrors.CodeInvalidConfig,
				fmt.Sprintf("grid.row_labels contains duplicate characters: %v", duplicates),
			)
		}
	}

	// Validate col labels if provided
	if c.Grid.ColLabels != "" {
		if len(c.Grid.ColLabels) < MinCharactersLength {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"grid.col_labels must contain at least 2 characters if specified",
			)
		}

		// Only check for reset key conflict if reset key is a single character (case-insensitive)
		if len(resetKey) == 1 &&
			strings.ContainsRune(
				strings.ToLower(c.Grid.ColLabels),
				rune(strings.ToLower(resetKey)[0]),
			) {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"grid.col_labels cannot contain '"+resetKey+"' as it is reserved for reset",
			)
		}

		for _, r := range c.Grid.ColLabels {
			if r > unicode.MaxASCII {
				return derrors.New(
					derrors.CodeInvalidConfig,
					"grid.col_labels can only contain ASCII characters",
				)
			}
		}

		// Check for duplicate characters in col_labels
		if duplicates := findDuplicateChars(c.Grid.ColLabels); len(duplicates) > 0 {
			return derrors.New(
				derrors.CodeInvalidConfig,
				fmt.Sprintf("grid.col_labels contains duplicate characters: %v", duplicates),
			)
		}
	}

	if c.Grid.FontSize < 6 || c.Grid.FontSize > 72 {
		return derrors.New(derrors.CodeInvalidConfig, "grid.font_size must be between 6 and 72")
	}

	if c.Grid.BorderWidth < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "grid.border_width must be non-negative")
	}

	// Validate per-action grid colors
	err := validateColors([]colorField{
		{c.Grid.BackgroundColorLight, "grid.background_color_light"},
		{c.Grid.BackgroundColorDark, "grid.background_color_dark"},
		{c.Grid.TextColorLight, "grid.text_color_light"},
		{c.Grid.TextColorDark, "grid.text_color_dark"},
		{c.Grid.MatchedTextColorLight, "grid.matched_text_color_light"},
		{c.Grid.MatchedTextColorDark, "grid.matched_text_color_dark"},
		{c.Grid.MatchedBackgroundColorLight, "grid.matched_background_color_light"},
		{c.Grid.MatchedBackgroundColorDark, "grid.matched_background_color_dark"},
		{c.Grid.MatchedBorderColorLight, "grid.matched_border_color_light"},
		{c.Grid.MatchedBorderColorDark, "grid.matched_border_color_dark"},
		{c.Grid.BorderColorLight, "grid.border_color_light"},
		{c.Grid.BorderColorDark, "grid.border_color_dark"},
	})
	if err != nil {
		return err
	}

	// Validate sublayer keys (fallback to grid.characters) for 3x3 subgrid
	keys := strings.TrimSpace(c.Grid.SublayerKeys)
	if keys == "" {
		keys = c.Grid.Characters
	}

	// Apply same ASCII and reserved character validation as grid.characters
	// Only check for reset key conflict if reset key is a single character (case-insensitive)
	if len(resetKey) == 1 &&
		strings.ContainsRune(strings.ToLower(keys), rune(strings.ToLower(resetKey)[0])) {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"grid.sublayer_keys cannot contain '"+resetKey+"' as it is reserved for reset",
		)
	}

	for _, r := range keys {
		if r > unicode.MaxASCII {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"grid.sublayer_keys can only contain ASCII characters",
			)
		}
	}

	// Check for duplicate characters in sublayer_keys
	if duplicates := findDuplicateChars(keys); len(duplicates) > 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			fmt.Sprintf("grid.sublayer_keys contains duplicate characters: %v", duplicates),
		)
	}

	// Subgrid is always 3x3, requiring at least 9 characters
	const required = 9
	if len([]rune(keys)) < required {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"grid.sublayer_keys must contain at least %d characters for 3x3 subgrid selection",
			required,
		)
	}

	// Validate backspace key format
	err = validateBackspaceKeyFormat(c.Grid.BackspaceKey, "grid.backspace_key")
	if err != nil {
		return err
	}

	// Validate backspace key doesn't conflict with grid characters/labels/sublayer keys
	bsChecks := []struct {
		chars     string
		fieldName string
	}{
		{c.Grid.Characters, "grid.characters"},
		{c.Grid.RowLabels, "grid.row_labels"},
		{c.Grid.ColLabels, "grid.col_labels"},
		{keys, "grid.sublayer_keys"},
	}
	for _, check := range bsChecks {
		err = checkBackspaceKeyCharConflict(
			c.Grid.BackspaceKey,
			check.chars,
			"grid.backspace_key",
			check.fieldName,
			"grid input",
		)
		if err != nil {
			return err
		}
	}

	err = validateAutoExitActions(
		c.Grid.AutoExitActions,
		"grid.auto_exit_actions",
	)
	if err != nil {
		return err
	}

	return nil
}

// ValidateAction validates the action configuration.
func (c *Config) ValidateAction() error {
	if c.Action.MoveMouseStep <= 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"action.move_mouse_step must be positive",
		)
	}

	return c.ValidateActionKeyBindings()
}

// ValidateActionKeyBindings validates the action key_bindings configuration.
func (c *Config) ValidateActionKeyBindings() error {
	bindings := []struct {
		value     string
		fieldName string
	}{
		{c.Action.KeyBindings.LeftClick, "action.key_bindings.left_click"},
		{c.Action.KeyBindings.RightClick, "action.key_bindings.right_click"},
		{c.Action.KeyBindings.MiddleClick, "action.key_bindings.middle_click"},
		{c.Action.KeyBindings.MouseDown, "action.key_bindings.mouse_down"},
		{c.Action.KeyBindings.MouseUp, "action.key_bindings.mouse_up"},
		{c.Action.KeyBindings.MoveMouseUp, "action.key_bindings.move_mouse_up"},
		{c.Action.KeyBindings.MoveMouseDown, "action.key_bindings.move_mouse_down"},
		{c.Action.KeyBindings.MoveMouseLeft, "action.key_bindings.move_mouse_left"},
		{c.Action.KeyBindings.MoveMouseRight, "action.key_bindings.move_mouse_right"},
	}

	for _, b := range bindings {
		err := ValidateActionKeyBinding(b.value, b.fieldName)
		if err != nil {
			return err
		}
	}

	return nil
}

// ValidateActionKeyBinding validates an action keybinding.
// Valid formats:
//   - Single printable ASCII character: "L", "w", "1" (any character from ! to ~)
//   - Single named key: Return, Enter, Up, Down, F1, etc.
//   - Modifiers + key: Cmd+L, Shift+Return (at least 1 modifier + character or named key)
func ValidateActionKeyBinding(keybinding, fieldName string) error {
	if strings.TrimSpace(keybinding) == "" {
		return nil
	}

	normalizedKey := strings.TrimSpace(keybinding)

	// Format 1: Single named key (e.g. Return, Enter, Up, F1)
	if IsValidNamedKey(normalizedKey) {
		return nil
	}

	// Format 2: Single printable ASCII character (e.g. "L", "w", "1")
	if len(normalizedKey) == 1 {
		r := rune(normalizedKey[0])
		if r >= '!' && r <= '~' {
			return nil
		}
	}

	// Format 3: Modifiers + key (e.g., Cmd+L, Shift+Return)
	parts := strings.Split(normalizedKey, "+")

	const minParts = 2
	if len(parts) < minParts {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s must be a single character, a named key (e.g. Return, Up, F1), or have at least one modifier (e.g., Cmd+L, Shift+Return): %s",
			fieldName,
			keybinding,
		)
	}

	for index := range parts[:len(parts)-1] {
		modifier := strings.TrimSpace(parts[index])
		if !isValidModifier(modifier) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s has invalid modifier '%s' in: %s (valid: Cmd, Ctrl, Alt, Shift, Option)",
				fieldName,
				modifier,
				keybinding,
			)
		}
	}

	lastPart := parts[len(parts)-1]
	// Don't trim \r as it's a valid key, not whitespace to be removed
	var trimmedKey string
	if lastPart == "\r" {
		trimmedKey = "\r"
	} else {
		trimmedKey = strings.TrimSpace(lastPart)
	}

	if trimmedKey == "" {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s has empty key in: %s",
			fieldName,
			keybinding,
		)
	}

	// Validate the key part: single printable ASCII character, named key, or \r
	if trimmedKey == "\r" {
		// Single \r is valid
		return nil
	}

	if IsValidNamedKey(trimmedKey) {
		return nil
	}

	if len(trimmedKey) == 1 {
		r := rune(trimmedKey[0])
		if r >= '!' && r <= '~' {
			return nil
		}
	}

	return derrors.Newf(
		derrors.CodeInvalidConfig,
		"%s has invalid key '%s' in: %s (must be a single character, a named key like Return, Up, F1, etc.)",
		fieldName,
		trimmedKey,
		keybinding,
	)
}

// ValidateSmoothCursor validates the smooth cursor configuration.
func (c *Config) ValidateSmoothCursor() error {
	if c.SmoothCursor.Steps < 1 {
		return derrors.New(derrors.CodeInvalidConfig, "smooth_cursor.steps must be at least 1")
	}

	if c.SmoothCursor.Delay < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "smooth_cursor.delay must be non-negative")
	}

	return nil
}

// ValidateHotkey validates a hotkey string format.
func ValidateHotkey(hotkey, fieldName string) error {
	if strings.TrimSpace(hotkey) == "" {
		return nil // Allow empty hotkey to disable the action
	}

	// Hotkey format: [Modifier+]*Key
	// Valid modifiers: Cmd, Ctrl, Alt, Shift, Option
	// Examples: "Cmd+Shift+Space", "Ctrl+D", "F1"

	parts := strings.Split(hotkey, "+")
	if len(parts) == 0 {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s has invalid format: %s",
			fieldName,
			hotkey,
		)
	}

	// Check all parts except the last (which is the key)
	for index := range parts[:len(parts)-1] {
		modifier := strings.TrimSpace(parts[index])
		if !isValidModifier(modifier) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s has invalid modifier '%s' in: %s (valid: Cmd, Ctrl, Alt, Shift, Option)",
				fieldName,
				modifier,
				hotkey,
			)
		}
	}

	// Last part should be the trimmedKey (non-empty)
	trimmedKey := strings.TrimSpace(parts[len(parts)-1])
	if trimmedKey == "" {
		return derrors.Newf(derrors.CodeInvalidConfig, "%s has empty key in: %s", fieldName, hotkey)
	}

	return nil
}

// ValidateColor validates a color string (hex format).
// Empty string is valid and represents a theme-aware default.
func ValidateColor(color, fieldName string) error {
	if color == "" {
		return nil
	}

	// Match hex color format: #RGB, #RRGGBB, #AARRGGBB
	hexColorRegex := regexp.MustCompile(`^#([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6}|[0-9A-Fa-f]{8})$`)

	if !hexColorRegex.MatchString(color) {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s has invalid hex color format: %s (expected #RGB, #RRGGBB, or #AARRGGBB)",
			fieldName,
			color,
		)
	}

	return nil
}

// findDuplicateChars finds duplicate characters in a string (case-insensitive).
// Returns a slice of duplicate characters found.
func findDuplicateChars(s string) []rune {
	seen := make(map[rune]bool)
	duplicates := make(map[rune]bool)

	for _, r := range strings.ToUpper(s) {
		if seen[r] {
			duplicates[r] = true
		} else {
			seen[r] = true
		}
	}

	result := make([]rune, 0, len(duplicates))
	for r := range duplicates {
		result = append(result, r)
	}

	// Sort for deterministic output
	slices.Sort(result)

	return result
}

// ValidateModeExitKeys validates the mode exit keys configuration.
func (c *Config) ValidateModeExitKeys() error {
	if len(c.General.ModeExitKeys) == 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"general.mode_exit_keys cannot be empty",
		)
	}

	for index, key := range c.General.ModeExitKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"general.mode_exit_keys[%d] cannot be empty",
				index,
			)
		}

		// Check if it's a named key (uses centralized validNamedKeys registry)
		if IsValidNamedKey(key) || strings.EqualFold(key, "esc") {
			continue
		}

		// Check if it's a modifier combo
		if strings.Contains(key, "+") {
			err := validateModeExitKeyCombo(key, index)
			if err != nil {
				return err
			}

			continue
		}

		// Check if it's a single character
		if len(key) == 1 {
			continue
		}

		// Invalid format
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"general.mode_exit_keys[%d] = '%s' is invalid; must be a named key (e.g. 'escape'), modifier combo (e.g. 'Ctrl+C'), or single character",
			index,
			key,
		)
	}

	// Check for conflicts with hint and grid characters
	err := c.checkModeExitKeysConflicts()
	if err != nil {
		return err
	}

	return nil
}

// checkModeExitKeysConflicts detects if any single-character exit keys conflict with input characters.
func (c *Config) checkModeExitKeysConflicts() error {
	// Check for conflicts between exit keys and reset keys first.
	// This check uses normalized key comparison and handles both named keys (e.g. "space")
	// and single-character keys, so it must run before the single-char-only early return below.
	// Exit keys are checked before reset keys at runtime (in key_dispatch.go),
	// so if an exit key matches the reset key, reset will never work.
	err := c.checkExitKeysResetKeyConflicts()
	if err != nil {
		return err
	}

	// Extract single-character exit keys (case-insensitive)
	var singleCharExitKeys []string
	for _, key := range c.General.ModeExitKeys {
		if len(strings.TrimSpace(key)) == 1 && !strings.Contains(key, "+") {
			singleCharExitKeys = append(singleCharExitKeys, strings.ToLower(key))
		}
	}

	if len(singleCharExitKeys) == 0 {
		return nil // No single-char exit keys, no character-set conflicts possible
	}

	// Check for conflicts between exit keys and character sets
	checks := []struct {
		chars      string
		fieldName  string
		actionDesc string
	}{
		{c.Hints.HintCharacters, "hints.hint_characters", "selecting a hint"},
		{c.Grid.Characters, "grid.characters", "grid input"},
		{c.Grid.RowLabels, "grid.row_labels", "row selection"},
		{c.Grid.ColLabels, "grid.col_labels", "column selection"},
		{c.RecursiveGrid.Keys, "recursive_grid.keys", "cell selection"},
	}

	for _, check := range checks {
		err := checkExitKeyConflict(
			singleCharExitKeys,
			check.chars,
			check.fieldName,
			check.actionDesc,
		)
		if err != nil {
			return err
		}
	}

	// Check grid sublayer keys (with fallback to grid.characters)
	sublayerKeys := strings.TrimSpace(c.Grid.SublayerKeys)
	if sublayerKeys == "" {
		sublayerKeys = c.Grid.Characters
	}

	err = checkExitKeyConflict(
		singleCharExitKeys,
		sublayerKeys,
		"grid.sublayer_keys",
		"subgrid selection",
	)
	if err != nil {
		return err
	}

	return nil
}

// checkBackspaceKeyCharConflict checks if a configured backspace key conflicts with a character set.
// It normalizes both the backspace key and each character in the set to their canonical forms
// (via NormalizeKeyForComparison) before comparing, so named keys like "space" are correctly
// detected as conflicting with a space character in the character set.
// Modifier combos (e.g. "Ctrl+H") are skipped since they cannot conflict with single-character input.
func checkBackspaceKeyCharConflict(
	backspaceKey string,
	chars string,
	bsFieldName string,
	charsFieldName string,
	actionDesc string,
) error {
	if backspaceKey == "" || chars == "" {
		return nil
	}

	// Modifier combos can't conflict with single-character input
	if strings.Contains(backspaceKey, "+") {
		return nil
	}

	normalizedBS := NormalizeKeyForComparison(backspaceKey)
	for _, r := range chars {
		if NormalizeKeyForComparison(string(r)) == normalizedBS {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s '%s' conflicts with %s; pressing this key would always trigger backspace instead of %s",
				bsFieldName,
				backspaceKey,
				charsFieldName,
				actionDesc,
			)
		}
	}

	return nil
}

// checkResetKeyActionKeyConflicts checks if any mode's reset_key conflicts with an action
// key binding. At runtime, direct action keys are checked before reset keys
// (in mode_handlers.go), so a conflict means the reset key will never fire — the action
// will always take priority.
func (c *Config) checkResetKeyActionKeyConflicts() error {
	bindings := []struct {
		value     string
		fieldName string
	}{
		{c.Action.KeyBindings.LeftClick, "action.key_bindings.left_click"},
		{c.Action.KeyBindings.RightClick, "action.key_bindings.right_click"},
		{c.Action.KeyBindings.MiddleClick, "action.key_bindings.middle_click"},
		{c.Action.KeyBindings.MouseDown, "action.key_bindings.mouse_down"},
		{c.Action.KeyBindings.MouseUp, "action.key_bindings.mouse_up"},
		{c.Action.KeyBindings.MoveMouseUp, "action.key_bindings.move_mouse_up"},
		{c.Action.KeyBindings.MoveMouseDown, "action.key_bindings.move_mouse_down"},
		{c.Action.KeyBindings.MoveMouseLeft, "action.key_bindings.move_mouse_left"},
		{c.Action.KeyBindings.MoveMouseRight, "action.key_bindings.move_mouse_right"},
	}

	type modeResetKey struct {
		key       string
		modeName  string
		isEnabled bool
	}

	gridResetKey := c.Grid.ResetKey
	if gridResetKey == "" {
		gridResetKey = " "
	}

	rgResetKey := c.RecursiveGrid.ResetKey
	if rgResetKey == "" {
		rgResetKey = " "
	}

	modes := []modeResetKey{
		{gridResetKey, "grid", c.Grid.Enabled},
		{rgResetKey, "recursive_grid", c.RecursiveGrid.Enabled},
	}
	for _, mode := range modes {
		if !mode.isEnabled {
			continue
		}

		normalizedReset := NormalizeKeyForComparison(mode.key)
		for _, binding := range bindings {
			if binding.value == "" {
				continue
			}

			if normalizedReset == NormalizeKeyForComparison(binding.value) {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s.reset_key '%s' conflicts with %s ('%s'); the action key is checked first at runtime, so reset will never fire",
					mode.modeName,
					mode.key,
					binding.fieldName,
					binding.value,
				)
			}
		}
	}

	return nil
}

// checkBackspaceKeyActionKeyConflicts checks if any mode's backspace_key conflicts with
// an action key binding. At runtime, direct action keys are checked before backspace keys
// (in mode_handlers.go), so a conflict means the backspace key will never fire — the action
// will always take priority.
func (c *Config) checkBackspaceKeyActionKeyConflicts() error {
	bindings := []struct {
		value     string
		fieldName string
	}{
		{c.Action.KeyBindings.LeftClick, "action.key_bindings.left_click"},
		{c.Action.KeyBindings.RightClick, "action.key_bindings.right_click"},
		{c.Action.KeyBindings.MiddleClick, "action.key_bindings.middle_click"},
		{c.Action.KeyBindings.MouseDown, "action.key_bindings.mouse_down"},
		{c.Action.KeyBindings.MouseUp, "action.key_bindings.mouse_up"},
		{c.Action.KeyBindings.MoveMouseUp, "action.key_bindings.move_mouse_up"},
		{c.Action.KeyBindings.MoveMouseDown, "action.key_bindings.move_mouse_down"},
		{c.Action.KeyBindings.MoveMouseLeft, "action.key_bindings.move_mouse_left"},
		{c.Action.KeyBindings.MoveMouseRight, "action.key_bindings.move_mouse_right"},
	}

	type modeBackspaceKey struct {
		key       string
		modeName  string
		isEnabled bool
	}

	modes := []modeBackspaceKey{
		{c.Hints.BackspaceKey, "hints", c.Hints.Enabled},
		{c.Grid.BackspaceKey, "grid", c.Grid.Enabled},
		{c.RecursiveGrid.BackspaceKey, "recursive_grid", c.RecursiveGrid.Enabled},
	}
	for _, mode := range modes {
		if !mode.isEnabled {
			continue
		}

		for _, binding := range bindings {
			if binding.value == "" {
				continue
			}

			if mode.key == "" {
				// Default backspace key: check if any action binding normalizes to "delete"
				// (which is the canonical form of backspace/delete).
				if NormalizeKeyForComparison(binding.value) == KeyNameDelete {
					return derrors.Newf(
						derrors.CodeInvalidConfig,
						"%s uses the default backspace key which conflicts with %s ('%s'); the action key is checked first at runtime, so backspace will never fire",
						mode.modeName,
						binding.fieldName,
						binding.value,
					)
				}

				continue
			}

			normalizedBS := NormalizeKeyForComparison(mode.key)

			if normalizedBS == NormalizeKeyForComparison(binding.value) {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s.backspace_key '%s' conflicts with %s ('%s'); the action key is checked first at runtime, so backspace will never fire",
					mode.modeName,
					mode.key,
					binding.fieldName,
					binding.value,
				)
			}
		}
	}

	return nil
}

// checkExitKeyConflict checks if any single-character exit key conflicts with a character set.
func checkExitKeyConflict(
	exitKeys []string,
	chars string,
	fieldName string,
	actionDesc string,
) error {
	if chars == "" {
		return nil
	}

	lowerChars := strings.ToLower(chars)
	for _, exitKey := range exitKeys {
		if strings.Contains(lowerChars, exitKey) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"general.mode_exit_keys contains '%s' which conflicts with %s; this key will always exit instead of being used for %s",
				strings.ToUpper(exitKey),
				fieldName,
				actionDesc,
			)
		}
	}

	return nil
}

// checkExitKeysResetKeyConflicts detects if any exit key conflicts with the grid or recursive-grid reset key,
// or with any mode's configured backspace key.
// At runtime, exit keys are checked before reset/backspace keys (in key_dispatch.go), so a conflict means
// the reset/backspace key will never fire — the mode will exit instead.
func (c *Config) checkExitKeysResetKeyConflicts() error {
	if len(c.General.ModeExitKeys) == 0 {
		return nil
	}

	// Normalize all exit keys for comparison
	var normalizedExitKeys []string
	for _, key := range c.General.ModeExitKeys {
		normalizedExitKeys = append(
			normalizedExitKeys,
			NormalizeKeyForComparison(strings.TrimSpace(key)),
		)
	}

	// Check grid reset key conflict (only when grid mode is enabled)
	if c.Grid.Enabled {
		gridResetKey := c.Grid.ResetKey

		if gridResetKey == "" {
			gridResetKey = " "
		}

		// Only check single-character (non-modifier) reset keys; modifier combos (e.g. "Ctrl+R")
		// won't collide with the named/single-char exit keys checked here.
		if !strings.Contains(gridResetKey, "+") {
			normalizedGridReset := NormalizeKeyForComparison(gridResetKey)
			if slices.Contains(normalizedExitKeys, normalizedGridReset) {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"general.mode_exit_keys contains a key that conflicts with grid.reset_key ('%s'); the exit key will always take priority, making grid reset non-functional",
					gridResetKey,
				)
			}
		}
	}

	// Check recursive-grid reset key conflict (only when recursive-grid mode is enabled)
	if c.RecursiveGrid.Enabled {
		rgResetKey := c.RecursiveGrid.ResetKey

		if rgResetKey == "" {
			rgResetKey = " "
		}

		if !strings.Contains(rgResetKey, "+") {
			normalizedRGReset := NormalizeKeyForComparison(rgResetKey)
			if slices.Contains(normalizedExitKeys, normalizedRGReset) {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"general.mode_exit_keys contains a key that conflicts with recursive_grid.reset_key ('%s'); the exit key will always take priority, making recursive-grid reset non-functional",
					rgResetKey,
				)
			}
		}
	}
	// Check backspace key conflicts with exit keys for each mode.
	// At runtime, exit keys are checked before backspace keys, so a conflict means
	// the backspace key will never fire — the mode will exit instead.
	err := c.checkExitKeysBackspaceKeyConflicts(normalizedExitKeys)
	if err != nil {
		return err
	}

	return nil
}

// checkExitKeysBackspaceKeyConflicts detects if any exit key conflicts with a mode's configured backspace key.
// At runtime, exit keys are checked before backspace keys (in key_dispatch.go), so a conflict means
// the backspace key will never fire — the mode will exit instead.
func (c *Config) checkExitKeysBackspaceKeyConflicts(normalizedExitKeys []string) error {
	type modeBackspaceKey struct {
		key       string
		modeName  string
		isEnabled bool
	}

	modes := []modeBackspaceKey{
		{c.Hints.BackspaceKey, "hints", c.Hints.Enabled},
		{c.Grid.BackspaceKey, "grid", c.Grid.Enabled},
		{c.RecursiveGrid.BackspaceKey, "recursive_grid", c.RecursiveGrid.Enabled},
	}
	for _, mode := range modes {
		if !mode.isEnabled {
			continue
		}

		bsKey := mode.key
		if bsKey == "" {
			// Default backspace key: check if any exit key normalizes to "delete"
			// (which is the canonical form of backspace/delete).
			if slices.Contains(normalizedExitKeys, KeyNameDelete) {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"general.mode_exit_keys contains a key that conflicts with the default backspace key used by %s mode; the exit key will always take priority, making backspace non-functional",
					mode.modeName,
				)
			}

			continue
		}
		// Custom backspace key: only check non-modifier keys; modifier combos (e.g. "Ctrl+H")
		// won't collide with the named/single-char exit keys checked here.
		if !strings.Contains(bsKey, "+") {
			normalizedBS := NormalizeKeyForComparison(bsKey)
			if slices.Contains(normalizedExitKeys, normalizedBS) {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"general.mode_exit_keys contains a key that conflicts with %s.backspace_key ('%s'); the exit key will always take priority, making backspace non-functional",
					mode.modeName,
					bsKey,
				)
			}
		}
	}

	return nil
}

// validateModifierCombo validates a modifier combo key format (e.g. "Ctrl+C").
// The fieldName parameter is used in error messages to identify which config field is being validated.
func validateModifierCombo(key string, fieldName string) error {
	const minComboPartsLen = 2

	parts := strings.Split(key, "+")

	if len(parts) < minComboPartsLen {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s = '%s' is invalid; modifier combos must have format 'Modifier+Key'",
			fieldName,
			key,
		)
	}

	// All parts except the last should be valid modifiers
	for i := range len(parts) - 1 {
		modifier := strings.TrimSpace(parts[i])
		if !isValidModifier(modifier) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s has invalid modifier '%s' in '%s' (valid: Cmd, Ctrl, Alt, Shift, Option)",
				fieldName,
				modifier,
				key,
			)
		}
	}

	// Last part is the key
	lastKey := strings.TrimSpace(parts[len(parts)-1])
	if lastKey == "" {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s = '%s' has empty key",
			fieldName,
			key,
		)
	}

	return nil
}

// validateModeExitKeyCombo validates a modifier combo key format for mode exit keys.
func validateModeExitKeyCombo(key string, index int) error {
	return validateModifierCombo(key, fmt.Sprintf("general.mode_exit_keys[%d]", index))
}

// validateResetKeyCombo validates a modifier combo reset key format (e.g. "Ctrl+R").
// The fieldName parameter is used in error messages to identify which config field is being validated.
func validateResetKeyCombo(key string, fieldName string) error {
	return validateModifierCombo(key, fieldName)
}

// ValidateRecursiveGrid validates the recursive-grid configuration.
func (c *Config) ValidateRecursiveGrid() error {
	if !c.RecursiveGrid.Enabled {
		return nil
	}

	// Validate grid_cols (must be at least 2)
	if c.RecursiveGrid.GridCols < DefaultRecursiveGridMinGridCols {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.grid_cols must be at least 2",
		)
	}

	// Validate grid_rows (must be at least 2)
	if c.RecursiveGrid.GridRows < DefaultRecursiveGridMinGridRows {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.grid_rows must be at least 2",
		)
	}

	// Calculate expected key count based on grid dimensions
	expectedKeyCount := c.RecursiveGrid.GridCols * c.RecursiveGrid.GridRows

	// Validate keys - must match grid dimensions
	keys := strings.TrimSpace(c.RecursiveGrid.Keys)
	if keys == "" {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.keys cannot be empty",
		)
	}

	if utf8.RuneCountInString(keys) != expectedKeyCount {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"recursive_grid.keys must be exactly %d characters for grid_cols %d and grid_rows %d",
			expectedKeyCount,
			c.RecursiveGrid.GridCols,
			c.RecursiveGrid.GridRows,
		)
	}

	// Check for duplicate keys
	keyMap := make(map[rune]bool)
	for _, key := range keys {
		if keyMap[key] {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"recursive_grid.keys contains duplicate character: %c",
				key,
			)
		}

		keyMap[key] = true
	}

	// Validate ASCII
	for _, r := range keys {
		if r > unicode.MaxASCII {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"recursive_grid.keys can only contain ASCII characters",
			)
		}
	}

	// Validate min size width
	if c.RecursiveGrid.MinSizeWidth < 10 { //nolint:mnd
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.min_size_width must be at least 10",
		)
	}

	// Validate min size height
	if c.RecursiveGrid.MinSizeHeight < 10 { //nolint:mnd
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.min_size_height must be at least 10",
		)
	}

	// Validate max depth
	if c.RecursiveGrid.MaxDepth < 1 || c.RecursiveGrid.MaxDepth > 20 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.max_depth must be between 1 and 20",
		)
	}

	resetKey := c.RecursiveGrid.ResetKey
	if resetKey == "" {
		resetKey = " "
	}

	// Validate reset key format: single character, named key, or modifier combo
	switch {
	case strings.Contains(resetKey, "+"):
		// Validate modifier combo (e.g. "Ctrl+R")
		err := validateResetKeyCombo(resetKey, "recursive_grid.reset_key")
		if err != nil {
			return err
		}

		// Even modifier-combo reset keys must not conflict with the configured backspace key
		if rgBackspaceKey := c.RecursiveGrid.BackspaceKey; rgBackspaceKey != "" &&
			NormalizeKeyForComparison(resetKey) == NormalizeKeyForComparison(rgBackspaceKey) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"recursive_grid.reset_key cannot be the same as recursive_grid.backspace_key ('%s')",
				rgBackspaceKey,
			)
		}

		// Modifier combos don't conflict with recursive_grid keys, so no further checks needed
	case IsValidNamedKey(resetKey):
		// Named key reset (e.g. "Home", "F1", "Tab")
		// Named keys don't conflict with recursive_grid character sets, but check backspace conflict
		rgBackspaceKey := c.RecursiveGrid.BackspaceKey
		if rgBackspaceKey == "" {
			normalizedResetKey := NormalizeKeyForComparison(resetKey)
			if normalizedResetKey == KeyNameDelete {
				return derrors.New(
					derrors.CodeInvalidConfig,
					"recursive_grid.reset_key cannot be 'backspace' or 'delete'; these keys are reserved for input correction",
				)
			}
		} else {
			normalizedResetKey := NormalizeKeyForComparison(resetKey)

			normalizedBackspaceKey := NormalizeKeyForComparison(rgBackspaceKey)
			if normalizedResetKey == normalizedBackspaceKey {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"recursive_grid.reset_key cannot be the same as recursive_grid.backspace_key ('%s')",
					rgBackspaceKey,
				)
			}
		}
	default:
		// Validate single character reset key
		if len(resetKey) != 1 {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"recursive_grid.reset_key must be a single character, a named key (e.g. 'Home', 'F1'), or a modifier combo (e.g. 'Ctrl+R')",
			)
		}

		// Validate reset key is ASCII
		if rune(resetKey[0]) > unicode.MaxASCII {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"recursive_grid.reset_key must be an ASCII character",
			)
		}

		// Reset key cannot match the configured backspace key (they would conflict)
		rgBackspaceKey := c.RecursiveGrid.BackspaceKey
		if rgBackspaceKey == "" {
			// Default: backspace and delete are reserved for input correction.
			// NormalizeKeyForComparison maps all backspace/delete variants to KeyNameDelete.
			normalizedResetKey := NormalizeKeyForComparison(resetKey)
			if normalizedResetKey == KeyNameDelete {
				return derrors.New(
					derrors.CodeInvalidConfig,
					"recursive_grid.reset_key cannot be 'backspace' or 'delete'; these keys are reserved for input correction",
				)
			}
		} else {
			normalizedResetKey := NormalizeKeyForComparison(resetKey)

			normalizedBackspaceKey := NormalizeKeyForComparison(rgBackspaceKey)
			if normalizedResetKey == normalizedBackspaceKey {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"recursive_grid.reset_key cannot be the same as recursive_grid.backspace_key ('%s')",
					rgBackspaceKey,
				)
			}
		}

		// Single-character reset key cannot be in recursive_grid keys
		if strings.ContainsRune(
			strings.ToLower(c.RecursiveGrid.Keys),
			rune(strings.ToLower(resetKey)[0]),
		) {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"recursive_grid.keys cannot contain '"+resetKey+"' as it is reserved for reset",
			)
		}
	}

	// Validate backspace key format
	err := validateBackspaceKeyFormat(
		c.RecursiveGrid.BackspaceKey,
		"recursive_grid.backspace_key",
	)
	if err != nil {
		return err
	}

	// Validate backspace key doesn't conflict with recursive_grid keys
	err = checkBackspaceKeyCharConflict(
		c.RecursiveGrid.BackspaceKey,
		keys,
		"recursive_grid.backspace_key",
		"recursive_grid.keys",
		"cell selection",
	)
	if err != nil {
		return err
	}

	// Validate styling
	if c.RecursiveGrid.LineWidth < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.line_width must be non-negative",
		)
	}

	if c.RecursiveGrid.FontSize < 6 || c.RecursiveGrid.FontSize > 72 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.font_size must be between 6 and 72",
		)
	}

	err = validateMinValue(
		c.RecursiveGrid.LabelBackgroundCornerRadius,
		-1,
		"recursive_grid.label_background_corner_radius",
	)
	if err != nil {
		return err
	}

	err = validateMinValue(
		c.RecursiveGrid.LabelBackgroundPaddingX,
		-1,
		"recursive_grid.label_background_padding_x",
	)
	if err != nil {
		return err
	}

	err = validateMinValue(
		c.RecursiveGrid.LabelBackgroundPaddingY,
		-1,
		"recursive_grid.label_background_padding_y",
	)
	if err != nil {
		return err
	}

	if c.RecursiveGrid.LabelBackgroundBorderWidth < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.label_background_border_width must be non-negative",
		)
	}

	// Validate colors
	colorFields := []colorField{
		{c.RecursiveGrid.LineColorLight, "recursive_grid.line_color_light"},
		{c.RecursiveGrid.LineColorDark, "recursive_grid.line_color_dark"},
		{c.RecursiveGrid.HighlightColorLight, "recursive_grid.highlight_color_light"},
		{c.RecursiveGrid.HighlightColorDark, "recursive_grid.highlight_color_dark"},
		{c.RecursiveGrid.TextColorLight, "recursive_grid.text_color_light"},
		{c.RecursiveGrid.TextColorDark, "recursive_grid.text_color_dark"},
		{c.RecursiveGrid.LabelBackgroundColorLight, "recursive_grid.label_background_color_light"},
		{c.RecursiveGrid.LabelBackgroundColorDark, "recursive_grid.label_background_color_dark"},
	}

	err = validateColors(colorFields)
	if err != nil {
		return err
	}

	err = validateAutoExitActions(
		c.RecursiveGrid.AutoExitActions,
		"recursive_grid.auto_exit_actions",
	)
	if err != nil {
		return err
	}

	return nil
}
