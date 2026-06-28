//go:build windows

package accessibility //nolint:testpackage // exercises unexported windowsRoleMatchesFilter

import "testing"

func TestWindowsRoleMatchesFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		elementRole string
		keptRoles   map[string]struct{}
		want        bool
	}{
		{
			name:        "empty filter accepts all",
			elementRole: "AXButton",
			keptRoles:   nil,
			want:        true,
		},
		{
			name:        "direct AX match",
			elementRole: "AXButton",
			keptRoles:   map[string]struct{}{"AXButton": {}},
			want:        true,
		},
		{
			name:        "legacy UIA name matches AX role",
			elementRole: "AXTextField",
			keptRoles:   map[string]struct{}{"Edit": {}},
			want:        true,
		},
		{
			name:        "unrelated roles do not match",
			elementRole: "AXButton",
			keptRoles:   map[string]struct{}{"AXLink": {}},
			want:        false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := windowsRoleMatchesFilter(testCase.elementRole, testCase.keptRoles)
			if got != testCase.want {
				t.Fatalf(
					"windowsRoleMatchesFilter(%q, %v) = %v, want %v",
					testCase.elementRole,
					testCase.keptRoles,
					got,
					testCase.want,
				)
			}
		})
	}
}
