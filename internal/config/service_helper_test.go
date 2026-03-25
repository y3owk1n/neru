//nolint:testpackage // Tests private service helper methods.
package config

import (
	"testing"
)

func TestFindNormalizedBindingsKey(t *testing.T) {
	bindings := map[string][]string{
		"Cmd+Shift+S":     {"scroll"},
		"Cmd+Shift+Space": {"hints"},
		"Cmd+Shift+G":     {"grid"},
	}

	tests := []struct {
		name     string
		rawKey   string
		expected string
	}{
		{
			name:     "exact match",
			rawKey:   "Cmd+Shift+S",
			expected: "Cmd+Shift+S",
		},
		{
			name:     "lowercase matches canonical",
			rawKey:   "cmd+shift+s",
			expected: "Cmd+Shift+S",
		},
		{
			name:     "mixed case matches canonical",
			rawKey:   "CMD+SHIFT+S",
			expected: "Cmd+Shift+S",
		},
		{
			name:     "no match returns rawKey",
			rawKey:   "Ctrl+X",
			expected: "Ctrl+X",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := findNormalizedBindingsKey(bindings, testCase.rawKey)
			if got != testCase.expected {
				t.Errorf("findNormalizedBindingsKey(%q) = %q, want %q",
					testCase.rawKey, got, testCase.expected)
			}
		})
	}
}

func TestFindNormalizedSOSAKey(t *testing.T) {
	_map := map[string]StringOrStringArray{
		"Escape":    {"idle"},
		"Shift+L":   {"action left_click"},
		"Up":        {"action move_mouse_relative --dx=0 --dy=-10"},
		"Backspace": {"action backspace"},
		"j":         {"action scroll_down"},
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
			expected: "Escape",
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
			got := findNormalizedSOSAKey(_map, testCase.rawKey)
			if got != testCase.expected {
				t.Errorf("findNormalizedSOSAKey(%q) = %q, want %q",
					testCase.rawKey, got, testCase.expected)
			}
		})
	}
}
