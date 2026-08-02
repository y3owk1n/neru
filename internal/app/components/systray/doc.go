// Package systray defines Neru's tray menu: which entries exist, what they
// say, and what each one does.
//
// That is application policy, so it lives here. The tray *mechanism* is a
// platform capability behind ports.SystrayPort — NSStatusItem on macOS, the
// D-Bus StatusNotifierItem + dbusmenu protocols on Linux, and Shell_NotifyIcon
// on Windows, all implemented in internal/adapter/systray. This package
// names none of them: it receives the port and builds a menu against it.
//
// Two couplings this package deliberately does not have:
//
//   - It does not import internal/adapter/systray. It used to, which made
//     the menu depend on a concrete tray backend and left it untestable.
//   - It does not import internal/cli for the version string. Build identity
//     comes from internal/buildinfo, so an application component no longer
//     depends on the outermost command layer.
package systray
