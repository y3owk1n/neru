//go:build !darwin

package cli

import (
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

func installService() error {
	return derrors.New(derrors.CodeNotSupported, "services install is only supported on macOS")
}

func uninstallService() error {
	return derrors.New(derrors.CodeNotSupported, "services uninstall is only supported on macOS")
}

func startService() error {
	return derrors.New(derrors.CodeNotSupported, "services start is only supported on macOS")
}

func stopService() error {
	return derrors.New(derrors.CodeNotSupported, "services stop is only supported on macOS")
}

func restartService() error {
	return derrors.New(derrors.CodeNotSupported, "services restart is only supported on macOS")
}

func statusService() string {
	return "Service management is not supported on this platform"
}
