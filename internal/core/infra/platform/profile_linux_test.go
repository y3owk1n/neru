//go:build linux

//nolint:testpackage // Same-package tests cover unexported KDE profile helpers.
package platform

import "testing"

func TestLinuxKDEProfile(t *testing.T) {
	got := linuxKDEProfile()

	if got.DisplayServer != DisplayServerWaylandKDE {
		t.Fatalf("DisplayServer = %q, want %q", got.DisplayServer, DisplayServerWaylandKDE)
	}

	if got.Accessibility.Name == "" || got.Accessibility.BuildMode != "" {
		t.Fatalf("Accessibility = %+v, want user-facing Name only", got.Accessibility)
	}

	if got.Hotkeys.Name == "" {
		t.Fatal("Hotkeys.Name should describe evdev hotkey setup")
	}

	if got.KeyboardCapture.Name == "" {
		t.Fatal("KeyboardCapture.Name should describe evdev + libei setup")
	}

	if got.Overlay.Name == "" {
		t.Fatal("Overlay.Name should describe wlr-layer-shell via KWin")
	}

	if got.Notifications.Name != "not implemented" {
		t.Fatalf("Notifications.Name = %q, want not implemented", got.Notifications.Name)
	}
}
