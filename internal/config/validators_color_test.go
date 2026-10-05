package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

const testColorFieldName = "test_color"

const testThemeFieldName = "theme.light.surface"

// TestColor_Validate pins the hex grammar a theme-aware color accepts, on both
// of its sides.
func TestColor_Validate(t *testing.T) {
	tests := []struct {
		name      string
		color     string
		fieldName string
		wantErr   bool
	}{
		{
			name:      "valid 6-digit hex",
			color:     "#FF0000",
			fieldName: testColorFieldName,
			wantErr:   false,
		},
		{
			name:      "valid 3-digit hex",
			color:     "#F00",
			fieldName: testColorFieldName,
			wantErr:   false,
		},
		{
			name:      "valid 8-digit hex with alpha",
			color:     "#FF0000AA",
			fieldName: testColorFieldName,
			wantErr:   false,
		},
		{
			name:      "lowercase hex",
			color:     "#ff0000",
			fieldName: testColorFieldName,
			wantErr:   false,
		},
		{
			name:      "empty color (theme-aware default)",
			color:     "",
			fieldName: testColorFieldName,
			wantErr:   false,
		},
		{
			name:      "missing hash",
			color:     "FF0000",
			fieldName: testColorFieldName,
			wantErr:   true,
		},
		{
			name:      "invalid hex length",
			color:     "#FF00",
			fieldName: testColorFieldName,
			wantErr:   true,
		},
		{
			name:      "invalid characters",
			color:     "#GG0000",
			fieldName: testColorFieldName,
			wantErr:   true,
		},
		{
			name:      "too long hex",
			color:     "#FF0000FFAA",
			fieldName: testColorFieldName,
			wantErr:   true,
		},
		{
			name:      "just hash",
			color:     "#",
			fieldName: testColorFieldName,
			wantErr:   true,
		},
		{
			name:      "hash with spaces",
			color:     "#FF 00 00",
			fieldName: testColorFieldName,
			wantErr:   true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			for side, color := range map[string]config.Color{
				"light": {Light: testCase.color},
				"dark":  {Dark: testCase.color},
			} {
				err := color.Validate(testCase.fieldName)
				if (err != nil) != testCase.wantErr {
					t.Errorf("Validate() on %s error = %v, wantErr %v", side, err, testCase.wantErr)
				}
			}
		})
	}
}

func TestValidateSolidColor(t *testing.T) {
	tests := []struct {
		name      string
		color     string
		fieldName string
		wantErr   bool
	}{
		{
			name:      "valid 6-digit hex",
			color:     "#FF0000",
			fieldName: testThemeFieldName,
			wantErr:   false,
		},
		{
			name:      "valid 3-digit hex",
			color:     "#F00",
			fieldName: testThemeFieldName,
			wantErr:   false,
		},
		{
			name:      "valid lowercase hex",
			color:     "#ff0000",
			fieldName: testThemeFieldName,
			wantErr:   false,
		},
		{
			name:      "empty color allowed",
			color:     "",
			fieldName: testThemeFieldName,
			wantErr:   false,
		},
		{
			name:      "8-digit hex with alpha rejected",
			color:     "#FF0000AA",
			fieldName: testThemeFieldName,
			wantErr:   true,
		},
		{
			name:      "missing hash",
			color:     "FF0000",
			fieldName: testThemeFieldName,
			wantErr:   true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := config.ValidateSolidColor(testCase.color, testCase.fieldName)
			if (err != nil) != testCase.wantErr {
				t.Errorf("ValidateSolidColor() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}
