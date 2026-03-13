// Package electron provides enhanced accessibility support for Electron, Chromium,
// and Firefox-based applications that don't expose full accessibility information
// by default.
//
// On macOS this works by toggling AXManualAccessibility / AXEnhancedUserInterface
// attributes via the platform accessibility API. The platform-specific call is
// isolated in platform_darwin.go / platform_stub.go so this package compiles
// on all platforms.
package electron
