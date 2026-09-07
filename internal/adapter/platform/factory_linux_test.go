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

func TestNewSystemPort_GNOMEWaylandWithXwaylandReturnsSystemPort(t *testing.T) {
	resetLinuxBackendCache()

	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", ":0")
	t.Setenv("XDG_CURRENT_DESKTOP", "ubuntu:GNOME")

	systemPort, err := NewSystemPort()
	if err != nil {
		t.Fatalf("NewSystemPort() error = %v, want nil", err)
	}

	if got := systemPort.Capabilities().Platform; got != "linux/wayland-gnome" {
		t.Fatalf("Capabilities().Platform = %q, want %q", got, "linux/wayland-gnome")
	}
}

func TestNewSystemPort_KDEWaylandReturnsSystemPort(t *testing.T) {
	resetLinuxBackendCache()

	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")

	systemPort, err := NewSystemPort()
	if err != nil {
		t.Fatalf("NewSystemPort() error = %v, want nil", err)
	}

	if systemPort == nil {
		t.Fatal("NewSystemPort() systemPort = nil, want non-nil")
	}

	if got := systemPort.Capabilities().Platform; got != "linux/wayland-kde" {
		t.Fatalf("Capabilities().Platform = %q, want %q", got, "linux/wayland-kde")
	}
}

func TestNewSystemPort_COSMICWaylandReturnsSystemPort(t *testing.T) {
	resetLinuxBackendCache()

	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "COSMIC")

	systemPort, err := NewSystemPort()
	if err != nil {
		t.Fatalf("NewSystemPort() error = %v, want nil", err)
	}

	if got := systemPort.Capabilities().Platform; got != "linux/wayland-cosmic" {
		t.Fatalf("Capabilities().Platform = %q, want %q", got, "linux/wayland-cosmic")
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

func TestNewSystemPort_SwayWaylandReturnsSystemPort(t *testing.T) {
	resetLinuxBackendCache()

	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "sway")

	systemPort, err := NewSystemPort()
	if err != nil {
		t.Fatalf("NewSystemPort() error = %v, want nil", err)
	}

	if systemPort == nil {
		t.Fatal("NewSystemPort() systemPort = nil, want non-nil")
	}

	if got := systemPort.Capabilities().Platform; got != "linux/wayland-wlroots" {
		t.Fatalf("Capabilities().Platform = %q, want %q", got, "linux/wayland-wlroots")
	}
}
