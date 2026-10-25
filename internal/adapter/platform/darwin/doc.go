// Package darwin provides macOS-specific platform implementations.
//
// It holds the cgo bridge that wraps the macOS APIs (Accessibility,
// CoreGraphics, and others) for the rest of Neru.
//
// All functional code in this package carries //go:build darwin. This file
// is intentionally untagged so that `go vet ./...` and other analysis tools
// can resolve the package on every OS without hitting
// "build constraints exclude all Go files".
package darwin
