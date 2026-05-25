//go:build linux

package eventtap

import "testing"

func TestLinuxKeyUpEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		press string
		want  string
	}{
		{"j", "__keyup_j"},
		{"Shift+j", "__keyup_j"},
		{"k", "__keyup_k"},
		{"Return", "__keyup_Return"},
	}

	for _, tc := range tests {
		t.Run(tc.press, func(t *testing.T) {
			t.Parallel()

			if got := linuxKeyUpEvent(tc.press); got != tc.want {
				t.Fatalf("linuxKeyUpEvent(%q) = %q, want %q", tc.press, got, tc.want)
			}
		})
	}
}
