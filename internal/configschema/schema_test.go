package configschema_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/y3owk1n/neru/configs"
	"github.com/y3owk1n/neru/internal/configschema"
)

// TestRender_AcceptsEveryShippedExample pins that the schema an editor applies
// does not flag a config the project ships, which would teach a user to ignore
// its warnings.
func TestRender_AcceptsEveryShippedExample(t *testing.T) {
	schema := renderedSchema(t)

	for _, name := range configs.ShippedExamples {
		document := decodeTOMLFile(t, filepath.Join("..", "..", "configs", name))

		err := check(schema, schema, document, "")
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

			err := check(schema, schema, document, "")
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

			if check(schema, schema, document, "") == nil {
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

// errSchema is what check returns when a value breaks the schema.
var errSchema = errors.New("schema violation")

// check validates value against node, for the keywords Render emits. root
// resolves $ref.
func check(root, node map[string]any, value any, path string) error {
	if target, isRef := node["$ref"].(string); isRef {
		definitions, _ := root["definitions"].(map[string]any)
		resolved, _ := definitions[strings.TrimPrefix(target, "#/definitions/")].(map[string]any)

		return check(root, resolved, value, path)
	}

	if alternatives, isOneOf := node["oneOf"].([]any); isOneOf {
		return checkAlternatives(root, alternatives, value, path, true)
	}

	if alternatives, isAnyOf := node["anyOf"].([]any); isAnyOf {
		return checkAlternatives(root, alternatives, value, path, false)
	}

	switch node["type"] {
	case "object":
		return checkObject(root, node, value, path)
	case "array":
		items, isArray := asArray(value)
		if !isArray {
			return fmt.Errorf("%w: %s: want an array, got %T", errSchema, path, value)
		}

		itemSchema, _ := node["items"].(map[string]any)
		for index, item := range items {
			err := check(root, itemSchema, item, fmt.Sprintf("%s[%d]", path, index))
			if err != nil {
				return err
			}
		}

		return nil
	case "string":
		text, isString := value.(string)
		if !isString {
			return fmt.Errorf("%w: %s: want a string, got %T", errSchema, path, value)
		}

		if pattern, hasPattern := node["pattern"].(string); hasPattern &&
			!regexp.MustCompile(pattern).MatchString(text) {
			return fmt.Errorf("%w: %s: %q does not match %s", errSchema, path, text, pattern)
		}

		return nil
	case "boolean":
		if _, isBool := value.(bool); !isBool {
			return fmt.Errorf("%w: %s: want a boolean, got %T", errSchema, path, value)
		}

		return nil
	case "integer":
		if _, isInt := value.(int64); !isInt {
			return fmt.Errorf("%w: %s: want an integer, got %T", errSchema, path, value)
		}

		return nil
	case "number":
		switch value.(type) {
		case int64, float64:
			return nil
		default:
			return fmt.Errorf("%w: %s: want a number, got %T", errSchema, path, value)
		}
	default:
		return fmt.Errorf(
			"%w: %s: the schema has a node this check does not know: %v",
			errSchema,
			path,
			node,
		)
	}
}

func checkObject(root, node map[string]any, value any, path string) error {
	table, isTable := value.(map[string]any)
	if !isTable {
		return fmt.Errorf("%w: %s: want a table, got %T", errSchema, path, value)
	}

	properties, _ := node["properties"].(map[string]any)

	for key, child := range table {
		childPath := strings.TrimPrefix(path+"."+key, ".")

		if property, known := properties[key].(map[string]any); known {
			err := check(root, property, child, childPath)
			if err != nil {
				return err
			}

			continue
		}

		switch extra := node["additionalProperties"].(type) {
		case bool:
			if !extra {
				return fmt.Errorf("%w: %s: unknown key", errSchema, childPath)
			}
		case map[string]any:
			err := check(root, extra, child, childPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func checkAlternatives(
	root map[string]any,
	alternatives []any,
	value any,
	path string,
	exactlyOne bool,
) error {
	matched := 0

	for _, alternative := range alternatives {
		node, _ := alternative.(map[string]any)
		if check(root, node, value, path) == nil {
			matched++
		}
	}

	switch {
	case matched == 0:
		return fmt.Errorf("%w: %s: %v matches none of the allowed shapes", errSchema, path, value)
	case exactlyOne && matched > 1:
		return fmt.Errorf("%w: %s: %v matches more than one shape", errSchema, path, value)
	default:
		return nil
	}
}

// asArray reads a TOML array, which decodes as []any, or as []map[string]any
// for an array of tables.
func asArray(value any) ([]any, bool) {
	switch items := value.(type) {
	case []any:
		return items, true
	case []map[string]any:
		out := make([]any, len(items))
		for index, item := range items {
			out[index] = item
		}

		return out, true
	default:
		return nil, false
	}
}
