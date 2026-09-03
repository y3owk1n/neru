// The uinput scroll OUTPUT device. Unlike everything else in the evdev
// files this is not input capture: scroll actions are injected into the
// session through a virtual mouse wheel so they reach the focused app
// directly instead of being drawn through the overlay.

//go:build linux && cgo

package linux

/*
#include "../../platform/linux/evdev.h"
#include "../../platform/linux/wayland_keymap.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"sync"
)

var (
	errUinputScrollUnavailable = errors.New("uinput scroll device unavailable")
	errUinputScrollSend        = errors.New("failed to send uinput scroll event")
)

var (
	uinputScrollOnce sync.Once
	uinputScrollFd   int
	errUinputScroll  error
)

// uinputScrollMu serializes writes to the shared scroll device fd. A single
// ScrollDeviceScroll emits three separate write() calls (hi-res, lo-res, SYN);
// without this lock, two concurrent scroll requests (each on its own IPC
// goroutine) could interleave those writes and deliver a malformed event
// stream.
var uinputScrollMu sync.Mutex

func initUinputScroll() error {
	var deviceFd C.int

	created, errno := C.neru_uinput_create_scroll(&deviceFd)
	if created == 0 {
		// errno is the /dev/uinput open failure, which is what a user has to
		// act on: "permission denied" means a udev rule or group, "no such
		// file" means the uinput module is not loaded.
		return fmt.Errorf("%w: /dev/uinput: %w", errUinputScrollUnavailable, errno)
	}

	uinputScrollFd = int(deviceFd)

	return nil
}

func getUinputScrollFd() (int, error) {
	uinputScrollOnce.Do(func() {
		errUinputScroll = initUinputScroll()
	})
	if errUinputScroll != nil {
		return 0, errUinputScroll
	}

	return uinputScrollFd, nil
}

// IsUinputScrollAvailable returns true if uinput scroll is available.
func IsUinputScrollAvailable() bool {
	_, _ = getUinputScrollFd()

	return errUinputScroll == nil
}

// ScrollDeviceScroll sends a scroll event via the uinput virtual device.
func ScrollDeviceScroll(axis, value int) error {
	scrollFd, err := getUinputScrollFd()
	if err != nil {
		return err
	}

	uinputScrollMu.Lock()
	defer uinputScrollMu.Unlock()

	if C.neru_uinput_scroll(C.int(scrollFd), C.int(axis), C.int(value)) == 0 {
		return fmt.Errorf("%w", errUinputScrollSend)
	}

	return nil
}

// ScrollDeviceScrollBatch sends multiple scroll events in a single write.
func ScrollDeviceScrollBatch(axis int, values []int) error {
	if len(values) == 0 {
		return nil
	}

	ufd, err := getUinputScrollFd()
	if err != nil {
		return err
	}

	cValues := make([]C.int, len(values))
	for i, v := range values {
		cValues[i] = C.int(v)
	}

	uinputScrollMu.Lock()
	defer uinputScrollMu.Unlock()

	if C.neru_uinput_scroll_batch(
		C.int(ufd),
		C.int(axis),
		&cValues[0],
		C.int(len(values)),
	) == 0 { //nolint:lll
		return fmt.Errorf("%w", errUinputScrollSend)
	}

	return nil
}
