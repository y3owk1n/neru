package config_test

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// idleAction is the action these bindings carry; which action it is does not
// matter to any of them, only that the binding is well formed.
const idleAction = "idle"

// mouseButtonKeys are the buttons the macOS event tap reports, in the spelling a
// config file writes.
var mouseButtonKeys = []string{"MouseLeft", "MouseRight", "MouseMiddle"}

// TestValidateHotkey_AcceptsMouseButtons covers the syntax gate a mode binding
// passes through: a mouse button is a named key, so it is accepted bare. It is
// only ever emitted bare — the tap does not encode modifiers into the name — so
// bare is the shape that has to validate.
func TestValidateHotkey_AcceptsMouseButtons(t *testing.T) {
	t.Parallel()

	for _, key := range mouseButtonKeys {
		err := config.ValidateHotkey(key, testHotkeyFieldName)
		if err != nil {
			t.Errorf("ValidateHotkey(%q) = %v, want nil", key, err)
		}
	}
}

// TestValidateHotkey_RefusesModifiedMouseButtons pins the other half of "a
// click reports the button alone": a combo ending in a mouse button matches
// nothing the tap emits, so it is refused wherever a hotkey is written rather
// than sitting in the file looking bound. The global-table refusal below is a
// separate rule and does not cover this one — it reads the whole key, which a
// combo is not.
func TestValidateHotkey_RefusesModifiedMouseButtons(t *testing.T) {
	t.Parallel()

	combos := []string{
		"Cmd+MouseLeft",
		"Shift+MouseRight",
		"Primary+Alt+MouseMiddle",
		"Cmd+mouseleft",
	}

	for _, combo := range combos {
		err := config.ValidateHotkey(combo, testHotkeyFieldName)
		if err == nil {
			t.Errorf("ValidateHotkey(%q) = nil, want refusal", combo)

			continue
		}

		if !strings.Contains(err.Error(), "mouse button") {
			t.Errorf("ValidateHotkey(%q) error %q is not the mouse-button refusal", combo, err)
		}
	}
}

// TestValidateHotkeys_ModeTablesRefuseModifiedMouseButtons checks the refusal
// reaches a mode table, which is the one place a bare button is legal — so it
// is the place a modified one could most plausibly have slipped through.
func TestValidateHotkeys_ModeTablesRefuseModifiedMouseButtons(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.RecursiveGrid.Hotkeys["Cmd+MouseLeft"] = config.StringOrStringArray{idleAction}

	err := cfg.ValidateHotkeys()
	if err == nil {
		t.Error("recursive_grid.hotkeys.Cmd+MouseLeft validated, want refusal")
	}
}

// TestValidateHotkeys_ModeTablesAcceptMouseButtons is the binding path a user
// actually writes: a mouse button bound in a mode's table loads.
func TestValidateHotkeys_ModeTablesAcceptMouseButtons(t *testing.T) {
	t.Parallel()

	for _, key := range mouseButtonKeys {
		cfg := config.DefaultConfig()
		cfg.RecursiveGrid.Hotkeys[key] = config.StringOrStringArray{idleAction}

		err := cfg.ValidateHotkeys()
		if err != nil {
			t.Errorf("recursive_grid.hotkeys.%s = %v, want nil", key, err)
		}
	}
}

// TestValidate_GlobalHotkeysRefuseMouseButtons pins the refusal. A global hotkey
// is resolved to a virtual key code and a mouse button has none, so accepting
// one would produce a binding that parses and never fires. The refusal also
// keeps the behavior these spellings already had before the vocabulary carried
// them, so no configuration that loads today stops loading.
func TestValidate_GlobalHotkeysRefuseMouseButtons(t *testing.T) {
	t.Parallel()

	for _, key := range mouseButtonKeys {
		cfg := config.DefaultConfig()
		if cfg.Hotkeys.Bindings == nil {
			cfg.Hotkeys.Bindings = map[string][]string{}
		}

		cfg.Hotkeys.Bindings[key] = []string{idleAction}

		err := cfg.Validate()
		if err == nil {
			t.Errorf("hotkeys.%s validated, want refusal", key)

			continue
		}

		// Pin the reason, so the test cannot pass on some unrelated refusal,
		// and the redirection, because a refusal that does not say where the
		// binding does belong leaves the reader nowhere to go.
		if !strings.Contains(err.Error(), "mouse button") {
			t.Errorf("hotkeys.%s error %q is not the mouse-button refusal", key, err)
		}

		if !strings.Contains(err.Error(), "[<mode>.hotkeys]") {
			t.Errorf("hotkeys.%s error %q does not say where to bind it instead", key, err)
		}
	}
}
