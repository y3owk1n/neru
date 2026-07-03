//go:build windows

// internal/core/infra/accessibility/uia_windows_test.go
// Unit tests for the pure UIA control-type mapping used by hint enumeration.
// Does not exercise live UIA (see accessibility integration tests on WIN-VM).

package accessibility //nolint:testpackage // exercises unexported mapControlType directly

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/element"
)

func TestMapControlType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		controlType   int32
		wantRole      string
		wantClickable bool
	}{
		{"button", 50000, string(element.RoleButton), true},
		{"checkbox", 50002, string(element.RoleCheckBox), true},
		{"combobox", 50003, string(element.RoleComboBox), true},
		{"edit", 50004, string(element.RoleTextField), true},
		{"hyperlink", 50005, string(element.RoleLink), true},
		{"menu item", 50011, string(element.RoleMenuItem), true},
		{"radio button", 50013, string(element.RoleRadioButton), true},
		{"tab item", 50019, string(element.RoleTabButton), true},
		{"split button", 50031, string(element.RoleButton), true},
		{"text is not clickable", 50020, roleUnknown, false},
		{"unknown control type", 99999, roleUnknown, false},
		{"zero control type", 0, roleUnknown, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			role, clickable := mapControlType(testCase.controlType)
			if role != testCase.wantRole || clickable != testCase.wantClickable {
				t.Fatalf(
					"mapControlType(%d) = (%q, %v), want (%q, %v)",
					testCase.controlType,
					role,
					clickable,
					testCase.wantRole,
					testCase.wantClickable,
				)
			}
		})
	}
}
