//go:build linux

package linux

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Inspecting a process by PID is display-server agnostic: procfs serves X11 and
// every Wayland backend alike, without CGO. Only "which window is focused"
// differs — see system_x11_cgo.go and system_focused_pid.go.
//
// linuxApplicationNameByPID returns the process name from /proc/<pid>/comm.
func linuxApplicationNameByPID(pid int) (string, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
	if err != nil {
		return "", derrors.Wrapf(
			err,
			derrors.CodeActionFailed,
			"failed to read /proc/%d/comm",
			pid,
		)
	}

	return strings.TrimSpace(string(data)), nil
}

// linuxApplicationBundleIDByPID derives a stable identifier for the process
// from argv[0] in /proc/<pid>/cmdline, falling back to the process name when
// cmdline is empty (e.g. kernel threads).
func linuxApplicationBundleIDByPID(pid int) (string, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return "", derrors.Wrapf(
			err,
			derrors.CodeActionFailed,
			"failed to read /proc/%d/cmdline",
			pid,
		)
	}

	parts := strings.Split(string(data), "\x00")
	if len(parts) == 0 || parts[0] == "" {
		return linuxApplicationNameByPID(pid)
	}

	return filepath.Base(parts[0]), nil
}
