package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// validateHints validates the hints configuration.
func (c *Config) validateHints() error {
	var validateErr error

	if strings.TrimSpace(c.Hints.HintCharacters) == "" {
		return errors.New("hint_characters cannot be empty")
	}

	if len(c.Hints.HintCharacters) < 2 {
		return errors.New("hint_characters must contain at least 2 characters")
	}

	if c.Hints.Opacity < 0 || c.Hints.Opacity > 1 {
		return errors.New("hints.opacity must be between 0 and 1")
	}

	validateErr = validateColor(c.Hints.BackgroundColor, "hints.background_color")
	if validateErr != nil {
		return validateErr
	}

	validateErr = validateColor(c.Hints.TextColor, "hints.text_color")
	if validateErr != nil {
		return validateErr
	}

	validateErr = validateColor(c.Hints.MatchedTextColor, "hints.matched_text_color")
	if validateErr != nil {
		return validateErr
	}

	validateErr = validateColor(c.Hints.BorderColor, "hints.border_color")
	if validateErr != nil {
		return validateErr
	}

	if c.Hints.FontSize < 6 || c.Hints.FontSize > 72 {
		return errors.New("hints.font_size must be between 6 and 72")
	}

	if c.Hints.BorderRadius < 0 {
		return errors.New("hints.border_radius must be non-negative")
	}

	if c.Hints.Padding < 0 {
		return errors.New("hints.padding must be non-negative")
	}

	if c.Hints.BorderWidth < 0 {
		return errors.New("hints.border_width must be non-negative")
	}

	for _, role := range c.Hints.ClickableRoles {
		if strings.TrimSpace(role) == "" {
			return errors.New("hints.clickable_roles cannot contain empty values")
		}
	}

	for _, bundle := range c.Hints.AdditionalAXSupport.AdditionalElectronBundles {
		if strings.TrimSpace(bundle) == "" {
			return errors.New(
				"hints.electron_support.additional_electron_bundles cannot contain empty values",
			)
		}
	}

	for _, bundle := range c.Hints.AdditionalAXSupport.AdditionalChromiumBundles {
		if strings.TrimSpace(bundle) == "" {
			return errors.New(
				"hints.electron_support.additional_chromium_bundles cannot contain empty values",
			)
		}
	}

	for _, bundle := range c.Hints.AdditionalAXSupport.AdditionalFirefoxBundles {
		if strings.TrimSpace(bundle) == "" {
			return errors.New(
				"hints.electron_support.additional_firefox_bundles cannot contain empty values",
			)
		}
	}

	return nil
}

// validateAppConfigs validates the app configurations.
func (c *Config) validateAppConfigs() error {
	var validateErr error

	for index, appConfig := range c.Hints.AppConfigs {
		if strings.TrimSpace(appConfig.BundleID) == "" {
			return fmt.Errorf("hints.app_configs[%d].bundle_id cannot be empty", index)
		}

		// Validate hotkey bindings
		for key, value := range c.Hotkeys.Bindings {
			if strings.TrimSpace(key) == "" {
				return errors.New("hotkeys.bindings contains an empty key")
			}

			validateErr = validateHotkey(key, "hotkeys.bindings")
			if validateErr != nil {
				return validateErr
			}

			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("hotkeys.bindings[%s] cannot be empty", key)
			}
		}

		for _, role := range appConfig.AdditionalClickable {
			if strings.TrimSpace(role) == "" {
				return fmt.Errorf(
					"hints.app_configs[%d].additional_clickable_roles cannot contain empty values",
					index,
				)
			}
		}
	}

	return nil
}

// validateGrid validates the grid configuration.
func (c *Config) validateGrid() error {
	var validateErr error

	if strings.TrimSpace(c.Grid.Characters) == "" {
		return errors.New("grid.characters cannot be empty")
	}

	if len(c.Grid.Characters) < 2 {
		return errors.New("grid.characters must contain at least 2 characters")
	}

	if c.Grid.FontSize < 6 || c.Grid.FontSize > 72 {
		return errors.New("grid.font_size must be between 6 and 72")
	}

	if c.Grid.BorderWidth < 0 {
		return errors.New("grid.border_width must be non-negative")
	}

	if c.Grid.Opacity < 0 || c.Grid.Opacity > 1 {
		return errors.New("grid.opacity must be between 0 and 1")
	}

	// Validate per-action grid colors
	validateErr = validateColor(c.Grid.BackgroundColor, "grid.background_color")
	if validateErr != nil {
		return validateErr
	}

	validateErr = validateColor(c.Grid.TextColor, "grid.text_color")
	if validateErr != nil {
		return validateErr
	}

	validateErr = validateColor(c.Grid.MatchedTextColor, "grid.matched_text_color")
	if validateErr != nil {
		return validateErr
	}

	validateErr = validateColor(c.Grid.MatchedBackgroundColor, "grid.matched_background_color")
	if validateErr != nil {
		return validateErr
	}

	validateErr = validateColor(c.Grid.MatchedBorderColor, "grid.matched_border_color")
	if validateErr != nil {
		return validateErr
	}

	validateErr = validateColor(c.Grid.BorderColor, "grid.border_color")
	if validateErr != nil {
		return validateErr
	}

	// Validate sublayer keys length (fallback to grid.characters) for 3x3 subgrid
	keys := strings.TrimSpace(c.Grid.SublayerKeys)
	if keys == "" {
		keys = c.Grid.Characters
	}
	// Subgrid is always 3x3, requiring at least 9 characters
	const required = 9
	if len([]rune(keys)) < required {
		return fmt.Errorf(
			"grid.sublayer_keys must contain at least %d characters for 3x3 subgrid selection",
			required,
		)
	}

	return nil
}

// validateAction validates the action configuration.
func (c *Config) validateAction() error {
	var validateErr error

	if c.Action.HighlightWidth < 1 {
		return errors.New("action.highlight_width must be at least 1")
	}

	validateErr = validateColor(c.Action.HighlightColor, "action.highlight_color")
	if validateErr != nil {
		return validateErr
	}

	return nil
}

// validateSmoothCursor validates the smooth cursor configuration.
func (c *Config) validateSmoothCursor() error {
	if c.SmoothCursor.Steps < 1 {
		return errors.New("smooth_cursor.steps must be at least 1")
	}

	if c.SmoothCursor.Delay < 0 {
		return errors.New("smooth_cursor.delay must be non-negative")
	}

	return nil
}

// validateHotkey validates a hotkey string format.
func validateHotkey(hotkey, fieldName string) error {
	if strings.TrimSpace(hotkey) == "" {
		return nil // Allow empty hotkey to disable the action
	}

	// Hotkey format: [Modifier+]*Key
	// Valid modifiers: Cmd, Ctrl, Alt, Shift, Option
	// Examples: "Cmd+Shift+Space", "Ctrl+D", "F1"

	parts := strings.Split(hotkey, "+")
	if len(parts) == 0 {
		return fmt.Errorf("%s has invalid format: %s", fieldName, hotkey)
	}

	validModifiers := map[string]bool{
		"Cmd":    true,
		"Ctrl":   true,
		"Alt":    true,
		"Shift":  true,
		"Option": true,
	}

	// Check all parts except the last (which is the key)
	for index := range parts[:len(parts)-1] {
		modifier := strings.TrimSpace(parts[index])
		if !validModifiers[modifier] {
			return fmt.Errorf(
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
		return fmt.Errorf("%s has empty key in: %s", fieldName, hotkey)
	}

	return nil
}

// validateColor validates a color string (hex format).
func validateColor(color, fieldName string) error {
	if strings.TrimSpace(color) == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}

	// Match hex color format: #RGB, #RRGGBB, #RRGGBBAA
	hexColorRegex := regexp.MustCompile(`^#([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6}|[0-9A-Fa-f]{8})$`)

	if !hexColorRegex.MatchString(color) {
		return fmt.Errorf(
			"%s has invalid hex color format: %s (expected #RGB, #RRGGBB, or #RRGGBBAA)",
			fieldName,
			color,
		)
	}

	return nil
}
