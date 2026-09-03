package ports

// FeatureStatus describes whether a feature is available on the current platform.
type FeatureStatus string

const (
	// FeatureStatusSupported means the feature is implemented and expected to work.
	FeatureStatusSupported FeatureStatus = "supported"
	// FeatureStatusHeadless means the feature is intentionally disabled in a healthy headless/no-op environment.
	FeatureStatusHeadless FeatureStatus = "headless"
	// FeatureStatusStub means the feature is intentionally stubbed and not yet implemented.
	FeatureStatusStub FeatureStatus = "stub"
)

// FeatureCapability describes support for a single platform capability.
type FeatureCapability struct {
	Status FeatureStatus
	Detail string
}

// Supported returns true when the feature is implemented on the current platform.
func (c FeatureCapability) Supported() bool {
	return c.Status == FeatureStatusSupported || c.Status == FeatureStatusHeadless
}

// CapabilityKey is the stable wire identifier for one platform capability.
//
// These keys are what `neru doctor` prints and what the IPC info response
// serializes, so they are part of the external contract: renaming one breaks
// every consumer that parses the output. Add keys freely; rename them never.
type CapabilityKey string

// Capability keys, in the canonical order Entries reports them.
const (
	// CapabilityProcess covers focused-application identification.
	CapabilityProcess CapabilityKey = "process"
	// CapabilityScreen covers display enumeration and bounds.
	CapabilityScreen CapabilityKey = "screen"
	// CapabilityCursor covers cursor movement and position tracking.
	CapabilityCursor CapabilityKey = "cursor"
	// CapabilityScroll covers scroll injection at the cursor.
	CapabilityScroll CapabilityKey = "scroll"
	// CapabilityAccessibility covers clickable-element discovery.
	CapabilityAccessibility CapabilityKey = "accessibility"
	// CapabilityOverlay covers native overlay windows.
	CapabilityOverlay CapabilityKey = "overlay"
	// CapabilityNotifications covers native alerts and toast notifications.
	CapabilityNotifications CapabilityKey = "notifications"
	// CapabilityGlobalHotkeys covers global hotkey registration.
	CapabilityGlobalHotkeys CapabilityKey = "global_hotkeys"
	// CapabilityKeyboardEventTap covers in-mode keyboard capture.
	CapabilityKeyboardEventTap CapabilityKey = "keyboard_event_tap"
	// CapabilityAppWatcher covers focused-application change detection.
	CapabilityAppWatcher CapabilityKey = "app_watcher"
	// CapabilityDarkModeDetection covers system dark-mode detection.
	CapabilityDarkModeDetection CapabilityKey = "dark_mode_detection"
	// CapabilityTextInput covers the native hint-search text field.
	CapabilityTextInput CapabilityKey = "text_input"
	// CapabilityVision covers OCR/vision-based element detection.
	CapabilityVision CapabilityKey = "vision"
	// CapabilityKeyFeed covers synthetic key injection into the focused app.
	CapabilityKeyFeed CapabilityKey = "key_feed"
	// CapabilitySystray covers the system tray / notification-area icon.
	CapabilitySystray CapabilityKey = "systray"
)

// PlatformCapabilities describes the current platform support surface.
//
// Every FeatureCapability field must also be listed in Entries — the registry
// is what `neru doctor` and the IPC info response iterate, and
// TestPlatformCapabilities_EntriesCoverEveryField fails if a field is added
// without registering it.
type PlatformCapabilities struct {
	Platform          string
	Process           FeatureCapability
	Screen            FeatureCapability
	Cursor            FeatureCapability
	Scroll            FeatureCapability
	Accessibility     FeatureCapability
	Overlay           FeatureCapability
	Notifications     FeatureCapability
	GlobalHotkeys     FeatureCapability
	KeyboardEventTap  FeatureCapability
	AppWatcher        FeatureCapability
	DarkModeDetection FeatureCapability
	TextInput         FeatureCapability
	Vision            FeatureCapability
	KeyFeed           FeatureCapability
	Systray           FeatureCapability
}

// CapabilityEntry pairs a capability with its stable wire key so callers can
// render or serialize the whole surface without restating the field list.
type CapabilityEntry struct {
	FeatureCapability

	// Key is the stable wire identifier for this capability.
	Key CapabilityKey
	// Field is the PlatformCapabilities struct field this entry reads. It
	// exists so the coverage test can prove the registry is exhaustive.
	Field string
}

// Entries returns every capability in a stable, canonical order.
//
// This is the single source of truth for the capability list. Renderers
// (`neru doctor`, the IPC info map) iterate it rather than spelling out the
// fields, so a new capability reaches every consumer by being added here once.
func (c PlatformCapabilities) Entries() []CapabilityEntry {
	return []CapabilityEntry{
		{Key: CapabilityProcess, Field: "Process", FeatureCapability: c.Process},
		{Key: CapabilityScreen, Field: "Screen", FeatureCapability: c.Screen},
		{Key: CapabilityCursor, Field: "Cursor", FeatureCapability: c.Cursor},
		{Key: CapabilityScroll, Field: "Scroll", FeatureCapability: c.Scroll},
		{
			Key:               CapabilityAccessibility,
			Field:             "Accessibility",
			FeatureCapability: c.Accessibility,
		},
		{Key: CapabilityOverlay, Field: "Overlay", FeatureCapability: c.Overlay},
		{
			Key:               CapabilityNotifications,
			Field:             "Notifications",
			FeatureCapability: c.Notifications,
		},
		{
			Key:               CapabilityGlobalHotkeys,
			Field:             "GlobalHotkeys",
			FeatureCapability: c.GlobalHotkeys,
		},
		{
			Key:               CapabilityKeyboardEventTap,
			Field:             "KeyboardEventTap",
			FeatureCapability: c.KeyboardEventTap,
		},
		{Key: CapabilityAppWatcher, Field: "AppWatcher", FeatureCapability: c.AppWatcher},
		{
			Key:               CapabilityDarkModeDetection,
			Field:             "DarkModeDetection",
			FeatureCapability: c.DarkModeDetection,
		},
		{Key: CapabilityTextInput, Field: "TextInput", FeatureCapability: c.TextInput},
		{Key: CapabilityVision, Field: "Vision", FeatureCapability: c.Vision},
		{Key: CapabilityKeyFeed, Field: "KeyFeed", FeatureCapability: c.KeyFeed},
		{Key: CapabilitySystray, Field: "Systray", FeatureCapability: c.Systray},
	}
}
