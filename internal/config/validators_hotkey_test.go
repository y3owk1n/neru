package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// TestValidateHotkey tests the ValidateHotkey function.
func TestValidateHotkey(t *testing.T) {
	tests := []struct {
		name      string
		hotkey    string
		fieldName string
		wantErr   bool
	}{
		{
			name:      "valid single modifier",
			hotkey:    "Cmd+Space",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "valid multiple modifiers",
			hotkey:    "Cmd+Shift+Space",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "valid all modifiers",
			hotkey:    "Cmd+Ctrl+Alt+Shift+A",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "valid function key",
			hotkey:    "F1",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "valid Option modifier",
			hotkey:    "Option+D",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "empty hotkey allowed",
			hotkey:    "",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "invalid modifier",
			hotkey:    "Super+Space",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
		{
			name:      "empty key",
			hotkey:    "Cmd+",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
		{
			name:      "just modifiers",
			hotkey:    "Cmd+Shift",
			fieldName: "test_hotkey",
			wantErr:   false, // Shift is treated as valid key
		},
		{
			name:      "duplicate modifiers",
			hotkey:    "Cmd+Cmd+Space",
			fieldName: "test_hotkey",
			wantErr:   false, // Duplicates not checked
		},
		{
			name:      "any key allowed",
			hotkey:    "Cmd+InvalidKey",
			fieldName: "test_hotkey",
			wantErr:   false, // Any non-empty key is valid
		},
		{
			name:      "single key without modifiers",
			hotkey:    "Space",
			fieldName: "test_hotkey",
			wantErr:   false, // Single keys allowed
		},
		{
			name:      "just plus",
			hotkey:    "+",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
		{
			name:      "multiple pluses",
			hotkey:    "Cmd++Space",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
		{
			name:      "leading plus",
			hotkey:    "+Space",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
		{
			name:      "trailing plus",
			hotkey:    "Space+",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
		{
			name:      "spaces in hotkey trimmed",
			hotkey:    "Cmd + Space",
			fieldName: "test_hotkey",
			wantErr:   false, // TrimSpace handles spaces
		},
		{
			name:      "lowercase modifiers",
			hotkey:    "cmd+space",
			fieldName: "test_hotkey",
			wantErr:   true, // Must be exact case
		},
		{
			name:      "mixed case modifiers",
			hotkey:    "CMD+shift+Space",
			fieldName: "test_hotkey",
			wantErr:   true, // Must be exact case
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.ValidateHotkey(tt.hotkey, tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHotkey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
