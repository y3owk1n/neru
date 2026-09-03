// Package appwatcher monitors application lifecycle events (launch, terminate,
// activate, deactivate, screen change) and dispatches them to registered callbacks.
//
// The platform-specific event source is abstracted behind build-tagged dispatch
// files (platform_darwin.go / platform_linux.go / platform_windows.go, with
// platform_other.go as the no-op fallback), so this package compiles on all
// platforms. On macOS the events come from the Objective-C NSWorkspace
// observer, on Linux from a compositor or X11 focus source, and on Windows
// from an EVENT_SYSTEM_FOREGROUND hook.
package appwatcher
