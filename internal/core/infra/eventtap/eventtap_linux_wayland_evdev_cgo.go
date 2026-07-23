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
	waylandEvdevHotplugBufSize            = 4096
	waylandEvdevHotplugSettleDelay        = 100 * time.Millisecond
)

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
	// initialKeys tracks keys that were already physically held when the event
	// tap was (re-)enabled. The kernel replays press events for these via the
	// SYN_DROPPED mechanism after EVIOCGRAB. We suppress dispatches for these
	// keys until they are released (removing from initialKeys) and re-pressed.
	initialKeys map[uint16]bool
}

type waylandEvdevCapture struct {
	files  []*os.File
	events chan waylandEvdevEvent
	logger *zap.Logger

	closeOnce        sync.Once
	done             sync.WaitGroup
	grabbed          bool
	startReadersOnce sync.Once

	deviceMu  sync.Mutex
	inotifyFd int
	hotplugWg sync.WaitGroup
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
			"No keyboard /dev/input/event* devices could be opened; "+
				"check read permissions on /dev/input/event*",
			zap.Int("total_event_devices", len(paths)),
			zap.Error(errWaylandEvdevUnavailable),
		)

		return nil, fmt.Errorf(
			"%w: no keyboard /dev/input/event* devices could be opened",
			errWaylandEvdevUnavailable,
		)
	}

	logger.Debug(
		"Evdev capture created",
		zap.Int("keyboard_devices", len(capture.files)),
		zap.Int("total_event_devices", len(paths)),
	)

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
	capture.deviceMu.Unlock()

	capture.hotplugWg.Add(1)
	go capture.hotplugLoop()
}

// stopHotplugWatcher closes the inotify fd, which unblocks the hotplugLoop
// goroutine, then waits for the goroutine to finish.
func (capture *waylandEvdevCapture) stopHotplugWatcher() {
	capture.deviceMu.Lock()
	fd := capture.inotifyFd
	capture.inotifyFd = -1
	capture.deviceMu.Unlock()

	if fd != -1 {
		_ = syscall.Close(fd)
	}

	capture.hotplugWg.Wait()
}

// hotplugLoop reads inotify events and handles new keyboard device creation.
func (capture *waylandEvdevCapture) hotplugLoop() {
	defer capture.hotplugWg.Done()

	buf := make([]byte, waylandEvdevHotplugBufSize)
	for {
		nread, err := syscall.Read(capture.inotifyFd, buf)
		if err != nil {
			if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
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
	// Get or create the persistent capture (initialized once, reused
	// across Enable/Disable cycles). This avoids re-scanning
	// /dev/input/event* devices on every mode activation, which was
	// the source of a mild delay before modes accepted input.
	capture, err := et.initEvdevCapture()
	if err != nil {
		return false
	}

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

	// Start reader goroutines on first invocation only; they run for
	// the entire lifetime of the capture (until EventTap.Destroy()).
	capture.startReadersOnce.Do(func() {
		capture.startReaders()
	})

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
		pressed:     pressed,
		initialKeys: make(map[uint16]bool),
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
			// Inject synthetic release events for keys that were
			// physically held when the mode was activated but have
			// since been released.  The evdev grab consumed those
			// release events; the compositor/terminal never saw them
			// and so continues repeating the key.  Releasing them
			// here unsticks the compositor's key state before we
			// ungrab.
			for code := range state.initialKeys {
				if !state.pressed[code] {
					_ = linux.WaylandKeyEvent(uint32(code), false)
				}
			}

			// Ungrab devices but keep the capture alive for reuse
			// on the next Enable(). The reader goroutines stay
			// running and will drop stale events via the non-blocking
			// send until we drain and re-grab.
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
		// If this key was already held when the event tap was enabled, the
		// press is from the kernel's SYN_DROPPED state replay after
		// EVIOCGRAB. Track it in pressed (so subsequent repeats are not
		// silently consumed) but skip dispatch — the user did not press
		// it during this mode session. The initialKeys entry persists
		// until the physical release so repeats continue to be suppressed.
		state.trackKey(event.code, true)

		if state.initialKeys[event.code] {
			return
		}
	case evdevValueRelease:
		delete(state.initialKeys, event.code)
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

		// Suppress repeat dispatch for keys that were held before mode
		// activation. The user must release and re-press to have the key
		// register as a fresh input in the active mode.
		if state.initialKeys[event.code] {
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
