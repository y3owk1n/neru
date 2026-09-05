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
	// name is the device name read at open, so a device that vanishes can be
	// named to the rescan it starts.
	name string

	// borrowedFor is the name of the vanished device whose rescan adopted this
	// one, or empty. A remapper that exits releases its input keyboards as its
	// output device disappears; a borrowed keyboard is one of those, and goes
	// back when a device of that name reappears (returnBorrowed).
	borrowedFor string

	// released is whether this capture revoked the file on purpose, so its
	// reader's exit is not mistaken for a device that vanished.
	released bool
}

// newWaylandEvdevCapture opens every keyboard under /dev/input. With grab set,
// each is taken exclusively on the way in and the keys the kernel reports held
// on it are pushed as seed events, ahead of anything the device sends.
//
// A device another process already holds (a key remapper such as kanata or keyd
// grabs its input keyboards for its own lifetime) refuses the grab and is
// skipped; the remapper's virtual output keyboard is taken instead, which is the
// one that carries the user's keys. That device also advertises mouse motion and
// buttons, so a key can move the pointer; they are re-emitted on the pointer
// proxy, which is created the first time such a keyboard is grabbed.
func newWaylandEvdevCapture(logger *zap.Logger, grab bool) (*waylandEvdevCapture, error) {
	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, err
	}

	capture := &waylandEvdevCapture{
		files:     make([]*os.File, 0, len(paths)),
		devices:   make(map[*os.File]trackedDevice),
		events:    make(chan waylandEvdevEvent, waylandEvdevEventBufferSize),
		logger:    logger,
		grab:      grab,
		stopCh:    make(chan struct{}),
		inotifyFd: -1,
	}

	for _, path := range paths {
		if _, seeds, ok := capture.addDeviceLocked(path); ok {
			capture.sendSeeds(seeds)
		}
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

// addDeviceLocked opens path and keeps it when it is a keyboard this capture
// should read, answering the file and the keys the kernel reported held on it
// at grab time. The caller holds deviceMu, or is the constructor before anyone
// else can, and sends the seeds itself once it no longer holds the lock — the
// proxy's run loop takes deviceMu too, so a send made under it could wait on
// the one goroutine that would drain it.
func (capture *waylandEvdevCapture) addDeviceLocked(path string) (*os.File, []uint16, bool) {
	// Read-write so the compositor's LED changes can be written back to the
	// device; a node that only grants reading is still a keyboard to read.
	file, openErr := os.OpenFile(path, os.O_RDWR, 0)
	if openErr != nil {
		file, openErr = os.Open(path)
	}

	if openErr != nil {
		return nil, nil, false
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

		return nil, nil, false
	}

	for _, tracked := range capture.files {
		if tracked.Name() == path {
			_ = file.Close()

			return nil, nil, false
		}
	}

	var seeds []uint16

	if capture.grab {
		// A keyboard with relative motion on it is grabbed only once that
		// motion has somewhere to go; otherwise it is left to the compositor,
		// keys and all, rather than grabbed with its motion swallowed.
		if C.neru_evdev_has_rel_axes(descriptor) != 0 && !capture.ensurePointerProxyLocked(path) {
			_ = file.Close()

			return nil, nil, false
		}

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

			return nil, nil, false
		}

		seeds = heldKeys(descriptor)
	}

	capture.files = append(capture.files, file)
	capture.devices[file] = trackedDevice{name: name}

	return file, seeds, true
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

// heldKeys is every key the kernel reports down on fd, read right after the
// grab so the compositor's picture of them can be honored (forwardRule.seed).
func heldKeys(fd C.int) []uint16 {
	var bits [evdevKeyCodeCount / evdevBitsPerWord]C.ulong
	if C.neru_evdev_get_key_state(fd, &bits[0], C.size_t(unsafe.Sizeof(bits))) != 0 {
		return nil
	}

	var codes []uint16

	for code := range uint16(evdevKeyCodeCount) {
		if bits[code/evdevBitsPerWord]&(1<<(code%evdevBitsPerWord)) != 0 {
			codes = append(codes, code)
		}
	}

	return codes
}

// sendSeeds pushes one seed event per held key. It has to run before the
// device's reader starts, so the seeds reach the proxy ahead of the device's
// first event.
func (capture *waylandEvdevCapture) sendSeeds(codes []uint16) {
	for _, code := range codes {
		select {
		case capture.events <- waylandEvdevEvent{eventType: evdevEventSeed, code: code}:
		case <-capture.stopCh:
			return
		}
	}
}

// startReaders launches reader goroutines for each captured keyboard device
// and starts the inotify hotplug watcher for detecting device hotplug.
// These goroutines run for the entire lifetime of the capture.
func (capture *waylandEvdevCapture) startReaders() {
	capture.deviceMu.Lock()
	for _, file := range capture.files {
		capture.startReader(file)
	}
	capture.deviceMu.Unlock()

	capture.startHotplugWatcher()
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
				go capture.rescan(device.name)
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
