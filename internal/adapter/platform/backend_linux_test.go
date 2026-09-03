//go:build linux

package platform

import "testing"

const waylandDisplay = "wayland-1"

// TestLinuxBackend_IsWayland pins the Wayland/X11 split every subsystem with two
// implementations dispatches on: the event tap's capture mechanism, the hotkey
// manager's listener. Both used to answer it for themselves.
func TestLinuxBackend_IsWayland(t *testing.T) {
	tests := []struct {
		backend LinuxBackend
		want    bool
	}{
		{backend: BackendWaylandWlroots, want: true},
		{backend: BackendWaylandKDE, want: true},
		{backend: BackendWaylandGNOME, want: true},
		{backend: BackendWaylandOther, want: true},
		{backend: BackendX11, want: false},
		{backend: BackendUnknown, want: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.backend.String(), func(t *testing.T) {
			if got := testCase.backend.IsWayland(); got != testCase.want {
				t.Fatalf("%v.IsWayland() = %v, want %v", testCase.backend, got, testCase.want)
			}
		})
	}
}

// TestLinuxBackend_DisplayServer pins the label every backend reports itself
// under. It is a total mapping rather than only the arms the profile reaches:
// the label is the answer to "what is the daemon driving", and a backend with no
// answer here would report the unknown stack while running perfectly.
func TestLinuxBackend_DisplayServer(t *testing.T) {
	tests := []struct {
		backend LinuxBackend
		want    DisplayServer
	}{
		{backend: BackendX11, want: DisplayServerX11},
		{backend: BackendWaylandWlroots, want: DisplayServerWayland},
		{backend: BackendWaylandGNOME, want: DisplayServerWayland},
		{backend: BackendWaylandOther, want: DisplayServerWayland},
		{backend: BackendWaylandKDE, want: DisplayServerWaylandKDE},
		{backend: BackendUnknown, want: DisplayServerUnknown},
	}

	for _, testCase := range tests {
		t.Run(testCase.backend.String(), func(t *testing.T) {
			if got := testCase.backend.displayServer(); got != testCase.want {
				t.Fatalf(
					"%v.displayServer() = %q, want %q",
					testCase.backend, got, testCase.want,
				)
			}
		})
	}
}

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

// TestIsHyprlandFromEnv pins the one compositor LinuxBackend does not name.
// The wlroots arm covers five compositors, so a caller with a Hyprland-only
// quirk has to ask this instead — and it has to answer no for the four others,
// for KDE (whose modifier goes out on libei, not the wlroots keyboard), and for
// a desktop variable left over from a session that is not running.
func TestIsHyprlandFromEnv(t *testing.T) {
	tests := []struct {
		name           string
		currentDesktop string
		waylandDisplay string
		want           bool
	}{
		{
			name:           "hyprland wayland session",
			currentDesktop: "Hyprland",
			waylandDisplay: waylandDisplay,
			want:           true,
		},
		{
			name:           "hyprland beside another desktop name",
			currentDesktop: "Hyprland:wlroots",
			waylandDisplay: waylandDisplay,
			want:           true,
		},
		{
			name:           "sway is not hyprland",
			currentDesktop: "sway",
			waylandDisplay: waylandDisplay,
			want:           false,
		},
		{
			name:           "kde is not hyprland",
			currentDesktop: "KDE",
			waylandDisplay: waylandDisplay,
			want:           false,
		},
		{
			name:           "a hyprland desktop with no wayland socket is a stale variable",
			currentDesktop: "Hyprland",
			want:           false,
		},
		{
			name:           "an unset desktop stays plain wlroots",
			waylandDisplay: waylandDisplay,
			want:           false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := isHyprlandFromEnv(testCase.currentDesktop, testCase.waylandDisplay)
			if got != testCase.want {
				t.Fatalf("isHyprlandFromEnv() = %v, want %v", got, testCase.want)
			}
		})
	}
}
