//go:build linux && cgo

package linux

/*
#include "evdev.h"
*/
import "C"

import "fmt"

// uinputScrollDeviceError reports why the uinput scroll wheel cannot be
// created, or nil when it can. It builds and destroys the same device the
// scroll path uses, so a node that opens but refuses an ioctl is reported
// too. The errno is the part a user acts on: "permission denied" means a
// udev rule or group, "no such file" means the uinput module is not loaded.
func uinputScrollDeviceError() error {
	created, errno := C.neru_uinput_probe_scroll()
	if created == 0 {
		return fmt.Errorf("%s: %w", uinputDevicePath, errno)
	}

	return nil
}
