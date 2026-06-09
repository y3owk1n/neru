//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: libei-1.0 liboeffis-1.0
#include <stdlib.h>
#include "libei_client.h"
*/
import "C"

import (
	"sync"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// libeiConnectTimeoutMs bounds how long the first input op waits for the user
// to approve (or dismiss) the RemoteDesktop portal consent dialog.
const libeiConnectTimeoutMs = 30000

// libeiState owns the libei/RemoteDesktop session used for input injection on
// compositors without zwlr_virtual_pointer_v1 (KWin/KDE). The session is
// established lazily on the first input operation so that read-only probes
// (screen bounds, `neru doctor`) never trigger the portal consent prompt.
type libeiState struct {
	mu     sync.Mutex
	client *C.NeruEiClient
	ready  bool
}

var globalLibeiState = &libeiState{}

// ensureLocked establishes the portal session on first use. The caller holds mu.
func (s *libeiState) ensureLocked() error {
	if s.ready {
		return nil
	}

	client := C.neru_ei_connect(C.int(libeiConnectTimeoutMs))
	if client == nil {
		return derrors.New(
			derrors.CodeActionFailed,
			"could not establish a libei input session via the RemoteDesktop "+
				"portal; approve the one-time \"Remote Control\" consent prompt "+
				"(KDE Plasma routes input through xdg-desktop-portal because KWin "+
				"does not implement zwlr_virtual_pointer_v1)",
		)
	}

	s.client = client
	s.ready = true

	return nil
}

// libeiEnsure establishes the portal session without injecting input. The
// daemon calls this at startup (via WarmWaylandInput) so the one-time consent
// prompt is handled before any action, instead of blocking the first action
// past the IPC timeout.
func libeiEnsure() error {
	globalLibeiState.mu.Lock()
	defer globalLibeiState.mu.Unlock()

	return globalLibeiState.ensureLocked()
}

func libeiMoveAbs(x, y int) error {
	globalLibeiState.mu.Lock()
	defer globalLibeiState.mu.Unlock()

	if err := globalLibeiState.ensureLocked(); err != nil {
		return err
	}

	if C.neru_ei_move_abs(globalLibeiState.client, C.int(x), C.int(y)) == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"libei failed to move pointer to (%d, %d)",
			x, y,
		)
	}

	return nil
}

func libeiButton(button int, pressed bool) error {
	globalLibeiState.mu.Lock()
	defer globalLibeiState.mu.Unlock()

	if err := globalLibeiState.ensureLocked(); err != nil {
		return err
	}

	pressedInt := C.int(0)
	if pressed {
		pressedInt = C.int(1)
	}

	if C.neru_ei_button(globalLibeiState.client, C.int(button), pressedInt) == 0 {
		return derrors.New(derrors.CodeActionFailed, "libei failed to emit button event")
	}

	return nil
}

func libeiScroll(axis, delta int) error {
	globalLibeiState.mu.Lock()
	defer globalLibeiState.mu.Unlock()

	if err := globalLibeiState.ensureLocked(); err != nil {
		return err
	}

	if C.neru_ei_scroll(globalLibeiState.client, C.int(axis), C.int(delta)) == 0 {
		return derrors.New(derrors.CodeActionFailed, "libei failed to emit scroll event")
	}

	return nil
}

func libeiKey(keycode int, pressed bool) error {
	globalLibeiState.mu.Lock()
	defer globalLibeiState.mu.Unlock()

	if err := globalLibeiState.ensureLocked(); err != nil {
		return err
	}

	pressedInt := C.int(0)
	if pressed {
		pressedInt = C.int(1)
	}

	if C.neru_ei_key(globalLibeiState.client, C.int(keycode), pressedInt) == 0 {
		return derrors.New(
			derrors.CodeNotSupported,
			"libei keyboard injection unavailable; the RemoteDesktop portal "+
				"session did not grant a keyboard device",
		)
	}

	return nil
}
