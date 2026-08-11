package parity

import (
	"runtime"
	"slices"
)

// Platform is one operating system Neru is built for, spelled the way Go
// spells it, so a reader never has to translate between GOOS and a second set
// of names.
type Platform string

const (
	// Darwin is macOS, the reference implementation.
	Darwin Platform = "darwin"
	// Linux is every supported Linux backend.
	Linux Platform = "linux"
	// Windows is Win32.
	Windows Platform = "windows"
)

// AllPlatforms is every platform, in the order a column is read and rendered.
//
// It is the order, as well as the set: a declaration that listed its platforms
// in whatever order it was typed would render two documentation rows that mean
// the same thing differently.
var AllPlatforms = Platforms{Darwin, Linux, Windows}

// Current is the platform this binary is running on, and false when it is one
// Neru declares nothing about.
//
// A build for anything but the three above — the CGO-less `other` slot — has no
// row in any declaration, so nothing is claimed for it either way. Warning
// there would be inventing a platform column, and staying silent is what the
// rest of the tree already does (config_other.go).
func Current() (Platform, bool) {
	return PlatformFor(runtime.GOOS)
}

// PlatformFor maps a GOOS to a platform. The bool is the whole answer for a
// GOOS with no column.
func PlatformFor(goos string) (Platform, bool) {
	candidate := Platform(goos)
	if slices.Contains(AllPlatforms, candidate) {
		return candidate, true
	}

	return "", false
}

// Platforms is the platform column: the set a word does something on.
//
// It is a set rather than a darwin-only flag because Windows has known gaps of
// its own, and a boolean would have to be widened at every call site the day
// the first of them is declared
// (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md).
type Platforms []Platform

// Supports reports whether a word declared with this column does something on
// the given platform.
func (p Platforms) Supports(target Platform) bool {
	return slices.Contains(p, target)
}

// Everywhere reports whether the column covers every platform, which is what
// most words carry.
func (p Platforms) Everywhere() bool {
	for _, platform := range AllPlatforms {
		if !p.Supports(platform) {
			return false
		}
	}

	return true
}

// Kind is the family of vocabulary a word belongs to. It is part of a word's
// identity: `strategy` is a config option and a mode flag, and the two are
// declared separately because they are two names.
type Kind string

const (
	// KindOption is a key in config.toml, written as its full TOML path.
	KindOption Kind = "option"
	// KindModeFlag is a flag on a mode command, written without its dashes.
	KindModeFlag Kind = "mode flag"
	// KindAction is an action name, as a binding or `neru action` writes it.
	KindAction Kind = "action"
)

// Word is one name a person can write, with the platforms writing it does
// something on.
//
// Value is empty for the name itself and set for one of the values it may
// carry: `hints.strategy` is recognized everywhere while `hints.strategy =
// vision` is not, and the two are separate declarations because they are
// separate promises.
type Word struct {
	Kind      Kind
	Name      string
	Value     string
	Platforms Platforms
	// Note says why the column is narrow, and is empty for a word supported
	// everywhere. It is prose a user reads in a warning and in the published
	// table, so it names the gap rather than the code.
	Note string
}

// Written renders the word the way a person writes it, which is the only form
// worth showing them: a TOML path, a `--flag`, an action name, each with the
// value when the declaration is about one.
func (w Word) Written() string {
	switch w.Kind {
	case KindModeFlag:
		if w.Value != "" {
			return "--" + w.Name + "=" + w.Value
		}

		return "--" + w.Name
	case KindOption:
		if w.Value != "" {
			return w.Name + " = " + w.Value
		}

		return w.Name
	case KindAction:
		return w.Name
	}

	return w.Name
}

// Declaration is a set of words with their platform column, in the order they
// are declared.
type Declaration []Word

// Everywhere declares words supported on every platform.
//
// Being supported everywhere is written down rather than assumed, for the
// reason a zero is written down in the config defaults: a column nobody wrote
// cannot be told from a forgotten one, and the forgotten one is exactly how
// `smooth_scroll` shipped.
func Everywhere(kind Kind, names ...string) Declaration {
	declaration := make(Declaration, 0, len(names))

	for _, name := range names {
		declaration = append(declaration, Word{
			Kind:      kind,
			Name:      name,
			Platforms: slices.Clone(AllPlatforms),
		})
	}

	return declaration
}

// On declares words supported only on the given platforms, with the note that
// says why.
func On(kind Kind, platforms Platforms, note string, names ...string) Declaration {
	declaration := make(Declaration, 0, len(names))

	for _, name := range names {
		declaration = append(declaration, Word{
			Kind:      kind,
			Name:      name,
			Platforms: slices.Clone(platforms),
			Note:      note,
		})
	}

	return declaration
}

// ValueOn declares one value of a word as supported only on the given
// platforms. The word itself is declared separately: it is recognized
// everywhere, and only this value of it is not.
func ValueOn(
	kind Kind,
	platforms Platforms,
	note, value string,
	names ...string,
) Declaration {
	declaration := On(kind, platforms, note, names...)
	for index := range declaration {
		declaration[index].Value = value
	}

	return declaration
}

// Join concatenates declarations, so a table can be written in sections
// without any section having to know the others exist.
func Join(declarations ...Declaration) Declaration {
	var joined Declaration

	for _, declaration := range declarations {
		joined = append(joined, declaration...)
	}

	return joined
}

// Lookup finds a word by kind, name and value. The bool is false for a word
// nothing declares, which is what the guardrails exist to prevent.
func (d Declaration) Lookup(kind Kind, name, value string) (Word, bool) {
	for _, word := range d {
		if word.Kind == kind && word.Name == name && word.Value == value {
			return word, true
		}
	}

	return Word{}, false
}

// Limited returns the words that are not supported everywhere, in declaration
// order. This is the published table: a row per promise Neru does not keep on
// every platform, and nothing for the vocabulary that carries no surprise.
func (d Declaration) Limited() Declaration {
	var limited Declaration

	for _, word := range d {
		if !word.Platforms.Everywhere() {
			limited = append(limited, word)
		}
	}

	return limited
}

// InertOn returns the words that do nothing on the given platform.
func (d Declaration) InertOn(target Platform) Declaration {
	var inert Declaration

	for _, word := range d {
		if !word.Platforms.Supports(target) {
			inert = append(inert, word)
		}
	}

	return inert
}

// Names returns the written form of every word, in declaration order, for a
// caller that reports a list to a person.
func (d Declaration) Names() []string {
	names := make([]string, 0, len(d))

	for _, word := range d {
		names = append(names, word.Written())
	}

	return names
}
