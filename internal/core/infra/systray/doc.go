// Package systray provides a cross-platform system tray menu implementation.
// This package wraps platform-specific native code for menu management.
//
// The Objective-C header (systray.h) lives in platform/darwin/ alongside all
// other macOS headers. The .m implementation must remain in this directory
// because CGo only compiles .c/.m files co-located with the Go package.
package systray
