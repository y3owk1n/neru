//go:build !windows

package cli

// detachConsoleIfOwned is a no-op outside Windows. Consoles are a Windows
// concept; on macOS and Linux the daemon either inherits the terminal it was
// started from or is started by launchd/systemd with no terminal at all.
func detachConsoleIfOwned() {}
