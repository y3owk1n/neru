//go:build windows

package windows

import "testing"

func TestKeyComboFromBaseAndModifiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base string
		mods []string
		want string
	}{
		{
			name: "shift left click binding",
			base: "l",
			mods: []string{"shift"},
			want: "shift+l",
		},
		{
			name: "shift right click binding",
			base: "r",
			mods: []string{"shift"},
			want: "shift+r",
		},
		{
			name: "ctrl shift grid activation",
			base: "g",
			mods: []string{"ctrl", "shift"},
			want: "ctrl+shift+g",
		},
		{
			name: "plain key",
			base: "a",
			mods: nil,
			want: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := KeyComboFromBaseAndModifiers(tt.base, tt.mods)
			if got != tt.want {
				t.Fatalf("KeyComboFromBaseAndModifiers() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestKeyNameFromVirtualKeyLetters(t *testing.T) {
	t.Parallel()

	if got := KeyNameFromVirtualKey(0x4C); got != "l" {
		t.Fatalf("KeyNameFromVirtualKey(0x4C) = %q, want l", got)
	}

	if got := ModifierNameFromVirtualKey(vkLShift); got != "shift" {
		t.Fatalf("ModifierNameFromVirtualKey(vkLShift) = %q, want shift", got)
	}
}
