package configschema_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/y3owk1n/neru/configs"
	"github.com/y3owk1n/neru/internal/configschema"
	"github.com/y3owk1n/neru/internal/schematest"
)

// TestRender_AcceptsEveryShippedExample pins that the schema an editor applies
// does not flag a config the project ships, which would teach a user to ignore
// its warnings.
func TestRender_AcceptsEveryShippedExample(t *testing.T) {
	schema := renderedSchema(t)

	for _, name := range configs.ShippedExamples {
		document := decodeTOMLFile(t, filepath.Join("..", "..", "configs", name))

		err := schematest.Check(schema, schema, document, "")
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestRender_AcceptsEveryShapeTheDecoderTakes pins the shapes no shipped
// example writes, so the schema cannot start refusing one unnoticed.
func TestRender_AcceptsEveryShapeTheDecoderTakes(t *testing.T) {
	schema := renderedSchema(t)

	tests := []struct {
		name     string
		document string
	}{
		{"binding of several steps", "[hotkeys]\n\"Primary+P\" = [\"hints\", \"idle\"]"},
		{
			"color per theme",
			"[mode_indicator.ui]\ntext_color = { light = \"#fff\", dark = \"#000000aa\" }",
		},
		{"unset color", "[mode_indicator.hints]\ntext_color = \"\""},
		{"solid palette color", "[theme.dark]\nsurface = \"#112233\""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			document := decodeTOML(t, testCase.document)

			err := schematest.Check(schema, schema, document, "")
			if err != nil {
				t.Errorf("the schema refused %q: %v", testCase.document, err)
			}
		})
	}
}

// TestRender_FlagsWhatTheDecoderWouldDrop pins the mistakes the schema exists
// to catch in an editor. The daemon ignores an unknown key, and refuses the
// others only when it loads the file.
func TestRender_FlagsWhatTheDecoderWouldDrop(t *testing.T) {
	schema := renderedSchema(t)

	tests := []struct {
		name     string
		document string
	}{
		{"unknown top-level key", "bogus_key = 1"},
		{"misspelled key", "[hints]\nenabeld = true"},
		{"wrong type", "[hints]\nenabled = \"yes\""},
		{"not a color", "[mode_indicator.ui]\ntext_color = \"red\""},
		{"unknown color theme", "[mode_indicator.ui]\ntext_color = { dim = \"#fff\" }"},
		{"step that is not a string", "[hotkeys]\n\"Primary+P\" = 1"},
		{"list holding a non-string step", "[hotkeys]\n\"Primary+P\" = [\"hints\", 1]"},
		{"named palette color", "[theme.light]\nsurface = \"red\""},
		{"palette color with alpha", "[theme.light]\nsurface = \"#FF0000AA\""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			document := decodeTOML(t, testCase.document)

			if schematest.Check(schema, schema, document, "") == nil {
				t.Errorf("the schema accepted %q", testCase.document)
			}
		})
	}
}

func renderedSchema(t *testing.T) map[string]any {
	t.Helper()

	out, err := configschema.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var schema map[string]any

	err = json.Unmarshal(out, &schema)
	if err != nil {
		t.Fatalf("the schema is not JSON: %v", err)
	}

	return schema
}

func decodeTOML(t *testing.T, text string) map[string]any {
	t.Helper()

	var document map[string]any

	_, err := toml.Decode(text, &document)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	return document
}

func decodeTOMLFile(t *testing.T, path string) map[string]any {
	t.Helper()

	var document map[string]any

	_, err := toml.DecodeFile(path, &document)
	if err != nil {
		t.Fatalf("DecodeFile(%s) error = %v", path, err)
	}

	return document
}
