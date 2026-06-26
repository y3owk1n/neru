//go:build windows

package cli

import "syscall"

// isRunningFromAppBundle returns true when the binary was launched via
// double-click from Explorer (i.e., no console window).
//
// The Windows build uses -H windowsgui (GUI subsystem) so that double-clicking
// does not open a console window. In that subsystem GetConsoleWindow returns
// zero, which tells the root command to auto-launch the daemon — the same
// convention used on macOS when running from a .app bundle.
func isRunningFromAppBundle() bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	hwnd, _, _ := procGetConsoleWindow.Call()

	return hwnd == 0
}
