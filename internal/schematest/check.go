// Package schematest checks a decoded document against one of the JSON
// Schemas Neru publishes, for tests that pin what a schema accepts.
//
// It knows the keywords the schema renderers emit and fails on any other, so
// a renderer that starts emitting a new keyword cannot be checked by a test
// that silently skips it.
package schematest

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
)

var (
	// ErrViolation is what Check returns when a value breaks the schema.
	ErrViolation = errors.New("schema violation")
	// ErrUnknownKeyword is what Check returns for a schema it cannot apply in
	// full, rather than passing a value against the part it understands.
	ErrUnknownKeyword = errors.New("unknown schema keyword")
)

// knownKeywords are the keywords the schema renderers emit. The annotations
// constrain nothing, and Check applies every other one.
var knownKeywords = map[string]bool{
	"$schema": true, "$id": true, "title": true, "description": true, "definitions": true,
	"$ref": true, "oneOf": true, "anyOf": true, "type": true,
	"properties": true, "required": true, "additionalProperties": true,
	"items": true, "pattern": true,
}

// Check validates value against node. root resolves $ref, and path names the
// value in an error. A value decoded from TOML or from JSON both work.
func Check(root, node map[string]any, value any, path string) error {
	for keyword := range node {
		if !knownKeywords[keyword] {
			return fmt.Errorf("%w: %s: the schema uses %q, which this check does not apply",
				ErrUnknownKeyword, path, keyword)
		}
	}

	if target, isRef := node["$ref"].(string); isRef {
		definitions, _ := root["definitions"].(map[string]any)
		resolved, _ := definitions[strings.TrimPrefix(target, "#/definitions/")].(map[string]any)

		return Check(root, resolved, value, path)
	}

	if alternatives, isOneOf := node["oneOf"].([]any); isOneOf {
		return checkAlternatives(root, alternatives, value, path, true)
	}

	if alternatives, isAnyOf := node["anyOf"].([]any); isAnyOf {
		return checkAlternatives(root, alternatives, value, path, false)
	}

	kind, constrained := node["type"]
	if !constrained {
		return nil
	}

	switch kind {
	case "object":
		return checkObject(root, node, value, path)
	case "array":
		return checkArray(root, node, value, path)
	case "string":
		return checkString(node, value, path)
	case "boolean":
		if _, isBool := value.(bool); !isBool {
			return fmt.Errorf("%w: %s: want a boolean, got %T", ErrViolation, path, value)
		}
	case "integer":
		if !isInteger(value) {
			return fmt.Errorf("%w: %s: want an integer, got %v", ErrViolation, path, value)
		}
	case "number":
		switch value.(type) {
		case int64, float64:
		default:
			return fmt.Errorf("%w: %s: want a number, got %T", ErrViolation, path, value)
		}
	case "null":
		if value != nil {
			return fmt.Errorf("%w: %s: want null, got %v", ErrViolation, path, value)
		}
	default:
		return fmt.Errorf("%w: %s: a node this check does not know: %v", ErrViolation, path, node)
	}

	return nil
}

func checkObject(root, node map[string]any, value any, path string) error {
	table, isTable := value.(map[string]any)
	if !isTable {
		return fmt.Errorf("%w: %s: want an object, got %T", ErrViolation, path, value)
	}

	required, _ := node["required"].([]any)
	for _, name := range required {
		key, _ := name.(string)
		if _, present := table[key]; !present {
			return fmt.Errorf("%w: %s: missing %s", ErrViolation, path, key)
		}
	}

	properties, _ := node["properties"].(map[string]any)

	for key, child := range table {
		childPath := strings.TrimPrefix(path+"."+key, ".")

		if property, known := properties[key].(map[string]any); known {
			err := Check(root, property, child, childPath)
			if err != nil {
				return err
			}

			continue
		}

		switch extra := node["additionalProperties"].(type) {
		case bool:
			if !extra {
				return fmt.Errorf("%w: %s: unknown key", ErrViolation, childPath)
			}
		case map[string]any:
			err := Check(root, extra, child, childPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func checkArray(root, node map[string]any, value any, path string) error {
	items, isArray := asArray(value)
	if !isArray {
		return fmt.Errorf("%w: %s: want an array, got %T", ErrViolation, path, value)
	}

	itemSchema, _ := node["items"].(map[string]any)
	for index, item := range items {
		err := Check(root, itemSchema, item, fmt.Sprintf("%s[%d]", path, index))
		if err != nil {
			return err
		}
	}

	return nil
}

func checkString(node map[string]any, value any, path string) error {
	text, isString := value.(string)
	if !isString {
		return fmt.Errorf("%w: %s: want a string, got %T", ErrViolation, path, value)
	}

	pattern, hasPattern := node["pattern"].(string)
	if hasPattern && !regexp.MustCompile(pattern).MatchString(text) {
		return fmt.Errorf("%w: %s: %q does not match %s", ErrViolation, path, text, pattern)
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
		if Check(root, node, value, path) == nil {
			matched++
		}
	}

	switch {
	case matched == 0:
		return fmt.Errorf(
			"%w: %s: %v matches none of the allowed shapes",
			ErrViolation,
			path,
			value,
		)
	case exactlyOne && matched > 1:
		return fmt.Errorf("%w: %s: %v matches more than one shape", ErrViolation, path, value)
	default:
		return nil
	}
}

// isInteger accepts a TOML integer, or a JSON number with no fraction.
func isInteger(value any) bool {
	switch number := value.(type) {
	case int64:
		return true
	case float64:
		return number == math.Trunc(number)
	default:
		return false
	}
}

// asArray reads an array, which decodes as []any, or from TOML as
// []map[string]any for an array of tables.
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
