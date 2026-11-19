//go:build !darwin

package axobserver

const observerSupported = false

// Platforms without an AX observer implementation get no-op entry points: every
// watch fails, so nothing is ever watched and no change is ever reported. Hints
// still work, they just do not auto-refresh.
//
// To add push-based auto-refresh for a platform, replace these with a
// build-tagged file (for example platform_linux.go) implementing them against
// the OS accessibility API, mirroring platform_darwin.go.

func platformWatch(_ int) bool { return false }

func platformUnwatch() {}

func platformSetChangeHandler(_ func(notif string)) {}
