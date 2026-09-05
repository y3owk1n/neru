// Test support for the probe's integration test, built only under the
// integration tag: a proxy keyboard of the test's own, and a grab from a second
// fd standing in for a remapper. Nothing here reaches a shipped binary.

//go:build integration && linux && cgo

package linux

/*
#include "../../platform/linux/evdev.h"
*/
import "C"

import (
	"errors"
	"os"
)

var errUinputRefused = errors.New("uinput refused to create the device")

// testProxyKeyboard is a uinput keyboard created the way the proxy creates its
// own, held by its uinput fd.
type testProxyKeyboard struct {
	fd C.int
}

func createTestProxyKeyboard() (*testProxyKeyboard, error) {
	var descriptor C.int = -1

	created, errno := C.neru_uinput_create_proxy_keyboard(&descriptor)
	if created == 0 {
		if errno == nil {
			errno = errUinputRefused
		}

		return nil, errno
	}

	return &testProxyKeyboard{fd: descriptor}, nil
}

// node finds the keyboard's event node as the proxy finds its own.
func (k *testProxyKeyboard) node() *proxyNode {
	return openProxyNode(k.fd)
}

func (k *testProxyKeyboard) destroy() {
	C.neru_uinput_destroy(k.fd)
}

// grabFile takes or releases an exclusive grab on file, as a remapper would.
func grabFile(file *os.File, grab bool) error {
	var flag C.int
	if grab {
		flag = 1
	}

	_, err := C.neru_evdev_grab(C.int(file.Fd()), flag)

	return err
}
