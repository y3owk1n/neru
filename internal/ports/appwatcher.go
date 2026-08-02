package ports

// AppEventCallback receives an application lifecycle event.
//
// appName is a human-readable name and bundleID is the platform application
// identifier — a bundle ID on macOS, WM_CLASS (X11) or app_id (Wayland) on
// Linux. bundleID is the value per-app configuration keys on, so it is the
// field implementations must get right; appName may be empty where the
// platform exposes no separate display name.
type AppEventCallback func(appName string, bundleID string)

// AppWatcherPort reports which application has focus, so per-app configuration
// and hotkeys can follow the user between windows.
//
// Not every event is available everywhere, and callers must not assume one is:
//
//   - Activate / Deactivate / ScreenParametersChanged are the portable core.
//   - Launch and Terminate are macOS-only (NSWorkspace publishes them; neither
//     AT-SPI nor Win32 has an equivalent).
//   - MissionControlActivated / MissionControlDeactivated are macOS-only by
//     definition — Mission Control is a macOS feature.
//
// A backend with no source for an event simply never fires its callbacks. That
// is a documented degrade rather than an error, because there is nothing to
// report and nothing for the caller to do about it; the app_watcher entry in
// PlatformCapabilities carries the per-platform detail.
//
// Registration is additive and safe to call from any goroutine. Callbacks fire
// on the watcher's own goroutine, never on the event-tap thread.
type AppWatcherPort interface {
	// Start begins monitoring. Calling it twice is a no-op.
	Start()

	// Stop halts monitoring. No callbacks fire after it returns.
	Stop()

	// OnActivate registers a callback for an application gaining focus.
	OnActivate(callback AppEventCallback)

	// OnDeactivate registers a callback for an application losing focus.
	OnDeactivate(callback AppEventCallback)

	// OnTerminate registers a callback for an application exiting.
	// macOS-only; never fires on Linux or Windows.
	OnTerminate(callback AppEventCallback)

	// OnScreenParametersChanged registers a callback for display
	// configuration changes (resolution, arrangement, monitor hotplug).
	OnScreenParametersChanged(callback func())

	// OnMissionControlActivated registers a callback for Mission Control
	// opening. macOS-only.
	OnMissionControlActivated(callback func())

	// OnMissionControlDeactivated registers a callback for Mission Control
	// closing. macOS-only.
	OnMissionControlDeactivated(callback func())

	// SetMCDetection enables or disables Mission Control detection. On
	// platforms without Mission Control this is an accepted no-op.
	SetMCDetection(enabled bool)
}
