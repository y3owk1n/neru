package ports

func supportedCapability(detail string) FeatureCapability {
	return FeatureCapability{
		Status: FeatureStatusSupported,
		Detail: detail,
	}
}

func stubCapability(detail string) FeatureCapability {
	return FeatureCapability{
		Status: FeatureStatusStub,
		Detail: detail,
	}
}

// DarwinCapabilities returns the supported macOS runtime capabilities.
func DarwinCapabilities() PlatformCapabilities {
	return PlatformCapabilities{
		Platform: "darwin",
		Process: supportedCapability(
			"focused app inspection available via Cocoa workspace APIs",
		),
		Screen: supportedCapability(
			"screen bounds and display enumeration available via Cocoa",
		),
		Cursor: supportedCapability(
			"cursor movement and tracking available via Quartz events",
		),
		Accessibility: supportedCapability(
			"macOS accessibility integration available via AXUIElement",
		),
		Overlay: supportedCapability("native overlays available via Cocoa windows"),
		Notifications: supportedCapability(
			"native alerts and notifications available via NSAlert/UserNotifications",
		),
		GlobalHotkeys: supportedCapability("global hotkeys available via per-key CGEventTaps"),
		KeyboardEventTap: supportedCapability(
			"keyboard event tap available via Quartz event taps",
		),
		AppWatcher: supportedCapability("focused-app watcher available via NSWorkspace"),
		DarkModeDetection: supportedCapability(
			"system dark mode detection available via Cocoa appearance APIs",
		),
	}
}

// LinuxCapabilities returns the supported Linux runtime capabilities.
func LinuxCapabilities() PlatformCapabilities {
	return PlatformCapabilities{
		Platform: "linux",
		Process: supportedCapability(
			"focused app inspection available via X11 _NET_WM_PID and Wayland " +
				"wlr-foreign-toplevel app_id (wlroots/KDE; PID best-effort via /proc)",
		),
		Screen: supportedCapability(
			"screen enumeration available via XRandR and Wayland xdg-output",
		),
		Cursor: supportedCapability(
			"cursor movement/tracking available via XTest and Wayland virtual-pointer",
		),
		Accessibility: supportedCapability(
			"clickable-element discovery via AT-SPI (D-Bus) tree walk; " +
				"click/scroll injection via XTest (X11) or virtual-pointer/libei " +
				"(Wayland wlroots/KDE). Coverage depends on the app's AT-SPI " +
				"support; there is no Vision/OCR fallback",
		),
		Overlay: supportedCapability(
			"native overlays available via X11 windows or Wayland layer-shell + Cairo",
		),
		Notifications: stubCapability(
			"native notifications not implemented yet; target freedesktop notifications",
		),
		GlobalHotkeys: supportedCapability(
			"global hotkeys available via X11 (Wayland relies on compositor bindings)",
		),
		KeyboardEventTap: supportedCapability(
			"keyboard event tap available via X11 grab and Wayland evdev/layer-shell " +
				"keyboard interactivity; modifier passthrough of unbound shortcuts is " +
				"supported on the Wayland evdev backend only (X11's exclusive grab and " +
				"the wl-keyboard fallback cannot re-inject selectively)",
		),
		AppWatcher: supportedCapability(
			"focused-app change detection via polling the WM_CLASS (X11) or " +
				"wlr-foreign-toplevel app_id (Wayland wlroots/KDE); GNOME/Mutter " +
				"exposes no focused-app source",
		),
		// Default placeholder; the Linux SystemAdapter overrides this with
		// the live-probed state (current color-scheme + source) on each
		// Capabilities() call. See linux.SystemAdapter.Capabilities.
		DarkModeDetection: supportedCapability(
			"dark mode detection via freedesktop appearance portal (Settings.Read), with kdeglobals fallback",
		),
	}
}

// WindowsCapabilities returns the current Windows runtime capabilities.
func WindowsCapabilities() PlatformCapabilities {
	return PlatformCapabilities{
		Platform: "windows",
		Process: supportedCapability(
			"focused app inspection available via Win32 foreground-window APIs",
		),
		Screen: supportedCapability(
			"screen bounds and display enumeration available via Win32 monitor APIs",
		),
		Cursor: supportedCapability(
			"cursor movement and tracking available via SetCursorPos/GetCursorPos",
		),
		Accessibility: supportedCapability(
			"clickable-element discovery available via UI Automation (initial coverage)",
		),
		Overlay: supportedCapability(
			"native overlays available via layered Win32 window + GDI",
		),
		Notifications: stubCapability(
			"native notifications not implemented yet; target Windows toast notifications",
		),
		GlobalHotkeys: supportedCapability(
			"global hotkeys available via RegisterHotKey",
		),
		KeyboardEventTap: supportedCapability(
			"keyboard event tap available via WH_KEYBOARD_LL hook",
		),
		AppWatcher: stubCapability(
			"app watcher not implemented yet; target Win32 foreground-window notifications",
		),
		DarkModeDetection: stubCapability(
			"dark mode detection not implemented yet; target Windows personalization APIs",
		),
	}
}
