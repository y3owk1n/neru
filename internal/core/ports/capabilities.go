package ports

// FeatureStatus describes whether a feature is available on the current platform.
type FeatureStatus string

const (
	// FeatureStatusSupported means the feature is implemented and expected to work.
	FeatureStatusSupported FeatureStatus = "supported"
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
	return c.Status == FeatureStatusSupported
}

// PlatformCapabilities describes the current platform support surface.
type PlatformCapabilities struct {
	Platform          string
	Process           FeatureCapability
	Screen            FeatureCapability
	Cursor            FeatureCapability
	Accessibility     FeatureCapability
	Overlay           FeatureCapability
	Notifications     FeatureCapability
	GlobalHotkeys     FeatureCapability
	KeyboardEventTap  FeatureCapability
	AppWatcher        FeatureCapability
	DarkModeDetection FeatureCapability
}

// CapabilityReporter exposes runtime capability information.
type CapabilityReporter interface {
	Capabilities() PlatformCapabilities
}
