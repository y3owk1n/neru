//nolint:testpackage
package accessibility

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/element"
)

func TestIsAdditionalMenuBarElement(t *testing.T) {
	tests := []struct {
		name string
		info *ElementInfo
		want bool
	}{
		{
			name: "allows menu bar container",
			info: &ElementInfo{
				role: string(element.RoleMenuBar),
			},
			want: true,
		},
		{
			name: "allows menu extras",
			info: &ElementInfo{
				role:    string(element.RoleMenuBarItem),
				subrole: "AXMenuExtra",
			},
			want: true,
		},
		{
			name: "rejects normal app menu bar items",
			info: &ElementInfo{
				role: string(element.RoleMenuBarItem),
			},
			want: false,
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
