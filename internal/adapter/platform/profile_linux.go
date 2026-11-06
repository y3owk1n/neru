//go:build linux

package platform

// Runtime Linux profile selection for doctor/status output. It does not probe
// capabilities or alter backend selection. It reads the one detector, because
// re-reading the environment to name the stack is how the profile came to
// disagree with what NewSystemPort drives (#1429).
func linuxProfileForCurrentBackend() Profile {
	backend := DetectLinuxBackend()
	if backend == BackendWaylandKDE {
		return linuxKDEProfile()
	}

	if backend == BackendWaylandCOSMIC {
		return linuxCOSMICProfile()
	}

	if backend == BackendWaylandGNOME {
		return linuxGNOMEProfile()
	}

	return linuxProfile(backend.displayServer())
}

// Every plan here carries a Name only. This profile describes one live stack to
// its user, so the contributor-facing BuildMode/Notes pair has nothing to add.
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

// linuxGNOMEProfile describes the GNOME stack: the portal-driven pointer KDE
// and COSMIC use, the overlay on Xwayland because Mutter has no layer shell,
// and window geometry from the Neru GNOME Shell extension.
func linuxGNOMEProfile() Profile {
	return Profile{
		OS:              Linux,
		PrimaryModifier: defaultPrimaryModifier,
		DisplayServer:   DisplayServerWaylandGNOME,
		Accessibility: BackendPlan{
			Name: "AT-SPI over D-Bus (hints corrected via the Neru GNOME Shell extension)",
		},
		Hotkeys: BackendPlan{
			Name: "evdev from /dev/input (requires input group; or bind triggers in GNOME Settings > Keyboard)",
		},
		KeyboardCapture: BackendPlan{
			Name: "evdev capture + key injection via uinput when /dev/uinput is writable, " +
				"else libei via RemoteDesktop portal; pointer via libei through the " +
				"RemoteDesktop portal (one-time consent, restored from a stored grant)",
		},
		Overlay: BackendPlan{
			Name: "override-redirect X window on Xwayland (Mutter has no layer shell)",
		},
		Notifications: BackendPlan{
			Name: "freedesktop notifications (" + notificationDaemonCaveat + ")",
		},
	}
}
