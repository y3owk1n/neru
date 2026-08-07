package fontgeneric

// Families are the concrete families one platform uses for the generic
// aliases. Which family a generic resolves to is a platform decision — macOS
// answers "sans" with Helvetica Neue and Linux with DejaVu Sans — so the
// values are declared by each backend; only the reading of the written name
// is shared.
type Families struct {
	// Sans is the family a name asking for sans-serif resolves to. The empty
	// name asks for this one.
	Sans string
	// Serif is the family a name asking for serif resolves to.
	Serif string
	// Mono is the family a name asking for monospace resolves to.
	Mono string
}

// Resolve returns the family to ask the platform's font system for: this
// platform's family when the written name is a generic alias, and the name
// itself, trimmed, when it is a family somebody named.
func (f Families) Resolve(family string) string {
	asked, trimmed := parse(family)

	switch asked {
	case classSans:
		return f.Sans
	case classSerif:
		return f.Serif
	case classMono:
		return f.Mono
	case classNamed:
		return trimmed
	default:
		return trimmed
	}
}
