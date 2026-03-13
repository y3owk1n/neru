// Package systray provides the system tray icon and menu for Neru.
//
// It exposes controls for application status, mode activation, and
// configuration management. The underlying systray infrastructure
// (internal/core/infra/systray) is platform-specific; on macOS it uses
// Cocoa, on Linux/Windows the implementation is a stub until contributed.
package systray
