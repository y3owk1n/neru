//go:build darwin

package accessibility

// supportsSupplementaryElements reports whether the platform exposes the
// macOS-specific auxiliary surfaces that hints can additionally scan — the menu
// bar, Dock, Notification Center, Stage Manager, Picture-in-Picture, and the
// screen-capture UI. These are resolved by bundle ID through the accessibility
// API and exist only on macOS.
func supportsSupplementaryElements() bool { return true }
