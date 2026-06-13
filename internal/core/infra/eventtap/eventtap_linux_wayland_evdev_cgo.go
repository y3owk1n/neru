//go:build linux && cgo

package eventtap

/*
#include "../platform/linux/evdev.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	_ "github.com/y3owk1n/neru/internal/core/infra/platform/linux"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

const (
	waylandEvdevEventBufferSize           = 128
	waylandEvdevModifierReleasePollPeriod = 5 * time.Millisecond
)

var (
	errWaylandEvdevUnavailable = errors.New("wayland evdev capture unavailable")
	errWaylandEvdevGrabFailed  = errors.New("wayland evdev grab failed")
	errUinputScrollUnavailable = errors.New("uinput scroll device unavailable")
	errUinputScrollSend        = errors.New("failed to send uinput scroll event")
)

const waylandEvdevDeviceNameSize = 256

const waylandEvdevBusVirtual = 0x06

var knownVirtualDevices = []string{"kanata"}

func isUinputVirtualDevice(fd C.int, name string) bool {
	bustype := int(C.neru_evdev_get_bustype(fd))
	if bustype == waylandEvdevBusVirtual {
		return true
	}

	if name == "" {
		return false
	}

	lower := strings.ToLower(name)
	for _, known := range knownVirtualDevices {
		if strings.Contains(lower, known) {
			return true
		}
	}

	return false
}

type waylandEvdevEvent struct {
	eventType uint16
	code      uint16
	value     int32
}

type waylandEvdevKeyState struct {
	modifiers evdevModifierState
	pressed   map[uint16]bool
}

type waylandEvdevCapture struct {
	files  []*os.File
	events chan waylandEvdevEvent
	logger *zap.Logger

	closeOnce sync.Once
	done      sync.WaitGroup
	grabbed   bool
}

func newWaylandEvdevCapture(logger *zap.Logger) (*waylandEvdevCapture, error) {
	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, err
	}

	capture := &waylandEvdevCapture{
		files:  make([]*os.File, 0, len(paths)),
		events: make(chan waylandEvdevEvent, waylandEvdevEventBufferSize),
		logger: logger,
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
		return nil, fmt.Errorf(
			"%w: no keyboard /dev/input/event* devices could be opened",
			errWaylandEvdevUnavailable,
		)
	}

	return capture, nil
}

func (capture *waylandEvdevCapture) Close() {
	if capture == nil {
		return
	}

	capture.closeOnce.Do(func() {
		capture.ungrabAll()

		for _, file := range capture.files {
			_ = file.Close()
		}
	})
}

func (capture *waylandEvdevCapture) startReaders() {
	for _, file := range capture.files {
		capture.done.Add(1)

		go capture.readLoop(file)
	}

	go func() {
		capture.done.Wait()
		close(capture.events)
	}()
}

func (capture *waylandEvdevCapture) readLoop(file *os.File) {
	defer capture.done.Done()

	fd := C.int(file.Fd())

	for {
		var inputEvent C.struct_input_event

		readResult := C.neru_evdev_read_event(fd, &inputEvent)
		if readResult <= 0 {
			return
		}

		capture.events <- waylandEvdevEvent{
			eventType: uint16(inputEvent._type),
			code:      uint16(inputEvent.code),
			value:     int32(inputEvent.value),
		}
	}
}

func (capture *waylandEvdevCapture) grabAll() error {
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
		if !isUinputVirtualDevice(fileDescriptor, name) {
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

// queryEvdevModifierState queries the current evdev key state and returns
// a linuxModifierState counting any held modifier keys across all captured
// devices. This mirrors x11QueryModifierState for the X11 path.
func queryEvdevModifierState(capture *waylandEvdevCapture) linuxModifierState {
	if capture == nil {
		return linuxModifierState{}
	}

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
			}
		}
	}

	return state
}

func (et *EventTap) runWaylandEvdev() bool {
	capture, err := newWaylandEvdevCapture(et.logger)
	if err != nil {
		if et.logger != nil {
			level := et.logger.Info
			if !errors.Is(err, errWaylandEvdevUnavailable) {
				level = et.logger.Warn
			}

			level(
				"Wayland evdev capture unavailable; falling back to overlay keyboard focus",
				zap.Error(err),
			)
		}

		return false
	}
	defer capture.Close()

	manager := overlay.Get()
	keyboardCaptureDisabled := false
	if manager != nil {
		defer func() {
			if keyboardCaptureDisabled {
				manager.SetKeyboardCaptureEnabled(true)
			}
		}()
	}

	for capture.modifierKeysHeld() {
		select {
		case <-et.stopCh:
			return true
		case <-time.After(waylandEvdevModifierReleasePollPeriod):
		}
	}

	grabErr := capture.grabAll()
	if grabErr != nil {
		if et.logger != nil {
			et.logger.Warn(
				"Failed to grab Wayland evdev keyboards; falling back to overlay keyboard focus",
				zap.Error(grabErr),
			)
		}

		return false
	}

	capture.startReaders()

	if manager != nil {
		manager.SetKeyboardCaptureEnabled(false)
		keyboardCaptureDisabled = true
	}

	if et.logger != nil {
		et.logger.Info(
			"Using Wayland evdev keyboard capture",
			zap.Int("devices", len(capture.files)),
		)
	}

	state := waylandEvdevKeyState{
		pressed:   make(map[uint16]bool),
		modifiers: evdevModifierState{linuxModifierState: queryEvdevModifierState(capture)},
	}

	for {
		select {
		case <-et.stopCh:
			return true
		case event, ok := <-capture.events:
			if !ok {
				return true
			}

			et.handleWaylandEvdevEvent(&state, event)
		}
	}
}

func (et *EventTap) handleWaylandEvdevEvent(
	state *waylandEvdevKeyState,
	event waylandEvdevEvent,
) {
	if event.eventType != evdevEventKey {
		return
	}

	if modifier := evdevModifierName(event.code); modifier != "" {
		if event.value == evdevValueRepeat {
			return
		}

		isDown := event.value == evdevValuePress

		if isDown {
			state.trackKey(event.code, true)
			state.modifiers.update(modifier, true)
		} else if state.pressed[event.code] {
			state.trackKey(event.code, false)
			state.modifiers.update(modifier, false)
		} else {
			// Release without a matching press (press happened before
			// fd was opened). Don't decrement — the count was never
			// incremented for this key, and doing so would drive it
			// negative, causing allZero() to return true prematurely.
			return
		}

		if et.consumeSyntheticModifierEvent(modifier, isDown) {
			return
		}

		if et.stickyToggleEnabled() && et.stickyDetectionArmed() {
			et.dispatchKey(linuxModifierToggleEvent(modifier, isDown))
		}

		// Re-arm detection when the modifier state reaches a clean slate,
		// matching macOS behavior where initial held-modifier releases from
		// an activation chord are not interpreted as sticky toggles.
		if !isDown && !et.stickyDetectionArmed() && state.modifiers.allZero() {
			et.stickyArmDetection()
		}

		return
	}

	switch event.value {
	case evdevValuePress:
		state.trackKey(event.code, true)
	case evdevValueRelease:
		state.trackKey(event.code, false)

		key := evdevKeyName(event.code)
		if key != "" {
			if keyUp := linuxKeyUpEvent(key); keyUp != "" {
				et.dispatchKey(keyUp)
			}
		}

		return
	case evdevValueRepeat:
		if !state.pressed[event.code] {
			return
		}
	default:
		return
	}

	key := evdevKeyName(event.code)
	if key == "" {
		return
	}

	key = normalizeLinuxKey(state.modifiers.prefix() + key)
	if key == "" {
		return
	}

	et.dispatchKey(key)
}

func (state *waylandEvdevKeyState) trackKey(code uint16, isDown bool) {
	if state == nil {
		return
	}

	if state.pressed == nil {
		state.pressed = make(map[uint16]bool)
	}

	if isDown {
		state.pressed[code] = true

		return
	}

	delete(state.pressed, code)
}

var (
	uinputScrollOnce sync.Once
	uinputScrollFd   int
	errUinputScroll  error
)

func initUinputScroll() error {
	var fd C.int
	if C.neru_uinput_create_scroll(&fd) == 0 {
		return fmt.Errorf("%w", errUinputScrollUnavailable)
	}
	uinputScrollFd = int(fd)

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
	fd, err := getUinputScrollFd()
	if err != nil {
		return err
	}
	if C.neru_uinput_scroll(C.int(fd), C.int(axis), C.int(value)) == 0 {
		return fmt.Errorf("%w", errUinputScrollSend)
	}

	return nil
}
