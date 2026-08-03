// Evdev capture lifecycle: device scanning, reader goroutines, and the
// exclusive grab/ungrab dance, plus modifier-state queries against the
// captured devices.

//go:build linux && cgo

package linux

/*
#include "../../platform/linux/evdev.h"
#include "../../platform/linux/wayland_keymap.h"
*/
import "C"

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"go.uber.org/zap"
)

func newWaylandEvdevCapture(logger *zap.Logger) (*waylandEvdevCapture, error) {
	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, err
	}

	capture := &waylandEvdevCapture{
		files:     make([]*os.File, 0, len(paths)),
		events:    make(chan waylandEvdevEvent, waylandEvdevEventBufferSize),
		logger:    logger,
		inotifyFd: -1,
	}

	for _, path := range paths {
		file, openErr := os.Open(path)
		if openErr != nil {
			continue
		}

		fileDescriptor := C.int(file.Fd())
		if C.neru_evdev_is_keyboard(fileDescriptor) == 0 {
			_ = file.Close()

			continue
		}

		var deviceName [waylandEvdevDeviceNameSize]C.char
		if C.neru_evdev_get_name(fileDescriptor, &deviceName[0], waylandEvdevDeviceNameSize) <= 0 {
			deviceName[0] = 0
		}

		name := C.GoString(&deviceName[0])
		if isUinputVirtualDevice(fileDescriptor, name) {
			_ = file.Close()

			continue
		}

		capture.files = append(capture.files, file)
	}

	if len(capture.files) == 0 {
		logger.Warn(
			"No keyboard /dev/input/event* devices could be opened initially; "+
				"hotplug watcher will monitor for newly connected keyboard devices",
			zap.Int("total_event_devices", len(paths)),
		)
	} else {
		logger.Debug(
			"Evdev capture created",
			zap.Int("keyboard_devices", len(capture.files)),
			zap.Int("total_event_devices", len(paths)),
		)
	}

	return capture, nil
}

func (capture *waylandEvdevCapture) Close() {
	if capture == nil {
		return
	}

	capture.closeOnce.Do(func() {
		// Stop the hotplug watcher first: closing the inotify fd unblocks
		// the hotplugLoop goroutine, causing it to exit.
		capture.stopHotplugWatcher()

		capture.deviceMu.Lock()
		capture.ungrabAllLocked()

		for _, file := range capture.files {
			_ = file.Close()
		}

		capture.files = nil
		capture.deviceMu.Unlock()

		// Wait for all reader goroutines to finish. Closing the files above
		// makes neru_evdev_read_event return immediately, causing each reader
		// to exit. We must wait here so that no reader can send on the events
		// channel after we close it below.
		capture.done.Wait()

		close(capture.events)

		if capture.xkbState != nil {
			C.neru_xkb_state_destroy((*C.neru_xkb_state)(capture.xkbState))
			capture.xkbState = nil
		}

		if capture.logger != nil {
			capture.logger.Debug("Evdev capture closed")
		}
	})
}

// startReaders launches reader goroutines for each captured keyboard device
// and starts the inotify hotplug watcher for detecting device hotplug.
// These goroutines run for the entire lifetime of the capture, outliving
// individual Enable/Disable cycles. Events are sent to capture.events with
// a non-blocking send so that a full buffer (e.g. while Neru is disabled)
// simply drops stale events instead of blocking the reader.
func (capture *waylandEvdevCapture) startReaders() {
	capture.deviceMu.Lock()
	for _, file := range capture.files {
		capture.done.Add(1)
		go capture.readLoop(file)
	}
	capture.deviceMu.Unlock()

	capture.startHotplugWatcher()
}

func (capture *waylandEvdevCapture) startReader(file *os.File) {
	capture.done.Add(1)
	go capture.readLoop(file)
}

func (capture *waylandEvdevCapture) readLoop(file *os.File) {
	defer capture.done.Done()

	fd := C.int(file.Fd())

	for {
		var inputEvent C.struct_input_event

		readResult := C.neru_evdev_read_event(fd, &inputEvent)
		if readResult <= 0 {
			if capture.logger != nil {
				capture.logger.Debug(
					"Evdev reader exiting",
					zap.String("device", file.Name()),
					zap.Int("read_result", int(readResult)),
				)
			}

			// Device disconnected — remove it from the tracked files slice
			// so we don't attempt to grab/query a stale fd on the next cycle.
			capture.deviceMu.Lock()
			capture.removeFileLocked(file)
			capture.deviceMu.Unlock()
			_ = file.Close()

			return
		}

		// Non-blocking send: if the events channel is full (Neru is disabled
		// between modes and stale events have accumulated), silently drop the
		// event rather than blocking the reader.
		select {
		case capture.events <- waylandEvdevEvent{
			eventType: uint16(inputEvent._type),
			code:      uint16(inputEvent.code),
			value:     int32(inputEvent.value),
		}:
		default:
		}
	}
}

// removeFileLocked removes file from the tracked files slice.
// Must be called with capture.deviceMu held.
func (capture *waylandEvdevCapture) removeFileLocked(file *os.File) {
	for i, f := range capture.files {
		if f == file {
			capture.files = append(capture.files[:i], capture.files[i+1:]...)

			return
		}
	}
}

func (capture *waylandEvdevCapture) grabAll() error {
	capture.deviceMu.Lock()
	defer capture.deviceMu.Unlock()

	return capture.grabAllLocked()
}

func (capture *waylandEvdevCapture) grabAllLocked() error {
	if capture == nil || capture.grabbed {
		return nil
	}

	var grabbedFiles []*os.File
	var failedFiles []string

	for _, file := range capture.files {
		fd := C.int(file.Fd())
		if C.neru_evdev_grab(fd, 1) != 0 {
			failedFiles = append(failedFiles, file.Name())

			continue
		}

		grabbedFiles = append(grabbedFiles, file)
	}

	if len(grabbedFiles) == 0 {
		for _, f := range capture.files {
			_ = f.Close()
		}

		virtualFile := capture.findVirtualDevice()
		if virtualFile != nil {
			kfd := C.int(virtualFile.Fd())
			if C.neru_evdev_grab(kfd, 1) != 0 {
				_ = virtualFile.Close()
			} else {
				capture.files = []*os.File{virtualFile}
				capture.grabbed = true

				return nil
			}
		}

		return fmt.Errorf(
			"%w: all keyboards failed to grab (tried: %v)",
			errWaylandEvdevGrabFailed,
			failedFiles,
		)
	}

	if capture.logger != nil && len(failedFiles) > 0 {
		capture.logger.Warn(
			"Partial keyboard grab failure; some keyboards not captured",
			zap.Strings("failed", failedFiles),
		)
	}

	var remainingFiles []*os.File
	for _, file := range capture.files {
		if !slices.Contains(grabbedFiles, file) {
			_ = file.Close()
		} else {
			remainingFiles = append(remainingFiles, file)
		}
	}

	capture.files = remainingFiles
	capture.grabbed = true

	return nil
}

func (capture *waylandEvdevCapture) findVirtualDevice() *os.File {
	paths, _ := filepath.Glob("/dev/input/event*")
	for _, path := range paths {
		file, openErr := os.Open(path)
		if openErr != nil {
			continue
		}

		fileDescriptor := C.int(file.Fd())

		var deviceName [waylandEvdevDeviceNameSize]C.char
		if C.neru_evdev_get_name(fileDescriptor, &deviceName[0], waylandEvdevDeviceNameSize) <= 0 {
			deviceName[0] = 0
		}

		name := C.GoString(&deviceName[0])
		if !isUinputVirtualDevice(fileDescriptor, name) || isNeruInjectionDevice(name) {
			_ = file.Close()

			continue
		}

		if C.neru_evdev_is_keyboard(fileDescriptor) != 0 {
			return file
		}

		_ = file.Close()
	}

	return nil
}

func (capture *waylandEvdevCapture) ungrabAll() {
	capture.deviceMu.Lock()
	defer capture.deviceMu.Unlock()

	capture.ungrabAllLocked()
}

func (capture *waylandEvdevCapture) ungrabAllLocked() {
	if capture == nil || !capture.grabbed {
		return
	}

	for _, file := range capture.files {
		fd := C.int(file.Fd())
		C.neru_evdev_grab(fd, 0)
	}

	capture.grabbed = false
}

func (capture *waylandEvdevCapture) modifierKeysHeld() bool {
	if capture == nil {
		return false
	}

	capture.deviceMu.Lock()
	defer capture.deviceMu.Unlock()

	modifierCodes := []uint16{
		evdevKeyLeftShift,
		evdevKeyRightShift,
		evdevKeyLeftCtrl,
		evdevKeyRightCtrl,
		evdevKeyLeftAlt,
		evdevKeyRightAlt,
		evdevKeyLeftMeta,
		evdevKeyRightMeta,
	}

	for _, file := range capture.files {
		fd := C.int(file.Fd())

		for _, code := range modifierCodes {
			if C.neru_evdev_key_down(fd, C.uint(code)) != 0 {
				return true
			}
		}
	}

	return false
}

// queryAllPressedKeys retrieves all currently pressed keys via EVIOCGKEY from
// each captured device and records them in the pressed map. This is called
// after EVIOCGRAB because the kernel replays the current key state through the
// SYN_DROPPED mechanism. By querying the state here we can distinguish keys
// that were held before mode activation from keys pressed during the mode.
func queryAllPressedKeys(capture *waylandEvdevCapture, pressed map[uint16]bool) {
	if capture == nil {
		return
	}

	capture.deviceMu.Lock()
	defer capture.deviceMu.Unlock()

	keycodes := make([]C.uint, evdevMaxPressedKeys)

	for _, file := range capture.files {
		fd := C.int(file.Fd())
		n := int(C.neru_evdev_get_pressed_keys(fd, &keycodes[0], C.int(len(keycodes))))
		if n <= 0 {
			continue
		}

		for i := range min(n, len(keycodes)) {
			code := uint16(keycodes[i])
			pressed[code] = true
		}
	}
}

// queryEvdevModifierState queries the current evdev key state and returns
// a linuxModifierState counting any held modifier keys across all captured
// devices. Keys that are physically held are also recorded in pressed so that
// the event-loop press handler can avoid double-counting when the
// corresponding evdev press event is processed from the buffer.
func queryEvdevModifierState(
	capture *waylandEvdevCapture,
	pressed map[uint16]bool,
) linuxModifierState {
	if capture == nil {
		return linuxModifierState{}
	}

	capture.deviceMu.Lock()
	defer capture.deviceMu.Unlock()

	var state linuxModifierState

	type modifierKey struct {
		code     uint16
		modifier string
	}
	modifierKeys := []modifierKey{
		{evdevKeyLeftShift, evdevModifierShift},
		{evdevKeyRightShift, evdevModifierShift},
		{evdevKeyLeftCtrl, evdevModifierCtrl},
		{evdevKeyRightCtrl, evdevModifierCtrl},
		{evdevKeyLeftAlt, evdevModifierAlt},
		{evdevKeyRightAlt, evdevModifierAlt},
		{evdevKeyLeftMeta, evdevModifierCmd},
		{evdevKeyRightMeta, evdevModifierCmd},
	}

	for _, file := range capture.files {
		fd := C.int(file.Fd())

		for _, mk := range modifierKeys {
			if C.neru_evdev_key_down(fd, C.uint(mk.code)) != 0 {
				state.update(mk.modifier, true)
				pressed[mk.code] = true
			}
		}
	}

	return state
}
