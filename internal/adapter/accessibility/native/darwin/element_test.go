//go:build darwin

package darwin

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
)

// AppKit reports several vocabulary names — AXSearchField, AXSwitch,
// AXToolbarButton, AXTabButton — as subroles: the element's AXRole is
// something more generic and the configured name arrives in AXSubrole
// (element.AXSubroleNames). These pin the role filter that IsClickable
// applies, through the same ElementInfo bundle the AX bridge fills in.

func TestElementInfo_MatchesRoleFilter_Subroles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		role    string
		subrole string
		allowed []string
		want    bool
	}{
		{
			name:    "search_field alone matches an AppKit search field by subrole",
			role:    string(element.RoleTextField),
			subrole: string(element.RoleSearchField),
			allowed: []string{string(element.RoleSearchField)},
			want:    true,
		},
		{
			name:    "switch alone matches a SwiftUI toggle by subrole",
			role:    string(element.RoleCheckBox),
			subrole: string(element.RoleSwitch),
			allowed: []string{string(element.RoleSwitch)},
			want:    true,
		},
		{
			name:    "toolbar_button matches a toolbar button by subrole",
			role:    string(element.RoleButton),
			subrole: string(element.RoleToolbarButton),
			allowed: []string{string(element.RoleToolbarButton)},
			want:    true,
		},
		{
			name:    "tab matches an AppKit tab button by subrole",
			role:    string(element.RoleRadioButton),
			subrole: string(element.RoleTabButton),
			allowed: []string{string(element.RoleTabButton)},
			want:    true,
		},
		{
			name:    "role matching still works without a subrole",
			role:    string(element.RoleButton),
			subrole: "",
			allowed: []string{string(element.RoleButton)},
			want:    true,
		},
		{
			name:    "plain text field does not match search_field",
			role:    string(element.RoleTextField),
			subrole: "",
			allowed: []string{string(element.RoleSearchField)},
			want:    false,
		},
		{
			name:    "unrelated subrole does not match",
			role:    string(element.RoleTextField),
			subrole: "AXSecureTextField",
			allowed: []string{string(element.RoleSearchField)},
			want:    false,
		},
		{
			// The filter is a single yes/no per element, so a search field
			// stays one hint target when text_field and search_field are
			// both configured.
			name:    "text_field and search_field together still yield one match",
			role:    string(element.RoleTextField),
			subrole: string(element.RoleSearchField),
			allowed: []string{string(element.RoleTextField), string(element.RoleSearchField)},
			want:    true,
		},
		{
			name:    "empty filter matches nothing",
			role:    string(element.RoleTextField),
			subrole: string(element.RoleSearchField),
			allowed: nil,
			want:    false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			allowed := make(map[string]struct{}, len(testCase.allowed))
			for _, name := range testCase.allowed {
				allowed[name] = struct{}{}
			}

			info := NewElementInfo(testCase.role, testCase.subrole, "")

			if got := info.matchesRoleFilter(allowed); got != testCase.want {
				t.Errorf(
					"matchesRoleFilter(role=%q, subrole=%q, allowed=%v) = %v, want %v",
					testCase.role, testCase.subrole, testCase.allowed, got, testCase.want,
				)
			}
		})
	}
}
