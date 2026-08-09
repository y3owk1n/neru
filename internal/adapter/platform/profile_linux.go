//go:build linux

package platform

// Runtime Linux profile selection for doctor/status output. When KDE Plasma
// Wayland is detected, returns user-facing backend descriptions for that stack.
// It does not perform live capability probes or alter runtime backend selection.
//
// Both halves come off the one detector: the profile must describe the stack
// NewSystemPort is driving, and reading the environment a second time to name
// it is how it came to disagree with itself (#1429).
func linuxProfileForCurrentBackend() Profile {
	backend := DetectLinuxBackend()
	if backend == BackendWaylandKDE {
		return linuxKDEProfile()
	}

	return linuxProfile(backend.displayServer())
}

func linuxKDEProfile() Profile {
	return Profile{
		OS:              Linux,
		PrimaryModifier: defaultPrimaryModifier,
		DisplayServer:   DisplayServerWaylandKDE,
		Accessibility: BackendPlan{
			Name: "AT-SPI over D-Bus (hints corrected via KWin geometry bridge)",
		},
		Hotkeys: BackendPlan{
			Name: "evdev from /dev/input (requires input group; bind triggers in KDE System Settings)",
		},
		KeyboardCapture: BackendPlan{
			Name: "evdev capture + key injection via uinput when /dev/uinput is writable, " +
				"else libei via RemoteDesktop portal (consent per daemon launch)",
		},
		Overlay: BackendPlan{
			Name: "wlr-layer-shell via KWin",
		},
		Notifications: BackendPlan{
			Name: "not implemented",
		},
	}
}
