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

// startHotplugWatcher starts an inotify watch on /dev/input/ to detect new
// keyboard devices being plugged in after initial capture creation.
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
		// A watcher is already running; clean up the duplicate.
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

// stopHotplugWatcher signals the hotplugLoop goroutine to stop via the stop
// channel, closes the inotify fd, then waits for the goroutine to finish.
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

// hotplugLoop polls for inotify events with a non-blocking read and
// rate-limits the poll via a ticker so the goroutine does not busy-wait
// on EAGAIN.  A dedicated stop channel provides an interruptible shutdown
// that does not rely on closing the inotify fd to unblock the read.
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

// handleInotifyEvents parses raw inotify event bytes and reacts to new device
// creation events.
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

// handleNewDevice opens a newly created /dev/input/event* device and, if it
// is a keyboard this capture should read, adds it under the same rules as the
// initial scan — grabbed and seeded when the capture grabs — and starts a
// reader for it. A uinput keyboard from another process is a remapper's output
// device, and the remapper is about to grab its inputs: the physical keyboards
// are yielded to it, whether it is starting for the first time with this
// capture already up, or starting again after an exit that had the capture
// take the keyboards it released (rescan).
func (capture *waylandEvdevCapture) handleNewDevice(name string) {
	// Give udev a moment to fully initialize the device node and populate
	// the input capabilities before we interrogate it.
	time.Sleep(waylandEvdevHotplugSettleDelay)

	adopted, virtual := capture.adopt(filepath.Join("/dev/input", name), "it appeared")
	if adopted && virtual {
		capture.yieldPhysicalKeyboards()
	}
}

// rescan runs after a captured device is gone, looking for keyboards that
// refused the grab earlier and are free now. A remapper (kanata, keyd) that
// exits releases the physical keyboards it held in the same instant its output
// device, the one this capture was reading, disappears; the released keyboards
// are existing nodes, so no inotify event announces them, and without this the
// user's keys would bypass the proxy until the daemon restarted.
func (capture *waylandEvdevCapture) rescan() {
	// The remapper's fds close in whatever order its process teardown picks;
	// give the release of its inputs a moment to land before trying them.
	time.Sleep(waylandEvdevHotplugSettleDelay)

	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return
	}

	for _, path := range paths {
		capture.adopt(path, "a remapper released it")
	}
}

// adopt takes path into the capture if it is a keyboard this capture should
// read and is not tracked already, and starts a reader for it. It reports
// whether it did, and whether the keyboard is another process's uinput device.
// A keyboard with a key down is adopted once it is idle instead. reason is
// what brought the path here, for the log. Nothing is adopted once the capture
// is closing: a reader started then would outlive the wait Close makes on the
// readers, which is why the reader is counted under the same lock the closing
// check takes.
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
