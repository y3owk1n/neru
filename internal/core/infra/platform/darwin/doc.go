//go:build darwin

// Package darwin provides macOS-specific platform implementations.
//
// It includes the CGO bridge code that wraps macOS APIs (Accessibility, CoreGraphics, etc.)
// and provides a Go-friendly interface to them. This package is the foundation
// for all macOS-specific integration in Neru.
package darwin
