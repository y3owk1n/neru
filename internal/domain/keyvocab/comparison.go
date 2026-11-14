package keyvocab

import (
	"runtime"
	"slices"
	"strings"

	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/parity"
)

// What NormalizeKeyForComparison returns for the control characters a tap
// delivers in place of a key name. This is not the list of named keys. Every
// other named key compares in its lowercase form.
const (
	KeyNameEscape = "escape"
	KeyNameReturn = "return"
	KeyNameTab    = "tab"
	KeyNameSpace  = "space"
	KeyNameDelete = "delete"
)

// The modifier spellings CanonicalHotkeyForPlatform writes.
const (
	ModifierDisplayCmd   = "Cmd"
	ModifierDisplayCtrl  = "Ctrl"
	ModifierDisplayAlt   = "Alt"
	ModifierDisplaySuper = "Super"
)

const minimumModifierParts = 2

// NormalizeKeyForComparison puts escape sequences and key names into one form,
// so "\x1b" and "escape" are the same key and "q" matches "Q".
//
// On macOS "backspace" and "delete" are both the DEL key (\x7f). Named keys,
// such as arrows, function keys and nav keys, become their lowercase form.
// Also normalizes fullwidth CJK characters to their halfwidth ASCII equivalents.
// Primary resolves to the modifier action.UsePrimaryModifier published.
func NormalizeKeyForComparison(key string) string {
	return NormalizeKeyWithPrimary(key, publishedPrimaryToken())
}

// NormalizeKeyWithPrimary is NormalizeKeyForComparison with Primary resolved
// to primary, a comparison token such as "cmd", rather than the published
// modifier. Config uses it to compare the keys of a configuration it is still
// loading or validating.
func NormalizeKeyWithPrimary(key, primary string) string {
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
		// On macOS the Delete key (above Return) sends \x7f. \x08 is the ASCII
		// BS control character. Both match "delete".
		return KeyNameDelete
	}

	// Resolve a key-name alias to the key it means, bare ("enter") or at the end
	// of a combo ("shift+enter"). The event tap always produces the canonical
	// form, so a binding written with an alias has to reach the same string.
	key = normalizeKeyAliases(key)

	// Normalize modifier aliases like "Primary" to the platform-native token
	// so shared config can map to Cmd on macOS and Ctrl elsewhere.
	key = normalizeModifierAliasesInCombo(key, primary)

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
		case ModifierCmd, ModifierCtrl, ModifierAlt:
			return true
		}
	}

	return false
}

// ComparisonModifier resolves a modifier spelling to its comparison token,
// with "primary" resolving to primary. Unlike CanonicalModifier, it knows the
// sided spellings ("leftcmd", "rightoption"), and it returns a token that is
// not a modifier trimmed and lowercased instead of "".
func ComparisonModifier(token, primary string) string {
	switch strings.TrimSpace(strings.ToLower(token)) {
	case "primary":
		return primary
	case ModifierCmd, "command", "super", "meta", "rightcmd", "leftcmd":
		return ModifierCmd
	case ModifierCtrl, "control", "rightctrl", "leftctrl":
		return ModifierCtrl
	case ModifierAlt, "option", "rightalt", "leftalt", "rightoption", "leftoption":
		return ModifierAlt
	case ModifierShift, "rightshift", "leftshift":
		return ModifierShift
	default:
		return strings.TrimSpace(strings.ToLower(token))
	}
}

// publishedPrimaryToken is the comparison token of the modifier
// action.UsePrimaryModifier published.
func publishedPrimaryToken() string {
	mod := action.PrimaryModifier()
	if mod == action.ModCmd {
		return ModifierCmd
	}

	if mod == action.ModAlt {
		return ModifierAlt
	}

	return ModifierCtrl
}

// normalizeKeyAliases resolves a key-name alias to the key it means, in the
// lowercase comparison form. "enter" becomes "return", "cmd+backspace" becomes
// "cmd+delete", and "esc" becomes "escape".
//
// Only the base key is a candidate, which is the segment after the last "+",
// or the whole string when there is none. A modifier name is never an alias, and
// splitting on the last "+" keeps a canonical form from being mangled into one
// that shares its prefix ("escape" vs "esc").
//
// This is a deliberate near-miss of NormalizeKey (ADR 0007). Both resolve the
// same aliases from the same declaration, but they canonicalize to different
// forms. NormalizeKey produces the display spelling a tap emits, and this
// produces the lowercase form a keystroke is matched in. Calling NormalizeKey
// and lowercasing would also trim the key, which comparison must not do.
func normalizeKeyAliases(key string) string {
	idx := strings.LastIndex(key, "+")
	prefix, base := key[:idx+1], key[idx+1:]

	if means, isAlias := ResolveAlias(base); isAlias {
		return prefix + strings.ToLower(means)
	}

	return key
}

// normalizeModifierAliasesInCombo resolves each modifier to its platform token
// and sorts the modifiers. Each tap spells modifiers in its own order, so the
// sort makes "Primary+Shift+C" from config match "Shift+Ctrl+c" from the Linux
// tap. A combo with a segment that is not a modifier ("ctrl++") keeps its order.
func normalizeModifierAliasesInCombo(key, primary string) string {
	parts := strings.Split(key, "+")
	if len(parts) < minimumModifierParts {
		return key
	}

	allModifiers := true

	for idx := range len(parts) - 1 {
		parts[idx] = ComparisonModifier(parts[idx], primary)

		switch parts[idx] {
		case ModifierCmd, ModifierCtrl, ModifierAlt, ModifierShift:
		default:
			allModifiers = false
		}
	}

	if allModifiers {
		slices.Sort(parts[:len(parts)-1])
	}

	return strings.Join(parts, "+")
}

func displayModifierTokenForOS(token, goos string) string {
	switch token {
	case ModifierCmd:
		if parity.Platform(goos) == parity.Darwin {
			return ModifierDisplayCmd
		}

		return ModifierDisplaySuper
	case ModifierCtrl:
		return ModifierDisplayCtrl
	case ModifierAlt:
		return ModifierDisplayAlt
	case ModifierShift:
		return "Shift"
	default:
		return token
	}
}

// CanonicalHotkeyForPlatform rewrites shared modifier aliases like "Primary"
// into the concrete tokens expected by the current platform backend.
func CanonicalHotkeyForPlatform(hotkey string) string {
	return canonicalHotkey(hotkey, publishedPrimaryToken(), runtime.GOOS)
}

func canonicalHotkey(hotkey, primary, goos string) string {
	if hotkey == "" {
		return hotkey
	}

	parts := strings.Split(hotkey, "+")

	for idx := range len(parts) - 1 {
		parts[idx] = displayModifierTokenForOS(ComparisonModifier(parts[idx], primary), goos)
	}

	last := strings.TrimSpace(parts[len(parts)-1])
	if canonical, ok := NamedKeyDisplay(last); ok {
		parts[len(parts)-1] = canonical
	} else {
		parts[len(parts)-1] = last
	}

	return strings.Join(parts, "+")
}

// normalizeFullwidthChars converts fullwidth CJK characters (U+FF01-U+FF5E)
// to their halfwidth ASCII equivalents (U+0021-U+007E).
// Without it, keys misfire under CJK input methods. strings.Map only allocates
// when a character changes.
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
			return char - halfwidthOffset
		case char == fullwidthSpace:
			return ' '
		default:
			return char
		}
	}, key)
}
