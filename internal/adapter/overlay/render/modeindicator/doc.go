// Package modeindicator renders the small on-screen badge naming the active
// mode.
//
// It owns the overlay's style and drawing only; when the badge is shown and
// what it says are decided by the mode indicator service in
// internal/app/services/modeindicator.
//
// Drawing is per-platform (overlay_darwin.go, overlay_linux_*.go,
// overlay_windows.go). This file is untagged so every target has a package
// comment — the comment used to live in overlay_linux_common.go, which meant
// macOS and Windows builds had none.
package modeindicator
