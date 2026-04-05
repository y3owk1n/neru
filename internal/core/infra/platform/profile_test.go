//nolint:testpackage // Same-package tests intentionally cover unexported backend detection helpers.
package platform

import "testing"

func TestProfileFor(t *testing.T) {
	tests := []struct {
		name              string
		target            OS
		wantPrimary       string
		wantDisplay       DisplayServer
		wantAccess        string
		wantAccessBuild   BuildMode
		wantHotkeys       string
		wantHotkeysBuild  BuildMode
		wantKeyboard      string
		wantKeyboardBuild BuildMode
		wantOverlay       string
		wantOverlayBuild  BuildMode
		wantNotify        string
		wantNotifyBuild   BuildMode
	}{
		{
			name:              "darwin",
			target:            Darwin,
			wantPrimary:       "cmd",
			wantDisplay:       DisplayServerCocoa,
			wantAccess:        "axuielement",
			wantAccessBuild:   BuildModeCGORequired,
			wantHotkeys:       "carbon-hotkeys",
			wantHotkeysBuild:  BuildModeCGORequired,
			wantKeyboard:      "quartz-event-tap",
			wantKeyboardBuild: BuildModeCGORequired,
			wantOverlay:       "cocoa-overlay-window",
			wantOverlayBuild:  BuildModeCGORequired,
			wantNotify:        "usernotifications/nsalert",
			wantNotifyBuild:   BuildModeCGORequired,
		},
		{
			name:              "linux",
			target:            Linux,
			wantPrimary:       "ctrl",
			wantDisplay:       DisplayServerUnknown,
			wantAccess:        "at-spi",
			wantAccessBuild:   BuildModePureGo,
			wantHotkeys:       "x11 or compositor-specific backend",
			wantHotkeysBuild:  BuildModeBackendDependent,
			wantKeyboard:      "x11 or compositor-specific backend",
			wantKeyboardBuild: BuildModeBackendDependent,
			wantOverlay:       "x11 window or wayland layer-shell",
			wantOverlayBuild:  BuildModeBackendDependent,
			wantNotify:        "freedesktop notifications",
			wantNotifyBuild:   BuildModePureGo,
		},
		{
			name:              "windows",
			target:            Windows,
			wantPrimary:       "ctrl",
			wantDisplay:       DisplayServerWin32,
			wantAccess:        "uia",
			wantAccessBuild:   BuildModePureGo,
			wantHotkeys:       "RegisterHotKey",
			wantHotkeysBuild:  BuildModePureGo,
			wantKeyboard:      "low-level keyboard hook",
			wantKeyboardBuild: BuildModePureGo,
			wantOverlay:       "layered win32 window",
			wantOverlayBuild:  BuildModePureGo,
			wantNotify:        "windows toast",
			wantNotifyBuild:   BuildModePureGo,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := ProfileFor(testCase.target)
			if got.PrimaryModifier != testCase.wantPrimary {
				t.Fatalf("PrimaryModifier = %q, want %q", got.PrimaryModifier, testCase.wantPrimary)
			}

			if got.DisplayServer != testCase.wantDisplay {
				t.Fatalf("DisplayServer = %q, want %q", got.DisplayServer, testCase.wantDisplay)
			}

			if got.Accessibility.Name != testCase.wantAccess {
				t.Fatalf(
					"Accessibility.Name = %q, want %q",
					got.Accessibility.Name,
					testCase.wantAccess,
				)
			}

			if got.Accessibility.BuildMode != testCase.wantAccessBuild {
				t.Fatalf(
					"Accessibility.BuildMode = %q, want %q",
					got.Accessibility.BuildMode,
					testCase.wantAccessBuild,
				)
			}

			if got.Hotkeys.Name != testCase.wantHotkeys {
				t.Fatalf("Hotkeys.Name = %q, want %q", got.Hotkeys.Name, testCase.wantHotkeys)
			}

			if got.Hotkeys.BuildMode != testCase.wantHotkeysBuild {
				t.Fatalf(
					"Hotkeys.BuildMode = %q, want %q",
					got.Hotkeys.BuildMode,
					testCase.wantHotkeysBuild,
				)
			}

			if got.KeyboardCapture.Name != testCase.wantKeyboard {
				t.Fatalf(
					"KeyboardCapture.Name = %q, want %q",
					got.KeyboardCapture.Name,
					testCase.wantKeyboard,
				)
			}

			if got.KeyboardCapture.BuildMode != testCase.wantKeyboardBuild {
				t.Fatalf(
					"KeyboardCapture.BuildMode = %q, want %q",
					got.KeyboardCapture.BuildMode,
					testCase.wantKeyboardBuild,
				)
			}

			if got.Overlay.Name != testCase.wantOverlay {
				t.Fatalf("Overlay.Name = %q, want %q", got.Overlay.Name, testCase.wantOverlay)
			}

			if got.Overlay.BuildMode != testCase.wantOverlayBuild {
				t.Fatalf(
					"Overlay.BuildMode = %q, want %q",
					got.Overlay.BuildMode,
					testCase.wantOverlayBuild,
				)
			}

			if got.Notifications.Name != testCase.wantNotify {
				t.Fatalf(
					"Notifications.Name = %q, want %q",
					got.Notifications.Name,
					testCase.wantNotify,
				)
			}

			if got.Notifications.BuildMode != testCase.wantNotifyBuild {
				t.Fatalf(
					"Notifications.BuildMode = %q, want %q",
					got.Notifications.BuildMode,
					testCase.wantNotifyBuild,
				)
			}
		})
	}
}

func TestDetectLinuxDisplayServer(t *testing.T) {
	tests := []struct {
		name        string
		sessionType string
		waylandEnv  string
		xDisplayEnv string
		wantDisplay DisplayServer
	}{
		{
			name:        "wayland from session type",
			sessionType: "wayland",
			wantDisplay: DisplayServerWayland,
		},
		{name: "wayland from env", waylandEnv: "wayland-0", wantDisplay: DisplayServerWayland},
		{name: "x11 from session type", sessionType: "x11", wantDisplay: DisplayServerX11},
		{name: "x11 from display env", xDisplayEnv: ":0", wantDisplay: DisplayServerX11},
		{name: "unknown", wantDisplay: DisplayServerUnknown},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := detectLinuxDisplayServer(
				testCase.sessionType,
				testCase.waylandEnv,
				testCase.xDisplayEnv,
			)
			if got != testCase.wantDisplay {
				t.Fatalf("detectLinuxDisplayServer() = %q, want %q", got, testCase.wantDisplay)
			}
		})
	}
}
