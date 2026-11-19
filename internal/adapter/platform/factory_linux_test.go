//go:build linux

package platform

import (
	"strings"
	"testing"
)

// A GNOME session draws its overlay on Xwayland, so one that exposes no X
// server has nowhere to draw and is refused up front, naming the reason.
func TestNewSystemPort_GNOMEWaylandWithoutXwaylandReturnsHelpfulError(t *testing.T) {
	resetLinuxBackendCache()

	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "GNOME")

	systemPort, err := NewSystemPort()
	if err == nil {
		t.Fatal("NewSystemPort() error = nil, want error")
	}

	if systemPort != nil {
		t.Fatal("NewSystemPort() systemPort != nil, want nil")
	}

	for _, want := range []string{"GNOME", "Xwayland", "DISPLAY"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("NewSystemPort() error = %q, want mention of %q", err.Error(), want)
		}
	}
}

func TestNewSystemPort_NoDisplayServerReturnsHelpfulError(t *testing.T) {
	resetLinuxBackendCache()

	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "")

	systemPort, err := NewSystemPort()
	if err == nil {
		t.Fatal("NewSystemPort() error = nil, want error")
	}

	if systemPort != nil {
		t.Fatal("NewSystemPort() systemPort != nil, want nil")
	}

	if !strings.Contains(err.Error(), "display server") {
		t.Fatalf("NewSystemPort() error = %q, want mention of display server", err.Error())
	}
}

// TestNewSystemPort_WaylandDesktopsReturnSystemPort pins that each Wayland
// desktop Neru drives gets a system port reporting that desktop's stack. A
// GNOME session draws its overlay on Xwayland, so its row carries a DISPLAY.
func TestNewSystemPort_WaylandDesktopsReturnSystemPort(t *testing.T) {
	tests := []struct {
		desktop string
		display string
		want    string
	}{
		{desktop: desktopSway, want: "linux/wayland-wlroots"},
		{desktop: desktopKDE, want: "linux/wayland-kde"},
		{desktop: desktopCOSMIC, want: "linux/wayland-cosmic"},
		{desktop: desktopUbuntuGNOME, display: ":0", want: "linux/wayland-gnome"},
	}

	for _, testCase := range tests {
		t.Run(testCase.desktop, func(t *testing.T) {
			resetLinuxBackendCache()

			t.Setenv("WAYLAND_DISPLAY", "wayland-0")
			t.Setenv("DISPLAY", testCase.display)
			t.Setenv("XDG_CURRENT_DESKTOP", testCase.desktop)

			systemPort, err := NewSystemPort()
			if err != nil {
				t.Fatalf("NewSystemPort() error = %v, want nil", err)
			}

			if systemPort == nil {
				t.Fatal("NewSystemPort() systemPort = nil, want non-nil")
			}

			if got := systemPort.Capabilities().Platform; got != testCase.want {
				t.Fatalf("Capabilities().Platform = %q, want %q", got, testCase.want)
			}
		})
	}
}
