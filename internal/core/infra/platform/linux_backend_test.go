//nolint:testpackage // test covers unexported function
package platform

import "testing"

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
			waylandDisplay: "wayland-1",
			want:           BackendWaylandWlroots,
		},
		{
			name:           "wayland hyprland desktop",
			currentDesktop: "Hyprland",
			waylandDisplay: "wayland-1",
			want:           BackendWaylandWlroots,
		},
		{
			name:           "wayland gnome desktop",
			currentDesktop: "ubuntu:GNOME",
			waylandDisplay: "wayland-1",
			want:           BackendWaylandGNOME,
		},
		{
			name:           "wayland kde desktop",
			currentDesktop: "KDE",
			waylandDisplay: "wayland-1",
			want:           BackendWaylandKDE,
		},
		{
			name:           "wayland unknown desktop",
			currentDesktop: "COSMIC",
			waylandDisplay: "wayland-1",
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
