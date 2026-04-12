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
		GlobalHotkeys: supportedCapability("global hotkeys available via Carbon hotkeys"),
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
			"focused app inspection available via XWayland/X11 properties",
		),
		Screen: supportedCapability(
			"screen enumeration available via XRandR and Wayland xdg-output",
		),
		Cursor: supportedCapability(
			"cursor movement/tracking available via XTest and Wayland virtual-pointer",
		),
		Accessibility: stubCapability("AT-SPI integration not implemented yet"),
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
			"keyboard event tap available via X11 grab and Wayland layer-shell keyboard interactivity",
		),
		AppWatcher: stubCapability(
			"app watcher not needed for Neru's current navigation model",
		),
		DarkModeDetection: stubCapability(
			"dark mode detection not implemented yet; target freedesktop appearance APIs",
		),
	}
}

// WindowsCapabilities returns the current Windows runtime capabilities.
func WindowsCapabilities() PlatformCapabilities {
	return PlatformCapabilities{
		Platform: "windows",
		Process: stubCapability(
			"focused app inspection not implemented yet; target Win32 foreground-window APIs",
		),
		Screen: stubCapability(
			"screen enumeration not implemented yet; target Win32 monitor APIs",
		),
		Cursor: stubCapability(
			"cursor movement/tracking not implemented yet; target SendInput/GetCursorPos",
		),
		Accessibility: stubCapability("UI Automation integration not implemented yet"),
		Overlay: stubCapability(
			"native overlays not implemented yet; target layered Win32 windows",
		),
		Notifications: stubCapability(
			"native notifications not implemented yet; target Windows toast notifications",
		),
		GlobalHotkeys: stubCapability(
			"global hotkeys not implemented yet; target RegisterHotKey",
		),
		KeyboardEventTap: stubCapability(
			"keyboard event tap not implemented yet; target low-level keyboard hooks",
		),
		AppWatcher: stubCapability(
			"app watcher not implemented yet; target Win32 foreground-window notifications",
		),
		DarkModeDetection: stubCapability(
			"dark mode detection not implemented yet; target Windows personalization APIs",
		),
	}
}
