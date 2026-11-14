// Test support for the late-keyboard integration test, built only under the
// integration tag. It adds a virtual keyboard to the compositor's seat through
// the wlroots client the daemon types with. Nothing here reaches a shipped
// binary.

//go:build integration && linux && cgo

package linux

/*
#cgo pkg-config: wayland-client xkbcommon
#include "../../platform/linux/wlroots_client.h"
*/
import "C"

// testSeatKeyboard is a virtual keyboard on the compositor's seat, there for
// as long as its client stays connected.
type testSeatKeyboard struct {
	client *C.NeruWlrootsClient
}

// addTestSeatKeyboard returns nil when the compositor offers no virtual
// keyboard.
func addTestSeatKeyboard() *testSeatKeyboard {
	client := C.neru_wlr_connect()
	if client == nil {
		return nil
	}

	if C.neru_wlr_has_virtual_keyboard(client) == 0 {
		C.neru_wlr_disconnect(client)

		return nil
	}

	return &testSeatKeyboard{client: client}
}

func (keyboard *testSeatKeyboard) remove() {
	C.neru_wlr_disconnect(keyboard.client)
}
