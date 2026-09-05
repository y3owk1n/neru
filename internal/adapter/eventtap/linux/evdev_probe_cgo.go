// The proxy's own event nodes, opened so the proxy can ask whether another
// process has grabbed one of them.
//
// A key remapper's device auto-detect takes any node that advertises keyboard
// keys, and kanata's excludes only a device named "kanata", keyd's only its own
// virtual keyboard. So a remapper grabs neru-keyboard-proxy when the daemon
// creates it, or on its own start when the daemon is already up. With the
// remapper's output grabbed here that closes a loop: every key the user types
// circles between the two processes and reaches the compositor from neither.
// Neru cannot change the remapper's filter, so it watches its own nodes and
// fails open when one is taken (evdevProxy.probeOwnDevices).

//go:build linux && cgo

package linux

/*
#include "../../platform/linux/evdev.h"
*/
import "C"

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// proxyNodeSysnameSize bounds the sysfs name UI_GET_SYSNAME answers, "inputN".
const proxyNodeSysnameSize = 64

// proxyNode is one of the proxy's event nodes, held open for probing. Nothing
// reads it: the kernel keeps its client buffer bounded on its own.
type proxyNode struct {
	file *os.File
}

// openProxyNode finds the event node behind a created uinput fd, by the sysfs
// name the fd answers and the one event directory sysfs lists under it. It
// returns nil until udev has created the node, and the caller asks again on its
// next tick.
func openProxyNode(uinputFd C.int) *proxyNode {
	var sysname [proxyNodeSysnameSize]C.char
	if C.neru_uinput_get_sysname(uinputFd, &sysname[0], proxyNodeSysnameSize) < 0 {
		return nil
	}

	matches, err := filepath.Glob(
		filepath.Join("/sys/devices/virtual/input", C.GoString(&sysname[0]), "event*"),
	)
	if err != nil || len(matches) != 1 {
		return nil
	}

	file, err := os.Open(filepath.Join("/dev/input", filepath.Base(matches[0])))
	if err != nil {
		return nil
	}

	return &proxyNode{file: file}
}

// heldByAnother reports whether another process has grabbed the node: a grab
// of our own answers EBUSY exactly then, and is let go again at once otherwise.
//
// Only the run goroutine calls it, and that goroutine is the one writer to the
// device, so no key can be emitted while the probe grab is in place and the
// compositor cannot lose one to it. What the grab window can drop is an LED
// change the compositor writes inside it, which the kernel discards from a
// non-grabber; the next LED change repairs that. A remapper's grab-ungrab-grab
// on registering a device can slip between two probes; the next one catches it.
func (n *proxyNode) heldByAnother() bool {
	descriptor := C.int(n.file.Fd())

	_, err := C.neru_evdev_grab(descriptor, 1)
	if err != nil {
		return errors.Is(err, syscall.EBUSY)
	}

	C.neru_evdev_grab(descriptor, 0)

	return false
}
