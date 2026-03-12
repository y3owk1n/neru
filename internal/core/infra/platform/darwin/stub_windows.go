// This file is intentionally excluded from all builds (//go:build ignore).
//
// The darwin package is only compiled on macOS — every real .go file carries
// //go:build darwin. No other platform should ever import this package.
// If you see a compile error pointing here, a non-darwin file is importing
// internal/core/infra/platform/darwin. Fix that import by using a
// platform_darwin.go / platform_stub.go dispatch pair instead.
// See docs/ARCHITECTURE_CROSS_PLATFORM.md.

//go:build ignore

package darwin
