package config_test

// The keymap is the bindings in force for one mode with one focused app's
// overrides applied, and these are the merge rules stated as behavior: what an
// override replaces, what the disabled sentinel removes, and what an empty
// focused app leaves alone.
//
// They are the cases that used to be written against the per-mode-and-app
// hotkey lookup, moved here because resolving a keymap is now the call that
// answers "what is bound right now" and the one the mode handler consults.

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

const (
	keymapTestApp   = "com.apple.Safari"
	keymapOtherApp  = "com.example.other"
	leftClickStep   = "action left_click"
	rightClickStep  = "action right_click"
	scrollDownSteps = "action scroll_down"
	keymapCmdL      = "Cmd+L"
)

// hintsKeymapConfig is a hints configuration with base bindings and one
// application's overrides on top of them.
func hintsKeymapConfig(t *testing.T) *config.Config {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Hints.Hotkeys = map[string]config.StringOrStringArray{
		testKeyReturn: {leftClickStep, config.ModeNameHints},
		"g":           {leftClickStep},
	}
	cfg.Hints.AppConfigs = []config.AppConfig{
		{
			BundleID: keymapTestApp,
			Hotkeys: map[string]config.StringOrStringArray{
				testKeyReturn: {rightClickStep},
				"g":           {config.DisabledSentinel},
				"x":           {rightClickStep},
			},
		},
	}

	return cfg
}

func TestConfig_ResolveKeymap_AppliesFocusedAppOverrides(t *testing.T) {
	t.Parallel()

	keymap := hintsKeymapConfig(t).ResolveKeymap(config.ModeNameHints, keymapTestApp)

	binding, bound := keymap.Lookup(config.NormalizeKeyForComparison(testKeyReturn))
	if !bound {
		t.Fatal("Return is not bound; the focused app's override replaced nothing")
	}

	if len(binding.Steps) != 1 || binding.Steps[0] != rightClickStep {
		t.Errorf("Return runs %v, want the focused app's override %v",
			binding.Steps, []string{rightClickStep})
	}

	if _, bound := keymap.Lookup(config.NormalizeKeyForComparison("g")); bound {
		t.Error("g is still bound; the disabled sentinel removed no inherited binding")
	}

	appOnly, found := keymap.Lookup(config.NormalizeKeyForComparison("x"))
	if !found {
		t.Fatal("x is not bound; the focused app's own binding was dropped")
	}

	if len(appOnly.Steps) != 1 || appOnly.Steps[0] != rightClickStep {
		t.Errorf("x runs %v, want %v", appOnly.Steps, []string{rightClickStep})
	}
}

func TestConfig_ResolveKeymap_UnknownAppKeepsBaseBindings(t *testing.T) {
	t.Parallel()

	cfg := hintsKeymapConfig(t)

	for _, focusedApp := range []string{"", keymapOtherApp} {
		keymap := cfg.ResolveKeymap(config.ModeNameHints, focusedApp)

		binding, bound := keymap.Lookup(config.NormalizeKeyForComparison(testKeyReturn))
		if !bound {
			t.Fatalf("focused app %q: Return is not bound", focusedApp)
		}

		if len(binding.Steps) != 2 || binding.Steps[0] != leftClickStep {
			t.Errorf("focused app %q: Return runs %v, want the base binding",
				focusedApp, binding.Steps)
		}

		if _, bound := keymap.Lookup(config.NormalizeKeyForComparison("g")); !bound {
			t.Errorf("focused app %q: g is unbound; another app's sentinel removed it",
				focusedApp)
		}
	}
}

// A keymap resolved for one application must not leave that application's
// overrides behind in the configuration, or the next mode to be entered
// somewhere else inherits them.
func TestConfig_ResolveKeymap_LeavesTheConfigurationAlone(t *testing.T) {
	t.Parallel()

	cfg := hintsKeymapConfig(t)

	cfg.ResolveKeymap(config.ModeNameHints, keymapTestApp)

	base := cfg.Hints.Hotkeys[testKeyReturn]
	if len(base) != 2 || base[0] != leftClickStep {
		t.Errorf("configured Return binding is now %v; resolving a keymap mutated it", base)
	}
}

// Configuration is written by hand, so the same key reaches a keymap in
// whatever spelling its writer preferred. Matching is on the normalized form,
// which is what makes an override in one spelling replace a base binding in
// another.
func TestConfig_ResolveKeymap_MatchesDifferentlySpelledKeys(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Scroll.Hotkeys = map[string]config.StringOrStringArray{
		testKeyReturn: {leftClickStep},
	}
	cfg.Scroll.AppConfigs = []config.AppConfig{
		{
			BundleID: keymapTestApp,
			Hotkeys: map[string]config.StringOrStringArray{
				"enter": {scrollDownSteps},
			},
		},
	}

	keymap := cfg.ResolveKeymap(config.ModeNameScroll, keymapTestApp)

	if got := keymap.Len(); got != 1 {
		t.Fatalf("keymap holds %d bindings, want 1: two spellings of one key are one binding",
			got)
	}

	binding, bound := keymap.Lookup(config.NormalizeKeyForComparison(testKeyReturn))
	if !bound {
		t.Fatal("Return is not bound")
	}

	if len(binding.Steps) != 1 || binding.Steps[0] != scrollDownSteps {
		t.Errorf("Return runs %v, want the override written as \"enter\"", binding.Steps)
	}

	// What the user wrote is what a log line and a running sequence are named
	// after, so the spelling survives the normalized index.
	if binding.Key != "enter" {
		t.Errorf("binding is named %q, want the spelling it was written in", binding.Key)
	}
}

// Two-letter sequences complete against the concatenation of what was typed,
// and a named key whose normalized form happens to be two letters ("Up") is not
// one of them.
func TestKeymap_SequenceBindings(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Grid.Hotkeys = map[string]config.StringOrStringArray{
		"gg": {leftClickStep},
		"Up": {rightClickStep},
	}

	keymap := cfg.ResolveKeymap(config.ModeNameGrid, "")

	if !keymap.IsSequenceStart("g") {
		t.Error("g does not start a sequence, but gg is bound")
	}

	if keymap.IsSequenceStart("u") {
		t.Error("u starts a sequence; the named key Up was mistaken for one")
	}

	if _, ok := keymap.LookupSequence("gg"); !ok {
		t.Error("gg does not complete a sequence")
	}

	if _, ok := keymap.LookupSequence("up"); ok {
		t.Error("typing u then p completed the named key Up as a sequence")
	}

	if _, ok := keymap.Lookup("up"); !ok {
		t.Error("the named key Up is not bound")
	}
}

// A mode with no bindings at all resolves to an empty keymap rather than to
// nothing a caller has to nil-check, and the zero keymap answers the same way.
func TestKeymap_EmptyIsUsable(t *testing.T) {
	t.Parallel()

	keymaps := map[string]config.Keymap{
		"resolved for a mode with no bindings": (&config.Config{}).
			ResolveKeymap(config.ModeNameGrid, keymapTestApp),
		"zero value": {},
	}

	for name, keymap := range keymaps {
		if got := keymap.Len(); got != 0 {
			t.Errorf("%s: holds %d bindings, want 0", name, got)
		}

		if _, ok := keymap.Lookup("g"); ok {
			t.Errorf("%s: g is bound", name)
		}

		if _, ok := keymap.LookupSequence("gg"); ok {
			t.Errorf("%s: gg completes a sequence", name)
		}

		if keymap.IsSequenceStart("g") {
			t.Errorf("%s: g starts a sequence", name)
		}

		if keys := keymap.Keys(); len(keys) != 0 {
			t.Errorf("%s: keys are %v, want none", name, keys)
		}
	}
}

// Keys is what the modifier-passthrough lists are built from: every key the
// keymap answers to, in the spelling it was written in.
func TestKeymap_KeysAreTheSpellingsWritten(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Grid.Hotkeys = map[string]config.StringOrStringArray{
		keymapCmdL: {leftClickStep},
		"g":        {rightClickStep},
	}

	keys := cfg.ResolveKeymap(config.ModeNameGrid, "").Keys()

	want := []string{keymapCmdL, "g"}
	if len(keys) != len(want) {
		t.Fatalf("keys are %v, want %v", keys, want)
	}

	for idx, key := range want {
		if keys[idx] != key {
			t.Errorf("keys are %v, want %v in that order", keys, want)

			break
		}
	}
}

// The global keymap is the other table a keystroke can resolve against: the
// mode handler falls back to it for a chord the active mode does not bind, so it
// has to answer the same merge rules the mode tables do.
func TestConfig_ResolveGlobalKeymap_AppliesFocusedAppOverrides(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hotkeys.Bindings = map[string][]string{
		keymapCmdL: {leftClickStep},
		"Cmd+G":    {config.ModeNameGrid},
	}
	cfg.AppConfigs = []config.AppConfig{
		{
			BundleID: keymapTestApp,
			Hotkeys: map[string]config.StringOrStringArray{
				keymapCmdL: {rightClickStep},
				"Cmd+G":    {config.DisabledSentinel},
			},
		},
	}

	keymap := cfg.ResolveGlobalKeymap(keymapTestApp)

	binding, bound := keymap.Lookup(config.NormalizeKeyForComparison(keymapCmdL))
	if !bound {
		t.Fatal("Cmd+L is not bound; the focused app's override replaced nothing")
	}

	if len(binding.Steps) != 1 || binding.Steps[0] != rightClickStep {
		t.Errorf("Cmd+L runs %v, want the focused app's override %v",
			binding.Steps, []string{rightClickStep})
	}

	if _, bound := keymap.Lookup(config.NormalizeKeyForComparison("Cmd+G")); bound {
		t.Error("Cmd+G is still bound; the disabled sentinel removed no global binding")
	}

	// The base table stands for an application with no entry of its own.
	base := cfg.ResolveGlobalKeymap(keymapOtherApp)

	baseBinding, bound := base.Lookup(config.NormalizeKeyForComparison(keymapCmdL))
	if !bound || len(baseBinding.Steps) != 1 || baseBinding.Steps[0] != leftClickStep {
		t.Errorf("Cmd+L runs %v for an app with no overrides, want the base %v",
			baseBinding.Steps, []string{leftClickStep})
	}
}

// ModifierChords is what keeps the global table from shadowing what a mode reads
// as input: only a chord carrying Ctrl, Alt or Cmd survives it.
func TestKeymap_ModifierChordsKeepsOnlyShortcuts(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hotkeys.Bindings = map[string][]string{
		keymapCmdL:  {leftClickStep},
		"Alt+J":     {scrollDownSteps},
		"Primary+K": {scrollDownSteps},
		"Shift+M":   {leftClickStep},
		"g":         {leftClickStep},
		"gg":        {leftClickStep},
	}

	chords := cfg.ResolveGlobalKeymap("").ModifierChords()

	for _, key := range []string{keymapCmdL, "Alt+J", "Primary+K"} {
		if _, bound := chords.Lookup(config.NormalizeKeyForComparison(key)); !bound {
			t.Errorf("%s is not bound; a modifier chord was dropped", key)
		}
	}

	// Shift-only combos and bare keys are a mode's own input, so they may not
	// reach a table consulted while a mode is open.
	for _, key := range []string{"Shift+M", "g", "gg"} {
		if _, bound := chords.Lookup(config.NormalizeKeyForComparison(key)); bound {
			t.Errorf("%s is bound; it is input a mode reads, not a shortcut", key)
		}
	}

	// A sequence cannot be spelled with a modifier, so nothing here starts one.
	if chords.IsSequenceStart("g") {
		t.Error("g starts a sequence in the modifier chords")
	}
}
