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
		Platform:          "darwin",
		Process:           supportedCapability("focused app inspection available"),
		Screen:            supportedCapability("screen bounds and display enumeration available"),
		Cursor:            supportedCapability("cursor movement and tracking available"),
		Accessibility:     supportedCapability("macOS accessibility integration available"),
		Overlay:           supportedCapability("native overlays available"),
		Notifications:     supportedCapability("native alerts and notifications available"),
		GlobalHotkeys:     supportedCapability("global hotkeys available"),
		KeyboardEventTap:  supportedCapability("keyboard event tap available"),
		AppWatcher:        supportedCapability("focused-app watcher available"),
		DarkModeDetection: supportedCapability("system dark mode detection available"),
	}
}

// LinuxCapabilities returns the current Linux runtime capabilities.
func LinuxCapabilities() PlatformCapabilities {
	return PlatformCapabilities{
		Platform:          "linux",
		Process:           stubCapability("focused app inspection not implemented yet"),
		Screen:            stubCapability("screen enumeration not implemented yet"),
		Cursor:            stubCapability("cursor movement/tracking not implemented yet"),
		Accessibility:     stubCapability("AT-SPI integration not implemented yet"),
		Overlay:           stubCapability("native overlays not implemented yet"),
		Notifications:     stubCapability("native notifications not implemented yet"),
		GlobalHotkeys:     stubCapability("global hotkeys not implemented yet"),
		KeyboardEventTap:  stubCapability("keyboard event tap not implemented yet"),
		AppWatcher:        stubCapability("app watcher not implemented yet"),
		DarkModeDetection: stubCapability("dark mode detection not implemented yet"),
	}
}

// WindowsCapabilities returns the current Windows runtime capabilities.
func WindowsCapabilities() PlatformCapabilities {
	return PlatformCapabilities{
		Platform:          "windows",
		Process:           stubCapability("focused app inspection not implemented yet"),
		Screen:            stubCapability("screen enumeration not implemented yet"),
		Cursor:            stubCapability("cursor movement/tracking not implemented yet"),
		Accessibility:     stubCapability("UI Automation integration not implemented yet"),
		Overlay:           stubCapability("native overlays not implemented yet"),
		Notifications:     stubCapability("native notifications not implemented yet"),
		GlobalHotkeys:     stubCapability("global hotkeys not implemented yet"),
		KeyboardEventTap:  stubCapability("keyboard event tap not implemented yet"),
		AppWatcher:        stubCapability("app watcher not implemented yet"),
		DarkModeDetection: stubCapability("dark mode detection not implemented yet"),
	}
}
