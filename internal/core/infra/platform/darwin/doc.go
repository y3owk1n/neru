// Package darwin provides macOS-specific platform implementations.
//
// It includes the CGO bridge code that wraps macOS APIs (Accessibility, CoreGraphics, etc.)
// and provides a Go-friendly interface to them. This package is the foundation
// for all macOS-specific integration in Neru.
//
// All functional code in this package carries //go:build darwin. This file
// is intentionally untagged so that `go vet ./...` and other analysis tools
// can resolve the package on every OS without hitting
// "build constraints exclude all Go files".
package darwin
