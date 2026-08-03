package config

import (
	"math"

	"github.com/y3owk1n/neru/internal/derrors"
)

// maxFontSize is the maximum font size accepted by the config validator.
// Values above this can overflow C.int (int32) on Darwin or platform int on
// Windows when passed to native overlay renderers.
const maxFontSize = math.MaxInt32

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

func validateUnitFloat(name string, value float64) error {
	if value < 0 || value > 1 {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s must be between 0 and 1",
			name,
		)
	}

	return nil
}

func validatePositiveUnitFloat(name string, value float64) error {
	if value <= 0 || value > 1 {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s must be between 0 (exclusive) and 1 (inclusive)",
			name,
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

func validOpacity(value float64) bool {
	return value >= 0 && value <= 1
}
