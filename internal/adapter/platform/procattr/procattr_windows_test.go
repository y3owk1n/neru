//go:build windows

package procattr_test

import (
	"os/exec"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/y3owk1n/neru/internal/adapter/platform/procattr"
)

// TestHideConsole pins both halves of the contract the callers rely on: the
// flag is set, and whatever the caller already put in SysProcAttr survives.
// The window itself is not observable from Go, so the flag is the only thing a
// test can hold on to.
func TestHideConsole(t *testing.T) {
	t.Run("sets the no-window creation flag", func(t *testing.T) {
		command := exec.CommandContext(t.Context(), "cmd", "/c", "exit")

		procattr.HideConsole(command)

		if command.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
			t.Error("CREATE_NO_WINDOW is unset, so Windows would give the child a console window")
		}
	})

	t.Run("keeps the process attributes the caller set", func(t *testing.T) {
		command := exec.CommandContext(t.Context(), "cmd", "/c", "exit")
		command.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		}

		procattr.HideConsole(command)

		if !command.SysProcAttr.HideWindow {
			t.Error("HideWindow was dropped")
		}

		if command.SysProcAttr.CreationFlags&syscall.CREATE_NEW_PROCESS_GROUP == 0 {
			t.Error("the caller's own creation flags were dropped")
		}

		if command.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
			t.Error("CREATE_NO_WINDOW is unset")
		}
	})
}
