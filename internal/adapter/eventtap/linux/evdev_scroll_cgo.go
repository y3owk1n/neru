// The uinput scroll output device, not input capture. Scroll actions go
// through a virtual mouse wheel straight to the focused app.

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
	"time"
)

// uinputScrollSettleDelay is how long a new scroll wheel waits before its
// first event. The compositor adds a new input device asynchronously, through
// udev and libinput, and drops what the device sends before then. Without the
// wait, the scroll that created the wheel never arrives.
const uinputScrollSettleDelay = 100 * time.Millisecond

var (
	errUinputScrollUnavailable = errors.New("uinput scroll device unavailable")
	errUinputScrollSend        = errors.New("failed to send uinput scroll event")
)

var (
	uinputScrollOnce sync.Once
	uinputScrollFd   int
	errUinputScroll  error
)

// uinputScrollMu serializes writes to the shared scroll device fd, so batches
// from concurrent IPC goroutines reach the device one after another.
var uinputScrollMu sync.Mutex

func initUinputScroll() error {
	var deviceFd C.int

	created, errno := C.neru_uinput_create_scroll(&deviceFd)
	if created == 0 {
		// errno is what the user acts on: "permission denied" means a udev
		// rule or group, "no such file" means the uinput module is not loaded.
		return fmt.Errorf("%w: /dev/uinput: %w", errUinputScrollUnavailable, errno)
	}

	uinputScrollFd = int(deviceFd)

	time.Sleep(uinputScrollSettleDelay)

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

// WarmScrollDevice creates the uinput scroll wheel at daemon start, so the
// settle wait a new wheel needs happens then and not during the first scroll.
func WarmScrollDevice() error {
	_, err := getUinputScrollFd()

	return err
}

// IsUinputScrollAvailable returns true if uinput scroll is available.
func IsUinputScrollAvailable() bool {
	_, _ = getUinputScrollFd()

	return errUinputScroll == nil
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
