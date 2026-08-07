package fontgeneric

import "strings"

// class is what a written font family name asks for.
type class int

const (
	// classNamed means the name asks for one particular family, spelled out —
	// "JetBrains Mono". What that family resolves to is the platform font
	// system's business, not this package's.
	classNamed class = iota
	// classSans means the name asks for the sans-serif face the platform
	// prefers. The empty name asks for this one.
	classSans
	// classSerif means the name asks for the serif face the platform prefers.
	classSerif
	// classMono means the name asks for the monospaced face the platform
	// prefers.
	classMono
)

// separators are the characters people write between the words of a generic
// alias. All three spellings mean the same thing, so they are removed before
// matching rather than enumerated as separate cases.
var separators = strings.NewReplacer(" ", "", "-", "", "_", "")

// parse classifies a written font family name, returning what it asks for and
// the name itself, trimmed. Matching ignores case, surrounding whitespace and
// the separators between words, so "sans serif", "Sans-Serif", "sans_serif"
// and "SANSSERIF" all ask for the same face; the empty name asks for sans.
// Anything else is a family somebody named.
func parse(family string) (class, string) {
	trimmed := strings.TrimSpace(family)

	switch separators.Replace(strings.ToLower(trimmed)) {
	case "", "sans", "sansserif":
		return classSans, trimmed
	case "serif":
		return classSerif, trimmed
	case "mono", "monospace":
		return classMono, trimmed
	default:
		return classNamed, trimmed
	}
}
