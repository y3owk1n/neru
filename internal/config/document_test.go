package config_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/configschema"
	"github.com/y3owk1n/neru/internal/schematest"
)

// sharedBorder is a color both themes use.
const sharedBorder = "#222222"

// TestDocument_PassesTheConfigSchema pins `neru config dump` to the schema
// editors check a config file against, so the dump is a config file in the
// published sense too.
func TestDocument_PassesTheConfigSchema(t *testing.T) {
	scrollStep := 7

	cfg := config.DefaultConfig()
	cfg.ModeIndicator.UI.TextColor = config.Color{Light: testUserLight, Dark: testUserDark}
	cfg.AppConfigs = []config.AppConfig{{BundleID: "com.apple.Safari", ScrollStep: &scrollStep}}

	rendered, err := configschema.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var schema map[string]any

	err = json.Unmarshal(rendered, &schema)
	if err != nil {
		t.Fatalf("the config schema is not JSON: %v", err)
	}

	encoded, err := json.Marshal(config.Document(cfg))
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var document any

	err = json.Unmarshal(encoded, &document)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	err = schematest.Check(schema, schema, document, "")
	if err != nil {
		t.Errorf("the dump breaks the config schema: %v", err)
	}
}

// TestDocument_WritesWhatTheFileWrites pins the shapes a dump shares with
// config.toml: its key names, a color per theme only when the themes differ,
// a hotkey table of bindings, and no entry for an optional value left unset.
func TestDocument_WritesWhatTheFileWrites(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ModeIndicator.UI.TextColor = config.Color{Light: testUserLight, Dark: testUserDark}
	cfg.ModeIndicator.UI.BorderColor = config.Color{Light: sharedBorder, Dark: sharedBorder}
	cfg.Hotkeys.Bindings = map[string][]string{"Primary+Shift+P": {config.ModeNameHints}}
	cfg.AppConfigs = []config.AppConfig{{BundleID: "com.apple.Safari"}}

	document := config.Document(cfg)

	tests := []struct {
		name string
		path []string
		want any
	}{
		{
			"color per theme",
			[]string{"mode_indicator", "ui", "text_color"},
			map[string]any{"light": testUserLight, "dark": testUserDark},
		},
		{
			"color shared by both themes",
			[]string{"mode_indicator", "ui", "border_color"},
			sharedBorder,
		},
		{
			"global hotkey table",
			[]string{"hotkeys", "Primary+Shift+P"},
			[]any{config.ModeNameHints},
		},
		{
			"key under its TOML name",
			[]string{"recursive_grid", "grid_cols"},
			cfg.RecursiveGrid.GridCols,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := lookup(t, document, testCase.path)
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("%v = %#v, want %#v", testCase.path, got, testCase.want)
			}
		})
	}

	apps, _ := document["app_configs"].([]any)
	if len(apps) != 1 {
		t.Fatalf("app_configs = %#v, want one entry", document["app_configs"])
	}

	app, _ := apps[0].(map[string]any)
	if _, present := app["scroll_step"]; present {
		t.Errorf("an unset scroll_step was dumped as %#v, want it left out", app["scroll_step"])
	}
}

// lookup follows path through nested tables.
func lookup(t *testing.T, document map[string]any, path []string) any {
	t.Helper()

	var current any = document

	for _, key := range path {
		table, isTable := current.(map[string]any)
		if !isTable {
			t.Fatalf("%v: %v is not a table", path, current)
		}

		current = table[key]
	}

	return current
}
