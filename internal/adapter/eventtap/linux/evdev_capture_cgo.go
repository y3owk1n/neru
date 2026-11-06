// The device layer of the evdev proxy: scanning /dev/input for keyboards,
// grabbing them, and reading them onto one channel for the proxy's run loop.

//go:build linux && cgo

package linux

/*
#include "../../platform/linux/evdev.h"
#include "../../platform/linux/wayland_keymap.h"
*/
import "C"

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"go.uber.org/zap"
)

type waylandEvdevCapture struct {
	files  []*os.File
	events chan waylandEvdevEvent
	logger *zap.Logger

	// grab is whether devices are taken exclusively (EVIOCGRAB) when opened.
	// A passive capture, with no uinput device to re-emit through, does not.
	grab bool

	closeOnce sync.Once
	stopCh    chan struct{}
	done      sync.WaitGroup

	deviceMu      sync.Mutex
	inotifyFd     int
	hotplugStopCh chan struct{}
	hotplugOnce   sync.Once
	hotplugWg     sync.WaitGroup

	// xkbState is a C xkb_state from the compositor's keymap, used to resolve
	// scan codes to key names that respect XKB options.
	xkbState unsafe.Pointer // *C.neru_xkb_state

	// appliedLayout is the requestedKeyboardLayout value last handed to
	// xkbState, so a change is noticed by pointer.
	appliedLayout *string

	// devices is what the capture knows about each tracked file beyond its
	// path. deviceMu guards it, as it does files.
	devices map[*os.File]trackedDevice

	// pending is the keyboards the constructor found with a key down, whose
	// grab waits for them to go idle. startReaders launches those waits.
	pending []string

	// pendingPaths is the keyboards whose wait is in flight, so overlapping
	// scans start one wait each and a session can be refused by name.
	// deviceMu guards it.
	pendingPaths map[string]struct{}

	// sessionActive is whether a mode session is capturing keys, kept under
	// deviceMu so a yield letting a keyboard go (releaseIfIdle) and a session
	// starting (beginSession) cannot cross. yielded counts keyboards let go and
	// not yet claimed or taken back, during which a session is refused.
	// deviceMu guards it too.
	sessionActive bool
	yielded       int

	// pointer is the pointer proxy, the uinput mouse a grabbed keyboard's
	// relative motion and buttons are re-emitted on, or nil until one needs
	// it. Created under deviceMu by the scan that grabs the first such
	// keyboard, read without it by the run goroutine.
	pointer atomic.Pointer[pointerProxy]
}

// pointerProxy is the pointer half of the proxy device, held by its uinput fd.
type pointerProxy struct {
	fd C.int
}

type trackedDevice struct {
	// virtual is whether the device is another process's uinput keyboard, a
	// remapper's output. Only physical keyboards are yielded
	// (yieldPhysicalKeyboards).
	virtual bool

	// released is whether this capture let go of the file on purpose
	// (yieldDevice), so its reader's exit is not taken for a vanished device.
	released bool
}

// newWaylandEvdevCapture opens every keyboard under /dev/input. With grab set,
// each is taken exclusively, but only while no key on it is down (deviceBusy).
// A keyboard with a key held is grabbed once it goes idle.
//
// A device another process holds (a remapper such as kanata or keyd) refuses
// the grab and is skipped, and the remapper's output keyboard is taken
// instead. A remapper that starts later finds its keyboards held, so they are
// yielded when its output device arrives (yieldPhysicalKeyboards). That
// device may also carry mouse motion and buttons, which go to the pointer
// proxy, created the first time such a keyboard is grabbed.
func newWaylandEvdevCapture(logger *zap.Logger, grab bool) (*waylandEvdevCapture, error) {
	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, err
	}

	capture := &waylandEvdevCapture{
		files:        make([]*os.File, 0, len(paths)),
		devices:      make(map[*os.File]trackedDevice),
		pendingPaths: make(map[string]struct{}),
		events:       make(chan waylandEvdevEvent, waylandEvdevEventBufferSize),
		logger:       logger,
		grab:         grab,
		stopCh:       make(chan struct{}),
		inotifyFd:    -1,
	}

	for _, path := range paths {
		if _, outcome := capture.addDeviceLocked(path); outcome == deviceBusy {
			capture.pending = append(capture.pending, path)
		}
	}

	if len(capture.files) == 0 && len(capture.pending) == 0 {
		logger.Warn(
			"No keyboard /dev/input/event* devices could be opened initially; "+
				"hotplug watcher will monitor for newly connected keyboard devices",
			zap.Int("total_event_devices", len(paths)),
		)
	} else {
		logger.Debug(
			"Evdev capture created",
			zap.Int("keyboard_devices", len(capture.files)),
			zap.Int("keyboards_with_a_key_down", len(capture.pending)),
			zap.Int("total_event_devices", len(paths)),
			zap.Bool("grabbed", grab),
		)
	}

	return capture, nil
}

func (capture *waylandEvdevCapture) Close() {
	if capture == nil {
		return
	}

	capture.closeOnce.Do(func() {
		close(capture.stopCh)

		// Stop the hotplug watcher first. Closing the inotify fd ends
		// hotplugLoop.
		capture.stopHotplugWatcher()

		capture.deviceMu.Lock()
		for _, file := range capture.files {
			if capture.grab {
				C.neru_evdev_grab(C.int(file.Fd()), 0)
			}

			_ = file.Close()
		}

		capture.files = nil

		if pointer := capture.pointer.Swap(nil); pointer != nil {
			C.neru_uinput_destroy(pointer.fd)
		}

		capture.deviceMu.Unlock()

		// Wait for every reader so none can send on the events channel after
		// it closes. A reader blocked in read returns on the next event or the
		// closed fd, and one blocked sending returns on stopCh.
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

// deviceOutcome is what addDeviceLocked made of a path.
type deviceOutcome int

const (
	// deviceSkipped is a node that is not a keyboard this capture reads, one
	// it already has, or one another grabber owns.
	deviceSkipped deviceOutcome = iota
	// deviceAdopted is a keyboard now tracked, grabbed when the capture grabs.
	deviceAdopted
	// deviceBusy is a keyboard with a key down, which the capture will not
	// hold. libinput keeps that key down until a release arrives on the same
	// device, and one re-emitted on the proxy is discarded (ADR 0014). It is
	// grabbed once idle instead (waitIdle).
	deviceBusy
)

// addDeviceLocked opens path and keeps it when it is a keyboard this capture
// should read with no key down. The caller holds deviceMu, or is the
// constructor before anyone else can.
func (capture *waylandEvdevCapture) addDeviceLocked(path string) (*os.File, deviceOutcome) {
	// Read-write so the compositor's LED changes can be written back. A
	// read-only node is still a keyboard to read.
	file, openErr := os.OpenFile(path, os.O_RDWR, 0)
	if openErr != nil {
		file, openErr = os.Open(path)
	}

	if openErr != nil {
		return nil, deviceSkipped
	}

	descriptor := C.int(file.Fd())

	var deviceName [waylandEvdevDeviceNameSize]C.char
	if C.neru_evdev_get_name(descriptor, &deviceName[0], waylandEvdevDeviceNameSize) <= 0 {
		deviceName[0] = 0
	}

	name := C.GoString(&deviceName[0])

	// A Neru device is known by vendor id as well as name, so the proxy never
	// grabs its own output even when the name did not read.
	if reason := skipReason(descriptor, name); reason != "" {
		_ = file.Close()

		if capture.logger != nil {
			capture.logger.Debug(
				"Input device is not a keyboard to capture",
				zap.String("device", path),
				zap.String("reason", reason),
			)
		}

		return nil, deviceSkipped
	}

	for _, tracked := range capture.files {
		if tracked.Name() == path {
			_ = file.Close()

			return nil, deviceSkipped
		}
	}

	if capture.grab {
		if C.neru_evdev_grab(descriptor, 1) != 0 {
			// Owned by another grabber, which is what a remapper looks like.
			// Debug, not warn, because its output device is captured instead.
			if capture.logger != nil {
				capture.logger.Debug(
					"Keyboard device refused the grab; leaving it to its owner",
					zap.String("device", path),
				)
			}

			_ = file.Close()

			return nil, deviceSkipped
		}

		// A keyboard with relative motion is kept only if that motion has
		// somewhere to go, otherwise it goes back to the compositor whole.
		// Checked after the grab, so a keyboard another grabber owns creates
		// no pointer proxy.
		if C.neru_evdev_has_rel_axes(descriptor) != 0 && !capture.ensurePointerProxyLocked(path) {
			C.neru_evdev_grab(descriptor, 0)
			_ = file.Close()

			return nil, deviceSkipped
		}

		// Asked under the grab, so no press lands between answer and grab. A
		// key found down lets the grab go at once, so its release reaches the
		// compositor as the press did.
		held, heldErr := keysHeld(descriptor)
		if held || heldErr != nil {
			C.neru_evdev_grab(descriptor, 0)
			_ = file.Close()

			return nil, deviceBusy
		}
	}

	capture.files = append(capture.files, file)
	capture.devices[file] = trackedDevice{virtual: isVirtualInputNode(sysfsRoot, path)}

	return file, deviceAdopted
}

// ensurePointerProxyLocked creates the pointer proxy if there is none yet and
// reports whether there is one. The caller holds deviceMu.
func (capture *waylandEvdevCapture) ensurePointerProxyLocked(path string) bool {
	if capture.pointer.Load() != nil {
		return true
	}

	var pointerFd C.int = -1

	created, errno := C.neru_uinput_create_proxy_pointer(&pointerFd)
	if created == 0 {
		if capture.logger != nil {
			capture.logger.Warn(
				"Keyboard reports pointer motion and the pointer proxy could not be created; "+
					"leaving it to the compositor, so its keys are not captured",
				zap.String("device", path),
				zap.Error(errno),
			)
		}

		return false
	}

	capture.pointer.Store(&pointerProxy{fd: pointerFd})

	if capture.logger != nil {
		capture.logger.Info(
			"Evdev pointer proxy created for a keyboard that reports pointer motion",
		)
	}

	return true
}

// skipReason names why the node behind descriptor is not a keyboard this
// capture reads, or is empty when it is one. The name is for the log, never
// the device's title.
func skipReason(descriptor C.int, name string) string {
	switch {
	case C.neru_evdev_is_keyboard(descriptor) == 0:
		return "no keyboard keys"
	case touchSurface(descriptor):
		return "touch or tablet axes"
	case C.neru_evdev_is_neru_device(descriptor) != 0 || isNeruInjectionDevice(name):
		return "neru's own device"
	}

	return ""
}

// touchSurface reports whether the node behind fd carries position axes
// (hasPointerAxes). A node whose axes cannot be read is not called one.
func touchSurface(fd C.int) bool {
	var bits C.ulong

	if C.neru_evdev_get_abs_bits(fd, &bits, C.size_t(unsafe.Sizeof(bits))) != 0 {
		return false
	}

	return hasPointerAxes(uint64(bits))
}

// keysHeld reports whether the kernel has any key down on fd. An error means
// the device is gone.
func keysHeld(fd C.int) (bool, error) {
	var bits [evdevKeyCodeCount / evdevBitsPerWord]C.ulong

	result, err := C.neru_evdev_get_key_state(fd, &bits[0], C.size_t(unsafe.Sizeof(bits)))
	if result != 0 {
		return false, err
	}

	for _, word := range bits {
		if word != 0 {
			return true, nil
		}
	}

	return false, nil
}

// startReaders launches a reader per captured keyboard, the idle waits for
// keyboards found with a key down, and the hotplug watcher, all for the
// capture's lifetime.
//
// A remapper output found by the scan may belong to a remapper still in its
// startup delay, whose inputs the scan just took, so they are yielded as if
// the output had arrived later. A remapper long up already holds its inputs,
// so the yield costs nothing.
func (capture *waylandEvdevCapture) startReaders() {
	capture.deviceMu.Lock()

	virtual := false

	for _, file := range capture.files {
		capture.startReader(file)

		virtual = virtual || capture.devices[file].virtual
	}

	for _, path := range capture.pending {
		capture.grabWhenIdleLocked(path)
	}

	capture.pending = nil
	capture.deviceMu.Unlock()

	if virtual {
		capture.yieldPhysicalKeyboards()
	}

	capture.startHotplugWatcher()
}

// grabWhenIdleLocked starts the wait that grabs a busy keyboard once idle,
// unless one is already waiting. The caller holds deviceMu.
func (capture *waylandEvdevCapture) grabWhenIdleLocked(path string) {
	if _, waiting := capture.pendingPaths[path]; waiting {
		return
	}

	capture.pendingPaths[path] = struct{}{}

	if capture.logger != nil {
		capture.logger.Info(
			"Keyboard has a key down; holding it once the key is released",
			zap.String("device", path),
		)
	}

	go capture.waitIdle(path)
}

// waitIdle adopts path once the kernel reports no key down on it. The wait is
// forgotten before adopt runs, not deferred past it, because a key pressed in
// the gap makes adopt register a replacement wait that a later cleanup would
// erase.
func (capture *waylandEvdevCapture) waitIdle(path string) {
	idle := capture.pollUntilIdle(path)

	capture.waitDone(path)

	if idle {
		capture.adopt(path, "its key came up")
	}
}

// pollUntilIdle reports whether path reached a state with no key down. It
// polls key state rather than reading, so it takes nothing from the device's
// owner. The poll cost is paid once per device, never per activation. False
// means the device is gone or the capture is closing.
func (capture *waylandEvdevCapture) pollUntilIdle(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}

	defer func() { _ = file.Close() }()

	descriptor := C.int(file.Fd())

	ticker := time.NewTicker(waylandEvdevIdlePollInterval)
	defer ticker.Stop()

	for {
		held, err := keysHeld(descriptor)
		if err != nil {
			return false
		}

		if !held {
			return true
		}

		select {
		case <-capture.stopCh:
			return false
		case <-ticker.C:
		}
	}
}

// waitDone forgets the wait on path.
func (capture *waylandEvdevCapture) waitDone(path string) {
	capture.deviceMu.Lock()
	delete(capture.pendingPaths, path)
	capture.deviceMu.Unlock()
}

// pendingCount is how many keyboards are waiting to go idle to be grabbed.
func (capture *waylandEvdevCapture) pendingCount() int {
	if capture == nil {
		return 0
	}

	capture.deviceMu.Lock()
	defer capture.deviceMu.Unlock()

	return len(capture.pendingPaths)
}

func (capture *waylandEvdevCapture) startReader(file *os.File) {
	capture.done.Add(1)
	go capture.readLoop(file)
}

// readLoop hands every event the device produces to the proxy. The send
// blocks rather than drops, because on a grabbed device a dropped event is a
// keypress that reaches nothing.
//
// It tracks which keys are down, because a device can vanish without
// reporting their release. The kernel queues one on unregister, but a read
// after the device is gone fails first, as a reader frozen across a suspend
// finds. releaseHeld sends the releases instead. Without them a modifier held
// into a suspend stays down on the proxy and chords every later key.
func (capture *waylandEvdevCapture) readLoop(file *os.File) {
	defer capture.done.Done()

	descriptor := C.int(file.Fd())
	held := make(map[uint16]struct{})

	for {
		var inputEvent C.struct_input_event

		readResult := C.neru_evdev_read_event(descriptor, &inputEvent)
		if readResult <= 0 {
			if capture.logger != nil {
				capture.logger.Debug(
					"Evdev reader exiting",
					zap.String("device", file.Name()),
					zap.Int("read_result", int(readResult)),
				)
			}

			// The device disconnected. Untrack it so nothing writes LEDs to a
			// stale fd.
			capture.deviceMu.Lock()
			device := capture.devices[file]
			capture.removeFileLocked(file)
			capture.deviceMu.Unlock()
			_ = file.Close()

			capture.releaseHeld(held)

			// A vanished remapper output also freed the keyboards it held, so
			// pick those up. Not for a device this capture let go itself, which
			// its remapper wants.
			if device.released {
				return
			}

			select {
			case <-capture.stopCh:
			default:
				go capture.rescan()
			}

			return
		}

		event := waylandEvdevEvent{
			eventType: uint16(inputEvent._type),
			code:      uint16(inputEvent.code),
			value:     int32(inputEvent.value),
		}

		if event.eventType == evdevEventKey {
			switch event.value {
			case evdevValuePress:
				held[event.code] = struct{}{}
			case evdevValueRelease:
				delete(held, event.code)
			}
		}

		select {
		case capture.events <- event:
		case <-capture.stopCh:
			return
		}
	}
}

// releaseHeld sends the proxy a release for each key a vanished device still
// had down, then a sync report, as if the user had let go.
func (capture *waylandEvdevCapture) releaseHeld(held map[uint16]struct{}) {
	if len(held) == 0 {
		return
	}

	events := make([]waylandEvdevEvent, 0, len(held)+1)
	for _, code := range slices.Sorted(maps.Keys(held)) {
		events = append(
			events,
			waylandEvdevEvent{eventType: evdevEventKey, code: code, value: evdevValueRelease},
		)
	}

	events = append(events, waylandEvdevEvent{eventType: evdevEventSyn, code: evdevSynReport})

	for _, event := range events {
		select {
		case capture.events <- event:
		case <-capture.stopCh:
			return
		}
	}
}

// removeFileLocked removes file from the tracked files. The caller holds
// capture.deviceMu.
func (capture *waylandEvdevCapture) removeFileLocked(file *os.File) {
	delete(capture.devices, file)

	for i, f := range capture.files {
		if f == file {
			capture.files = append(capture.files[:i], capture.files[i+1:]...)

			return
		}
	}
}

// yieldPhysicalKeyboards hands every physical keyboard this capture holds to
// the remapper whose output device just arrived and is about to grab them.
// Each is let go only once no key on it is down (yieldDevice), and taken back
// after the remapper's grab window if unclaimed. Virtual keyboards stay.
//
// Any other process's uinput keyboard looks like a remapper output, so a key
// injector costs the same window, with keys unremapped and hotkeys silent
// until the keyboards are taken back. A mode never pays it. No keyboard is let
// go during a session (releaseIfIdle), and a session asked for while yielded
// is refused (beginSession), so the mode falls back to overlay focus.
func (capture *waylandEvdevCapture) yieldPhysicalKeyboards() {
	capture.deviceMu.Lock()

	if !capture.grab {
		capture.deviceMu.Unlock()

		return
	}

	yielded := make([]*os.File, 0, len(capture.files))

	for _, file := range capture.files {
		device := capture.devices[file]
		if device.virtual || device.released {
			continue
		}

		device.released = true
		capture.devices[file] = device
		yielded = append(yielded, file)
	}

	capture.deviceMu.Unlock()

	if len(yielded) == 0 {
		return
	}

	if capture.logger != nil {
		capture.logger.Info(
			"Yielding physical keyboards to a remapper that just started",
			zap.Int("keyboards", len(yielded)),
		)
	}

	for _, file := range yielded {
		go capture.yieldDevice(file)
	}
}

// beginSession records that a mode session is capturing keys, or refuses it
// while a keyboard is yielded, because its keys would reach the focused app.
func (capture *waylandEvdevCapture) beginSession() error {
	capture.deviceMu.Lock()
	defer capture.deviceMu.Unlock()

	if capture.yielded > 0 {
		return errWaylandEvdevYielded
	}

	capture.sessionActive = true
	waylandEvdevKeyboardActive.Store(true)

	return nil
}

func (capture *waylandEvdevCapture) endSession() {
	capture.deviceMu.Lock()
	defer capture.deviceMu.Unlock()

	capture.sessionActive = false
	waylandEvdevKeyboardActive.Store(false)
}

// yieldDevice lets go of one held keyboard once no key on it is down and no
// mode session is capturing, so a forwarded press never loses its release
// (the ADR 0014 invariant) and a mode's keys never reach the focused app. The
// ungrab and revoke wake the reader with ENODEV so the remapper's grab
// succeeds. After the grace the device is adopted again if still free.
func (capture *waylandEvdevCapture) yieldDevice(file *os.File) {
	ticker := time.NewTicker(waylandEvdevIdlePollInterval)
	defer ticker.Stop()

	for {
		released, gone := capture.releaseIfIdle(file)
		if gone {
			return
		}

		if released {
			break
		}

		select {
		case <-capture.stopCh:
			return
		case <-ticker.C:
		}
	}

	select {
	case <-capture.stopCh:
		return
	case <-time.After(waylandEvdevYieldGrace):
	}

	capture.adopt(file.Name(), "the remapper left it")

	capture.deviceMu.Lock()
	capture.yielded--
	capture.deviceMu.Unlock()
}

// releaseIfIdle ungrabs and revokes file if no key on it is down and no mode
// session is capturing, and reports whether it did and whether the device is
// gone. It runs under deviceMu after checking the file is still tracked. The
// reader removes a vanished device under that lock before closing it, so the
// fd cannot close under the ioctl, and beginSession takes the same lock, so a
// session cannot slip in between check and release.
func (capture *waylandEvdevCapture) releaseIfIdle(file *os.File) (bool, bool) {
	capture.deviceMu.Lock()
	defer capture.deviceMu.Unlock()

	if _, tracked := capture.devices[file]; !tracked {
		return false, true
	}

	if capture.sessionActive {
		return false, false
	}

	descriptor := C.int(file.Fd())

	held, err := keysHeld(descriptor)
	if err != nil {
		return false, true
	}

	if held {
		return false, false
	}

	C.neru_evdev_grab(descriptor, 0)
	C.neru_evdev_revoke(descriptor)
	capture.yielded++

	return true, false
}

// ungrabAll hands every device back to the compositor and stops grabbing new
// ones. It is the proxy failing open, with nothing to re-emit through.
func (capture *waylandEvdevCapture) ungrabAll() {
	capture.deviceMu.Lock()
	defer capture.deviceMu.Unlock()

	if !capture.grab {
		return
	}

	capture.grab = false

	for _, file := range capture.files {
		C.neru_evdev_grab(C.int(file.Fd()), 0)
	}
}

func (capture *waylandEvdevCapture) deviceCount() int {
	if capture == nil {
		return 0
	}

	capture.deviceMu.Lock()
	defer capture.deviceMu.Unlock()

	return len(capture.files)
}

// writeToDevices writes one event to every captured device, carrying the
// compositor's LED changes from the proxy keyboard to the physical lights.
func (capture *waylandEvdevCapture) writeToDevices(event *C.struct_input_event) {
	capture.deviceMu.Lock()
	defer capture.deviceMu.Unlock()

	for _, file := range capture.files {
		C.neru_evdev_write_event(C.int(file.Fd()), event)
	}
}
