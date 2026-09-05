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
	"os"
	"path/filepath"
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

	// grab is whether devices are taken exclusively (EVIOCGRAB) as they are
	// opened. The proxy grabs; a passive capture, which has no uinput device
	// to re-emit through, reads alongside the compositor instead.
	grab bool

	closeOnce sync.Once
	stopCh    chan struct{}
	done      sync.WaitGroup

	deviceMu      sync.Mutex
	inotifyFd     int
	hotplugStopCh chan struct{}
	hotplugOnce   sync.Once
	hotplugWg     sync.WaitGroup

	// xkbState holds a C xkb_state initialized from the compositor's keymap.
	// Used to resolve evdev scan codes to key names that respect XKB options.
	xkbState unsafe.Pointer // *C.neru_xkb_state

	// devices is what the capture knows about each tracked file beyond its
	// path; guarded by deviceMu like files.
	devices map[*os.File]trackedDevice

	// pending is the keyboards the constructor found with a key down, whose
	// grab waits for them to go idle; startReaders launches those waits.
	pending []string

	// pendingPaths is the keyboards whose wait is in flight, so overlapping
	// scans start one wait per keyboard and a session asked for while one is
	// still on its way in can be refused by name; deviceMu.
	pendingPaths map[string]struct{}

	// pointer is the pointer proxy, or nil until a grabbed keyboard needs one:
	// the uinput mouse the relative motion and the buttons of a keyboard that
	// carries them are re-emitted on. Created under deviceMu by whichever
	// scan grabs the first such keyboard, read without it by the run goroutine.
	pointer atomic.Pointer[pointerProxy]
}

// pointerProxy is the pointer half of the proxy device, held by its uinput fd.
type pointerProxy struct {
	fd C.int
}

type trackedDevice struct {
	// virtual is whether the device is another process's uinput keyboard, a
	// remapper's output. Physical keyboards are yielded to a remapper when
	// one starts (yieldPhysicalKeyboards); virtual ones never are.
	virtual bool

	// released is whether this capture let go of the file on purpose, or is
	// about to (yieldDevice), so its reader's exit is not mistaken for a
	// device that vanished.
	released bool
}

// newWaylandEvdevCapture opens every keyboard under /dev/input. With grab set,
// each is taken exclusively on the way in, and only while no key on it is down
// (deviceBusy): a keyboard with a key held is grabbed once it goes idle.
//
// A device another process already holds (a key remapper such as kanata or keyd
// grabs its input keyboards for its own lifetime) refuses the grab and is
// skipped; the remapper's virtual output keyboard is taken instead, which is the
// one that carries the user's keys. A remapper that starts after this capture
// finds its keyboards held instead, so they are yielded to it when its output
// device arrives (yieldPhysicalKeyboards). That device also advertises mouse
// motion and buttons, so a key can move the pointer; they are re-emitted on the
// pointer proxy, which is created the first time such a keyboard is grabbed.
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

		// Stop the hotplug watcher first: closing the inotify fd unblocks
		// the hotplugLoop goroutine, causing it to exit.
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

		// Wait for all reader goroutines to finish. A reader blocked in read
		// returns on the device's next event or on the closed fd, and one
		// blocked handing an event on returns on stopCh. We must wait here so
		// that no reader can send on the events channel after we close it.
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
	// deviceSkipped is a node that is not a keyboard this capture reads, or
	// one it already has, or one another grabber owns.
	deviceSkipped deviceOutcome = iota
	// deviceAdopted is a keyboard now tracked, grabbed when the capture grabs.
	deviceAdopted
	// deviceBusy is a keyboard with a key down, which the capture will not
	// hold: libinput keeps that key down until a release arrives on the same
	// device, and a release re-emitted on the proxy is discarded as a release
	// for a key never pressed there (ADR 0014). The device is grabbed once it
	// is idle instead (waitIdle).
	deviceBusy
)

// addDeviceLocked opens path and keeps it when it is a keyboard this capture
// should read and no key on it is down. The caller holds deviceMu, or is the
// constructor before anyone else can.
func (capture *waylandEvdevCapture) addDeviceLocked(path string) (*os.File, deviceOutcome) {
	// Read-write so the compositor's LED changes can be written back to the
	// device; a node that only grants reading is still a keyboard to read.
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

	// A Neru device is known by its vendor id as well as its name, so the proxy
	// cannot grab its own output even on a node whose name did not read.
	if C.neru_evdev_is_keyboard(descriptor) == 0 ||
		C.neru_evdev_has_abs_axes(descriptor) != 0 ||
		C.neru_evdev_is_neru_device(descriptor) != 0 ||
		isNeruInjectionDevice(name) {
		_ = file.Close()

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
			// Debug, not warn: its output device is captured in its place.
			if capture.logger != nil {
				capture.logger.Debug(
					"Keyboard device refused the grab; leaving it to its owner",
					zap.String("device", path),
				)
			}

			_ = file.Close()

			return nil, deviceSkipped
		}

		// A keyboard with relative motion on it is kept only once that motion
		// has somewhere to go; otherwise it is handed back to the compositor,
		// keys and all, rather than held with its motion swallowed. After the
		// grab, so a keyboard another grabber owns creates no pointer proxy.
		if C.neru_evdev_has_rel_axes(descriptor) != 0 && !capture.ensurePointerProxyLocked(path) {
			C.neru_evdev_grab(descriptor, 0)
			_ = file.Close()

			return nil, deviceSkipped
		}

		// Asked under the grab, so no press can land between the answer and
		// the grab; a key found down means the grab is let go again at once,
		// and its release reaches the compositor as the press did.
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
// reports whether there is one. The caller holds deviceMu, so two scans cannot
// create two.
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

// keysHeld reports whether the kernel has any key down on fd. The error is a
// device that cannot answer, which is one that is gone.
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

// startReaders launches reader goroutines for each captured keyboard device,
// the idle waits for the keyboards found with a key down, and the inotify
// hotplug watcher. These goroutines run for the entire lifetime of the capture.
//
// A remapper's output device found by the scan may belong to a remapper still
// inside its startup delay, launched alongside the daemon: its inputs were
// free, so the scan took them, and the remapper is about to want them. They
// are yielded as they would be had the output device arrived later; a
// remapper that is long up holds its inputs already, so there is nothing to
// yield and the yield is free.
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

// grabWhenIdleLocked starts the wait that grabs a busy keyboard once no key
// on it is down, unless one is already waiting on it. The caller holds deviceMu.
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
// forgotten before adopt runs, not deferred past it: a key pressed again in the
// gap makes adopt register a replacement wait on the same path, which a
// cleanup running afterwards would erase.
func (capture *waylandEvdevCapture) waitIdle(path string) {
	idle := capture.pollUntilIdle(path)

	capture.waitDone(path)

	if idle {
		capture.adopt(path, "its key came up")
	}
}

// pollUntilIdle reports whether path reached a state with no key down. It
// polls the key state rather than reading the device, so it takes nothing from
// the device's owner; the poll quantum is paid once per device, when a daemon
// or a remapper is launched from a binding, and never per activation. False
// is a device that is gone, or a capture that is closing.
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

// pendingCount is how many keyboards are waiting to go idle before they are
// grabbed.
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

// readLoop hands every event the device produces to the proxy. The send blocks
// rather than drops: with the device grabbed, an event dropped here is a key the
// user pressed that reaches nothing.
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
			// so we don't attempt to write LEDs to a stale fd.
			capture.deviceMu.Lock()
			device := capture.devices[file]
			capture.removeFileLocked(file)
			capture.deviceMu.Unlock()
			_ = file.Close()

			// A device that is gone may have been a remapper's output, whose
			// exit also freed the keyboards it held; pick those up. Not for a
			// device this capture let go of itself, which its remapper wants.
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

		select {
		case capture.events <- waylandEvdevEvent{
			eventType: uint16(inputEvent._type),
			code:      uint16(inputEvent.code),
			value:     int32(inputEvent.value),
		}:
		case <-capture.stopCh:
			return
		}
	}
}

// removeFileLocked removes file from the tracked files slice.
// Must be called with capture.deviceMu held.
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
// the remapper whose output device just arrived, which is about to grab its
// inputs and would find them held. Each is let go once no key on it is down
// (yieldDevice), and taken back after the remapper's grab window if the
// remapper did not want it. Virtual keyboards, another remapper's output among
// them, are not a remapper's input and stay.
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

// yieldDevice lets go of one held keyboard once no key on it is down, so a
// press this capture forwarded is not left without its release in the
// compositor's picture of the proxy (the invariant ADR 0014 rests on). The
// release is an ungrab and a revoke, which wakes the reader with ENODEV; the
// remapper's grab then succeeds. After the grace the device is adopted again
// if it is still free, which is a remapper that did not want it.
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
}

// releaseIfIdle ungrabs and revokes file if no key on it is down, and reports
// whether it did, or whether the device is gone. The key state is read under
// deviceMu after checking the file is still tracked: the reader removes a
// vanished device under the same lock before it closes the file, so the
// descriptor cannot be closed under the ioctl.
func (capture *waylandEvdevCapture) releaseIfIdle(file *os.File) (bool, bool) {
	capture.deviceMu.Lock()
	defer capture.deviceMu.Unlock()

	if _, tracked := capture.devices[file]; !tracked {
		return false, true
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

	return true, false
}

// ungrabAll hands every device back to the compositor and stops grabbing the
// ones that arrive later. It is the proxy failing open: with nothing to re-emit
// through, holding the keyboards would be holding the user's keys.
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

// writeToDevices writes one event to every captured device. It is how the
// compositor's LED changes, which land on the proxy keyboard, reach the lights
// on the physical ones.
func (capture *waylandEvdevCapture) writeToDevices(event *C.struct_input_event) {
	capture.deviceMu.Lock()
	defer capture.deviceMu.Unlock()

	for _, file := range capture.files {
		C.neru_evdev_write_event(C.int(file.Fd()), event)
	}
}
