//go:build linux

package linux

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Process inspection by PID is display-server agnostic: it reads the procfs
// entries the kernel exposes for every process, so the same implementation
// serves X11 and every Wayland backend (and works without CGO). Only the
// "which window is focused" question is display-server specific — see
// system_linux_x11_cgo.go (X11) and system_linux_focused_pid.go (Wayland).

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
