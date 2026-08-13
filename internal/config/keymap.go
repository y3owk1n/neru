package config

import (
	"maps"
	"slices"
)

// Keymap is the bindings in force: the ones a mode answers to, with the
// focused app's overrides already applied. It is the single answer to "what is
// bound right now", handed to whoever needs it rather than merged again by
// each of them.
//
// A keymap is settled when the mode, the focused app or the configuration
// changes — never on a keystroke, which only ever consults one. The decision,
// and why the merge is not memoized deeper down instead, is ADR 0005.
//
// The zero Keymap is an empty one: nothing is bound and every lookup misses,
// so a holder that has not settled one yet needs no nil check.
type Keymap struct {
	// byKey indexes every binding under the normalized form of its key, so
	// matching a keystroke is one map lookup rather than a normalize-and-
	// compare over the whole table.
	byKey map[string]Binding
	// sequenceStarts holds the first character of every binding that a
	// two-keystroke sequence can complete. Which bindings those are is a
	// property of the configuration, so it is decided here rather than
	// re-derived on the keystroke that might start one.
	sequenceStarts map[string]struct{}
}

// Binding is one entry of a Keymap: the steps a key runs, under the spelling
// the key was written in.
//
// Matching uses the normalized form, but nothing downstream depends on it: the
// name flows to the log line naming what matched and to the source a sequence
// runs under, both of which are better off saying what the user wrote.
type Binding struct {
	// Key is the key as it is written in the configuration ("Cmd+Return").
	Key string
	// Steps is the ordered sequence the binding runs.
	Steps []string
	// named records that Key is a named key ("Up", "F1"). Those are excluded
	// from sequence completion, so that typing "u" then "p" cannot complete
	// the binding for Up, whose normalized form is also "up".
	named bool
}

// ResolveKeymap resolves the bindings in force for a mode with an application
// focused: the mode's base bindings, with that app's [[<mode>.app_configs]]
// hotkey overrides applied on top. An empty focusedApp, an app with no entry,
// or an entry with no hotkeys all leave the base bindings alone.
//
// The merge rules live here because they are configuration semantics —
// precedence, the disabled sentinel, and matching a key written one way
// against the same key written another. The caller holds the result.
func (c *Config) ResolveKeymap(modeName, focusedApp string) Keymap {
	return newKeymap(c.HotkeysForModeAndApp(modeName, focusedApp))
}

// ResolveGlobalKeymap resolves the global [hotkeys] bindings in force with an
// application focused: the global table with that app's [[app_configs]] hotkey
// overrides applied on top, under the same rules ResolveKeymap applies to a
// mode's own table.
//
// It exists because the global table is read in two places that have to agree
// about what it says. The platform hotkey backend registers from it, and a mode
// falls back to it for a chord the mode itself does not bind — which is the only
// thing that can run a global binding where the in-mode capture is exclusive
// (internal/app/modes/keymap.go).
func (c *Config) ResolveGlobalKeymap(focusedApp string) Keymap {
	bindings := c.GlobalHotkeysForApp(focusedApp)
	if len(bindings) == 0 {
		return Keymap{}
	}

	// GlobalHotkeysForApp answers in the plain-slice shape the hotkey backend
	// registers from; newKeymap indexes the table shape the mode tables use.
	table := make(map[string]StringOrStringArray, len(bindings))
	for key, steps := range bindings {
		table[key] = StringOrStringArray(steps)
	}

	return newKeymap(table)
}

// ModifierChords returns the bindings whose key carries a Ctrl, Alt or Cmd
// modifier, and drops the rest.
//
// It is the line between a shortcut and typing, drawn where
// HasPassthroughModifier already draws it: a bare key or a Shift-only combo is
// something a mode reads as input — a hint label, a grid cell key, a navigation
// key — and a table that may shadow a mode's own keys must not offer one.
//
// The result binds no sequence starts. A two-keystroke sequence is spelled as a
// pair of bare letters, so nothing that survives this filter could begin one.
func (k Keymap) ModifierChords() Keymap {
	if len(k.byKey) == 0 {
		return Keymap{}
	}

	// The map key is already normalized, so "Primary+G" is tested in the form
	// the platform resolved it to rather than the alias it was written as.
	byKey := make(map[string]Binding, len(k.byKey))

	for normalized, binding := range k.byKey {
		if !HasPassthroughModifier(normalized) {
			continue
		}

		byKey[normalized] = binding
	}

	if len(byKey) == 0 {
		return Keymap{}
	}

	return Keymap{byKey: byKey}
}

// newKeymap indexes a merged hotkey table by normalized key.
//
// Two spellings of the same key are one binding, and the table they come from
// is a map, so the keys are indexed in a stable order and the first one wins.
// Which of the two survives matters less than that the same configuration
// always resolves to the same keymap.
func newKeymap(hotkeys map[string]StringOrStringArray) Keymap {
	if len(hotkeys) == 0 {
		return Keymap{}
	}

	keymap := Keymap{
		byKey:          make(map[string]Binding, len(hotkeys)),
		sequenceStarts: make(map[string]struct{}),
	}

	for _, key := range slices.Sorted(maps.Keys(hotkeys)) {
		normalized := NormalizeKeyForComparison(key)
		if _, taken := keymap.byKey[normalized]; taken {
			continue
		}

		binding := Binding{
			Key:   key,
			Steps: slices.Clone([]string(hotkeys[key])),
			named: IsValidNamedKey(key),
		}
		keymap.byKey[normalized] = binding

		// Only a genuine two-letter sequence can be started, never a named key
		// that happens to normalize to two letters.
		if !binding.named && len(normalized) == 2 && IsAllLetters(normalized) {
			keymap.sequenceStarts[normalized[:1]] = struct{}{}
		}
	}

	return keymap
}

// Len reports how many keys the keymap binds.
func (k Keymap) Len() int {
	return len(k.byKey)
}

// Lookup returns the binding for a normalized key, and false when the keymap
// binds it to nothing. The caller normalizes with NormalizeKeyForComparison,
// because a keystroke is normalized once and then asked about more than once.
func (k Keymap) Lookup(normalizedKey string) (Binding, bool) {
	binding, ok := k.byKey[normalizedKey]

	return binding, ok
}

// LookupSequence is Lookup for the concatenation of two keystrokes. It skips
// named keys: "u" then "p" is not the Up key, however it normalizes.
func (k Keymap) LookupSequence(normalizedKey string) (Binding, bool) {
	binding, ok := k.Lookup(normalizedKey)
	if !ok || binding.named {
		return Binding{}, false
	}

	return binding, true
}

// IsSequenceStart reports whether a normalized keystroke could be the first
// half of a two-letter sequence this keymap binds.
func (k Keymap) IsSequenceStart(normalizedKey string) bool {
	if len(normalizedKey) != 1 {
		return false
	}

	_, ok := k.sequenceStarts[normalizedKey]

	return ok
}

// Keys returns every key the keymap binds, in the spelling it was written in
// and in a stable order. It is what the modifier-passthrough lists are built
// from: the event tap has to be told which keys the mode answers to, in the
// form the platform canonicalizes.
func (k Keymap) Keys() []string {
	if len(k.byKey) == 0 {
		return nil
	}

	keys := make([]string, 0, len(k.byKey))
	for _, binding := range k.byKey {
		keys = append(keys, binding.Key)
	}

	slices.Sort(keys)

	return keys
}
