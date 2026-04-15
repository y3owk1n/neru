package action

import (
	"runtime"
	"strings"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// Modifiers is a bitmask of keyboard modifier keys held during an action.
type Modifiers uint64

const (
	// ModCmd is the Command (⌘) modifier.
	ModCmd Modifiers = 1 << iota
	// ModShift is the Shift (⇧) modifier.
	ModShift
	// ModAlt is the Option/Alt (⌥) modifier.
	ModAlt
	// ModCtrl is the Control (⌃) modifier.
	ModCtrl
)

// modifierNames maps lowercase modifier names to their bitmask values.
var modifierNames = map[string]Modifiers{
	"cmd":     ModCmd,
	"command": ModCmd,
	"super":   ModCmd,
	"meta":    ModCmd,
	"shift":   ModShift,
	"alt":     ModAlt,
	"option":  ModAlt,
	"ctrl":    ModCtrl,
	"control": ModCtrl,
}

// PrimaryModifier returns the platform's default "main" accelerator modifier.
// On macOS this is Command; on Linux and Windows it is Control.
func PrimaryModifier() Modifiers {
	if runtime.GOOS == "darwin" {
		return ModCmd
	}

	return ModCtrl
}

// ParseModifiers parses a comma-separated modifier string (e.g. "cmd,shift")
// into a Modifiers bitmask. An empty string returns 0 (no modifiers).
func ParseModifiers(input string) (Modifiers, error) {
	if input == "" {
		return 0, nil
	}

	var mods Modifiers
	for part := range strings.SplitSeq(input, ",") {
		name := strings.TrimSpace(strings.ToLower(part))
		if name == "" {
			continue
		}

		if name == "primary" {
			mods |= PrimaryModifier()

			continue
		}

		mod, ok := modifierNames[name]
		if !ok {
			return 0, derrors.Newf(
				derrors.CodeInvalidInput,
				"unknown modifier %q (valid: primary, cmd, super, meta, shift, alt, option, ctrl)",
				part,
			)
		}

		mods |= mod
	}

	return mods, nil
}

// Has reports whether m contains all the modifier bits in other.
func (m Modifiers) Has(other Modifiers) bool {
	return m&other == other
}

// String returns a human-readable representation (e.g. "Cmd+Shift").
func (m Modifiers) String() string {
	if m == 0 {
		return ""
	}

	var parts []string
	if m.Has(ModCmd) {
		parts = append(parts, "Cmd")
	}

	if m.Has(ModShift) {
		parts = append(parts, "Shift")
	}

	if m.Has(ModAlt) {
		parts = append(parts, "Alt")
	}

	if m.Has(ModCtrl) {
		parts = append(parts, "Ctrl")
	}

	return strings.Join(parts, "+")
}
