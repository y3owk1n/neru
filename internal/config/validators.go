package config

import (
	"regexp"
	"strings"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// ValidateHints validates the hints configuration.
func (c *Config) ValidateHints() error {
	var validateErr error

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

	if c.Hints.Opacity < 0 || c.Hints.Opacity > 1 {
		return derrors.New(derrors.CodeInvalidConfig, "hints.opacity must be between 0 and 1")
	}

	validateErr = ValidateColor(c.Hints.BackgroundColor, "hints.background_color")
	if validateErr != nil {
		return validateErr
	}

	validateErr = ValidateColor(c.Hints.TextColor, "hints.text_color")
	if validateErr != nil {
		return validateErr
	}

	validateErr = ValidateColor(c.Hints.MatchedTextColor, "hints.matched_text_color")
	if validateErr != nil {
		return validateErr
	}

	validateErr = ValidateColor(c.Hints.BorderColor, "hints.border_color")
	if validateErr != nil {
		return validateErr
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
	var validateErr error

	if strings.TrimSpace(c.Grid.Characters) == "" {
		return derrors.New(derrors.CodeInvalidConfig, "grid.characters cannot be empty")
	}

	if len(c.Grid.Characters) < MinCharactersLength {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"grid.characters must contain at least 2 characters",
		)
	}

	if strings.Contains(c.Grid.Characters, "<") {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"grid.characters cannot contain '<' as it is reserved for reset",
		)
	}

	if c.Grid.FontSize < 6 || c.Grid.FontSize > 72 {
		return derrors.New(derrors.CodeInvalidConfig, "grid.font_size must be between 6 and 72")
	}

	if c.Grid.BorderWidth < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "grid.border_width must be non-negative")
	}

	if c.Grid.Opacity < 0 || c.Grid.Opacity > 1 {
		return derrors.New(derrors.CodeInvalidConfig, "grid.opacity must be between 0 and 1")
	}

	// Validate per-action grid colors
	validateErr = ValidateColor(c.Grid.BackgroundColor, "grid.background_color")
	if validateErr != nil {
		return validateErr
	}

	validateErr = ValidateColor(c.Grid.TextColor, "grid.text_color")
	if validateErr != nil {
		return validateErr
	}

	validateErr = ValidateColor(c.Grid.MatchedTextColor, "grid.matched_text_color")
	if validateErr != nil {
		return validateErr
	}

	validateErr = ValidateColor(c.Grid.MatchedBackgroundColor, "grid.matched_background_color")
	if validateErr != nil {
		return validateErr
	}

	validateErr = ValidateColor(c.Grid.MatchedBorderColor, "grid.matched_border_color")
	if validateErr != nil {
		return validateErr
	}

	validateErr = ValidateColor(c.Grid.BorderColor, "grid.border_color")
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
	var validateErr error

	if c.Action.HighlightWidth < 1 {
		return derrors.New(derrors.CodeInvalidConfig, "action.highlight_width must be at least 1")
	}

	validateErr = ValidateColor(c.Action.HighlightColor, "action.highlight_color")
	if validateErr != nil {
		return validateErr
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

	// Match hex color format: #RGB, #RRGGBB, #RRGGBBAA
	hexColorRegex := regexp.MustCompile(`^#([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6}|[0-9A-Fa-f]{8})$`)

	if !hexColorRegex.MatchString(color) {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s has invalid hex color format: %s (expected #RGB, #RRGGBB, or #RRGGBBAA)",
			fieldName,
			color,
		)
	}

	return nil
}
