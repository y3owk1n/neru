package ax

// Settings is what the accessibility backends read from the user's
// configuration while they walk a tree. The live *config.Config satisfies it,
// so reading one costs a method call and allocates nothing per element.
type Settings interface {
	HintsMaxDepth() int
	ShouldIgnoreClickableCheckForApp(bundleID string) bool
	ShouldEnableVisibleCheckForApp(bundleID string) bool
}

// SettingsSource returns the current Settings, or nil when no configuration is
// loaded. The nil must be untyped, never a nil pointer inside the interface,
// because callers test the result against nil before they read it.
//
// A new configuration must come back as a new value, and the value must be
// comparable, which in practice means a pointer. The darwin backend clears its
// per-process visible-check cache when the value differs from the last one it
// saw. A source that returned the same value across a reload would leave that
// cache stale.
type SettingsSource func() Settings
