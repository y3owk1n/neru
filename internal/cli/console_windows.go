//go:build windows

package cli

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	procFreeConsole           = kernel32.NewProc("FreeConsole")
)

// consoleProcessCount returns how many processes are attached to this process's
// console, or zero when it has none.
//
// neru.exe is built for the console subsystem so that every subcommand can
// write to the terminal it was invoked from. The trade-off is that the Windows
// shell also allocates a console when it starts the binary — from Explorer, a
// Start Menu shortcut, or the autostart Run key. The attached-process count
// tells the two apart: a terminal has at least the shell attached alongside
// Neru, whereas a shell-allocated console has only Neru.
func consoleProcessCount() uintptr {
	// Two slots is enough to distinguish "only us" from "more than us"; the
	// call reports the true total even when the buffer cannot hold it.
	var pids [2]uint32

	count, _, _ := procGetConsoleProcessList.Call(
		uintptr(unsafe.Pointer(&pids[0])),
		uintptr(len(pids)),
	)

	return count
}

// startedByShell reports whether Neru was started by the Windows shell rather
// than typed into a terminal: either it owns its console outright, or it has no
// console at all because a parent created it detached.
func startedByShell() bool {
	return consoleProcessCount() <= 1
}

// ownsConsole reports whether Neru has a console that was allocated for this
// process alone, and so is Neru's to close.
func ownsConsole() bool {
	return consoleProcessCount() == 1
}

// detachConsoleIfOwned releases a console that Windows allocated for this
// process, so the daemon does not leave a stray window on screen for the rest
// of the session. It deliberately does nothing when Neru was started from a
// terminal: that console belongs to the user, and `neru launch` in a terminal
// should keep printing there.
func detachConsoleIfOwned() {
	if !ownsConsole() {
		return
	}

	if ret, _, _ := procFreeConsole.Call(); ret == 0 {
		return
	}

	// The inherited standard handles refer to the console that just went away.
	// Point them at NUL so later writes are no-ops rather than errors.
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return
	}

	os.Stdin = devNull
	os.Stdout = devNull
	os.Stderr = devNull
}
