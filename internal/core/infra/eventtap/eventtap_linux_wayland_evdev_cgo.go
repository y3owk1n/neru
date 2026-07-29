//go:build linux && cgo

package eventtap

/*
#include "../platform/linux/evdev.h"
#include "../platform/linux/wayland_keymap.h"
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
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"go.uber.org/zap"

	linux "github.com/y3owk1n/neru/internal/core/infra/platform/linux"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

const (
	waylandEvdevEventBufferSize           = 128
	waylandEvdevModifierReleasePollPeriod = 5 * time.Millisecond
	waylandEvdevPreGrabHoldPollPeriod     = 50 * time.Millisecond
	waylandEvdevPreGrabTimeout            = 5 * time.Second
	waylandEvdevHotplugBufSize            = 4096
	waylandEvdevHotplugSettleDelay        = 100 * time.Millisecond
	waylandEvdevHotplugPollInterval       = 500 * time.Millisecond
)

// waylandEvdevKeyboardActive reports whether an evdev keyboard grab is currently
// held, i.e. keys are captured directly from the input devices rather than via
// the overlay layer-surface's keyboard grab. When true, the overlay must NOT
// request exclusive keyboard focus: on wlroots compositors (niri, Sway, …) a
// layer-surface keyboard grab deactivates the focused app's toplevel, which
// makes a hints refresh re-read the wrong "focused window" and tear the overlay
// down. See IsWaylandEvdevKeyboardActive and its use in the indicator polling.
var waylandEvdevKeyboardActive atomic.Bool

// IsWaylandEvdevKeyboardActive reports whether keys are currently being captured
// via an evdev grab (so the overlay's keyboard grab is redundant and must stay
// off). False on non-Wayland sessions, when the grab failed, and between modes.
func IsWaylandEvdevKeyboardActive() bool {
	return waylandEvdevKeyboardActive.Load()
}

var (
	errWaylandEvdevUnavailable = errors.New("wayland evdev capture unavailable")
	errWaylandEvdevGrabFailed  = errors.New("wayland evdev grab failed")
	errUinputScrollUnavailable = errors.New("uinput scroll device unavailable")
	errUinputScrollSend        = errors.New("failed to send uinput scroll event")
)

const (
	waylandEvdevDeviceNameSize = 256
	evdevMaxPressedKeys        = 256
)

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
	// initialKeys tracks keys that were already physically held when
	// the event tap was (re-)enabled.  The kernel replays press events
	// via SYN_DROPPED after EVIOCGRAB; we suppress dispatch for these
	// until the physical release (removing from initialKeys).
	initialKeys map[uint16]bool
	// releasedDuringGrab is a subset of initialKeys: keys that were
	// released while the evdev grab was active and whose release events
	// never reached libinput.  We inject synthetic releases for these
	// at shutdown so libinput's per-keycode hw_is_key_down is cleared.
	releasedDuringGrab map[uint16]bool
	// passthroughHeld maps a base keycode that is currently being passed
	// through to the focused app to the modifier names we pressed on the
	// virtual keyboard for it. The modifiers stay held (refcounted by the
	// wlroots modifier dispatcher) across the whole press→repeat→release
	// span so auto-repeat works without flapping the modifier, and are
	// released on the physical key-up. Presence also marks the release as
	// "already passed through" so no stray key-up leaks into Neru.
	passthroughHeld map[uint16][]string
}

type waylandEvdevCapture struct {
	files  []*os.File
	events chan waylandEvdevEvent
	logger *zap.Logger

	closeOnce        sync.Once
	done             sync.WaitGroup
	grabbed          bool
	startReadersOnce sync.Once

	deviceMu      sync.Mutex
	inotifyFd     int
	hotplugStopCh chan struct{}
	hotplugOnce   sync.Once
	hotplugWg     sync.WaitGroup

	// xkbState holds a C xkb_state initialized from the compositor's keymap.
	// Used to resolve evdev scan codes to key names that respect XKB options.
	xkbState unsafe.Pointer // *C.neru_xkb_state
}

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

// initEvdevCapture initializes the persistent waylandEvdevCapture.
// A failed attempt can be retried later, allowing detection of newly
// connected keyboards after startup.
func (et *EventTap) initEvdevCapture() (*waylandEvdevCapture, error) {
	et.evdevWaylandCaptureInit.Lock()
	defer et.evdevWaylandCaptureInit.Unlock()

	if et.evdevWaylandCapture != nil {
		c, ok := et.evdevWaylandCapture.(*waylandEvdevCapture)
		if !ok {
			return nil, errWaylandEvdevUnavailable
		}

		return c, nil
	}

	wlCapture, capErr := newWaylandEvdevCapture(et.logger)
	if capErr != nil {
		if et.logger != nil {
			level := et.logger.Info
			if !errors.Is(capErr, errWaylandEvdevUnavailable) {
				level = et.logger.Warn
			}

			level(
				"Wayland evdev capture unavailable; falling back to overlay keyboard focus",
				zap.Error(capErr),
			)
		}

		return nil, capErr
	}

	et.evdevWaylandCapture = wlCapture

	return wlCapture, nil
}

// closeEvdevCapture closes the persistent evdev capture, releasing all file
// descriptors and stopping reader goroutines. It is safe to call multiple
// times — the underlying Close() uses sync.Once.
func (et *EventTap) closeEvdevCapture() {
	if et.evdevWaylandCapture == nil {
		return
	}

	capture, ok := et.evdevWaylandCapture.(*waylandEvdevCapture)
	if !ok {
		return
	}

	capture.Close()
	et.evdevWaylandCapture = nil
}

func (et *EventTap) runWaylandEvdev() bool {
	// Clear the evdev-active flag on every exit path (mode end / ungrab), so the
	// overlay may reclaim the keyboard grab if it ever becomes the fallback.
	defer waylandEvdevKeyboardActive.Store(false)

	// Get or create the persistent capture (initialized once, reused
	// across Enable/Disable cycles). This avoids re-scanning
	// /dev/input/event* devices on every mode activation, which was
	// the source of a mild delay before modes accepted input.
	capture, err := et.initEvdevCapture()
	if err != nil {
		return false
	}

	// Refresh xkb_state on every activation so lock modifiers (Num Lock,
	// Caps Lock) and layout group reflect the current compositor state.
	if capture.xkbState != nil {
		C.neru_xkb_state_destroy((*C.neru_xkb_state)(capture.xkbState))
	}
	xkb := C.neru_xkb_state_create()
	capture.xkbState = unsafe.Pointer(xkb)
	if xkb == nil && et.logger != nil {
		et.logger.Error(
			"Failed to initialize Wayland xkb_state; XKB options will be ignored, " +
				"falling back to hardcoded evdev key names",
		)
	}
	if xkb != nil {
		numLock := C.int(0)
		capsLock := C.int(0)
		capture.deviceMu.Lock()
		for _, file := range capture.files {
			fd := C.int(file.Fd())
			if C.neru_evdev_led_is_on(fd, C.uint(0)) != 0 {
				numLock = 1
			}
			if C.neru_evdev_led_is_on(fd, C.uint(1)) != 0 {
				capsLock = 1
			}
		}
		capture.deviceMu.Unlock()
		C.neru_xkb_state_sync_leds(xkb, numLock, capsLock)
	}

	manager := overlay.Get()

	for capture.modifierKeysHeld() {
		select {
		case <-et.stopCh:
			return true
		case <-time.After(waylandEvdevModifierReleasePollPeriod):
		}
	}

	// Wait for ALL physically held keys to be released before
	// grabbing.  Grabbing while a key is held causes the kernel to
	// route that key's release to our fd only — libinput never sees
	// it and permanently considers the key pressed.  The next press
	// of the same key after the mode exits is then treated as a
	// duplicate by libinput and silently consumed.
	{
		held := make(map[uint16]bool)
		queryAllPressedKeys(capture, held)
		if len(held) > 0 {
			if manager != nil {
				manager.SetKeyboardCaptureEnabled(true)
			}

			deadline := time.After(waylandEvdevPreGrabTimeout)
			ticker := time.NewTicker(waylandEvdevPreGrabHoldPollPeriod)
		waitLoop:
			for {
				pressed := make(map[uint16]bool)
				queryAllPressedKeys(capture, pressed)
				if len(pressed) == 0 {
					break waitLoop
				}

				select {
				case <-et.stopCh:
					ticker.Stop()
					if manager != nil {
						manager.SetKeyboardCaptureEnabled(false)
					}

					return true
				case <-deadline:
					break waitLoop
				case <-ticker.C:
				case _, ok := <-capture.events:
					if !ok {
						ticker.Stop()

						return true
					}
				}
			}
			ticker.Stop()

			if manager != nil {
				manager.SetKeyboardCaptureEnabled(false)
			}
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

	// The evdev grab now owns the keyboard for this mode session, so the overlay
	// must stay keyboard-passive (see waylandEvdevKeyboardActive). Cleared by the
	// deferred Store(false) when this function returns on mode exit / ungrab.
	waylandEvdevKeyboardActive.Store(true)

	// Start reader goroutines on first invocation only; they run for
	// the entire lifetime of the capture (until EventTap.Destroy()).
	capture.startReadersOnce.Do(func() {
		capture.startReaders()
	})

	if manager != nil {
		// Keep the overlay keyboard-passive for the whole session and do NOT
		// restore it to EXCLUSIVE on exit: the evdev grab owns the keyboard, so a
		// layer-surface grab would only deactivate the focused app's toplevel on
		// wlroots. Every subsequent mode therefore also starts from NONE. The
		// non-evdev fallback (runWayland) raises EXCLUSIVE itself when it needs it.
		manager.SetKeyboardCaptureEnabled(false)
	}

	if et.logger != nil {
		et.logger.Info(
			"Using Wayland evdev keyboard capture",
			zap.Int("devices", len(capture.files)),
		)
	}

	// Drain any stale events that accumulated in the channel while
	// Neru was disabled between modes. These are events from other
	// applications that were pushed into the buffer when we were
	// ungrabbed. A labeled break is required here — plain break
	// inside select only exits the select, not the for loop.
drainStale:
	for {
		select {
		case <-capture.events:
		default:
			break drainStale
		}
	}

	pressed := make(map[uint16]bool)
	state := waylandEvdevKeyState{
		pressed:            pressed,
		initialKeys:        make(map[uint16]bool),
		releasedDuringGrab: make(map[uint16]bool),
		modifiers: evdevModifierState{
			linuxModifierState: queryEvdevModifierState(capture, pressed),
		},
	}

	// Query all currently pressed (not just modifier) keys so we can suppress
	// dispatch for keys that were held before this mode session started.
	// Without this, the kernel's SYN_DROPPED replay after EVIOCGRAB delivers
	// stale press events that would be interpreted as fresh key presses.
	queryAllPressedKeys(capture, pressed)

	// Copy the queried keys into initialKeys so the event handler can
	// distinguish pre-existing presses from new ones. Keys that were already
	// held when the event tap was enabled will have their repeat events
	// suppressed until the user releases and re-presses them.
	for code := range pressed {
		state.initialKeys[code] = true
	}

	for {
		select {
		case <-et.stopCh:
			// Inject synthetic releases for keys that were in
			// initialKeys and physically released during the grab
			// (their release never reached libinput), or are still
			// in initialKeys but no longer pressed (released during
			// the grab before the event handler processed them).
			for code := range state.releasedDuringGrab {
				err := linux.WaylandKeyEvent(uint32(code), false)
				if err != nil && et.logger != nil {
					et.logger.Warn(
						"Failed to inject synthetic key release at shutdown",
						zap.Uint16("keycode", code),
						zap.Error(err),
					)
				}
			}
			for code := range state.initialKeys {
				if !state.pressed[code] {
					err := linux.WaylandKeyEvent(uint32(code), false)
					if err != nil && et.logger != nil {
						et.logger.Warn(
							"Failed to inject synthetic key release at shutdown",
							zap.Uint16("keycode", code),
							zap.Error(err),
						)
					}
				}
			}

			// Release any modifiers still held for a passthrough key that was
			// down when the mode exited, so a virtual modifier can never stay
			// latched after the grab ends.
			for code, mods := range state.passthroughHeld {
				delete(state.passthroughHeld, code)
				et.releasePassthroughModifiers(mods)
			}

			capture.ungrabAll()

			return true
		case event, ok := <-capture.events:
			if !ok {
				return true
			}

			et.handleWaylandEvdevEvent(&state, event)
		}
	}
}

func (et *EventTap) xkbEvdevKeyName(capture *waylandEvdevCapture, code uint16) string {
	if capture == nil || capture.xkbState == nil {
		return evdevKeyName(code)
	}

	var buf [64]C.char
	if C.neru_xkb_state_key_get_name(
		(*C.neru_xkb_state)(capture.xkbState),
		C.uint16_t(code),
		&buf[0],
		64, //nolint:nlreturn
	) == 0 {
		return C.GoString(&buf[0])
	}

	return evdevKeyName(code)
}

// xkbStateModifierName returns the canonical evdev modifier name for the
// given scan code as resolved by the XKB keymap, or empty string when the
// key is not a modifier.  When XKB remaps a physical modifier to a different
// function (e.g. ctrl:swapcaps makes Caps Lock act as Control), this returns
// the remapped modifier name so the handler tracks the correct modifier.
func (et *EventTap) xkbStateModifierName(capture *waylandEvdevCapture, code uint16) string {
	if capture == nil || capture.xkbState == nil {
		return evdevModifierName(code)
	}
	key := et.xkbEvdevKeyName(capture, code)
	if key == "" {
		return evdevModifierName(code)
	}
	switch key {
	case "Shift_L", "Shift_R":
		return evdevModifierShift
	case "Control_L", "Control_R":
		return evdevModifierCtrl
	case "Alt_L", "Alt_R":
		return evdevModifierAlt
	case "Meta_L", "Meta_R", "Super_L", "Super_R", "Hyper_L", "Hyper_R":
		return evdevModifierCmd
	}

	return ""
}

func (et *EventTap) handleWaylandEvdevEvent(
	state *waylandEvdevKeyState,
	event waylandEvdevEvent,
) {
	if event.eventType != evdevEventKey {
		return
	}

	// Feed all key events to xkb_state so its internal state stays
	// consistent for key symbol resolution via XKB (respects options
	// like caps:swapescape set by the compositor).
	capture, _ := et.evdevWaylandCapture.(*waylandEvdevCapture)
	if capture != nil && capture.xkbState != nil {
		switch event.value {
		case evdevValuePress:
			C.neru_xkb_state_key((*C.neru_xkb_state)(capture.xkbState), C.uint16_t(event.code), 1)
		case evdevValueRelease:
			C.neru_xkb_state_key((*C.neru_xkb_state)(capture.xkbState), C.uint16_t(event.code), 0)
		}
	}

	// Resolve the modifier name through the XKB keymap so that compositor
	// options like ctrl:swapcaps and caps:swapescape are respected: when
	// XKB remaps a physical modifier to a different function, the handler
	// uses the remapped modifier name (or bypasses modifier handling when
	// the key is remapped to a non-modifier).
	modifier := et.xkbStateModifierName(capture, event.code)
	if modifier != "" {
		if event.value == evdevValueRepeat {
			return
		}

		isDown := event.value == evdevValuePress

		switch {
		case isDown:
			alreadyTracked := state.pressed[event.code]
			state.trackKey(event.code, true)
			if !alreadyTracked {
				state.modifiers.update(modifier, true)
			}
		case state.pressed[event.code]:
			state.trackKey(event.code, false)
			state.modifiers.update(modifier, false)
		default:
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
		// If this key was already held when the event tap was e kernel's SYN_DROPPED state replay after
		// EVIOCGRAB. Track it in pressed (so subsequent repeats are not
		// silently consumed) but skip dispatch — the user did not press
		// it during this mode session. The initialKeys entry persists
		// until the physical release so repeats continue to be suppressed.
		state.trackKey(event.code, true)

		if state.initialKeys[event.code] {
			return
		}
	case evdevValueRelease:
		if state.initialKeys[event.code] {
			state.releasedDuringGrab[event.code] = true
			delete(state.initialKeys, event.code)
		}
		state.trackKey(event.code, false)

		// A key that was passed through never reached Neru as a press, so its
		// release must not leak a key-up into Neru either. Release the virtual
		// modifiers we were holding for it (refcounted, so a modifier shared
		// with another passthrough key or a sticky hold stays down).
		if mods, ok := state.passthroughHeld[event.code]; ok {
			delete(state.passthroughHeld, event.code)
			et.releasePassthroughModifiers(mods)

			return
		}

		key := et.xkbEvdevKeyName(capture, event.code)
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

		// Suppress repeat dispatch for keys that were held before mode
		// activation. The user must release and re-press to have the key
		// register as a fresh input in the active mode.
		if state.initialKeys[event.code] {
			return
		}
	default:
		return
	}

	// A key already owned by passthrough keeps routing its repeats to the app for
	// as long as it is physically held, regardless of the current modifier state.
	// Releasing a modifier mid-hold must not reclassify the key back into Neru
	// (the virtual modifier stays held until the physical key-up). Keyed by code,
	// so this runs before key-name resolution.
	if _, held := state.passthroughHeld[event.code]; held {
		et.passthroughEvdevChord(state, event.code, false)

		return
	}

	key := et.xkbEvdevKeyName(capture, event.code)
	if key == "" {
		return
	}

	key = normalizeLinuxKey(state.modifiers.prefix() + key)
	if key == "" {
		return
	}

	// Modifier passthrough: when an unbound Ctrl/Alt/Cmd chord should reach the
	// focused application, re-inject it through the virtual keyboard (a distinct
	// device from the EVIOCGRAB'd physical keyboard, so it does not re-enter this
	// reader) and skip Neru's own dispatch. If injection fails, fall through to
	// normal dispatch rather than silently dropping the shortcut.
	if et.shouldPassthroughChord(key) && et.passthroughEvdevChord(state, event.code, true) {
		return
	}

	et.dispatchKey(key)
}

// passthroughEvdevChord re-injects a modifier chord to the focused application
// through the zwp_virtual_keyboard_v1 path and reports whether it was delivered.
// On the initial press it holds the currently-held modifiers down (refcounted by
// the wlroots modifier dispatcher) and taps the base keycode, records the hold so
// the physical release can drop those modifiers, and notifies the mode layer
// once. On auto-repeat it re-taps only the base key, leaving the modifiers held
// so the app sees a steadily-held modifier instead of it flapping around every
// repeat.
//
// It returns false when the essential injection (a held modifier or the base-key
// press) failed on the initial press, having rolled back any modifiers it pressed
// so none stays latched; the caller then falls back to normal dispatch rather
// than reporting a delivered shortcut. Returns true once the key is owned by
// passthrough — a dropped auto-repeat is tolerated, since the key stays owned and
// its modifiers held until the physical release either way.
func (et *EventTap) passthroughEvdevChord(
	state *waylandEvdevKeyState,
	code uint16,
	isPress bool,
) bool {
	if _, held := state.passthroughHeld[code]; held && !isPress {
		// Auto-repeat: modifiers are already held; just re-tap the base key.
		_ = linux.WaylandKeyEvent(uint32(code), true)
		_ = linux.WaylandKeyEvent(uint32(code), false)

		return true
	}

	mods := heldPassthroughModifiers(state)

	// Press the held modifiers, remembering which actually took so a
	// mid-sequence failure can be unwound without leaving one latched.
	pressed := make([]string, 0, len(mods))

	for _, mod := range mods {
		err := linux.WaylandModifierEvent(mod, true)
		if err != nil {
			et.releasePassthroughModifiers(pressed)

			return false
		}

		pressed = append(pressed, mod)
	}

	// The app acts on the key-down (the chord is delivered there); a failed
	// key-up only leaves cleanup pending, so only the down gates success.
	keyDownErr := linux.WaylandKeyEvent(uint32(code), true)
	if keyDownErr != nil {
		et.releasePassthroughModifiers(pressed)

		return false
	}

	keyUpErr := linux.WaylandKeyEvent(uint32(code), false)
	if keyUpErr != nil && et.logger != nil {
		// The chord was already delivered by the key-down; a rejected key-up is
		// not retried (the injection channel would have to recover), but log it
		// so a stuck virtual key is diagnosable.
		et.logger.Warn(
			"Failed to release passthrough key",
			zap.Uint16("keycode", code),
			zap.Error(keyUpErr),
		)
	}

	if state.passthroughHeld == nil {
		state.passthroughHeld = make(map[uint16][]string)
	}

	state.passthroughHeld[code] = mods

	et.firePassthroughCallback()

	return true
}

// releasePassthroughModifiers releases the given modifiers in reverse press
// order (refcounted, so a modifier shared with another passthrough key or a
// sticky hold stays down). Used both to unwind a failed press and to drop a
// held chord's modifiers on release/shutdown. A rejected release is not retried
// (a dead injection channel would keep failing), but is logged so a latched
// virtual modifier is diagnosable — matching the shutdown synthetic-release path.
func (et *EventTap) releasePassthroughModifiers(mods []string) {
	for _, mod := range slices.Backward(mods) {
		err := linux.WaylandModifierEvent(mod, false)
		if err != nil && et.logger != nil {
			et.logger.Warn(
				"Failed to release passthrough modifier",
				zap.String("modifier", mod),
				zap.Error(err),
			)
		}
	}
}

// passthroughModifierCount is the number of distinct modifier groups
// (shift/ctrl/alt/cmd) a re-injected chord can carry.
const passthroughModifierCount = 4

// heldPassthroughModifiers returns the canonical names of the modifiers
// currently held, in a stable shift→ctrl→alt→cmd order, for chord re-injection.
func heldPassthroughModifiers(state *waylandEvdevKeyState) []string {
	if state == nil {
		return nil
	}

	mods := make([]string, 0, passthroughModifierCount)

	if state.modifiers.shift > 0 {
		mods = append(mods, evdevModifierShift)
	}

	if state.modifiers.ctrl > 0 {
		mods = append(mods, evdevModifierCtrl)
	}

	if state.modifiers.alt > 0 {
		mods = append(mods, evdevModifierAlt)
	}

	if state.modifiers.cmd > 0 {
		mods = append(mods, evdevModifierCmd)
	}

	return mods
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
