//go:build linux

package linux

import (
	"sort"
	"strings"
)

// Canonicalizes hotkey chord strings into a stable, order-independent signature
// so config keybindings and live evdev key events compare reliably.
// Does NOT read devices or fire callbacks; that lives in the cgo listener.
//
// Canonical base-key spellings shared between config parsing and the evdev
// decoder so both sides compare equal without drifting on string literals.
const (
	canonicalKeyReturn    = "return"
	canonicalKeySpace     = "space"
	canonicalKeyTab       = "tab"
	canonicalKeyEscape    = "escape"
	canonicalKeyBackspace = "backspace"

	// canonicalKeyDelete is the spelling a configured "Delete" arrives as. It
	// is a binding-side name only: canonicalBindingBaseKey folds it to
	// canonicalKeyBackspace, so no stored chord ever carries it.
	canonicalKeyDelete = "delete"
)

// canonicalBindingSignature is canonicalChordSignature for a chord read from
// the config, the side that can spell "Delete". In [hotkeys] that name means
// the backspace key on every platform — kVK_Delete on macOS, VK_BACK on
// Windows, XK_BackSpace on X11 — so it is folded to the signature a press of
// that key produces. A live press never takes this fold: the forward-delete
// key keeps the "delete" signature, which no configured chord can carry, and
// so matches nothing, as it does on the other platforms.
func canonicalBindingSignature(chord string) string {
	return chordSignature(chord, canonicalBindingBaseKey)
}

// canonicalChordSignature normalizes a chord such as "Ctrl+Shift+G" or the
// evdev-decoded "Shift+Ctrl+g" into a stable signature like "ctrl+shift+g":
// modifiers lowercased, de-duplicated and sorted, base key lowercased/normalized.
// This lets the config side and the live keyboard side match regardless of the
// order or casing each produced.
func canonicalChordSignature(chord string) string {
	return chordSignature(chord, canonicalBaseKey)
}

// chordSignature is the normalization both signatures share; baseKey is how
// the non-modifier key is spelled.
func chordSignature(chord string, baseKey func(string) string) string {
	chord = strings.TrimSpace(chord)
	if chord == "" {
		return ""
	}

	parts := strings.Split(chord, "+")
	if len(parts) == 0 {
		return ""
	}

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

// canonicalModifierToken maps the many modifier spellings Neru and the various
// platforms use down to one of four canonical tokens. "Primary" resolves to
// ctrl here because it is already Ctrl on Linux by the time hotkeys register.
func canonicalModifierToken(token string) string {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case evdevModifierShift:
		return evdevModifierShift
	case evdevModifierCtrl, evdevModifierAliasControl, "primary":
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
// canonicalBindingSignature describes: "Delete" names the backspace key.
func canonicalBindingBaseKey(base string) string {
	switch strings.ToLower(strings.TrimSpace(base)) {
	case canonicalKeyDelete:
		return canonicalKeyBackspace
	default:
		return canonicalBaseKey(base)
	}
}

// canonicalBaseKey normalizes the non-modifier key. Single characters are
// lowercased; common named keys are folded to one spelling.
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
