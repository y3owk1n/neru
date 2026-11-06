//go:build linux

package linux

import (
	"sort"
	"strings"

	"github.com/y3owk1n/neru/internal/domain/action"
)

// Hotkey chords are canonicalized into an order-independent signature so
// config bindings and live evdev key events compare equal. The constants are
// the base-key spellings config parsing and the evdev decoder share.
const (
	canonicalKeyReturn    = "return"
	canonicalKeySpace     = "space"
	canonicalKeyTab       = "tab"
	canonicalKeyEscape    = "escape"
	canonicalKeyBackspace = "backspace"

	// canonicalKeyDelete is the spelling a configured "Delete" arrives as.
	// canonicalBindingBaseKey folds it to canonicalKeyBackspace, so no stored
	// chord carries it.
	canonicalKeyDelete = "delete"
)

// canonicalBindingSignature is canonicalChordSignature for a chord from the
// config. In [hotkeys] "Delete" means backspace on every platform (kVK_Delete
// on macOS, VK_BACK on Windows, XK_BackSpace on X11), so it folds to the
// signature that key's press produces. Live presses never fold, so the
// forward-delete key keeps "delete" and matches nothing, as elsewhere.
func canonicalBindingSignature(chord string) string {
	return chordSignature(chord, canonicalBindingBaseKey)
}

// canonicalChordSignature normalizes a chord such as "Ctrl+Shift+G" or the
// evdev-decoded "Shift+Ctrl+g" into a stable signature like "ctrl+shift+g":
// modifiers lowercased, de-duplicated and sorted, and the base key normalized.
func canonicalChordSignature(chord string) string {
	return chordSignature(chord, canonicalBaseKey)
}

// chordSignature is the normalization both signatures share. baseKey spells
// the non-modifier key.
func chordSignature(chord string, baseKey func(string) string) string {
	chord = strings.TrimSpace(chord)
	if chord == "" {
		return ""
	}

	parts := strings.Split(chord, "+")

	base := baseKey(parts[len(parts)-1])
	if base == "" {
		return ""
	}

	mods := make([]string, 0, len(parts)-1)
	seen := make(map[string]bool, len(parts))

	for _, part := range parts[:len(parts)-1] {
		mod := canonicalModifierToken(part)
		if mod == "" || seen[mod] {
			continue
		}

		seen[mod] = true
		mods = append(mods, mod)
	}

	sort.Strings(mods)

	if len(mods) == 0 {
		return base
	}

	return strings.Join(mods, "+") + "+" + base
}

// canonicalModifierToken maps modifier spellings to one of four canonical
// tokens. Hotkeys register with Primary already resolved, but a "primary"
// that still arrives resolves to general.primary_modifier.
func canonicalModifierToken(token string) string {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "primary":
		return canonicalModifierToken(action.PrimaryModifier().Canonical())
	case evdevModifierShift:
		return evdevModifierShift
	case evdevModifierCtrl, evdevModifierAliasControl:
		return evdevModifierCtrl
	case evdevModifierAlt, evdevModifierAliasOption, "opt":
		return evdevModifierAlt
	case evdevModifierCmd, "command", evdevModifierAliasSuper, "meta", "win", "windows":
		return evdevModifierCmd
	default:
		return ""
	}
}

// canonicalBindingBaseKey is canonicalBaseKey plus the fold
// canonicalBindingSignature describes, in which "Delete" names the backspace key.
func canonicalBindingBaseKey(base string) string {
	switch strings.ToLower(strings.TrimSpace(base)) {
	case canonicalKeyDelete:
		return canonicalKeyBackspace
	default:
		return canonicalBaseKey(base)
	}
}

// canonicalBaseKey normalizes the non-modifier key. Single characters are
// lowercased, and common named keys are folded to one spelling.
func canonicalBaseKey(base string) string {
	lowered := strings.ToLower(strings.TrimSpace(base))

	switch lowered {
	case "":
		return ""
	case canonicalKeyReturn, "enter":
		return canonicalKeyReturn
	case canonicalKeySpace, "spacebar":
		return canonicalKeySpace
	case canonicalKeyTab:
		return canonicalKeyTab
	case canonicalKeyEscape, "esc":
		return canonicalKeyEscape
	case canonicalKeyBackspace:
		return canonicalKeyBackspace
	default:
		return lowered
	}
}
