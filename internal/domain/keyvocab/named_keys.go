package keyvocab

import (
	"slices"
	"strconv"
	"strings"
)

// The named keys, in the display form a config file writes and the event taps
// emit. A key that is neither one of these nor a single character is not a key
// Neru binds.
const (
	KeySpace     = "Space"
	KeyReturn    = "Return"
	KeyEnter     = "Enter" // means Return
	KeyEscape    = "Escape"
	KeyTab       = "Tab"
	KeyDelete    = "Delete"
	KeyBackspace = "Backspace" // means Delete
	KeyUp        = "Up"
	KeyDown      = "Down"
	KeyLeft      = "Left"
	KeyRight     = "Right"
	KeyHome      = "Home"
	KeyEnd       = "End"
	KeyPageUp    = "PageUp"
	KeyPageDown  = "PageDown"
	KeyInsert    = "Insert"
)

// shorthandEscape is the one spelling that resolves to a named key without
// being one. It is deliberately absent from namedKeys: a keystroke arriving as
// "esc" compares equal to Escape, while a binding written "esc" is refused, so
// a config file has exactly one spelling for the key.
const shorthandEscape = "esc"

// functionKeyCount closes the function key range. F21-F24 have no macOS virtual
// keycode (Carbon stops at F20), so they reach a binding only on Linux and
// Windows. They stay in the shared set so one config file can be shared across
// platforms without failing validation.
const functionKeyCount = 24

// CapsLock is deliberately not here: a modifier-shaped key with per-platform
// semantics nobody has asked for (docs/adr/0008-a-vocabulary-has-one-home.md).
var namedKeys = buildNamedKeys()

// namedKeyAliases maps a spelling to the named key it means. Enter and
// Backspace are named keys in their own right — a config file may write either,
// and a diagnostic echoes back what was written — that mean Return and Delete.
var namedKeyAliases = map[string]string{
	KeyEnter:        KeyReturn,
	KeyBackspace:    KeyDelete,
	shorthandEscape: KeyEscape,
}

// namedKeyDisplay indexes the set by lowercase spelling, and aliasTargets does
// the same for the aliases. Both are derived from the declarations above, so a
// key is added in exactly one place.
var (
	namedKeyDisplay = lowercaseIndex(namedKeys)
	aliasTargets    = lowercaseAliases(namedKeyAliases)
)

func buildNamedKeys() []string {
	keys := []string{
		KeySpace,
		KeyReturn,
		KeyEnter,
		KeyEscape,
		KeyTab,
		KeyDelete,
		KeyBackspace,
		KeyUp,
		KeyDown,
		KeyLeft,
		KeyRight,
		KeyHome,
		KeyEnd,
		KeyPageUp,
		KeyPageDown,
		KeyInsert,
	}

	for index := 1; index <= functionKeyCount; index++ {
		keys = append(keys, "F"+strconv.Itoa(index))
	}

	return keys
}

func lowercaseIndex(keys []string) map[string]string {
	index := make(map[string]string, len(keys))
	for _, key := range keys {
		index[strings.ToLower(key)] = key
	}

	return index
}

func lowercaseAliases(aliases map[string]string) map[string]string {
	index := make(map[string]string, len(aliases))
	for alias, means := range aliases {
		index[strings.ToLower(alias)] = means
	}

	return index
}

// IsNamedKey reports whether name is one of the named keys, case-insensitively.
// A single character key ("j") is not a named key, and neither is a modifier
// combo — callers that accept those check for them separately.
func IsNamedKey(name string) bool {
	_, ok := namedKeyDisplay[strings.ToLower(name)]

	return ok
}

// NamedKeyDisplay returns the display spelling of a named key ("pagedown" →
// "PageDown", "f1" → "F1"). An alias keeps its own spelling — "enter" is
// "Enter", not "Return" — because this answers how to write a key, not which
// key it means; ResolveAlias answers that. A name that is not a named key comes
// back unchanged with ok false.
func NamedKeyDisplay(name string) (string, bool) {
	display, ok := namedKeyDisplay[strings.ToLower(name)]
	if !ok {
		return name, false
	}

	return display, true
}

// ResolveAlias returns the named key an alias means, in display form ("enter" →
// "Return", "backspace" → "Delete", "esc" → "Escape"). ok is false for anything
// that is not an alias, including a named key that means itself.
func ResolveAlias(name string) (string, bool) {
	means, ok := aliasTargets[strings.ToLower(name)]

	return means, ok
}

// NamedKeys returns every named key in display form, sorted. It exists so the
// set can be checked against a written-out list rather than sampled: the pin in
// this package's tests reads it, which is what makes an entry added here fail
// loudly instead of quietly widening the vocabulary.
func NamedKeys() []string {
	keys := slices.Clone(namedKeys)
	slices.Sort(keys)

	return keys
}
