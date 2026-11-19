package config

import (
	"testing"
)

const testKeyCmdShiftS = "Cmd+Shift+S"

// TestFindNormalizedMapKey_Bindings pins the case-insensitive match that keeps
// one key from being bound twice under two spellings, returning the spelling
// the table holds, and the raw key on a miss. The function is generic with no
// code per value type, so one instantiation covers the hotkey tables too.
func TestFindNormalizedMapKey_Bindings(t *testing.T) {
	bindings := map[string][]string{
		testKeyCmdShiftS:    {ModeNameScroll},
		"Cmd+Shift+Space":   {ModeNameHints},
		"Cmd+Shift+G":       {ModeNameGrid},
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
		{
			name:     "named key exact match",
			rawKey:   "Escape",
			expected: KeyDisplayEscape,
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
			name:     "single char miss returns rawKey",
			rawKey:   "x",
			expected: "x",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := FindNormalizedMapKey(bindings, testCase.rawKey, defaultPrimaryModifier())
			if got != testCase.expected {
				t.Errorf("FindNormalizedMapKey(%q) = %q, want %q",
					testCase.rawKey, got, testCase.expected)
			}
		})
	}
}
