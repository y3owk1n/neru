//go:build linux

package platform

import (
	"strings"
	"testing"
)

func TestNewSystemPort_GNOMEWaylandReturnsHelpfulError(t *testing.T) {
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

	if !strings.Contains(err.Error(), "GNOME") {
		t.Fatalf("NewSystemPort() error = %q, want mention of GNOME", err.Error())
	}
}

func TestNewSystemPort_KDEWaylandReturnsHelpfulError(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")

	systemPort, err := NewSystemPort()
	if err == nil {
		t.Fatal("NewSystemPort() error = nil, want error")
	}

	if systemPort != nil {
		t.Fatal("NewSystemPort() systemPort != nil, want nil")
	}

	if !strings.Contains(err.Error(), "KDE") {
		t.Fatalf("NewSystemPort() error = %q, want mention of KDE", err.Error())
	}
}

func TestNewSystemPort_NoDisplayServerReturnsHelpfulError(t *testing.T) {
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
