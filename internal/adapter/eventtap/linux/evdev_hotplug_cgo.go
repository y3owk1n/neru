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
// reader for it. A device that comes back under the name of one whose exit
// had this capture borrow keyboards gets those keyboards back.
func (capture *waylandEvdevCapture) handleNewDevice(name string) {
	// Give udev a moment to fully initialize the device node and populate
	// the input capabilities before we interrogate it.
	time.Sleep(waylandEvdevHotplugSettleDelay)

	deviceName, ok := capture.adopt(filepath.Join("/dev/input", name), "")
	if !ok {
		return
	}

	capture.returnBorrowed(deviceName)
}

// rescan runs after a captured device named vanished is gone, looking for
// keyboards that refused the grab earlier and are free now. A remapper (kanata,
// keyd) that exits releases the physical keyboards it held in the same instant
// its output device, the one this capture was reading, disappears; the released
// keyboards are existing nodes, so no inotify event announces them, and without
// this the user's keys would bypass the proxy until the daemon restarted. What
// it adopts is borrowed against the vanished name (returnBorrowed).
func (capture *waylandEvdevCapture) rescan(vanished string) {
	// The remapper's fds close in whatever order its process teardown picks;
	// give the release of its inputs a moment to land before trying them.
	time.Sleep(waylandEvdevHotplugSettleDelay)

	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return
	}

	for _, path := range paths {
		capture.adopt(path, vanished)
	}
}

// adopt takes path into the capture if it is a keyboard this capture should
// read and is not tracked already, starts a reader for it, and answers the
// device's name. A keyboard with a key down is adopted once it is idle instead.
// borrowedFor names the vanished device a rescan is adopting on behalf of, or
// is empty. Nothing is adopted once the capture is closing: a reader started
// then would outlive the wait Close makes on the readers, which is why the
// reader is counted under the same lock the closing check takes.
func (capture *waylandEvdevCapture) adopt(path string, borrowedFor string) (string, bool) {
	capture.deviceMu.Lock()

	select {
	case <-capture.stopCh:
		capture.deviceMu.Unlock()

		return "", false
	default:
	}

	file, outcome := capture.addDeviceLocked(path)
	if outcome != deviceAdopted {
		if outcome == deviceBusy {
			capture.grabWhenIdleLocked(path, borrowedFor)
		}

		capture.deviceMu.Unlock()

		return "", false
	}

	device := capture.devices[file]
	device.borrowedFor = borrowedFor
	capture.devices[file] = device
	capture.startReader(file)
	capture.deviceMu.Unlock()

	if capture.logger != nil {
		capture.logger.Info(
			"New keyboard device detected and captured",
			zap.String("device", path),
			zap.Bool("borrowed", borrowedFor != ""),
		)
	}

	return device.name, true
}

// returnBorrowed hands back every keyboard borrowed against name: the remapper
// whose output device just came back is about to grab its inputs again, and a
// keyboard this capture still held would refuse it. Each is ungrabbed and
// revoked, which wakes its reader with ENODEV so the reader closes it on its own
// fd (readLoop); the release is recorded so that exit starts no rescan.
func (capture *waylandEvdevCapture) returnBorrowed(name string) {
	capture.deviceMu.Lock()

	returned := 0

	for file, device := range capture.devices {
		if device.borrowedFor != name {
			continue
		}

		device.borrowedFor = ""
		device.released = true
		capture.devices[file] = device

		fd := C.int(file.Fd())
		C.neru_evdev_grab(fd, 0)
		C.neru_evdev_revoke(fd)

		returned++
	}

	capture.deviceMu.Unlock()

	if returned > 0 && capture.logger != nil {
		capture.logger.Info(
			"Returned keyboards to a remapper whose output device came back",
			zap.Int("keyboards", returned),
		)
	}
}
