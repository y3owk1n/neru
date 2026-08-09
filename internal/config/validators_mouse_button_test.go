package config_test

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

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

// TestValidateHotkeys_ModeTablesAcceptMouseButtons is the binding path a user
// actually writes: a mouse button bound in a mode's table loads.
func TestValidateHotkeys_ModeTablesAcceptMouseButtons(t *testing.T) {
	t.Parallel()

	for _, key := range mouseButtonKeys {
		cfg := config.DefaultConfig()
		cfg.RecursiveGrid.Hotkeys[key] = config.StringOrStringArray{"idle"}

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

		cfg.Hotkeys.Bindings[key] = []string{"idle"}

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
