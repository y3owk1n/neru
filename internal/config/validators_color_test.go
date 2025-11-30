package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// TestValidateColor tests the validateColor function.
func TestValidateColor(t *testing.T) {
	tests := []struct {
		name      string
		color     string
		fieldName string
		wantErr   bool
	}{
		{
			name:      "valid 6-digit hex",
			color:     "#FF0000",
			fieldName: "test_color",
			wantErr:   false,
		},
		{
			name:      "valid 3-digit hex",
			color:     "#F00",
			fieldName: "test_color",
			wantErr:   false,
		},
		{
			name:      "valid 8-digit hex with alpha",
			color:     "#FF0000AA",
			fieldName: "test_color",
			wantErr:   false,
		},
		{
			name:      "lowercase hex",
			color:     "#ff0000",
			fieldName: "test_color",
			wantErr:   false,
		},
		{
			name:      "empty color",
			color:     "",
			fieldName: "test_color",
			wantErr:   true,
		},
		{
			name:      "missing hash",
			color:     "FF0000",
			fieldName: "test_color",
			wantErr:   true,
		},
		{
			name:      "invalid hex length",
			color:     "#FF00",
			fieldName: "test_color",
			wantErr:   true,
		},
		{
			name:      "invalid characters",
			color:     "#GG0000",
			fieldName: "test_color",
			wantErr:   true,
		},
		{
			name:      "too long hex",
			color:     "#FF0000FFAA",
			fieldName: "test_color",
			wantErr:   true,
		},
		{
			name:      "just hash",
			color:     "#",
			fieldName: "test_color",
			wantErr:   true,
		},
		{
			name:      "hash with spaces",
			color:     "#FF 00 00",
			fieldName: "test_color",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.ValidateColor(tt.color, tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateColor() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
