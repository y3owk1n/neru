// Package appwatcher monitors application lifecycle events (launch, terminate,
// activate, deactivate, screen change) and dispatches them to registered callbacks.
//
// The platform-specific event source is abstracted behind build-tagged dispatch
// files (platform_darwin.go / platform_other.go), so this package compiles on
// all platforms. On macOS the events come from the Objective-C NSWorkspace
// observer; on other platforms the watcher is a no-op until implemented.
package appwatcher
