// Package modeindicator renders the small on-screen badge naming the active
// mode.
//
// It owns the overlay's style and drawing only; when the badge is shown and
// what it says are decided by the mode indicator service in
// internal/app/services/modeindicator.
//
// Drawing is per-platform (overlay_darwin.go, overlay_linux_*.go,
// overlay_windows.go). This file carries no build tag, so the package comment
// is present on every target rather than only the one whose drawing file
// happens to hold it.
package modeindicator
