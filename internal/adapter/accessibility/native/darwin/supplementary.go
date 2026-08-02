//go:build darwin

package darwin

// SupportsSupplementaryElements reports that macOS exposes the auxiliary
// surfaces hints can also scan: the Dock, the menu bar, Notification Center,
// Stage Manager and Picture-in-Picture.
func SupportsSupplementaryElements() bool { return true }
