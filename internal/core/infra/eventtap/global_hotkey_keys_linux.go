//go:build linux

// internal/core/infra/eventtap/global_hotkey_keys_linux.go
// Canonicalizes hotkey chord strings into a stable, order-independent signature
// so config keybindings and live evdev key events compare reliably.
// Does NOT read devices or fire callbacks; that lives in the cgo listener.

package eventtap

import (
	"sort"
	"strings"
)

// Canonical base-key spellings shared between config parsing and the evdev
// decoder so both sides compare equal without drifting on string literals.
const (
	canonicalKeyReturn    = "return"
	canonicalKeySpace     = "space"
	canonicalKeyTab       = "tab"
	canonicalKeyEscape    = "escape"
	canonicalKeyBackspace = "backspace"
)

// canonicalChordSignature normalizes a chord such as "Ctrl+Shift+G" or the
// evdev-decoded "Shift+Ctrl+g" into a stable signature like "ctrl+shift+g":
// modifiers lowercased, de-duplicated and sorted, base key lowercased/normalized.
// This lets the config side and the live keyboard side match regardless of the
// order or casing each produced.
func canonicalChordSignature(chord string) string {
	chord = strings.TrimSpace(chord)
	if chord == "" {
		return ""
	}

	parts := strings.Split(chord, "+")
	if len(parts) == 0 {
		return ""
	}

	base := canonicalBaseKey(parts[len(parts)-1])
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
