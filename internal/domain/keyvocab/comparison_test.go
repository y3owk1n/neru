package keyvocab_test

import (
	"runtime"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

const (
	wantShiftReturn = "shift+return"
	wantCtrlShiftC  = "ctrl+shift+c"
	wantCmdSpace    = "Cmd+Space"
	superSpace      = "Super+Space"
)

var isDarwin = runtime.GOOS == "darwin"

func TestNormalizeKeyForComparison_FullwidthChars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Fullwidth comma, the most common case (the reset key)
		{
			name:     "fullwidth comma",
			input:    "\uFF0C",
			expected: ",",
		},
		{
			name:     "fullwidth comma uppercase",
			input:    "\uFF0C",
			expected: ",",
		},
		// Fullwidth space (should normalize to canonical "space")
		{
			name:     "fullwidth space U+3000",
			input:    "\u3000",
			expected: keyvocab.KeyNameSpace,
		},
		// Regular space (should also normalize to "space")
		{
			name:     "regular space",
			input:    " ",
			expected: keyvocab.KeyNameSpace,
		},
		// Other fullwidth punctuation
		{
			name:     "fullwidth period",
			input:    "\uFF0E",
			expected: ".",
		},
		{
			name:     "fullwidth exclamation",
			input:    "\uFF01",
			expected: "!",
		},
		{
			name:     "fullwidth question mark",
			input:    "\uFF1F",
			expected: "?",
		},
		// Fullwidth letters
		{
			name:     "fullwidth A",
			input:    "\uFF21",
			expected: "a",
		},
		{
			name:     "fullwidth z",
			input:    "\uFF5A",
			expected: "z",
		},
		// Fullwidth numbers
		{
			name:     "fullwidth 0",
			input:    "\uFF10",
			expected: "0",
		},
		{
			name:     "fullwidth 9",
			input:    "\uFF19",
			expected: "9",
		},
		// ASCII characters (should pass through unchanged)
		{
			name:     "regular comma",
			input:    ",",
			expected: ",",
		},
		{
			name:     "regular letter",
			input:    "a",
			expected: "a",
		},
		{
			name:     "regular uppercase letter",
			input:    "A",
			expected: "a",
		},
		// Special keys (should use canonical forms)
		{
			name:     keyvocab.KeyNameEscape,
			input:    keyvocab.KeyNameEscape,
			expected: keyvocab.KeyNameEscape,
		},
		{
			name:     "fullwidth escape letters normalize to canonical escape",
			input:    "\uFF25\uFF33\uFF23\uFF21\uFF30\uFF25",
			expected: keyvocab.KeyNameEscape,
		},
		// Edge cases
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "multiple fullwidth chars",
			input:    "\uFF0C\uFF0E", // fullwidth comma + period
			expected: ",.",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := keyvocab.NormalizeKeyForComparison(testCase.input)
			if got != testCase.expected {
				t.Errorf("NormalizeKeyForComparison(%q) = %q, want %q",
					testCase.input, got, testCase.expected)
			}
		})
	}
}

func TestNormalizeKeyForComparison_ModifierComboAliases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Enter/Return aliases in modifier combos
		{
			name:     "Shift+Enter normalizes to shift+return",
			input:    "Shift+Enter",
			expected: wantShiftReturn,
		},
		{
			name:     "Shift+Return normalizes to shift+return",
			input:    "Shift+Return",
			expected: wantShiftReturn,
		},
		{
			name:     "Cmd+Enter normalizes to cmd+return",
			input:    "Cmd+Enter",
			expected: "cmd+return",
		},
		{
			name:     "Cmd+Shift+Enter normalizes to cmd+shift+return",
			input:    "Cmd+Shift+Enter",
			expected: "cmd+shift+return",
		},
		// Bare Enter still works
		{
			name:     "bare Enter normalizes to return",
			input:    "Enter",
			expected: keyvocab.KeyNameReturn,
		},
		{
			name:     "bare Return normalizes to return",
			input:    keyvocab.KeyReturn,
			expected: keyvocab.KeyNameReturn,
		},
		// Backspace/Delete aliases in modifier combos
		{
			name:     "Shift+Backspace normalizes to shift+delete",
			input:    "Shift+Backspace",
			expected: "shift+delete",
		},
		{
			name:     "Cmd+Backspace normalizes to cmd+delete",
			input:    "Cmd+Backspace",
			expected: "cmd+delete",
		},
		// Esc alias in modifier combos
		{
			name:     "Ctrl+Esc normalizes to ctrl+escape",
			input:    "Ctrl+Esc",
			expected: "ctrl+escape",
		},
		// Non-aliased keys should pass through
		{
			name:     "Shift+Space unchanged",
			input:    "Shift+Space",
			expected: "shift+space",
		},
		{
			name:     "Cmd+L unchanged",
			input:    "Cmd+L",
			expected: "cmd+l",
		},
		// Canonical forms must pass through unchanged (regression: +esc prefix of +escape)
		{
			name:     "Ctrl+Escape stays ctrl+escape",
			input:    "Ctrl+Escape",
			expected: "ctrl+escape",
		},
		{
			name:     "Shift+Return stays shift+return",
			input:    "Shift+Return",
			expected: wantShiftReturn,
		},
		{
			name:     "Cmd+Delete stays cmd+delete",
			input:    "Cmd+Delete",
			expected: "cmd+delete",
		},
		{
			name:     "Primary+Space normalizes to platform primary modifier",
			input:    "Primary+Space",
			expected: map[bool]string{true: "cmd+space", false: "ctrl+space"}[isDarwin],
		},
		// Modifier order: a tap spells modifiers in its own order (#1733)
		{
			name:     "Shift+Ctrl+c matches Ctrl+Shift+C",
			input:    "Shift+Ctrl+c",
			expected: wantCtrlShiftC,
		},
		{
			name:     "Shift+Primary+C sorts after resolving primary",
			input:    "Shift+Primary+C",
			expected: map[bool]string{true: "cmd+shift+c", false: wantCtrlShiftC}[isDarwin],
		},
		{
			name:     "plus as base key keeps its segments",
			input:    "Shift+Ctrl++",
			expected: "shift+ctrl++",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := keyvocab.NormalizeKeyForComparison(testCase.input)
			if got != testCase.expected {
				t.Errorf("NormalizeKeyForComparison(%q) = %q, want %q",
					testCase.input, got, testCase.expected)
			}
		})
	}
}

func TestCanonicalHotkeyForPlatform(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "primary modifier becomes current platform token",
			input:    "Primary+Space",
			expected: map[bool]string{true: wantCmdSpace, false: "Ctrl+Space"}[isDarwin],
		},
		{
			name:     "named key is canonicalized",
			input:    "Primary+enter",
			expected: map[bool]string{true: "Cmd+Enter", false: "Ctrl+Enter"}[isDarwin],
		},
		{
			name:     "super alias becomes platform cmd token",
			input:    superSpace,
			expected: map[bool]string{true: wantCmdSpace, false: superSpace}[isDarwin],
		},
		{
			name:     "meta alias becomes platform cmd token",
			input:    "Meta+Space",
			expected: map[bool]string{true: wantCmdSpace, false: superSpace}[isDarwin],
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := keyvocab.CanonicalHotkeyForPlatform(testCase.input)
			if got != testCase.expected {
				t.Fatalf(
					"CanonicalHotkeyForPlatform(%q) = %q, want %q",
					testCase.input,
					got,
					testCase.expected,
				)
			}
		})
	}
}

func TestHasPassthroughModifier(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{name: "cmd combo", key: "Cmd+Tab", want: true},
		{name: "ctrl combo", key: "Ctrl+D", want: true},
		{name: "option combo", key: "Option+Space", want: true},
		{name: "shift only combo", key: "Shift+Tab", want: false},
		{name: "plain key", key: "j", want: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := keyvocab.HasPassthroughModifier(testCase.key)
			if got != testCase.want {
				t.Errorf(
					"HasPassthroughModifier(%q) = %v, want %v",
					testCase.key,
					got,
					testCase.want,
				)
			}
		})
	}
}

func TestNormalizeKeyForComparison_CJKInputMethodScenarios(t *testing.T) {
	// These tests simulate real-world CJK input method scenarios
	tests := []struct {
		name     string
		input    string
		expected string
		desc     string
	}{
		{
			name:     "Chinese input comma key",
			input:    "，",
			expected: ",",
			desc:     "User presses comma key with Chinese IM active",
		},
		{
			name:     "fullwidth period key (U+FF0E)",
			input:    "\uFF0E",
			expected: ".",
			desc:     "Fullwidth period from keyboard layout",
		},
		{
			name:     "Chinese input space key",
			input:    "　",
			expected: keyvocab.KeyNameSpace,
			desc:     "User presses space key with Chinese IM active",
		},
		{
			name:     "Japanese fullwidth exclamation",
			input:    "！",
			expected: "!",
			desc:     "Japanese fullwidth exclamation mark",
		},
		{
			name:     "Korean input (also uses fullwidth chars)",
			input:    "，",
			expected: ",",
			desc:     "Korean input methods also produce fullwidth punctuation",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := keyvocab.NormalizeKeyForComparison(testCase.input)
			if got != testCase.expected {
				t.Errorf("%s: NormalizeKeyForComparison(%q) = %q, want %q",
					testCase.desc, testCase.input, got, testCase.expected)
			}
		})
	}
}
