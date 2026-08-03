// Package stickyindicator renders the badge showing which modifiers are
// currently held sticky.
//
// It owns the overlay's style and drawing only; which modifiers are sticky is
// tracked by the mode handler's modifier state.
//
// Drawing is per-platform (overlay_darwin.go, overlay_linux_*.go,
// overlay_windows.go). This file carries no build tag, so the package comment
// is present on every target rather than only the one whose drawing file
// happens to hold it.
package stickyindicator
