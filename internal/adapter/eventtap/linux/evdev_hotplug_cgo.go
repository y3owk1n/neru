// Hotplug tracking for the evdev capture: an inotify watcher over
// /dev/input that folds newly attached keyboards into the capture.

//go:build linux && cgo

package linux

/*
#include "../../platform/linux/evdev.h"
*/
import "C"

import (
	"errors"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"go.uber.org/zap"
)

// startHotplugWatcher starts an inotify watch on /dev/input/ for keyboards
// plugged in later.
func (capture *waylandEvdevCapture) startHotplugWatcher() {
	if capture == nil {
		return
	}

	inotifyFd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK)
	if err != nil {
		if capture.logger != nil {
			capture.logger.Debug(
				"Inotify init failed, keyboard hotplug detection disabled",
				zap.Error(err),
			)
		}

		return
	}

	_, err = syscall.InotifyAddWatch(inotifyFd, "/dev/input", syscall.IN_CREATE)
	if err != nil {
		_ = syscall.Close(inotifyFd)

		if capture.logger != nil {
			capture.logger.Debug(
				"Inotify add watch failed, keyboard hotplug detection disabled",
				zap.Error(err),
			)
		}

		return
	}

	capture.deviceMu.Lock()
	if capture.inotifyFd != -1 {
		// A watcher is already running, so drop the duplicate.
		capture.deviceMu.Unlock()

		_ = syscall.Close(inotifyFd)

		return
	}

	capture.inotifyFd = inotifyFd
	capture.hotplugStopCh = make(chan struct{})
	capture.deviceMu.Unlock()

	capture.hotplugWg.Add(1)
	go capture.hotplugLoop()
}

// stopHotplugWatcher stops hotplugLoop, closes the inotify fd, and waits for
// the goroutine.
func (capture *waylandEvdevCapture) stopHotplugWatcher() {
	capture.deviceMu.Lock()
	inotifyFd := capture.inotifyFd
	capture.inotifyFd = -1
	capture.deviceMu.Unlock()

	capture.hotplugOnce.Do(func() {
		if capture.hotplugStopCh != nil {
			close(capture.hotplugStopCh)
		}
	})

	if inotifyFd != -1 {
		_ = syscall.Close(inotifyFd)
	}

	capture.hotplugWg.Wait()
}

// hotplugLoop polls inotify with a non-blocking read, rate-limited by a
// ticker so it does not busy-wait on EAGAIN. A stop channel gives a shutdown
// that does not rely on closing the fd.
func (capture *waylandEvdevCapture) hotplugLoop() {
	defer capture.hotplugWg.Done()

	buf := make([]byte, waylandEvdevHotplugBufSize)

	ticker := time.NewTicker(waylandEvdevHotplugPollInterval)
	defer ticker.Stop()

	for {
		nread, err := syscall.Read(capture.inotifyFd, buf)
		if err != nil {
			if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
				select {
				case <-capture.hotplugStopCh:
					return
				case <-ticker.C:
				}

				continue
			}

			return
		}

		capture.handleInotifyEvents(buf[:nread])
	}
}

// handleInotifyEvents parses raw inotify events and reacts to new devices.
func (capture *waylandEvdevCapture) handleInotifyEvents(buf []byte) {
	offset := 0
	for offset+syscall.SizeofInotifyEvent <= len(buf) {
		event := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[offset]))
		nameLen := int(event.Len)

		if nameLen > 0 && event.Mask&syscall.IN_CREATE != 0 {
			nameStart := offset + syscall.SizeofInotifyEvent
			nameEnd := min(nameStart+nameLen, len(buf))
			name := strings.TrimRight(string(buf[nameStart:nameEnd]), "\x00")

			if strings.HasPrefix(name, "event") {
				capture.handleNewDevice(name)
			}
		}

		offset += syscall.SizeofInotifyEvent + nameLen
	}
}

// handleNewDevice adopts a new /dev/input/event* keyboard under the initial
// scan's rules and starts a reader. Another process's uinput keyboard is a
// remapper's output, and the remapper is about to grab its inputs, so the
// physical keyboards are yielded to it, whether it is starting fresh or
// restarting after an exit whose released keyboards the capture took
// (rescan).
func (capture *waylandEvdevCapture) handleNewDevice(name string) {
	// Give udev a moment to populate the node's capabilities.
	time.Sleep(waylandEvdevHotplugSettleDelay)

	adopted, virtual := capture.adopt(filepath.Join("/dev/input", name), "it appeared")
	if adopted && virtual {
		capture.yieldPhysicalKeyboards()
	}
}

// rescan runs after a captured device is gone, looking for keyboards that
// refused the grab earlier and are free now. An exiting remapper releases its
// physical keyboards as its output device disappears. They are existing nodes
// that no inotify event announces, so without this the user's keys would
// bypass the proxy until a restart.
func (capture *waylandEvdevCapture) rescan() {
	// The remapper's fds close in any order, so give its inputs' release a
	// moment to land.
	time.Sleep(waylandEvdevHotplugSettleDelay)

	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return
	}

	for _, path := range paths {
		capture.adopt(path, "a remapper released it")
	}
}

// adopt takes path into the capture if it is an untracked keyboard this
// capture should read, and starts a reader. It reports whether it did, and
// whether the keyboard is another process's uinput device. A keyboard with a
// key down is adopted once idle. reason is for the log. Nothing is adopted
// once the capture is closing, since a reader started then would outlive
// Close's wait, which is why readers are counted under the closing check's
// lock.
func (capture *waylandEvdevCapture) adopt(path string, reason string) (bool, bool) {
	capture.deviceMu.Lock()

	select {
	case <-capture.stopCh:
		capture.deviceMu.Unlock()

		return false, false
	default:
	}

	file, outcome := capture.addDeviceLocked(path)
	if outcome != deviceAdopted {
		if outcome == deviceBusy {
			capture.grabWhenIdleLocked(path)
		}

		capture.deviceMu.Unlock()

		return false, false
	}

	virtual := capture.devices[file].virtual
	capture.startReader(file)
	capture.deviceMu.Unlock()

	if capture.logger != nil {
		capture.logger.Info(
			"Keyboard device captured",
			zap.String("device", path),
			zap.String("reason", reason),
			zap.Bool("virtual", virtual),
		)
	}

	return true, virtual
}
