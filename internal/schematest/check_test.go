package schematest_test

import (
	"errors"
	"testing"

	"github.com/y3owk1n/neru/internal/schematest"
)

// The words the cases below repeat.
const (
	keyType    = "type"
	typeString = "string"
	keyName    = "name"
	keyTags    = "tags"
	validName  = "abc"
)

// TestCheck_RefusesAKeywordItDoesNotApply pins that a schema rule the check
// cannot apply fails the check, rather than passing every value as though the
// rule held.
func TestCheck_RefusesAKeywordItDoesNotApply(t *testing.T) {
	node := map[string]any{keyType: typeString, "minLength": float64(1)}

	err := schematest.Check(node, node, "", keyName)
	if !errors.Is(err, schematest.ErrUnknownKeyword) {
		t.Errorf("Check() error = %v, want %v", err, schematest.ErrUnknownKeyword)
	}
}

// TestCheck_AppliesTheKeywordsItKnows pins one value each way through the
// keywords the renderers emit, so the check is not passing everything.
func TestCheck_AppliesTheKeywordsItKnows(t *testing.T) {
	node := map[string]any{
		keyType:                "object",
		"required":             []any{keyName},
		"additionalProperties": false,
		"properties": map[string]any{
			keyName: map[string]any{keyType: typeString, "pattern": "^a"},
			keyTags: map[string]any{keyType: "array", "items": map[string]any{keyType: typeString}},
		},
	}

	tests := []struct {
		name  string
		value map[string]any
		valid bool
	}{
		{"matches", map[string]any{keyName: validName, keyTags: []any{"x"}}, true},
		{"missing a required key", map[string]any{keyTags: []any{}}, false},
		{"unknown key", map[string]any{keyName: validName, "other": true}, false},
		{"pattern not matched", map[string]any{keyName: "xyz"}, false},
		{"item of the wrong type", map[string]any{keyName: validName, keyTags: []any{1.0}}, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := schematest.Check(node, node, testCase.value, "")
			if (err == nil) != testCase.valid {
				t.Errorf(
					"Check(%v) error = %v, want valid = %t",
					testCase.value,
					err,
					testCase.valid,
				)
			}
		})
	}
}
