package keyvocab

import "strings"

const (
	// KeyUpPrefix marks a synthetic key-release event ("__keyup_<base key>"),
	// used to stop held-key repeat.
	KeyUpPrefix = "__keyup_"

	// ModifierTogglePrefix marks a synthetic modifier transition event
	// ("__modifier_<name>_<down|up>"), used for sticky-modifier tracking.
	ModifierTogglePrefix = "__modifier_"
)

// Canonical modifier names as they appear in configs, key strings and
// modifier toggle events.
const (
	ModifierCmd   = "cmd"
	ModifierShift = "shift"
	ModifierAlt   = "alt"
	ModifierCtrl  = "ctrl"
)

const (
	toggleSuffixDown = "_down"
	toggleSuffixUp   = "_up"
)

// NormalizeKey canonicalizes a key string of the form
// "modifier+...+baseKey": an alias base key becomes the key it means
// (enter -> Return, esc -> Escape, backspace -> Delete), every other named base
// key gets its display spelling (pagedown -> PageDown), and single-rune base
// keys are lowercased. Anything else passes through unchanged, as do modifier
// segments. Empty input normalizes to "".
func NormalizeKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}

	parts := strings.Split(key, "+")
	parts[len(parts)-1] = normalizeBaseKey(parts[len(parts)-1])

	return strings.Join(parts, "+")
}

// normalizeBaseKey canonicalizes the key a combo ends in, reading the named-key
// declaration rather than listing spellings a second time.
func normalizeBaseKey(baseKey string) string {
	if means, isAlias := ResolveAlias(baseKey); isAlias {
		return means
	}

	if display, isNamed := NamedKeyDisplay(baseKey); isNamed {
		return display
	}

	if len([]rune(baseKey)) == 1 {
		return strings.ToLower(baseKey)
	}

	return baseKey
}

// CanonicalModifier maps a modifier spelling (including platform aliases:
// command/super/meta/win -> cmd, option -> alt, control -> ctrl) to its
// canonical name, or "" when the input is not a modifier.
func CanonicalModifier(modifier string) string {
	switch strings.ToLower(strings.TrimSpace(modifier)) {
	case ModifierCmd, "command", "super", "meta", "win":
		return ModifierCmd
	case ModifierShift:
		return ModifierShift
	case ModifierAlt, "option":
		return ModifierAlt
	case ModifierCtrl, "control":
		return ModifierCtrl
	default:
		return ""
	}
}

// KeyUpEvent formats the synthetic key-release event for a pressed key.
// The event carries only the normalized base key (no modifier prefix), to
// match how the mode handler tracks held keys. Returns "" for empty input.
func KeyUpEvent(key string) string {
	key = NormalizeKey(key)
	if key == "" {
		return ""
	}

	parts := strings.Split(key, "+")
	baseKey := parts[len(parts)-1]

	return KeyUpPrefix + baseKey
}

// ModifierToggleEvent formats the synthetic modifier transition event for a
// modifier press (isDown true) or release. The modifier is canonicalized
// first; a non-modifier input returns "".
func ModifierToggleEvent(modifier string, isDown bool) string {
	modifier = CanonicalModifier(modifier)
	if modifier == "" {
		return ""
	}

	if isDown {
		return ModifierTogglePrefix + modifier + toggleSuffixDown
	}

	return ModifierTogglePrefix + modifier + toggleSuffixUp
}

// ParseModifierToggle decodes a modifier transition event back into its
// canonical modifier name and direction. ok is false when the input is not a
// well-formed toggle event for a known modifier.
func ParseModifierToggle(event string) (string, bool, bool) {
	if !strings.HasPrefix(event, ModifierTogglePrefix) {
		return "", false, false
	}

	body := strings.ToLower(strings.TrimPrefix(event, ModifierTogglePrefix))

	if name, found := strings.CutSuffix(body, toggleSuffixDown); found {
		if canonical := CanonicalModifier(name); canonical != "" {
			return canonical, true, true
		}

		return "", false, false
	}

	if name, found := strings.CutSuffix(body, toggleSuffixUp); found {
		if canonical := CanonicalModifier(name); canonical != "" {
			return canonical, false, true
		}

		return "", false, false
	}

	return "", false, false
}
