//go:build windows

package windows

import (
	"maps"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
)

// Unit tests for the pure UIA control-type naming used by hint enumeration.
// Does not exercise live UIA (see accessibility integration tests on WIN-VM).
func TestControlTypeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		controlType int32
		wantName    string
		wantKnown   bool
	}{
		{"button", 50000, uiaControlButton, true},
		{"checkbox", 50002, uiaControlCheckBox, true},
		{"combobox", 50003, "ComboBox", true},
		{"edit", 50004, uiaControlEdit, true},
		{"hyperlink", 50005, uiaControlHyperlink, true},
		{"menu item", 50011, "MenuItem", true},
		{"radio button", 50013, "RadioButton", true},
		{"tab item", 50019, "TabItem", true},
		{"split button", 50031, uiaControlSplitButton, true},
		// Control types that were previously discarded are now named, so a
		// config can address them through the uia: prefix.
		{"text", 50020, "Text", true},
		{"custom", 50025, uiaControlCustom, true},
		{"pane", 50033, uiaControlPane, true},
		{"document", 50030, "Document", true},
		{"last known control type", 50040, "AppBar", true},
		{"unknown control type", 99999, roleUnknown, false},
		{"zero control type", 0, roleUnknown, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			name, known := controlTypeName(testCase.controlType)
			if name != testCase.wantName || known != testCase.wantKnown {
				t.Fatalf(
					"controlTypeName(%d) = (%q, %v), want (%q, %v)",
					testCase.controlType,
					name,
					known,
					testCase.wantName,
					testCase.wantKnown,
				)
			}
		})
	}
}

// TestControlTypeNamesAreContiguous guards the transcription of the
// UIA_*ControlTypeId range. A gap or duplicate would mean an id was mistyped,
// which silently misnames a control type.
func TestControlTypeNamesAreContiguous(t *testing.T) {
	t.Parallel()

	const first, last = 50000, 50040

	seen := make(map[string]int32, len(controlTypeNames))

	for controlType := int32(first); controlType <= last; controlType++ {
		name, ok := controlTypeNames[controlType]
		if !ok {
			t.Errorf("controlTypeNames lacks id %d", controlType)

			continue
		}

		if previous, duplicate := seen[name]; duplicate {
			t.Errorf("controlTypeNames has %q for both %d and %d", name, previous, controlType)
		}

		seen[name] = controlType
	}

	if len(controlTypeNames) != last-first+1 {
		t.Errorf(
			"controlTypeNames has %d entries, want %d",
			len(controlTypeNames), last-first+1,
		)
	}
}

// TestControlTypeNamesCoverTheVocabulary pins enumeration against the semantic
// vocabulary: every UIA name a semantic role expands to must be a name the
// enumerator can actually produce, or that role would never match anything.
func TestControlTypeNamesCoverTheVocabulary(t *testing.T) {
	t.Parallel()

	names := slices.Collect(maps.Values(controlTypeNames))

	for _, mapping := range element.RoleVocabulary {
		for _, native := range mapping.UIA {
			if !slices.Contains(names, native) {
				t.Errorf(
					"semantic role %q expands to UIA control type %q, which the "+
						"enumerator never produces",
					mapping.Semantic, native,
				)
			}
		}
	}
}
