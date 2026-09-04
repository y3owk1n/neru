//go:build !darwin && !linux && !windows

package cli

import (
	"github.com/y3owk1n/neru/internal/derrors"
)

// unsupportedServices is the one sentence every subcommand says on a platform
// with no service manager behind it. macOS drives a launchd agent, Linux a
// systemd user unit and Windows a Task Scheduler task; nothing else does, so
// the refusal names all three rather than leaving a reader to guess which
// platform they are missing out on.
func unsupportedServices(action string) error {
	return derrors.New(
		derrors.CodeNotSupported,
		"services "+action+" is supported on macOS (launchd), Linux (systemd "+
			"user units) and Windows (Task Scheduler) only",
	)
}

func installService() error {
	return unsupportedServices("install")
}

func uninstallService() error {
	return unsupportedServices("uninstall")
}

func startService() error {
	return unsupportedServices("start")
}

func stopService() error {
	return unsupportedServices("stop")
}

func restartService() error {
	return unsupportedServices("restart")
}

func statusService() string {
	return "Service management is not supported on this platform"
}
