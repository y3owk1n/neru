//go:build darwin

//nolint:testpackage
package native

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/accessibility/native/darwin"
	"github.com/y3owk1n/neru/internal/domain/element"
)

func TestIsAdditionalMenuBarElement(t *testing.T) {
	tests := []struct {
		name string
		info *ElementInfo
		want bool
	}{
		{
			name: "allows menu bar container",
			info: elementInfo(string(element.RoleMenuBar), ""),
			want: true,
		},
		{
			name: "allows menu extras",
			info: elementInfo(string(element.RoleMenuBarItem), string(element.SubroleMenuExtra)),
			want: true,
		},
		{
			name: "rejects normal app menu bar items",
			info: elementInfo(string(element.RoleMenuBarItem), ""),
			want: false,
		},
		{
			name: "allows menu container",
			info: elementInfo(string(element.RoleMenu), ""),
			want: true,
		},
		{
			name: "allows menu items",
			info: elementInfo(string(element.RoleMenuItem), ""),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAdditionalMenuBarElement(tt.info); got != tt.want {
				t.Fatalf("isAdditionalMenuBarElement() = %v, want %v", got, tt.want)
			}
		})
	}
}

// elementInfo builds the macOS attribute bundle the menu-bar filter reads.
func elementInfo(role, subrole string) *ElementInfo {
	info := darwin.NewElementInfo(role, subrole, "")

	return &info
}
