//go:build linux

package platform

import (
	"strings"
	"testing"
)

// sessionTypeWayland is what a Wayland login session writes into
// XDG_SESSION_TYPE. No detector reads it any more; the cases below set it to
// show that a session announcing Wayland does not by itself make the backend
// Wayland — reading it was how the two answers came apart.
const sessionTypeWayland = "wayland"

// TestProfileFor_LinuxReportsTheStackTheDaemonDrives pins the profile's
// DisplayServer to the backend NewSystemPort actually builds. The two used to be
// read from different environment variables, and the session that pulled them
// apart is real: sway and Hyprland launched from a systemd user unit that
// imported SWAYSOCK but not WAYLAND_DISPLAY run the X11 adapter, while
// `neru info` and the health output reported display_server: wayland from
// XDG_SESSION_TYPE.
func TestProfileFor_LinuxReportsTheStackTheDaemonDrives(t *testing.T) {
	tests := []struct {
		name           string
		sessionType    string
		currentDesktop string
		waylandDisplay string
		xDisplay       string
		wantBackend    LinuxBackend
		wantDisplay    DisplayServer
	}{
		{
			name:           "sway launched without WAYLAND_DISPLAY in its unit",
			sessionType:    sessionTypeWayland,
			currentDesktop: "sway",
			xDisplay:       ":0",
			wantBackend:    BackendX11,
			wantDisplay:    DisplayServerX11,
		},
		{
			name:        "plain x11 session",
			sessionType: "x11",
			xDisplay:    ":0",
			wantBackend: BackendX11,
			wantDisplay: DisplayServerX11,
		},
		{
			name:           "wlroots wayland session",
			sessionType:    sessionTypeWayland,
			currentDesktop: "Hyprland",
			waylandDisplay: waylandDisplay,
			wantBackend:    BackendWaylandWlroots,
			wantDisplay:    DisplayServerWayland,
		},
		{
			name:           "kde wayland session",
			sessionType:    sessionTypeWayland,
			currentDesktop: "KDE",
			waylandDisplay: waylandDisplay,
			wantBackend:    BackendWaylandKDE,
			wantDisplay:    DisplayServerWaylandKDE,
		},
		{
			name:           "gnome wayland session",
			sessionType:    sessionTypeWayland,
			currentDesktop: "ubuntu:GNOME",
			waylandDisplay: waylandDisplay,
			wantBackend:    BackendWaylandGNOME,
			wantDisplay:    DisplayServerWayland,
		},
		{
			name:           "unsupported wayland compositor",
			sessionType:    sessionTypeWayland,
			currentDesktop: "COSMIC",
			waylandDisplay: waylandDisplay,
			wantBackend:    BackendWaylandOther,
			wantDisplay:    DisplayServerWayland,
		},
		{
			name:        "no display server at all",
			sessionType: "tty",
			wantBackend: BackendUnknown,
			wantDisplay: DisplayServerUnknown,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			resetLinuxBackendCache()
			t.Cleanup(resetLinuxBackendCache)

			t.Setenv("XDG_SESSION_TYPE", testCase.sessionType)
			t.Setenv("XDG_CURRENT_DESKTOP", testCase.currentDesktop)
			t.Setenv("WAYLAND_DISPLAY", testCase.waylandDisplay)
			t.Setenv("DISPLAY", testCase.xDisplay)

			if got := DetectLinuxBackend(); got != testCase.wantBackend {
				t.Fatalf("DetectLinuxBackend() = %v, want %v", got, testCase.wantBackend)
			}

			if got := ProfileFor(Linux).DisplayServer; got != testCase.wantDisplay {
				t.Fatalf(
					"ProfileFor(Linux).DisplayServer = %q, want %q",
					got, testCase.wantDisplay,
				)
			}
		})
	}
}

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

	// Notifications are a session-bus service, so the KDE stack gets the same
	// freedesktop backend every other Linux backend gets (#1471). This plan read
	// "not implemented" long after it shipped and doctor read that to the user
	// (#1486), so the assertions below pin the agreement rather than a string:
	// KDE names the backend linuxProfile names, and states the one precondition
	// it has in the words linuxProfile states it in.
	x11Notifications := linuxProfile(DisplayServerX11).Notifications

	if !strings.Contains(got.Notifications.Name, x11Notifications.Name) {
		t.Fatalf(
			"Notifications.Name = %q, want it to name the %q backend",
			got.Notifications.Name,
			x11Notifications.Name,
		)
	}

	if !strings.Contains(x11Notifications.Notes, notificationDaemonCaveat) {
		t.Fatalf(
			"linuxProfile Notifications.Notes = %q, want it to state %q",
			x11Notifications.Notes,
			notificationDaemonCaveat,
		)
	}

	if !strings.Contains(got.Notifications.Name, notificationDaemonCaveat) {
		t.Fatalf(
			"Notifications.Name = %q, want it to state %q",
			got.Notifications.Name,
			notificationDaemonCaveat,
		)
	}
}
