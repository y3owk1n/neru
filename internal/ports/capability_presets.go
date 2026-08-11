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
		TextInput: supportedCapability(
			"native hint-search field available via an NSTextField overlay",
		),
		Vision: supportedCapability(
			"OCR element detection available via the Vision framework and ScreenCaptureKit",
		),
		KeyFeed: supportedCapability("key injection available via CGEventPost"),
		Systray: supportedCapability("tray icon available via NSStatusItem"),
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
				"support; hints.strategy = vision is the OCR fallback where it is thin",
		),
		Overlay: supportedCapability(
			"native overlays available via X11 windows or Wayland layer-shell + Cairo",
		),
		// Live-probed by the Linux SystemAdapter, which downgrades this to a
		// stub when nothing owns the notification name on the session bus.
		// See linux.SystemAdapter.notificationCapability.
		Notifications: supportedCapability(
			"notifications and alerts delivered to the session's freedesktop " +
				"notification daemon over D-Bus (org.freedesktop.Notifications); " +
				"an alert is a critical-urgency notification that stays until " +
				"dismissed, not a modal dialog",
		),
		GlobalHotkeys: supportedCapability(
			"global hotkeys available via X11 XGrabKey; on Wayland via a passive " +
				"evdev listener on /dev/input that honors neru's own keybindings " +
				"(needs input-group access and a cgo build), falling back to " +
				"compositor keybindings otherwise",
		),
		KeyboardEventTap: supportedCapability(
			"keyboard event tap available via X11 grab and Wayland evdev/layer-shell " +
				"keyboard interactivity; modifier passthrough of unbound shortcuts is " +
				"supported on the Wayland evdev backend only (X11's exclusive grab and " +
				"the wl-keyboard fallback cannot re-inject selectively)",
		),
		AppWatcher: supportedCapability(
			"focused-app change detection keyed on the WM_CLASS (X11) or " +
				"wlr-foreign-toplevel app_id (Wayland wlroots/KDE), event-driven " +
				"where the compositor/X11 exposes a focus-change signal and polling " +
				"otherwise; GNOME/Mutter exposes no focused-app source",
		),
		// Default placeholder; the Linux SystemAdapter overrides this with
		// the live-probed state (current color-scheme + source) on each
		// Capabilities() call. See linux.SystemAdapter.Capabilities.
		DarkModeDetection: supportedCapability(
			"dark mode detection via freedesktop appearance portal (Settings.Read), with kdeglobals fallback",
		),
		// What is a stub here is the *field*: no Linux text control owns
		// keyboard focus for hint search, so the characters come from the event
		// tap and an input method never sees them. The query is on screen —
		// the overlay draws the badge — which is why the detail says so rather
		// than leaving "not implemented" to imply nothing appears.
		TextInput: stubCapability(
			"no native hint-search field: the query is read from the event tap's " +
				"key stream, so dead keys and IME composition do not reach it; " +
				"the overlay draws the search badge",
		),
		// Both halves are implemented: capture through wlr-screencopy or
		// XGetImage, recognition through tesseract. The detail names the three
		// things that can still be missing on a given machine — a KWin session
		// has no capture path, the tesseract language data is a separate
		// distribution package from the library Neru links, and the CGO-off
		// build has no engine — because this preset is static and `neru doctor`
		// reports it without probing. VisionPort.Health answers the same
		// question for the live session, and names which one it is.
		Vision: supportedCapability(
			"vision element detection via tesseract OCR over a screen capture " +
				"(wlr-screencopy on wlroots, XGetImage on X11, unavailable on KWin); " +
				"text only, with no rectangle detection; needs a cgo build and the " +
				"tesseract eng language data installed",
		),
		KeyFeed: supportedCapability(
			"key injection via a uinput virtual keyboard when /dev/uinput is " +
				"writable (works on X11, wlroots, and KWin), falling back to " +
				"zwp_virtual_keyboard_v1 on wlroots compositors",
		),
		Systray: supportedCapability(
			"tray icon available via the D-Bus StatusNotifierItem + dbusmenu protocols",
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
		DarkModeDetection: supportedCapability(
			"dark mode detection available via the Windows personalization registry " +
				"(Themes\\Personalize AppsUseLightTheme)",
		),
		TextInput: stubCapability(
			"native hint-search field not implemented yet; hint search falls back " +
				"to the event tap's key stream",
		),
		Vision: stubCapability(
			"no OCR/vision element detection; hints come from UI Automation only",
		),
		KeyFeed: stubCapability(
			"key injection not implemented yet; target SendInput",
		),
		Systray: supportedCapability(
			"tray icon available via the Win32 notification area (Shell_NotifyIcon)",
		),
	}
}
