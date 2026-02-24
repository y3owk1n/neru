package config

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

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
		{c.Hints.BackgroundColor, "hints.background_color"},
		{c.Hints.TextColor, "hints.text_color"},
		{c.Hints.MatchedTextColor, "hints.matched_text_color"},
		{c.Hints.BorderColor, "hints.border_color"},
	})
	if err != nil {
		return err
	}

	if c.Hints.FontSize < 6 || c.Hints.FontSize > 72 {
		return derrors.New(derrors.CodeInvalidConfig, "hints.font_size must be between 6 and 72")
	}

	if c.Hints.BorderRadius < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "hints.border_radius must be non-negative")
	}

	if c.Hints.Padding < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "hints.padding must be non-negative")
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
		resetKey = ","
	}

	// Validate reset key format: either single character or modifier combo
	if strings.Contains(resetKey, "+") {
		// Validate modifier combo (e.g. "Ctrl+R")
		err := validateResetKeyCombo(resetKey)
		if err != nil {
			return err
		}
		// Modifier combos don't conflict with grid characters, so we can return early
	} else {
		// Validate single character reset key
		if len(resetKey) != 1 {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"grid.reset_key must be either a single character or a modifier combo (e.g. 'Ctrl+R')",
			)
		}

		// Validate reset key is ASCII
		if rune(resetKey[0]) > unicode.MaxASCII {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"grid.reset_key must be an ASCII character",
			)
		}

		// Backspace and delete are reserved for input correction
		normalizedResetKey := NormalizeKeyForComparison(resetKey)
		if normalizedResetKey == KeyNameBackspace || normalizedResetKey == KeyNameDelete {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"grid.reset_key cannot be 'backspace' or 'delete'; these keys are reserved for input correction",
			)
		}

		// Single-character reset key cannot be in grid characters
		if strings.Contains(strings.ToLower(c.Grid.Characters), strings.ToLower(resetKey)) {
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
		lowerResetKey := strings.ToLower(resetKey)
		lowerRowLabels := strings.ToLower(c.Grid.RowLabels)

		if !strings.Contains(resetKey, "+") && strings.Contains(lowerRowLabels, lowerResetKey) {
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

		lowerResetKey := strings.ToLower(resetKey)

		lowerColLabels := strings.ToLower(c.Grid.ColLabels)
		if !strings.Contains(resetKey, "+") && strings.Contains(lowerColLabels, lowerResetKey) {
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
		{c.Grid.BackgroundColor, "grid.background_color"},
		{c.Grid.TextColor, "grid.text_color"},
		{c.Grid.MatchedTextColor, "grid.matched_text_color"},
		{c.Grid.MatchedBackgroundColor, "grid.matched_background_color"},
		{c.Grid.MatchedBorderColor, "grid.matched_border_color"},
		{c.Grid.BorderColor, "grid.border_color"},
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

	lowerResetKey := strings.ToLower(resetKey)

	lowerKeys := strings.ToLower(keys)
	if !strings.Contains(resetKey, "+") && strings.Contains(lowerKeys, lowerResetKey) {
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
//   - Modifiers + key: Cmd+L, Shift+Return (at least 1 modifier + alphabet or Return/Enter)
//   - Single special key: Return, Enter
func ValidateActionKeyBinding(keybinding, fieldName string) error {
	if strings.TrimSpace(keybinding) == "" {
		return nil
	}

	normalizedKey := strings.TrimSpace(keybinding)

	// Format 1: Single special key (Return, Enter)
	if normalizedKey == "Return" || normalizedKey == "Enter" {
		return nil
	}

	// Format 2: Modifiers + key (e.g., Cmd+L, Shift+Return)
	parts := strings.Split(normalizedKey, "+")

	const minParts = 2
	if len(parts) < minParts {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s must have at least one modifier (e.g., Cmd+L, Shift+Return): %s",
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

	// Validate the key part (must be alphabet A-Z, Return, Enter, or \r)
	if len(trimmedKey) == 1 {
		if trimmedKey == "\r" {
			// Single \r is valid
		} else {
			r := rune(trimmedKey[0])
			if r < 'A' || r > 'Z' {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s has invalid key '%s' in: %s (must be alphabet A-Z, Return, or Enter)",
					fieldName,
					trimmedKey,
					keybinding,
				)
			}
		}
	} else if trimmedKey != "Return" && trimmedKey != "Enter" {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s has invalid key '%s' in: %s (must be alphabet A-Z, Return, or Enter)",
			fieldName,
			trimmedKey,
			keybinding,
		)
	}

	return nil
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
func ValidateColor(color, fieldName string) error {
	if strings.TrimSpace(color) == "" {
		return derrors.Newf(derrors.CodeInvalidConfig, "%s cannot be empty", fieldName)
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

	validNamedKeys := map[string]bool{
		"escape":    true,
		"esc":       true,
		"return":    true,
		"enter":     true,
		"tab":       true,
		"space":     true,
		"backspace": true,
		"delete":    true,
		"home":      true,
		"end":       true,
		"pageup":    true,
		"pagedown":  true,
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

		// Check if it's a named key
		if validNamedKeys[strings.ToLower(key)] {
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
	// Extract single-character exit keys (case-insensitive)
	var singleCharExitKeys []string
	for _, key := range c.General.ModeExitKeys {
		if len(strings.TrimSpace(key)) == 1 && !strings.Contains(key, "+") {
			singleCharExitKeys = append(singleCharExitKeys, strings.ToLower(key))
		}
	}

	if len(singleCharExitKeys) == 0 {
		return nil // No single-char exit keys, no conflicts possible
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

	err := checkExitKeyConflict(
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
func validateResetKeyCombo(key string) error {
	return validateModifierCombo(key, "grid.reset_key")
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
		resetKey = ","
	}

	// Validate reset key format: either single character or modifier combo
	if strings.Contains(resetKey, "+") {
		// Validate modifier combo (e.g. "Ctrl+R")
		err := validateResetKeyCombo(resetKey)
		if err != nil {
			return err
		}
		// Modifier combos don't conflict with recursive_grid keys, so we can return early
	} else {
		// Validate single character reset key
		if len(resetKey) != 1 {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"recursive_grid.reset_key must be either a single character or a modifier combo (e.g. 'Ctrl+R')",
			)
		}

		// Validate reset key is ASCII
		if rune(resetKey[0]) > unicode.MaxASCII {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"recursive_grid.reset_key must be an ASCII character",
			)
		}

		// Backspace and delete are reserved for input correction
		normalizedResetKey := NormalizeKeyForComparison(resetKey)
		if normalizedResetKey == KeyNameBackspace || normalizedResetKey == KeyNameDelete {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"recursive_grid.reset_key cannot be 'backspace' or 'delete'; these keys are reserved for input correction",
			)
		}

		// Single-character reset key cannot be in recursive_grid keys
		if strings.Contains(strings.ToLower(c.RecursiveGrid.Keys), strings.ToLower(resetKey)) {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"recursive_grid.keys cannot contain '"+resetKey+"' as it is reserved for reset",
			)
		}
	}

	// Validate styling
	if c.RecursiveGrid.LineWidth < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.line_width must be non-negative",
		)
	}

	if c.RecursiveGrid.LabelFontSize < 6 || c.RecursiveGrid.LabelFontSize > 72 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.label_font_size must be between 6 and 72",
		)
	}

	// Validate colors
	err := validateColors([]colorField{
		{c.RecursiveGrid.LineColor, "recursive_grid.line_color"},
		{c.RecursiveGrid.HighlightColor, "recursive_grid.highlight_color"},
		{c.RecursiveGrid.LabelColor, "recursive_grid.label_color"},
	})
	if err != nil {
		return err
	}

	return nil
}
