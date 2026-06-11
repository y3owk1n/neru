//nolint:testpackage // Tests private service helper methods.
package config

import (
	"testing"
)

const testKeyCmdShiftS = "Cmd+Shift+S"

func TestFindNormalizedMapKey_Bindings(t *testing.T) {
	bindings := map[string][]string{
		testKeyCmdShiftS:  {ModeNameScroll},
		"Cmd+Shift+Space": {ModeNameHints},
		"Cmd+Shift+G":     {ModeNameGrid},
	}

	tests := []struct {
		name     string
		rawKey   string
		expected string
	}{
		{
			name:     "exact match",
			rawKey:   testKeyCmdShiftS,
			expected: testKeyCmdShiftS,
		},
		{
			name:     "lowercase matches canonical",
			rawKey:   "cmd+shift+s",
			expected: testKeyCmdShiftS,
		},
		{
			name:     "mixed case matches canonical",
			rawKey:   "CMD+SHIFT+S",
			expected: testKeyCmdShiftS,
		},
		{
			name:     "no match returns rawKey",
			rawKey:   "Ctrl+X",
			expected: "Ctrl+X",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := findNormalizedMapKey(bindings, testCase.rawKey)
			if got != testCase.expected {
				t.Errorf("findNormalizedMapKey(%q) = %q, want %q",
					testCase.rawKey, got, testCase.expected)
			}
		})
	}
}

func TestFindNormalizedMapKey_SOSA(t *testing.T) {
	_map := map[string]StringOrStringArray{
		KeyDisplayEscape:    {CmdIdle},
		KeyComboShiftL:      {CmdLeftClick},
		"Up":                {CmdMoveMouseUp},
		KeyDisplayBackspace: {CmdBackspace},
		"j":                 {"action scroll_down"},
	}

	tests := []struct {
		name     string
		rawKey   string
		expected string
	}{
		{
			name:     "exact match",
			rawKey:   "Escape",
			expected: "Escape",
		},
		{
			name:     "lowercase escape matches Escape",
			rawKey:   "escape",
			expected: KeyDisplayEscape,
		},
		{
			name:     "lowercase up matches Up",
			rawKey:   "up",
			expected: "Up",
		},
		{
			name:     "lowercase shift+l matches Shift+L",
			rawKey:   "shift+l",
			expected: "Shift+L",
		},
		{
			name:     "single char exact match",
			rawKey:   "j",
			expected: "j",
		},
		{
			name:     "no match returns rawKey",
			rawKey:   "x",
			expected: "x",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := findNormalizedMapKey(_map, testCase.rawKey)
			if got != testCase.expected {
				t.Errorf("findNormalizedMapKey(%q) = %q, want %q",
					testCase.rawKey, got, testCase.expected)
			}
		})
	}
}
