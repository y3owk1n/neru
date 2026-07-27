//go:build windows

package cli

// isRunningFromAppBundle returns true when the binary was launched by the
// Windows shell — double-clicked from Explorer, opened from its Start Menu
// shortcut, or started by the autostart Run key — rather than typed into a
// terminal. In that case the root command auto-launches the daemon, the same
// convention used on macOS when running from a .app bundle.
//
// The signal is that no terminal shares Neru's console; see startedByShell.
func isRunningFromAppBundle() bool {
	return startedByShell()
}
