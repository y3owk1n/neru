//go:build linux && !cgo

package linux

import "github.com/y3owk1n/neru/internal/derrors"

// uinputScrollDeviceError reports that the uinput scroll wheel is compiled
// out: the device is created through cgo.
func uinputScrollDeviceError() error {
	return derrors.New(
		derrors.CodeNotSupported,
		"uinput scroll device unavailable: this binary was built with CGO_ENABLED=0",
	)
}
