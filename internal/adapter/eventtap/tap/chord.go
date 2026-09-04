package tap

import (
	"strings"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

// The modifier names a canonical chord is spelled with, in the order they
// appear.
const (
	modifierShift = "shift"
	modifierCtrl  = "ctrl"
	modifierAlt   = "alt"
	modifierCmd   = "cmd"
)

// CanonicalChord normalizes a modifier chord to a stable, order-independent
// form for passthrough matching: aliases are resolved and tokens lowercased via
// config.NormalizeKeyForComparison, then the modifiers are re-emitted in a
// fixed shift+ctrl+alt+cmd order with the base key last.
//
// Applying it to both the configured blacklist and intercepted entries and to
// the chord a backend reads at runtime makes lookups agree regardless of the
// order the user wrote their hotkeys in, or the order the backend assembles
// modifiers in.
func CanonicalChord(chord string) string {
	normalized := config.NormalizeKeyForComparison(strings.TrimSpace(chord))
	if normalized == "" {
		return ""
	}

	parts := strings.Split(normalized, "+")
	base := strings.TrimSpace(parts[len(parts)-1])

	var hasShift, hasCtrl, hasAlt, hasCmd bool

	for _, part := range parts[:len(parts)-1] {
		switch keyvocab.CanonicalModifier(part) {
		case modifierShift:
			hasShift = true
		case modifierCtrl:
			hasCtrl = true
		case modifierAlt:
			hasAlt = true
		case modifierCmd:
			hasCmd = true
		}
	}

	var builder strings.Builder

	for _, mod := range []struct {
		held bool
		name string
	}{
		{hasShift, modifierShift},
		{hasCtrl, modifierCtrl},
		{hasAlt, modifierAlt},
		{hasCmd, modifierCmd},
	} {
		if mod.held {
			builder.WriteString(mod.name)
			builder.WriteByte('+')
		}
	}

	builder.WriteString(base)

	return builder.String()
}

// CanonicalChordSet builds a lookup set of chords normalized via
// CanonicalChord.
func CanonicalChordSet(chords []string) map[string]struct{} {
	set := make(map[string]struct{}, len(chords))

	for _, chord := range chords {
		if canonical := CanonicalChord(chord); canonical != "" {
			set[canonical] = struct{}{}
		}
	}

	return set
}
