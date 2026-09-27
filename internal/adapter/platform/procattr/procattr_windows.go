//go:build windows

package procattr

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// HideConsole keeps Windows from giving the child a console window.
//
// CREATE_NO_WINDOW gives the child a console of its own that never appears on
// screen, so its stdio still works and a caller reading its output through a
// pipe sees no difference. HideConsole leaves SysProcAttr.HideWindow alone on
// purpose. That field sets wShowWindow to SW_HIDE, which reaches a GUI program
// the child goes on to launch and hides its window. An exec step that opens an
// application needs that window.
func HideConsole(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}

	command.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}
