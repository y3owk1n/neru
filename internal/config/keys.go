package config

import (
	"runtime"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
)

// StringOrStringArray is a type that can unmarshal from either a TOML string
// or a TOML array of strings. Used for backward compatibility.
type StringOrStringArray []string

// UnmarshalTOML implements custom unmarshaling for TOML compatibility.
// It accepts both single string values and arrays of strings.
func (s *StringOrStringArray) UnmarshalTOML(value any) error {
	switch val := value.(type) {
	case string:
		*s = []string{val}

	case []any:
		*s = make([]string, 0, len(val))
		for _, a := range val {
			actionStr, ok := a.(string)
			if !ok {
				return derrors.Newf(derrors.CodeInvalidConfig, "expected string, got %T", a)
			}

			*s = append(*s, actionStr)
		}

	case []string:
		*s = val

	default:
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"cannot unmarshal %T into StringOrStringArray",
			value,
		)
	}

	return nil
}

func init() {
	validNamedKeysLower = make(map[string]bool, len(validNamedKeys))
	namedKeyDisplayForm = make(map[string]string, len(validNamedKeys))

	for k := range validNamedKeys {
		lower := strings.ToLower(k)
		validNamedKeysLower[lower] = true
		namedKeyDisplayForm[lower] = k
	}
}

// IsValidNamedKey checks whether a key name is a recognized named key (case-insensitive).
func IsValidNamedKey(key string) bool {
	return validNamedKeysLower[strings.ToLower(key)]
}

// CanonicalNamedKeyForm returns the canonical display form of a named key
// (e.g. "pagedown" → "PageDown", "UP" → "Up", "f1" → "F1").
// If the key is not a recognized named key, it returns the input unchanged
// and false as the second return value.
func CanonicalNamedKeyForm(key string) (string, bool) {
	display, displayOk := namedKeyDisplayForm[strings.ToLower(key)]

	if !displayOk {
		return key, false
	}

	return display, displayOk
}

// NormalizeKeyForComparison converts escape sequences and key names to a canonical form for comparison.
// This ensures that "\x1b" and "escape" are treated as the same key, and provides case-insensitive
// matching for all keys (e.g. "q" matches "Q", "Ctrl+R" matches "ctrl+r").
// On macOS, both "backspace" and "delete" are treated as synonyms for the DEL key (\x7f).
// Named keys (arrows, function keys, nav keys) are normalized to their canonical lowercase form.
// Also normalizes fullwidth CJK characters to their halfwidth ASCII equivalents.
func NormalizeKeyForComparison(key string) string {
	// Normalize fullwidth CJK characters first, before lowercasing and canonical matching.
	// This ensures e.g. fullwidth space (U+3000) → " " → "space" in a single pass.
	key = normalizeFullwidthChars(key)
	key = strings.ToLower(key)

	// Handle escape sequences and aliases that map to a different canonical form.
	switch key {
	case "\x1b", "esc":
		return KeyNameEscape
	case "\r", "enter":
		return KeyNameReturn
	case "\t":
		return KeyNameTab
	case " ":
		return KeyNameSpace
	case "\x08", "\x7f", KeyNameBackspace:
		// On macOS, the Delete key (above Return) sends \x7f.
		// \x08 is the ASCII BS control character (rarely generated on macOS but included for completeness).
		// Treat "delete", "backspace", \x7f, and \x08 as synonyms for user-friendly matching.
		return KeyNameDelete
	}

	// Normalize key aliases inside modifier combos.
	// The switch above only handles bare "enter" / "backspace" etc., but users may
	// write "Shift+Enter" which lowercases to "shift+enter". The event tap always
	// produces the canonical form "shift+return", so we must resolve the alias here.
	key = normalizeKeyAliasesInCombo(key)

	// Normalize modifier aliases like "Primary" to the platform-native token
	// so shared config can map to Cmd on macOS and Ctrl elsewhere.
	key = normalizeModifierAliasesInCombo(key, runtime.GOOS)

	// All other keys (named keys, plain characters, modifier combos) are already
	// lowercased by strings.ToLower above and pass through as-is.
	return key
}

// HasPassthroughModifier reports whether the key contains a modifier that can
// be allowed through to macOS while a mode is active. Shift-only combos are
// excluded because they are commonly used inside modes.
func HasPassthroughModifier(key string) bool {
	parts := strings.Split(NormalizeKeyForComparison(key), "+")
	if len(parts) < minimumModifierParts {
		return false
	}

	for _, part := range parts[:len(parts)-1] {
		trimmed := strings.TrimSpace(part)
		switch trimmed {
		case modifierNameCmd, modifierNameCtrl, modifierNameAlt:
			return true
		}
	}

	return false
}

func primaryModifierTokenForOS(goos string) string {
	if goos == "darwin" {
		return "cmd"
	}

	return modifierNameCtrl
}

func normalizeModifierTokenForOS(token, goos string) string {
	switch strings.TrimSpace(strings.ToLower(token)) {
	case "primary":
		return primaryModifierTokenForOS(goos)
	case modifierNameCmd, "command", "super", "meta", "rightcmd", "leftcmd":
		return modifierNameCmd
	case modifierNameCtrl, "control", "rightctrl", "leftctrl":
		return modifierNameCtrl
	case modifierNameAlt, "option", "rightalt", "leftalt", "rightoption", "leftoption":
		return modifierNameAlt
	case modifierNameShift, "rightshift", "leftshift":
		return modifierNameShift
	default:
		return strings.TrimSpace(strings.ToLower(token))
	}
}

// normalizeKeyAliasesInCombo resolves key name aliases inside modifier combos.
// e.g. "shift+enter" → "shift+return", "cmd+backspace" → "cmd+delete".
// Only applies to compound keys (containing "+"); bare keys are handled by the
// switch in NormalizeKeyForComparison.
// Splits on the last "+" and only normalizes the final segment to avoid mangling
// modifier names or canonical forms that share a prefix (e.g. "escape" vs "esc").
func normalizeKeyAliasesInCombo(key string) string {
	idx := strings.LastIndex(key, "+")
	if idx < 0 {
		return key
	}

	prefix, suffix := key[:idx+1], key[idx+1:]
	if canonical, ok := comboKeyAliases[suffix]; ok {
		return prefix + canonical
	}

	return key
}

func normalizeModifierAliasesInCombo(key, goos string) string {
	parts := strings.Split(key, "+")
	if len(parts) < minimumModifierParts {
		return key
	}

	for idx := range len(parts) - 1 {
		parts[idx] = normalizeModifierTokenForOS(parts[idx], goos)
	}

	return strings.Join(parts, "+")
}

func displayModifierTokenForOS(token, goos string) string {
	switch token {
	case modifierNameCmd:
		if goos == "darwin" {
			return "Cmd"
		}

		return "Super"
	case modifierNameCtrl:
		return "Ctrl"
	case modifierNameAlt:
		return "Alt"
	case modifierNameShift:
		return "Shift"
	default:
		return token
	}
}

// CanonicalHotkeyForPlatform rewrites shared modifier aliases like "Primary"
// into the concrete tokens expected by the current platform backend.
func CanonicalHotkeyForPlatform(hotkey string) string {
	return canonicalHotkeyForOS(hotkey, runtime.GOOS)
}

func canonicalHotkeyForOS(hotkey, goos string) string {
	if hotkey == "" {
		return hotkey
	}

	parts := strings.Split(hotkey, "+")

	for idx := range len(parts) - 1 {
		parts[idx] = displayModifierTokenForOS(normalizeModifierTokenForOS(parts[idx], goos), goos)
	}

	last := strings.TrimSpace(parts[len(parts)-1])
	if canonical, ok := CanonicalNamedKeyForm(last); ok {
		parts[len(parts)-1] = canonical
	} else {
		parts[len(parts)-1] = last
	}

	return strings.Join(parts, "+")
}

// normalizeFullwidthChars converts fullwidth CJK characters (U+FF01-U+FF5E)
// to their halfwidth ASCII equivalents (U+0021-U+007E).
// This ensures keys work correctly when using CJK input methods.
// Uses strings.Map for efficiency - only allocates when transformation occurs.
func normalizeFullwidthChars(key string) string {
	const (
		fullwidthStart  = 0xFF01 // Fullwidth exclamation mark
		fullwidthEnd    = 0xFF5E // Fullwidth tilde
		halfwidthOffset = 0xFEE0 // Difference between fullwidth and halfwidth
		fullwidthSpace  = 0x3000 // CJK fullwidth space
	)

	return strings.Map(func(char rune) rune {
		switch {
		case char >= fullwidthStart && char <= fullwidthEnd:
			// Convert fullwidth to halfwidth
			return char - halfwidthOffset
		case char == fullwidthSpace:
			// Fullwidth space -> regular space
			return ' '
		default:
			// Return unchanged (strings.Map optimizes this case)
			return char
		}
	}, key)
}
