//go:build linux && cgo

package eventtap

/*
#include <errno.h>
#include <fcntl.h>
#include <linux/input.h>
#include <linux/uinput.h>
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

static int neru_uinput_create_scroll(int *out_fd) {
	int fd = open("/dev/uinput", O_RDWR);
	if (fd < 0) {
		fd = open("/dev/input/uinput", O_RDWR);
	}
	if (fd < 0) {
		return 0;
	}

	struct input_event ev;
	memset(&ev, 0, sizeof(ev));
	if (ioctl(fd, UI_SET_EVBIT, EV_REL) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_SET_RELBIT, REL_WHEEL) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_SET_RELBIT, REL_HWHEEL) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_SET_RELBIT, REL_WHEEL_HI_RES) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_SET_RELBIT, REL_HWHEEL_HI_RES) < 0) {
		close(fd);
		return 0;
	}

	struct uinput_setup usetup;
	memset(&usetup, 0, sizeof(usetup));
	usetup.id.bustype = BUS_USB;
	usetup.id.vendor = 0x1234;
	usetup.id.product = 0x5678;
	strcpy(usetup.name, "neru-scroll");
	if (ioctl(fd, UI_DEV_SETUP, &usetup) < 0) {
		close(fd);
		return 0;
	}
	if (ioctl(fd, UI_DEV_CREATE) < 0) {
		close(fd);
		return 0;
	}

	*out_fd = fd;
	return 1;
}

static int neru_uinput_scroll(int fd, int axis, int value) {
	struct input_event ev;
	memset(&ev, 0, sizeof(ev));

	ev.type = EV_REL;
	ev.code = (axis == 0) ? REL_WHEEL_HI_RES : REL_HWHEEL_HI_RES;
	ev.value = value * 120;
	ssize_t w1 = write(fd, &ev, sizeof(ev));

	memset(&ev, 0, sizeof(ev));
	ev.type = EV_REL;
	ev.code = (axis == 0) ? REL_WHEEL : REL_HWHEEL;
	ev.value = value;
	ssize_t w2 = write(fd, &ev, sizeof(ev));

	memset(&ev, 0, sizeof(ev));
	ev.type = EV_SYN;
	ev.code = SYN_REPORT;
	ev.value = 0;
	ssize_t w3 = write(fd, &ev, sizeof(ev));

	return (w1 == sizeof(ev) && w2 == sizeof(ev) && w3 == sizeof(ev)) ? 1 : 0;
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
		events: make(chan waylandEvdevEvent, waylandEvdevEventBufferSize),
	}

	for _, path := range paths {
		file, openErr := os.Open(path)
		if openErr != nil {
			continue
		}

		fd := C.int(file.Fd())
		if C.neru_evdev_is_keyboard(fd) == 0 {
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

	for _, file := range capture.files {
		fd := C.int(file.Fd())
		if C.neru_evdev_grab(fd, 1) != 0 {
			capture.ungrabAll()

			return fmt.Errorf("%w: %s", errWaylandEvdevGrabFailed, file.Name())
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
