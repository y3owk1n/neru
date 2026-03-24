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
	// Right/Left-prefixed modifiers (commonly used by remappers like Karabiner)
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
		{c.Hints.UI.BackgroundColorLight, "hints.ui.background_color_light"},
		{c.Hints.UI.BackgroundColorDark, "hints.ui.background_color_dark"},
		{c.Hints.UI.TextColorLight, "hints.ui.text_color_light"},
		{c.Hints.UI.TextColorDark, "hints.ui.text_color_dark"},
		{c.Hints.UI.MatchedTextColorLight, "hints.ui.matched_text_color_light"},
		{c.Hints.UI.MatchedTextColorDark, "hints.ui.matched_text_color_dark"},
		{c.Hints.UI.BorderColorLight, "hints.ui.border_color_light"},
		{c.Hints.UI.BorderColorDark, "hints.ui.border_color_dark"},
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

	// Validate per-mode exit keys
	if len(c.Hints.ModeExitKeys) > 0 {
		err = validatePerModeExitKeysFormat(c.Hints.ModeExitKeys, "hints.mode_exit_keys")
		if err != nil {
			return err
		}

		err = checkPerModeExitKeyCharConflict(
			c.Hints.ModeExitKeys,
			c.Hints.HintCharacters,
			"hints.mode_exit_keys",
			"hints.hint_characters",
			"selecting a hint",
		)
		if err != nil {
			return err
		}

		err = checkPerModeExitKeysBackspaceConflict(
			c.Hints.ModeExitKeys,
			c.Hints.BackspaceKey,
			"hints.mode_exit_keys",
			"hints",
		)
		if err != nil {
			return err
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
	for key, actions := range c.Hotkeys.Bindings {
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

		if len(actions) == 0 {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"hotkeys.bindings[%s] cannot have an empty action array",
				key,
			)
		}

		for _, action := range actions {
			if strings.TrimSpace(action) == "" {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"hotkeys.bindings[%s] contains an empty action",
					key,
				)
			}
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

	if c.Grid.UI.FontSize < 6 || c.Grid.UI.FontSize > 72 {
		return derrors.New(derrors.CodeInvalidConfig, "grid.ui.font_size must be between 6 and 72")
	}

	if c.Grid.UI.BorderWidth < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "grid.ui.border_width must be non-negative")
	}

	// Validate per-action grid colors
	err := validateColors([]colorField{
		{c.Grid.UI.BackgroundColorLight, "grid.ui.background_color_light"},
		{c.Grid.UI.BackgroundColorDark, "grid.ui.background_color_dark"},
		{c.Grid.UI.TextColorLight, "grid.ui.text_color_light"},
		{c.Grid.UI.TextColorDark, "grid.ui.text_color_dark"},
		{c.Grid.UI.MatchedTextColorLight, "grid.ui.matched_text_color_light"},
		{c.Grid.UI.MatchedTextColorDark, "grid.ui.matched_text_color_dark"},
		{c.Grid.UI.MatchedBackgroundColorLight, "grid.ui.matched_background_color_light"},
		{c.Grid.UI.MatchedBackgroundColorDark, "grid.ui.matched_background_color_dark"},
		{c.Grid.UI.MatchedBorderColorLight, "grid.ui.matched_border_color_light"},
		{c.Grid.UI.MatchedBorderColorDark, "grid.ui.matched_border_color_dark"},
		{c.Grid.UI.BorderColorLight, "grid.ui.border_color_light"},
		{c.Grid.UI.BorderColorDark, "grid.ui.border_color_dark"},
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

	// Validate per-mode exit keys
	if len(c.Grid.ModeExitKeys) > 0 {
		err = validatePerModeExitKeysFormat(c.Grid.ModeExitKeys, "grid.mode_exit_keys")
		if err != nil {
			return err
		}

		gridExitKeyChecks := []struct {
			chars      string
			fieldName  string
			actionDesc string
		}{
			{c.Grid.Characters, "grid.characters", "grid input"},
			{c.Grid.RowLabels, "grid.row_labels", "row selection"},
			{c.Grid.ColLabels, "grid.col_labels", "column selection"},
			{keys, "grid.sublayer_keys", "subgrid selection"},
		}
		for _, check := range gridExitKeyChecks {
			err = checkPerModeExitKeyCharConflict(
				c.Grid.ModeExitKeys,
				check.chars,
				"grid.mode_exit_keys",
				check.fieldName,
				check.actionDesc,
			)
			if err != nil {
				return err
			}
		}

		err = checkPerModeExitKeysBackspaceConflict(
			c.Grid.ModeExitKeys,
			c.Grid.BackspaceKey,
			"grid.mode_exit_keys",
			"grid",
		)
		if err != nil {
			return err
		}

		err = checkPerModeExitKeysResetKeyConflict(
			c.Grid.ModeExitKeys,
			c.Grid.ResetKey,
			"grid.mode_exit_keys",
			"grid",
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
				"%s has invalid modifier '%s' in: %s (valid: Cmd, Ctrl, Alt, Shift, Option, and Right*/Left* variants e.g. RightCmd, LeftShift)",
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

// ValidateStickyModifiers validates the sticky modifiers configuration.
func (c *Config) ValidateStickyModifiers() error {
	if c.StickyModifiers.TapMaxDuration < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"sticky_modifiers.tap_max_duration must be non-negative",
		)
	}

	if c.StickyModifiers.TapCooldown < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"sticky_modifiers.tap_cooldown must be non-negative",
		)
	}

	err := validateMinValue(
		c.StickyModifiers.UI.FontSize,
		1,
		"sticky_modifiers.ui.font_size",
	)
	if err != nil {
		return err
	}

	err = validateMinValue(
		c.StickyModifiers.UI.BorderWidth,
		0,
		"sticky_modifiers.ui.border_width",
	)
	if err != nil {
		return err
	}

	err = validateMinValue(c.StickyModifiers.UI.PaddingX, -1, "sticky_modifiers.ui.padding_x")
	if err != nil {
		return err
	}

	err = validateMinValue(c.StickyModifiers.UI.PaddingY, -1, "sticky_modifiers.ui.padding_y")
	if err != nil {
		return err
	}

	err = validateMinValue(
		c.StickyModifiers.UI.BorderRadius,
		-1,
		"sticky_modifiers.ui.border_radius",
	)
	if err != nil {
		return err
	}

	err = validateColors([]colorField{
		{c.StickyModifiers.UI.BackgroundColorLight, "sticky_modifiers.ui.background_color_light"},
		{c.StickyModifiers.UI.BackgroundColorDark, "sticky_modifiers.ui.background_color_dark"},
		{c.StickyModifiers.UI.TextColorLight, "sticky_modifiers.ui.text_color_light"},
		{c.StickyModifiers.UI.TextColorDark, "sticky_modifiers.ui.text_color_dark"},
		{c.StickyModifiers.UI.BorderColorLight, "sticky_modifiers.ui.border_color_light"},
		{c.StickyModifiers.UI.BorderColorDark, "sticky_modifiers.ui.border_color_dark"},
	})
	if err != nil {
		return err
	}

	return nil
}

// ValidateSmoothCursor validates the smooth cursor configuration.
func (c *Config) ValidateSmoothCursor() error {
	if c.SmoothCursor.Steps < 1 {
		return derrors.New(derrors.CodeInvalidConfig, "smooth_cursor.steps must be at least 1")
	}

	if c.SmoothCursor.MaxDuration < 1 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"smooth_cursor.max_duration must be at least 1",
		)
	}

	if c.SmoothCursor.DurationPerPixel <= 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"smooth_cursor.duration_per_pixel must be positive",
		)
	}

	return nil
}

// ValidateCustomHotkeys validates the custom_hotkeys configuration for all modes.
func (c *Config) ValidateCustomHotkeys() error {
	type modeCustomHotkeys struct {
		hotkeys  map[string]StringOrStringArray
		modeName string
	}

	modes := []modeCustomHotkeys{
		{c.Hints.CustomHotkeys, modeNameHints},
		{c.Grid.CustomHotkeys, modeNameGrid},
		{c.RecursiveGrid.CustomHotkeys, modeNameRecursiveGrid},
		{c.Scroll.CustomHotkeys, modeNameScroll},
	}
	for _, mode := range modes {
		for key, actions := range mode.hotkeys {
			fieldName := mode.modeName + ".custom_hotkeys"
			if strings.TrimSpace(key) == "" {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s contains an empty key",
					fieldName,
				)
			}

			err := ValidateHotkey(key, fieldName)
			if err != nil {
				return err
			}

			if len(actions) == 0 {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s[%s] cannot have an empty action array",
					fieldName,
					key,
				)
			}

			for _, action := range actions {
				if strings.TrimSpace(action) == "" {
					return derrors.Newf(
						derrors.CodeInvalidConfig,
						"%s[%s] contains an empty action",
						fieldName,
						key,
					)
				}
			}
		}
	}
	// Check for conflicts between custom hotkeys and other mode bindings.
	// At runtime, exit keys are checked before custom hotkeys (making the
	// custom hotkey unreachable), and custom hotkeys are checked before
	// mode-specific keys (shadowing them).
	err := c.checkCustomHotkeysConflicts()
	if err != nil {
		return err
	}

	return nil
}

// checkCustomHotkeysConflicts detects conflicts between per-mode custom hotkeys and
// other key bindings. At runtime the priority order is:
//
//	exit keys > custom hotkeys > mode-specific keys
//
// So a custom hotkey that matches an exit key is unreachable, and a custom hotkey that
// matches a mode-specific binding (action keys, backspace, reset, characters, scroll
// bindings) will shadow that binding.
func (c *Config) checkCustomHotkeysConflicts() error {
	// Collect action key bindings once — they apply to every mode.
	actionBindings := []struct {
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

	type modeInfo struct {
		hotkeys  map[string]StringOrStringArray
		modeName string
	}

	modes := []modeInfo{
		{c.Hints.CustomHotkeys, modeNameHints},
		{c.Grid.CustomHotkeys, modeNameGrid},
		{c.RecursiveGrid.CustomHotkeys, modeNameRecursiveGrid},
		{c.Scroll.CustomHotkeys, modeNameScroll},
	}
	for _, mode := range modes {
		if len(mode.hotkeys) == 0 {
			continue
		}

		fieldName := mode.modeName + ".custom_hotkeys"
		// Resolve effective exit keys for this mode (global + per-mode merged).
		exitKeys := c.ResolvedExitKeys(mode.modeName)
		for hotkeyKey := range mode.hotkeys {
			normalizedHK := NormalizeKeyForComparison(hotkeyKey)
			// 1. Custom hotkey vs exit keys → custom hotkey is unreachable.
			if IsExitKey(hotkeyKey, exitKeys) {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s[%s] conflicts with an exit key; exit keys are checked first at runtime, so this custom hotkey will never fire",
					fieldName,
					hotkeyKey,
				)
			}

			// 2. Custom hotkey vs action key bindings → shadows the action.
			for _, actionBinding := range actionBindings {
				if actionBinding.value == "" {
					continue
				}

				if normalizedHK == NormalizeKeyForComparison(actionBinding.value) {
					return derrors.Newf(
						derrors.CodeInvalidConfig,
						"%s[%s] conflicts with %s ('%s'); the custom hotkey is checked first at runtime, so the action binding will never fire in %s mode",
						fieldName,
						hotkeyKey,
						actionBinding.fieldName,
						actionBinding.value,
						mode.modeName,
					)
				}
			}

			// 3. Mode-specific conflicts.
			err := c.checkCustomHotkeyModeSpecificConflict(
				mode.modeName, fieldName, hotkeyKey, normalizedHK,
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// checkCustomHotkeyModeSpecificConflict checks a single custom hotkey against
// mode-specific bindings (characters, backspace, reset, scroll bindings).
func (c *Config) checkCustomHotkeyModeSpecificConflict(
	modeName, fieldName, hotkeyKey, normalizedHK string,
) error {
	switch modeName {
	case modeNameHints:
		// Hint characters are single chars; only single-char (non-modifier) hotkeys can conflict.
		if !strings.Contains(hotkeyKey, "+") && len(hotkeyKey) == 1 {
			if strings.Contains(
				strings.ToLower(c.Hints.HintCharacters),
				strings.ToLower(hotkeyKey),
			) {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s[%s] conflicts with hints.hint_characters; the custom hotkey is checked first at runtime, so the hint character will be consumed",
					fieldName,
					hotkeyKey,
				)
			}
		}

		err := checkCustomHotkeyBackspaceConflict(
			fieldName, hotkeyKey, normalizedHK,
			c.Hints.BackspaceKey, modeNameHints,
		)
		if err != nil {
			return err
		}
	case modeNameGrid:
		if !strings.Contains(hotkeyKey, "+") && len(hotkeyKey) == 1 {
			lowerHK := strings.ToLower(hotkeyKey)

			// Resolve sublayer keys with fallback to grid.characters (same as ValidateGrid).
			sublayerKeys := strings.TrimSpace(c.Grid.SublayerKeys)
			if sublayerKeys == "" {
				sublayerKeys = c.Grid.Characters
			}

			charSets := []struct {
				chars     string
				fieldDesc string
			}{
				{c.Grid.Characters, "grid.characters"},
				{c.Grid.RowLabels, "grid.row_labels"},
				{c.Grid.ColLabels, "grid.col_labels"},
				{sublayerKeys, "grid.sublayer_keys"},
			}
			for _, char := range charSets {
				if char.chars != "" && strings.Contains(strings.ToLower(char.chars), lowerHK) {
					return derrors.Newf(
						derrors.CodeInvalidConfig,
						"%s[%s] conflicts with %s; the custom hotkey is checked first at runtime, so the grid character will be consumed",
						fieldName,
						hotkeyKey,
						char.fieldDesc,
					)
				}
			}
		}

		err := checkCustomHotkeyBackspaceConflict(
			fieldName, hotkeyKey, normalizedHK,
			c.Grid.BackspaceKey, modeNameGrid,
		)
		if err != nil {
			return err
		}

		err = checkCustomHotkeyResetKeyConflict(
			fieldName, hotkeyKey, normalizedHK,
			c.Grid.ResetKey, modeNameGrid,
		)
		if err != nil {
			return err
		}
	case modeNameRecursiveGrid:
		if !strings.Contains(hotkeyKey, "+") && len(hotkeyKey) == 1 {
			allKeys := c.RecursiveGrid.AllKeysIncludingLayers()
			if strings.Contains(strings.ToLower(allKeys), strings.ToLower(hotkeyKey)) {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s[%s] conflicts with recursive_grid.keys (including layers); the custom hotkey is checked first at runtime, so the cell key will be consumed",
					fieldName,
					hotkeyKey,
				)
			}
		}

		err := checkCustomHotkeyBackspaceConflict(
			fieldName, hotkeyKey, normalizedHK,
			c.RecursiveGrid.BackspaceKey, modeNameRecursiveGrid,
		)
		if err != nil {
			return err
		}

		err = checkCustomHotkeyResetKeyConflict(
			fieldName, hotkeyKey, normalizedHK,
			c.RecursiveGrid.ResetKey, modeNameRecursiveGrid,
		)
		if err != nil {
			return err
		}
	case modeNameScroll:
		for scrollAction, keys := range c.Scroll.KeyBindings {
			for _, scrollKey := range keys {
				if NormalizeKeyForComparison(scrollKey) == normalizedHK {
					return derrors.Newf(
						derrors.CodeInvalidConfig,
						"%s[%s] conflicts with scroll.key_bindings['%s'] ('%s'); the custom hotkey is checked first at runtime, so the scroll binding will never fire",
						fieldName,
						hotkeyKey,
						scrollAction,
						scrollKey,
					)
				}

				// Check prefix conflicts: a custom hotkey that matches the first
				// character of a multi-letter scroll sequence (e.g. custom hotkey "g"
				// vs scroll "gg") will consume the keystroke before the scroll
				// handler can start the sequence, silently breaking it.
				if len(scrollKey) >= 2 && IsAllLetters(scrollKey) {
					prefix := strings.ToLower(scrollKey[:1])
					if prefix == normalizedHK {
						return derrors.Newf(
							derrors.CodeInvalidConfig,
							"%s[%s] conflicts with the first key of scroll.key_bindings['%s'] sequence '%s'; the custom hotkey is checked first at runtime, so the sequence can never start",
							fieldName,
							hotkeyKey,
							scrollAction,
							scrollKey,
						)
					}
				}
			}
		}
	}

	return nil
}

// checkCustomHotkeyBackspaceConflict checks if a custom hotkey conflicts with a mode's
// backspace key. Custom hotkeys are checked before backspace at runtime.
func checkCustomHotkeyBackspaceConflict(
	fieldName, hotkeyKey, normalizedHK string,
	backspaceKey, modeName string,
) error {
	var normalizedBS string
	if backspaceKey == "" {
		normalizedBS = KeyNameDelete
	} else {
		normalizedBS = NormalizeKeyForComparison(backspaceKey)
	}

	if normalizedHK == normalizedBS {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s[%s] conflicts with %s.backspace_key; the custom hotkey is checked first at runtime, so backspace will never fire",
			fieldName,
			hotkeyKey,
			modeName,
		)
	}

	return nil
}

// checkCustomHotkeyResetKeyConflict checks if a custom hotkey conflicts with a mode's
// reset key. Custom hotkeys are checked before reset at runtime.
func checkCustomHotkeyResetKeyConflict(
	fieldName, hotkeyKey, normalizedHK string,
	resetKey, modeName string,
) error {
	if resetKey == "" {
		resetKey = " "
	}

	if normalizedHK == NormalizeKeyForComparison(resetKey) {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s[%s] conflicts with %s.reset_key; the custom hotkey is checked first at runtime, so reset will never fire",
			fieldName,
			hotkeyKey,
			modeName,
		)
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
				"%s has invalid modifier '%s' in: %s (valid: Cmd, Ctrl, Alt, Shift, Option, and Right*/Left* variants e.g. RightCmd, LeftShift)",
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
		{
			c.RecursiveGrid.AllKeysIncludingLayers(),
			"recursive_grid.keys (including layers)",
			"cell selection",
		},
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
				"%s has invalid modifier '%s' in '%s' (valid: Cmd, Ctrl, Alt, Shift, Option, and Right*/Left* variants e.g. RightCmd, LeftShift)",
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

// validatePerModeExitKeysFormat validates the format of a per-mode mode_exit_keys slice.
// It reuses the same rules as general.mode_exit_keys: named keys, modifier combos, or single characters.
func validatePerModeExitKeysFormat(keys []string, fieldName string) error {
	for index, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s[%d] cannot be empty",
				fieldName,
				index,
			)
		}

		if IsValidNamedKey(key) || strings.EqualFold(key, "esc") {
			continue
		}

		if strings.Contains(key, "+") {
			err := validateModifierCombo(key, fmt.Sprintf("%s[%d]", fieldName, index))
			if err != nil {
				return err
			}

			continue
		}

		if len(key) == 1 {
			continue
		}

		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s[%d] = '%s' is invalid; must be a named key (e.g. 'escape'), modifier combo (e.g. 'Ctrl+C'), or single character",
			fieldName,
			index,
			key,
		)
	}

	return nil
}

// extractSingleCharExitKeys returns the single-character exit keys (lowercased) from a key list.
func extractSingleCharExitKeys(keys []string) []string {
	var result []string
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if len(trimmed) == 1 && !strings.Contains(trimmed, "+") {
			result = append(result, strings.ToLower(trimmed))
		}
	}

	return result
}

// checkPerModeExitKeyCharConflict checks if any single-character exit key in modeExitKeys
// conflicts with a character set.
func checkPerModeExitKeyCharConflict(
	modeExitKeys []string,
	chars string,
	exitKeysFieldName string,
	charsFieldName string,
	actionDesc string,
) error {
	singleCharKeys := extractSingleCharExitKeys(modeExitKeys)
	if len(singleCharKeys) == 0 || chars == "" {
		return nil
	}

	lowerChars := strings.ToLower(chars)
	for _, exitKey := range singleCharKeys {
		if strings.Contains(lowerChars, exitKey) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s contains '%s' which conflicts with %s; this key will always exit instead of being used for %s",
				exitKeysFieldName,
				strings.ToUpper(exitKey),
				charsFieldName,
				actionDesc,
			)
		}
	}

	return nil
}

// checkPerModeExitKeysBackspaceConflict checks if any per-mode exit key conflicts with
// the mode's configured backspace key. At runtime, exit keys are checked before backspace,
// so a conflict means backspace will never fire.
func checkPerModeExitKeysBackspaceConflict(
	modeExitKeys []string,
	backspaceKey string,
	exitKeysFieldName string,
	modeName string,
) error {
	if len(modeExitKeys) == 0 {
		return nil
	}

	var normalizedExitKeys []string
	for _, key := range modeExitKeys {
		normalizedExitKeys = append(
			normalizedExitKeys,
			NormalizeKeyForComparison(strings.TrimSpace(key)),
		)
	}

	if backspaceKey == "" {
		if slices.Contains(normalizedExitKeys, KeyNameDelete) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s contains a key that conflicts with the default backspace key used by %s mode; the exit key will always take priority, making backspace non-functional",
				exitKeysFieldName,
				modeName,
			)
		}

		return nil
	}

	if !strings.Contains(backspaceKey, "+") {
		normalizedBS := NormalizeKeyForComparison(backspaceKey)
		if slices.Contains(normalizedExitKeys, normalizedBS) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s contains a key that conflicts with %s.backspace_key ('%s'); the exit key will always take priority, making backspace non-functional",
				exitKeysFieldName,
				modeName,
				backspaceKey,
			)
		}
	}

	return nil
}

// checkPerModeExitKeysResetKeyConflict checks if any per-mode exit key conflicts with
// the mode's configured reset key.
func checkPerModeExitKeysResetKeyConflict(
	modeExitKeys []string,
	resetKey string,
	exitKeysFieldName string,
	modeName string,
) error {
	if len(modeExitKeys) == 0 {
		return nil
	}

	if resetKey == "" {
		resetKey = " "
	}

	if strings.Contains(resetKey, "+") {
		return nil
	}

	var normalizedExitKeys []string
	for _, key := range modeExitKeys {
		normalizedExitKeys = append(
			normalizedExitKeys,
			NormalizeKeyForComparison(strings.TrimSpace(key)),
		)
	}

	normalizedReset := NormalizeKeyForComparison(resetKey)
	if slices.Contains(normalizedExitKeys, normalizedReset) {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s contains a key that conflicts with %s.reset_key ('%s'); the exit key will always take priority, making reset non-functional",
			exitKeysFieldName,
			modeName,
			resetKey,
		)
	}

	return nil
}

// checkScrollKeyBindingsActionKeyConflicts checks if any scroll key binding conflicts
// with an action key binding. At runtime, action keys are checked before scroll keys
// (in scroll.go handleGenericScrollKey), so a conflict means the scroll binding will
// never fire — the action will always take priority.
func (c *Config) checkScrollKeyBindingsActionKeyConflicts() error {
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
	for scrollAction, keys := range c.Scroll.KeyBindings {
		for _, scrollKey := range keys {
			normalizedScrollKey := NormalizeKeyForComparison(scrollKey)
			for _, binding := range bindings {
				if binding.value == "" {
					continue
				}

				if normalizedScrollKey == NormalizeKeyForComparison(binding.value) {
					return derrors.Newf(
						derrors.CodeInvalidConfig,
						"scroll.key_bindings['%s'] contains '%s' which conflicts with %s ('%s'); the action key is checked first at runtime, so the scroll binding will never fire",
						scrollAction,
						scrollKey,
						binding.fieldName,
						binding.value,
					)
				}

				// Check prefix conflicts: an action key that matches the first
				// character of a multi-letter scroll sequence (e.g. action "g"
				// vs scroll "gg") will consume the keystroke before the scroll
				// handler can start the sequence, silently breaking it.
				if len(scrollKey) >= 2 && IsAllLetters(scrollKey) {
					prefix := strings.ToLower(scrollKey[:1])
					if prefix == NormalizeKeyForComparison(binding.value) {
						return derrors.Newf(
							derrors.CodeInvalidConfig,
							"scroll.key_bindings['%s'] sequence '%s' starts with '%s' which conflicts with %s ('%s'); the action key is checked first at runtime, so the sequence can never start",
							scrollAction,
							scrollKey,
							prefix,
							binding.fieldName,
							binding.value,
						)
					}
				}

				// Check Shift+Letter fallback shadow: at runtime, both the
				// action service and scroll keymap treat a bare uppercase
				// letter (e.g. "G") as also matching "Shift+G". If a scroll
				// binding uses a single uppercase letter and an action binding
				// is "Shift+<that letter>", the action service will match
				// first via the fallback, silently shadowing the scroll
				// binding.
				if len(scrollKey) == 1 {
					r := rune(scrollKey[0])
					if r >= 'A' && r <= 'Z' {
						shiftForm := NormalizeKeyForComparison("Shift+" + scrollKey)
						if shiftForm == NormalizeKeyForComparison(binding.value) {
							return derrors.Newf(
								derrors.CodeInvalidConfig,
								"scroll.key_bindings['%s'] contains '%s' which conflicts with %s ('%s') via Shift+Letter fallback; the action key is checked first at runtime, so the scroll binding will never fire",
								scrollAction,
								scrollKey,
								binding.fieldName,
								binding.value,
							)
						}
					}
				}

				// Reverse Shift+Letter shadow: if the action binding is a
				// bare uppercase letter (e.g. "G") and a scroll binding is
				// "Shift+G", the action service's direct match will consume
				// bare "G" events before the scroll keymap sees them. The
				// scroll binding only works when the event tap sends the full
				// "Shift+G" modifier form, leading to inconsistent behavior.
				if len(binding.value) == 1 {
					r := rune(binding.value[0])
					if r >= 'A' && r <= 'Z' {
						actionShiftForm := NormalizeKeyForComparison(
							"Shift+" + binding.value,
						)
						if normalizedScrollKey == actionShiftForm {
							return derrors.Newf(
								derrors.CodeInvalidConfig,
								"scroll.key_bindings['%s'] contains '%s' which conflicts with %s ('%s') via reverse Shift+Letter fallback; the action key consumes bare '%s' events at runtime, so the scroll binding will only work inconsistently",
								scrollAction,
								scrollKey,
								binding.fieldName,
								binding.value,
								binding.value,
							)
						}
					}
				}
			}
		}
	}

	return nil
}

// checkPerModeExitKeysActionKeyConflicts checks if any per-mode exit key conflicts with
// an action key binding. At runtime, exit keys are checked before action keys
// (in key_dispatch.go), so a conflict means the action key will never fire — the mode
// will exit instead.
func (c *Config) checkPerModeExitKeysActionKeyConflicts() error {
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

	type modeExitKeys struct {
		keys      []string
		modeName  string
		isEnabled bool
	}

	// Scroll mode is always available (no "enabled" toggle), so use true.
	modes := []modeExitKeys{
		{c.Hints.ModeExitKeys, "hints", c.Hints.Enabled},
		{c.Grid.ModeExitKeys, "grid", c.Grid.Enabled},
		{c.RecursiveGrid.ModeExitKeys, "recursive_grid", c.RecursiveGrid.Enabled},
		{c.Scroll.ModeExitKeys, "scroll", true},
	}
	for _, mode := range modes {
		if !mode.isEnabled || len(mode.keys) == 0 {
			continue
		}

		var normalizedExitKeys []string
		for _, key := range mode.keys {
			normalizedExitKeys = append(
				normalizedExitKeys,
				NormalizeKeyForComparison(strings.TrimSpace(key)),
			)
		}

		for _, binding := range bindings {
			if binding.value == "" {
				continue
			}

			normalizedBinding := NormalizeKeyForComparison(binding.value)
			if slices.Contains(normalizedExitKeys, normalizedBinding) {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s.mode_exit_keys contains a key that conflicts with %s ('%s'); the exit key is checked first at runtime, so the action will never fire",
					mode.modeName,
					binding.fieldName,
					binding.value,
				)
			}
		}
	}

	return nil
}

// checkPerModeExitKeysScrollBindingConflicts checks if any per-mode exit key for scroll mode
// conflicts with the configured scroll key bindings.
func checkPerModeExitKeysScrollBindingConflicts(
	modeExitKeys []string,
	keyBindings map[string][]string,
	exitKeysFieldName string,
) error {
	if len(modeExitKeys) == 0 || len(keyBindings) == 0 {
		return nil
	}

	var normalizedExitKeys []string
	for _, key := range modeExitKeys {
		normalizedExitKeys = append(
			normalizedExitKeys,
			NormalizeKeyForComparison(strings.TrimSpace(key)),
		)
	}

	for action, keys := range keyBindings {
		for _, scrollKey := range keys {
			normalizedScrollKey := NormalizeKeyForComparison(scrollKey)
			if slices.Contains(normalizedExitKeys, normalizedScrollKey) {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s contains a key that conflicts with scroll.key_bindings['%s'] ('%s'); the exit key will always take priority instead of scrolling",
					exitKeysFieldName,
					action,
					scrollKey,
				)
			}

			// Check prefix conflicts: a single-character exit key that matches the
			// first character of a multi-letter sequence (e.g. exit key "g" vs binding "gg")
			// will intercept the key at dispatch time before the scroll handler can
			// start the sequence, silently breaking the binding.
			// Currently validateScrollKey only allows 2-letter sequences, but we use
			// >= 2 defensively so this stays correct if longer sequences are added.
			if len(scrollKey) >= 2 && IsAllLetters(scrollKey) {
				prefix := strings.ToLower(scrollKey[:1])
				if slices.Contains(normalizedExitKeys, prefix) {
					return derrors.Newf(
						derrors.CodeInvalidConfig,
						"%s contains '%s' which is the first key of scroll.key_bindings['%s'] sequence '%s'; the exit key will intercept before the sequence can complete",
						exitKeysFieldName,
						prefix,
						action,
						scrollKey,
					)
				}
			}
		}
	}

	return nil
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

	// Validate max depth (must run before layer validation so the depth-reachability
	// check inside validateRecursiveGridLayers can rely on MaxDepth being in range)
	if c.RecursiveGrid.MaxDepth < 1 || c.RecursiveGrid.MaxDepth > 20 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.max_depth must be between 1 and 20",
		)
	}

	// Validate per-depth layers
	err := c.validateRecursiveGridLayers()
	if err != nil {
		return err
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
	err = validateBackspaceKeyFormat(
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
	if c.RecursiveGrid.UI.LineWidth < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.ui.line_width must be non-negative",
		)
	}

	if c.RecursiveGrid.UI.FontSize < 6 || c.RecursiveGrid.UI.FontSize > 72 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.ui.font_size must be between 6 and 72",
		)
	}

	if c.RecursiveGrid.UI.SubKeyPreviewFontSize < 4 ||
		c.RecursiveGrid.UI.SubKeyPreviewFontSize > 72 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.ui.sub_key_preview_font_size must be between 4 and 72",
		)
	}

	err = validateMinValue(
		c.RecursiveGrid.UI.LabelBackgroundBorderRadius,
		-1,
		"recursive_grid.ui.label_background_border_radius",
	)
	if err != nil {
		return err
	}

	err = validateMinValue(
		c.RecursiveGrid.UI.LabelBackgroundPaddingX,
		-1,
		"recursive_grid.ui.label_background_padding_x",
	)
	if err != nil {
		return err
	}

	if c.RecursiveGrid.UI.SubKeyPreviewAutohideMultiplier < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.ui.sub_key_preview_autohide_multiplier must be >= 0",
		)
	}

	err = validateMinValue(
		c.RecursiveGrid.UI.LabelBackgroundPaddingY,
		-1,
		"recursive_grid.ui.label_background_padding_y",
	)
	if err != nil {
		return err
	}

	if c.RecursiveGrid.UI.LabelBackgroundBorderWidth < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.ui.label_background_border_width must be non-negative",
		)
	}

	// Validate colors
	colorFields := []colorField{
		{c.RecursiveGrid.UI.LineColorLight, "recursive_grid.ui.line_color_light"},
		{c.RecursiveGrid.UI.LineColorDark, "recursive_grid.ui.line_color_dark"},
		{c.RecursiveGrid.UI.HighlightColorLight, "recursive_grid.ui.highlight_color_light"},
		{c.RecursiveGrid.UI.HighlightColorDark, "recursive_grid.ui.highlight_color_dark"},
		{c.RecursiveGrid.UI.TextColorLight, "recursive_grid.ui.text_color_light"},
		{c.RecursiveGrid.UI.TextColorDark, "recursive_grid.ui.text_color_dark"},
		{
			c.RecursiveGrid.UI.LabelBackgroundColorLight,
			"recursive_grid.ui.label_background_color_light",
		},
		{
			c.RecursiveGrid.UI.LabelBackgroundColorDark,
			"recursive_grid.ui.label_background_color_dark",
		},
		{
			c.RecursiveGrid.UI.SubKeyPreviewTextColorLight,
			"recursive_grid.ui.sub_key_preview_text_color_light",
		},
		{
			c.RecursiveGrid.UI.SubKeyPreviewTextColorDark,
			"recursive_grid.ui.sub_key_preview_text_color_dark",
		},
	}

	err = validateColors(colorFields)
	if err != nil {
		return err
	}

	// Validate per-mode exit keys
	if len(c.RecursiveGrid.ModeExitKeys) > 0 {
		err = validatePerModeExitKeysFormat(
			c.RecursiveGrid.ModeExitKeys,
			"recursive_grid.mode_exit_keys",
		)
		if err != nil {
			return err
		}

		err = checkPerModeExitKeyCharConflict(
			c.RecursiveGrid.ModeExitKeys,
			c.RecursiveGrid.AllKeysIncludingLayers(),
			"recursive_grid.mode_exit_keys",
			"recursive_grid.keys (including layers)",
			"cell selection",
		)
		if err != nil {
			return err
		}

		err = checkPerModeExitKeysBackspaceConflict(
			c.RecursiveGrid.ModeExitKeys,
			c.RecursiveGrid.BackspaceKey,
			"recursive_grid.mode_exit_keys",
			"recursive_grid",
		)
		if err != nil {
			return err
		}

		err = checkPerModeExitKeysResetKeyConflict(
			c.RecursiveGrid.ModeExitKeys,
			c.RecursiveGrid.ResetKey,
			"recursive_grid.mode_exit_keys",
			"recursive_grid",
		)
		if err != nil {
			return err
		}
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

// validateRecursiveGridLayers validates the per-depth layer overrides.
func (c *Config) validateRecursiveGridLayers() error {
	if len(c.RecursiveGrid.Layers) == 0 {
		return nil
	}

	seenDepths := make(map[int]bool)
	for index, layer := range c.RecursiveGrid.Layers {
		fieldPrefix := fmt.Sprintf("recursive_grid.layers[%d]", index)
		// Validate depth >= 0
		if layer.Depth < 0 {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s.depth must be non-negative",
				fieldPrefix,
			)
		}

		// Validate depth < max_depth (layers at or beyond max_depth are unreachable)
		if layer.Depth >= c.RecursiveGrid.MaxDepth {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s.depth %d is unreachable; it must be less than max_depth (%d)",
				fieldPrefix,
				layer.Depth,
				c.RecursiveGrid.MaxDepth,
			)
		}

		// Check for duplicate depths
		if seenDepths[layer.Depth] {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s.depth %d is duplicated; each depth can only appear once",
				fieldPrefix,
				layer.Depth,
			)
		}

		seenDepths[layer.Depth] = true
		// Validate grid_cols (must be at least 2)
		if layer.GridCols < DefaultRecursiveGridMinGridCols {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s.grid_cols must be at least 2",
				fieldPrefix,
			)
		}

		// Validate grid_rows (must be at least 2)
		if layer.GridRows < DefaultRecursiveGridMinGridRows {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s.grid_rows must be at least 2",
				fieldPrefix,
			)
		}

		// Validate keys
		layerKeys := strings.TrimSpace(layer.Keys)
		if layerKeys == "" {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s.keys cannot be empty",
				fieldPrefix,
			)
		}

		expectedKeyCount := layer.GridCols * layer.GridRows
		if utf8.RuneCountInString(layerKeys) != expectedKeyCount {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s.keys must be exactly %d characters for grid_cols %d and grid_rows %d",
				fieldPrefix,
				expectedKeyCount,
				layer.GridCols,
				layer.GridRows,
			)
		}

		// Check for duplicate keys within this layer
		keyMap := make(map[rune]bool)
		for _, key := range layerKeys {
			if keyMap[key] {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s.keys contains duplicate character: %c",
					fieldPrefix,
					key,
				)
			}

			keyMap[key] = true
		}

		// Validate ASCII
		for _, r := range layerKeys {
			if r > unicode.MaxASCII {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s.keys can only contain ASCII characters",
					fieldPrefix,
				)
			}
		}

		// Validate layer keys don't conflict with reset key
		resetKey := c.RecursiveGrid.ResetKey
		if resetKey == "" {
			resetKey = " "
		}

		if len(resetKey) == 1 && !strings.Contains(resetKey, "+") {
			if strings.ContainsRune(
				strings.ToLower(layerKeys),
				rune(strings.ToLower(resetKey)[0]),
			) {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s.keys cannot contain '%s' as it is reserved for reset",
					fieldPrefix,
					resetKey,
				)
			}
		}

		// Validate layer keys don't conflict with backspace key
		err := checkBackspaceKeyCharConflict(
			c.RecursiveGrid.BackspaceKey,
			layerKeys,
			"recursive_grid.backspace_key",
			fieldPrefix+".keys",
			"cell selection",
		)
		if err != nil {
			return err
		}
	}

	return nil
}
