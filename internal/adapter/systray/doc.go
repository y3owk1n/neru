// Package systray implements ports.SystrayPort over the platform tray backends.
//
// Each backend wraps a single process-wide native tray and exposes it as
// package-level functions rather than a type. Adapter turns that into the
// injectable port, and menuItemAdapter bridges the backend's menu item to
// ports.SystrayMenuItem.
//
// backend_<os>.go aliases the backend's MenuItem and forwards its functions, so
// the adapter and the bridge are written once rather than per platform.
//
// The backends are subpackages: darwin (Cocoa NSStatusItem, via cgo), linux
// (D-Bus StatusNotifierItem with a dbusmenu server), and windows
// (Shell_NotifyIcon). The tray icon they draw lives in the icon subpackage,
// because go:embed only reaches files beneath the embedding package.
package systray
