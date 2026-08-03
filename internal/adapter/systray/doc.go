// Package systray implements ports.SystrayPort over the platform tray
// backends: darwin (NSStatusItem via cgo), linux (D-Bus StatusNotifierItem
// with dbusmenu), windows (Shell_NotifyIcon). Each backend is a process-wide
// native tray exposed as functions; backend_<os>.go aliases them so Adapter
// and the menu bridge are written once. The icon lives in its own subpackage
// because go:embed only reaches files beneath the embedding package.
package systray
