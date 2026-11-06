// The proxy's own event nodes, opened so the proxy can ask whether another
// process has grabbed one of them.
//
// Remapper auto-detect takes any node with keyboard keys (kanata excludes only
// "kanata", keyd only its own keyboard), so a remapper grabs
// neru-keyboard-proxy. With the remapper's output grabbed here, every key
// circles between the two processes. Neru cannot change the remapper's filter,
// so it fails open when one of its nodes is taken (evdevProxy.probeOwnDevices).

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
// reads it. The kernel keeps its client buffer bounded on its own.
type proxyNode struct {
	file *os.File
}

// openProxyNode finds the event node behind a uinput fd through the sysfs name
// the fd answers. It returns nil until udev has created the node, and the
// caller retries on its next tick.
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

// heldByAnother reports whether another process has grabbed the node. Our own
// grab answers EBUSY exactly then, and is released at once otherwise.
//
// Only the run goroutine calls it, and it is the device's one writer, so no
// key can be emitted during the probe grab. An LED change written inside the
// window is dropped by the kernel and repaired by the next one. A remapper's
// grab-ungrab-grab can slip between two probes, and the next catches it.
func (n *proxyNode) heldByAnother() bool {
	descriptor := C.int(n.file.Fd())

	_, err := C.neru_evdev_grab(descriptor, 1)
	if err != nil {
		return errors.Is(err, syscall.EBUSY)
	}

	C.neru_evdev_grab(descriptor, 0)

	return false
}
