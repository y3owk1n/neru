//go:build windows

package accessibility //nolint:testpackage // exercises unexported windowsRoleMatchesFilter

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/element"
)

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
			elementRole: string(element.RoleButton),
			keptRoles:   nil,
			want:        true,
		},
		{
			name:        "direct AX match",
			elementRole: string(element.RoleButton),
			keptRoles:   map[string]struct{}{string(element.RoleButton): {}},
			want:        true,
		},
		{
			name:        "legacy UIA name matches AX role",
			elementRole: string(element.RoleTextField),
			keptRoles:   map[string]struct{}{"Edit": {}},
			want:        true,
		},
		{
			name:        "unrelated roles do not match",
			elementRole: string(element.RoleButton),
			keptRoles:   map[string]struct{}{string(element.RoleLink): {}},
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
