//go:build linux

//nolint:testpackage // test covers unexported function
package platform

import "testing"

const waylandDisplay = "wayland-1"

func TestDetectLinuxBackendFromEnv(t *testing.T) {
	tests := []struct {
		name           string
		currentDesktop string
		waylandDisplay string
		xDisplay       string
		want           LinuxBackend
	}{
		{
			name:           "wayland wlroots desktop",
			currentDesktop: "sway",
			waylandDisplay: waylandDisplay,
			want:           BackendWaylandWlroots,
		},
		{
			name:           "wayland hyprland desktop",
			currentDesktop: "Hyprland",
			waylandDisplay: waylandDisplay,
			want:           BackendWaylandWlroots,
		},
		{
			name:           "wayland gnome desktop",
			currentDesktop: "ubuntu:GNOME",
			waylandDisplay: waylandDisplay,
			want:           BackendWaylandGNOME,
		},
		{
			name:           "wayland kde desktop",
			currentDesktop: "KDE",
			waylandDisplay: waylandDisplay,
			want:           BackendWaylandKDE,
		},
		{
			name:           "wayland unknown desktop",
			currentDesktop: "COSMIC",
			waylandDisplay: waylandDisplay,
			want:           BackendWaylandOther,
		},
		{
			name:     "x11 desktop",
			xDisplay: ":0",
			want:     BackendX11,
		},
		{
			name: "unknown backend",
			want: BackendUnknown,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := detectLinuxBackendFromEnv(
				testCase.currentDesktop,
				testCase.waylandDisplay,
				testCase.xDisplay,
			)
			if got != testCase.want {
				t.Fatalf("detectLinuxBackendFromEnv() = %v, want %v", got, testCase.want)
			}
		})
	}
}
