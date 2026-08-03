// Package systray defines Neru's tray menu: which entries exist, what they
// say, and what each does. That is application policy; the tray mechanism is a
// platform capability behind ports.SystrayPort, implemented in
// internal/adapter/systray. This package receives the port and builds a menu
// against it, naming no backend and taking its version from buildinfo.
package systray
