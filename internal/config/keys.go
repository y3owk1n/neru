package config

import (
	"runtime"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
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

// IsValidNamedKey checks whether a key name is a recognized named key
// (case-insensitive). The set it asks is declared in internal/domain/keyvocab.
func IsValidNamedKey(key string) bool {
	return keyvocab.IsNamedKey(key)
}

// CanonicalNamedKeyForm returns the canonical display form of a named key
// (e.g. "pagedown" → "PageDown", "UP" → "Up", "f1" → "F1").
// If the key is not a recognized named key, it returns the input unchanged
// and false as the second return value.
func CanonicalNamedKeyForm(key string) (string, bool) {
	return keyvocab.NamedKeyDisplay(key)
}

// NormalizeKeyForComparison puts escape sequences and key names into one form,
// so "\x1b" and "escape" are the same key and "q" matches "Q".
//
// On macOS "backspace" and "delete" are both the DEL key (\x7f). Named keys —
// arrows, function keys, nav keys — become their lowercase form.
// Also normalizes fullwidth CJK characters to their halfwidth ASCII equivalents.
func NormalizeKeyForComparison(key string) string {
	// Normalize fullwidth CJK characters first, before lowercasing and canonical matching.
	// So a fullwidth space (U+3000) becomes " " becomes "space" in one pass.
	key = normalizeFullwidthChars(key)
	key = strings.ToLower(key)

	// Handle the control characters a tap delivers instead of a key name.
	switch key {
	case "\x1b":
		return KeyNameEscape
	case "\r":
		return KeyNameReturn
	case "\t":
		return KeyNameTab
	case " ":
		return KeyNameSpace
	case "\x08", "\x7f":
		// On macOS, the Delete key (above Return) sends \x7f.
		// \x08 is the ASCII BS control character (rarely generated on macOS but included for completeness).
		// Treat \x7f and \x08 as synonyms of "delete" for user-friendly matching.
		return KeyNameDelete
	}

	// Resolve a key-name alias to the key it means, bare ("enter") or at the end
	// of a combo ("shift+enter"). The event tap always produces the canonical
	// form, so a binding written with an alias has to reach the same string.
	key = normalizeKeyAliases(key)

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

// normalizeKeyAliases resolves a key-name alias to the key it means, in the
// lowercase comparison form: "enter" → "return", "cmd+backspace" →
// "cmd+delete", "esc" → "escape". The aliases are keyvocab's, so config does
// not carry a second list of them.
//
// Only the base key — the segment after the last "+", or the whole string when
// there is none — is a candidate. A modifier name is never an alias, and
// splitting on the last "+" keeps a canonical form from being mangled into one
// that shares its prefix ("escape" vs "esc").
//
// This is a deliberate near-miss of keyvocab.NormalizeKey (ADR 0007): both
// resolve the same aliases from the same declaration, but they canonicalize to
// different forms — NormalizeKey produces the display spelling a tap emits,
// this produces the lowercase form a keystroke is matched in. Calling
// NormalizeKey and lowercasing would also trim the key, which comparison must
// not do.
func normalizeKeyAliases(key string) string {
	idx := strings.LastIndex(key, "+")
	prefix, base := key[:idx+1], key[idx+1:]

	if means, isAlias := keyvocab.ResolveAlias(base); isAlias {
		return prefix + strings.ToLower(means)
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
// Without it, keys misfire under CJK input methods.
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
