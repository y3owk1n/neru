//go:build !darwin

package native

// supportsSupplementaryElements reports whether the platform exposes the
// macOS-specific auxiliary surfaces (Dock, menu bar, Notification Center, Stage
// Manager, Picture-in-Picture, screen-capture UI). They exist only on macOS, so
// hints skip them elsewhere instead of probing for nonexistent system apps by
// bundle ID — which otherwise just logs "application not found" warnings.
func supportsSupplementaryElements() bool { return false }
