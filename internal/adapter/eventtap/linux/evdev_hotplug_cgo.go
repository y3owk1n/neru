// Hotplug tracking for the evdev capture: an inotify watcher over
// /dev/input that folds newly attached keyboards into an active grab.

//go:build linux && cgo

package linux

/*
#include "../../platform/linux/evdev.h"
#include "../../platform/linux/wayland_keymap.h"
*/
import "C"

import (
	"errors"
	"os"
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
			nameEnd := nameStart + nameLen
			nameEnd = min(nameEnd, len(buf))
			name := strings.TrimRight(string(buf[nameStart:nameEnd]), "\x00")
			if strings.HasPrefix(name, "event") {
				capture.handleNewDevice(name)
			}
		}
		offset += syscall.SizeofInotifyEvent + nameLen
	}
}

// handleNewDevice opens a newly created /dev/input/event* device and, if it
// is a keyboard, adds it to the capture and starts a reader goroutine. If
// the capture is currently in a grabbed state, the new device is also grabbed
// immediately so Neru stays in full control.
func (capture *waylandEvdevCapture) handleNewDevice(name string) {
	// Give udev a moment to fully initialize the device node and populate
	// the input capabilities before we interrogate it.
	time.Sleep(waylandEvdevHotplugSettleDelay)

	path := filepath.Join("/dev/input", name)
	file, err := os.Open(path)
	if err != nil {
		return
	}

	fd := C.int(file.Fd())
	if C.neru_evdev_is_keyboard(fd) == 0 {
		_ = file.Close()

		return
	}

	capture.deviceMu.Lock()

	// Avoid duplicates: the device might already be tracked if the inotify
	// event fired for a device that was open at initial scan time (unlikely
	// but possible on some kernels).
	for _, f := range capture.files {
		if f.Name() == path {
			capture.deviceMu.Unlock()
			_ = file.Close()

			return
		}
	}

	// If the capture is currently grabbed, grab the new device under the
	// same lock so Disable cannot race ahead and ungrab before we finish.
	if capture.grabbed && C.neru_evdev_grab(C.int(file.Fd()), 1) != 0 {
		capture.deviceMu.Unlock()
		_ = file.Close()

		return
	}

	capture.files = append(capture.files, file)
	capture.deviceMu.Unlock()

	capture.startReader(file)

	if capture.logger != nil {
		capture.logger.Info(
			"New keyboard device detected and captured",
			zap.String("device", path),
		)
	}
}
