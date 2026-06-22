//go:build windows

// internal/core/infra/accessibility/uia_windows_test.go
// Unit tests for the pure UIA control-type mapping used by hint enumeration.
// Does not exercise live UIA (see accessibility integration tests on WIN-VM).

package accessibility

import "testing"

func TestMapControlType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		controlType   int32
		wantRole      string
		wantClickable bool
	}{
		{"button", 50000, "AXButton", true},
		{"checkbox", 50002, "AXCheckBox", true},
		{"combobox", 50003, "AXComboBox", true},
		{"edit", 50004, "AXTextField", true},
		{"hyperlink", 50005, "AXLink", true},
		{"menu item", 50011, "AXMenuItem", true},
		{"radio button", 50013, "AXRadioButton", true},
		{"tab item", 50019, "AXTabButton", true},
		{"split button", 50031, "AXButton", true},
		{"text is not clickable", 50020, "AXUnknown", false},
		{"unknown control type", 99999, "AXUnknown", false},
		{"zero control type", 0, "AXUnknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			role, clickable := mapControlType(tt.controlType)
			if role != tt.wantRole || clickable != tt.wantClickable {
				t.Fatalf("mapControlType(%d) = (%q, %v), want (%q, %v)",
					tt.controlType, role, clickable, tt.wantRole, tt.wantClickable)
			}
		})
	}
}
