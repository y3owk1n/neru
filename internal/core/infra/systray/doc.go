// Package systray provides the system tray menu implementation.
//
// Platform-specific files:
//   - systray_darwin.go + systray.m  — macOS Cocoa implementation (CGo)
//   - systray_linux.go               — Linux stub (no-op until contributed)
//   - systray_windows.go             — Windows stub (no-op until contributed)
//
// Note: The Objective-C header (systray.h) lives in platform/darwin/ alongside
// all other macOS headers. The .m implementation must remain in this directory
// because CGo only compiles .c/.m files co-located with the importing Go package.
package systray
