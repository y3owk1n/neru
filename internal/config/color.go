package config

import (
	"fmt"
	"regexp"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

var colorRegex = regexp.MustCompile(`^#([A-Fa-f0-9]{3}|[A-Fa-f0-9]{6}|[A-Fa-f0-9]{8})$`)

// Color represents a color that can be specified as either a single value
// (same for both light/dark themes) or as separate light and dark values.
type Color struct {
	Light string `json:"light" toml:"light"`
	Dark  string `json:"dark"  toml:"dark"`
}

// errUnsupportedColorType is returned when an unsupported type is provided
// during color unmarshaling.
var errUnsupportedColorType = derrors.New(derrors.CodeInvalidConfig, "unsupported color type")

// UnmarshalTOML implements custom unmarshaling for TOML.
// It accepts both a plain string (same color for both themes)
// and an inline table with "light" and "dark" keys.
func (c *Color) UnmarshalTOML(data any) error {
	switch val := data.(type) {
	case string:
		c.Light = val
		c.Dark = val
	case map[string]any:
		if light, ok := val["light"]; ok {
			lightStr, isStr := light.(string)
			if !isStr {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"color 'light' must be a string, got %T",
					light,
				)
			}

			c.Light = lightStr
		}

		if dark, ok := val["dark"]; ok {
			darkStr, isStr := dark.(string)
			if !isStr {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"color 'dark' must be a string, got %T",
					dark,
				)
			}

			c.Dark = darkStr
		}
	default:
		return errUnsupportedColorType
	}

	return nil
}

// MarshalTOML implements custom marshaling for TOML.
// If light and dark are the same, it outputs a simple string.
// Otherwise, it outputs an inline table.
func (c *Color) MarshalTOML() ([]byte, error) {
	if c.Light == c.Dark {
		return fmt.Appendf(nil, `"%s"`, c.Light), nil
	}

	return fmt.Appendf(nil, `{ light = "%s", dark = "%s" }`, c.Light, c.Dark), nil
}

// IsEmpty returns true if both light and dark colors are empty.
func (c *Color) IsEmpty() bool {
	return c.Light == "" && c.Dark == ""
}

// IsSameForBothThemes returns true if light and dark are the same value.
func (c *Color) IsSameForBothThemes() bool {
	return c.Light == c.Dark
}

// ForTheme returns the appropriate color based on the current theme.
// If a color variant is not set, it falls back to the provided defaults.
func (c *Color) ForTheme(theme ThemeProvider, defaultLight, defaultDark string) string {
	light := c.Light
	dark := c.Dark

	if light == "" {
		light = defaultLight
	}

	if dark == "" {
		dark = defaultDark
	}

	if theme != nil && theme.IsDarkMode() {
		if dark != "" {
			return dark
		}

		return defaultDark
	}

	if light != "" {
		return light
	}

	return defaultLight
}

// ForThemeWithOverride resolves a color with a three-tier fallback:
// per-mode override (receiver) → shared UI default → hardcoded default.
func (c *Color) ForThemeWithOverride(
	uiDefault Color,
	theme ThemeProvider,
	defaultLight, defaultDark string,
) string {
	effLight := c.Light
	effDark := c.Dark

	if effLight == "" {
		effLight = uiDefault.Light
	}

	if effDark == "" {
		effDark = uiDefault.Dark
	}

	return (&Color{Light: effLight, Dark: effDark}).ForTheme(theme, defaultLight, defaultDark)
}

// Validate checks if the color values are valid hex colors.
func (c *Color) Validate(fieldName string) error {
	if c.Light != "" && !colorRegex.MatchString(c.Light) {
		msg := fmt.Sprintf("%s (light) has invalid color format: %s", fieldName, c.Light)

		return derrors.New(derrors.CodeInvalidConfig, msg)
	}

	if c.Dark != "" && !colorRegex.MatchString(c.Dark) {
		msg := fmt.Sprintf("%s (dark) has invalid color format: %s", fieldName, c.Dark)

		return derrors.New(derrors.CodeInvalidConfig, msg)
	}

	return nil
}
