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

	if backend == BackendWaylandCOSMIC {
		return linuxCOSMICProfile()
	}

	return linuxProfile(backend.displayServer())
}

// Every plan here carries a Name only. This profile describes one live stack to
// the user running on it, so the contributor-facing BuildMode/Notes pair the
// target-by-target profiles use has nothing to add: the answer is whatever the
// KDE backend already does.
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
				"else libei via RemoteDesktop portal (one-time consent, restored from " +
				"a stored grant on later starts)",
		},
		Overlay: BackendPlan{
			Name: "wlr-layer-shell via KWin",
		},
		Notifications: BackendPlan{
			Name: "freedesktop notifications (" + notificationDaemonCaveat + ")",
		},
	}
}

// linuxCOSMICProfile describes the COSMIC stack the way linuxKDEProfile does
// KDE's: the same portal-driven pointer and layer-shell overlay, with window
// geometry read off cosmic-comp's own toplevel protocol instead of a bridge.
func linuxCOSMICProfile() Profile {
	return Profile{
		OS:              Linux,
		PrimaryModifier: defaultPrimaryModifier,
		DisplayServer:   DisplayServerWaylandCOSMIC,
		Accessibility: BackendPlan{
			Name: "AT-SPI over D-Bus (hints corrected via cosmic-comp toplevel geometry)",
		},
		Hotkeys: BackendPlan{
			Name: "evdev from /dev/input (requires input group; bind triggers in COSMIC Settings)",
		},
		KeyboardCapture: BackendPlan{
			Name: "evdev capture + key injection via uinput when /dev/uinput is writable, " +
				"else zwp_virtual_keyboard_v1; pointer via libei through the RemoteDesktop " +
				"portal (xdg-desktop-portal-cosmic 1.7 or later, one-time consent)",
		},
		Overlay: BackendPlan{
			Name: "wlr-layer-shell via cosmic-comp",
		},
		Notifications: BackendPlan{
			Name: "freedesktop notifications (" + notificationDaemonCaveat + ")",
		},
	}
}
