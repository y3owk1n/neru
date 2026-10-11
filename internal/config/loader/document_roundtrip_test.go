package loader_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/y3owk1n/neru/internal/config"
)

// TestLoadWithValidation_ReadsBackADumpedConfig pins that `neru config dump`
// is a config file in full. Written out as TOML and loaded the way the daemon
// loads one, it gives back the hotkey tables it came from. The typed decode
// skips those tables, so only the loader can show they survive.
func TestLoadWithValidation_ReadsBackADumpedConfig(t *testing.T) {
	original, _ := loadWithObservedLogger(t, `
[hotkeys]
"Primary+Shift+P" = ["hints", "idle"]

[hints.hotkeys]
"Shift+Q" = "idle"

[modes.win]
indicator = "W"

[modes.win.hotkeys]
"h" = "exec echo left"
`, "")
	if original.ValidationError != nil {
		t.Fatalf("the source config was refused: %v", original.ValidationError)
	}

	var dumped bytes.Buffer

	err := toml.NewEncoder(&dumped).Encode(config.Document(original.Config))
	if err != nil {
		t.Fatalf("the dump does not encode as TOML: %v", err)
	}

	reloaded, _ := loadWithObservedLogger(t, dumped.String(), "")
	if reloaded.ValidationError != nil {
		t.Fatalf(
			"the dump was refused as a config file: %v\n%s",
			reloaded.ValidationError,
			dumped.String(),
		)
	}

	tables := []struct {
		name      string
		got, want any
	}{
		{"[hotkeys]", reloaded.Config.Hotkeys.Bindings, original.Config.Hotkeys.Bindings},
		{"[hints.hotkeys]", reloaded.Config.Hints.Hotkeys, original.Config.Hints.Hotkeys},
		{
			"[modes.win.hotkeys]",
			reloaded.Config.Modes["win"].Hotkeys,
			original.Config.Modes["win"].Hotkeys,
		},
	}

	for _, table := range tables {
		if !reflect.DeepEqual(table.got, table.want) {
			t.Errorf("%s came back as %v, want %v", table.name, table.got, table.want)
		}
	}
}
