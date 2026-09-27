//go:build windows

package procattr

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// HideConsole keeps Windows from giving the child a console window.
//
// CREATE_NO_WINDOW runs a console program without a console window, and without
// a console handle either. Windows words it as "the console handle for the
// application is not set". The child's standard handles are still whatever the
// caller wired up, so a caller reading its output through a pipe sees no
// difference. A program that talks to the console itself, rather than to its
// standard handles, finds none. Windows ignores the flag for a program that is
// not a console application.
//
// HideConsole leaves SysProcAttr.HideWindow alone on purpose. That field sets
// wShowWindow to SW_HIDE, which reaches a GUI program the child goes on to
// launch and hides its window. An exec step that opens an application needs
// that window.
func HideConsole(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}

	command.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}
