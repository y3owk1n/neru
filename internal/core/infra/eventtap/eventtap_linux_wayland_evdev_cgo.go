//go:build linux && cgo

package eventtap

/*
#include <errno.h>
#include <linux/input.h>
#include <string.h>
#include <sys/ioctl.h>
#include <unistd.h>

static int neru_evdev_grab(int fd, int grab) {
	return ioctl(fd, EVIOCGRAB, grab);
}

static int neru_evdev_key_down(int fd, unsigned int keycode) {
	unsigned long key_bits[(KEY_MAX + 8 * sizeof(unsigned long)) / (8 * sizeof(unsigned long))];
	memset(key_bits, 0, sizeof(key_bits));

	if (ioctl(fd, EVIOCGKEY(sizeof(key_bits)), key_bits) < 0) {
		return 0;
	}

	return (key_bits[keycode / (8 * sizeof(unsigned long))] >>
		(keycode % (8 * sizeof(unsigned long)))) & 1UL;
}

static int neru_evdev_is_keyboard(int fd) {
	unsigned long key_bits[(KEY_MAX + 8 * sizeof(unsigned long)) / (8 * sizeof(unsigned long))];
	memset(key_bits, 0, sizeof(key_bits));

	if (ioctl(fd, EVIOCGBIT(EV_KEY, sizeof(key_bits)), key_bits) < 0) {
		return 0;
	}

	#define NERU_TEST_KEY(bits, key) \
		((bits[(key) / (8 * sizeof(unsigned long))] >> ((key) % (8 * sizeof(unsigned long)))) & 1UL)

	return NERU_TEST_KEY(key_bits, KEY_Q) &&
		NERU_TEST_KEY(key_bits, KEY_W) &&
		NERU_TEST_KEY(key_bits, KEY_E) &&
		NERU_TEST_KEY(key_bits, KEY_R) &&
		NERU_TEST_KEY(key_bits, KEY_SPACE) &&
		NERU_TEST_KEY(key_bits, KEY_ENTER);
}

static ssize_t neru_evdev_read_event(int fd, struct input_event *event) {
	return read(fd, event, sizeof(struct input_event));
}
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/ui/overlay"
)

var errWaylandEvdevUnavailable = errors.New("wayland evdev capture unavailable")

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

	closeOnce sync.Once
	done      sync.WaitGroup
	grabbed   bool
}

func newWaylandEvdevCapture() (*waylandEvdevCapture, error) {
	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, err
	}

	capture := &waylandEvdevCapture{
		files:  make([]*os.File, 0, len(paths)),
		events: make(chan waylandEvdevEvent, 128),
	}

	for _, path := range paths {
		file, openErr := os.Open(path)
		if openErr != nil {
			continue
		}

		fd := C.int(file.Fd())                 //nolint:nlreturn
		if C.neru_evdev_is_keyboard(fd) == 0 { //nolint:nlreturn
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

	fd := C.int(file.Fd()) //nolint:nlreturn

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

	for _, file := range capture.files {
		fd := C.int(file.Fd())             //nolint:nlreturn
		if C.neru_evdev_grab(fd, 1) != 0 { //nolint:nlreturn
			capture.ungrabAll()

			return fmt.Errorf("failed to grab %s", file.Name())
		}
	}

	capture.grabbed = true

	return nil
}

func (capture *waylandEvdevCapture) ungrabAll() {
	if capture == nil || !capture.grabbed {
		return
	}

	for _, file := range capture.files {
		fd := C.int(file.Fd()) //nolint:nlreturn
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
		fd := C.int(file.Fd()) //nolint:nlreturn

		for _, code := range modifierCodes {
			if C.neru_evdev_key_down(fd, C.uint(code)) != 0 { //nolint:nlreturn
				return true
			}
		}
	}

	return false
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

func (et *EventTap) runWaylandEvdev() bool {
	capture, err := newWaylandEvdevCapture()
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
		default:
		}

		time.Sleep(5 * time.Millisecond)
	}

	if grabErr := capture.grabAll(); grabErr != nil {
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
		pressed: make(map[uint16]bool),
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
		state.trackKey(event.code, isDown)
		state.modifiers.update(modifier, isDown)

		if et.consumeSyntheticModifierEvent(modifier, isDown) {
			return
		}

		if et.stickyToggleEnabled() {
			et.dispatchKey(linuxModifierToggleEvent(modifier, isDown))
		}

		return
	}

	switch event.value {
	case evdevValuePress:
		state.trackKey(event.code, true)
	case evdevValueRelease:
		state.trackKey(event.code, false)

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
