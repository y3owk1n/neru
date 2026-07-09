package config

// macOS system process bundle identifiers for the supplementary hint sources.
// These are fixed Apple identifiers, shared by the accessibility scan and the
// push-based change observer so both target the same processes without drifting.
const (
	// BundleDock is the Dock.
	BundleDock = "com.apple.dock"
	// BundleNotificationCenter is the Notification Center UI agent.
	BundleNotificationCenter = "com.apple.notificationcenterui"
	// BundleWindowManager is the window/stage manager.
	BundleWindowManager = "com.apple.WindowManager"
	// BundlePIPAgent is the Picture-in-Picture agent.
	BundlePIPAgent = "com.apple.PIPAgent"
	// BundleScreenCaptureUI is the screen capture / screenshot UI.
	BundleScreenCaptureUI = "com.apple.screencaptureui"
	// BundleNeru is neru's own bundle identifier, excluded from observation so its
	// overlay redraws cannot trigger a refresh loop.
	BundleNeru = "com.y3owk1n.neru"
)
