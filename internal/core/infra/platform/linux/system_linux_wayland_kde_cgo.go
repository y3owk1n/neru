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

// This file is the KDE Plasma Wayland input slot (compositor sub-slot "kde",
// sibling to the "wlroots" slot). KWin does not implement
// zwlr_virtual_pointer_v1, so input is injected through libei via the
// org.freedesktop.portal.RemoteDesktop portal. The libei mechanism itself
// (libei_client.c) is DE-agnostic; if another compositor (e.g. GNOME) later
// routes input through libei, factor the shared pieces out rather than
// duplicating them here. Runtime selection happens in
// system_linux_wayland_input.go via the LinuxBackend family.

// libeiConnectTimeoutMs bounds how long a lazy (mid-action) input op waits for
// the libei/RemoteDesktop session. It MUST stay short: mid-action calls run on
// the eventtap goroutine that holds the keyboard grab, so any blocking here
// freezes the global hotkey listener and buffers the user's keystrokes until it
// unblocks. If warm-up did not already establish the session the overlay is on
// screen and hides the consent dialog anyway, so a long wait only stalls the UI.
// Establishing the session is warm-up's job (libeiWarmupTimeoutMs); the lazy
// path just fails fast so the mode can exit and release the grab.
const libeiConnectTimeoutMs = 1500

// libeiWarmupTimeoutMs bounds the startup warm-up wait. It is long because the
// consent dialog appears while no overlay is up, giving the user a comfortable
// window to find and approve the one-time "Remote Control" prompt. Once
// approved here, every later action reuses the session with no further wait.
const libeiWarmupTimeoutMs = 120000

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
	return s.ensureLockedTimeout(libeiConnectTimeoutMs)
}

// ensureLockedTimeout establishes the portal session with an explicit connect
// timeout. The caller holds mu.
func (s *libeiState) ensureLockedTimeout(timeoutMs int) error {
	if s.ready {
		return nil
	}

	client := C.neru_ei_connect(C.int(timeoutMs))
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
// past the IPC timeout. This is the only path allowed to hold mu across the
// long consent wait; mid-action input uses tryAcquire so it never blocks here.
func libeiEnsure() error {
	globalLibeiState.mu.Lock()
	defer globalLibeiState.mu.Unlock()

	return globalLibeiState.ensureLockedTimeout(libeiWarmupTimeoutMs)
}

// tryAcquire grabs mu without blocking and guarantees the session is ready.
// It exists so mid-action input calls never stall the eventtap goroutine (which
// holds the keyboard grab) behind warm-up's long-held lock: if warm-up is still
// waiting on the consent prompt, TryLock fails immediately and the action fails
// fast instead of freezing every hotkey. On success the caller owns mu and must
// Unlock; on any error mu is already released.
func (s *libeiState) tryAcquire() error {
	if !s.mu.TryLock() {
		return derrors.New(
			derrors.CodeActionFailed,
			"libei input session busy (RemoteDesktop warm-up in progress); "+
				"approve the one-time \"Remote Control\" consent prompt, then retry",
		)
	}

	err := s.ensureLocked()
	if err != nil {
		s.mu.Unlock()

		return err
	}

	return nil
}

func libeiMoveAbs(posX, posY int) error {
	err := globalLibeiState.tryAcquire()
	if err != nil {
		return err
	}
	defer globalLibeiState.mu.Unlock()

	client := globalLibeiState.client

	if C.neru_ei_move_abs(client, C.int(posX), C.int(posY)) == 0 { //nolint:nlreturn
		return derrors.Newf(
			derrors.CodeActionFailed,
			"libei failed to move pointer to (%d, %d)",
			posX, posY,
		)
	}

	return nil
}

func libeiButton(button int, pressed bool) error {
	err := globalLibeiState.tryAcquire()
	if err != nil {
		return err
	}
	defer globalLibeiState.mu.Unlock()

	client := globalLibeiState.client

	pressedInt := C.int(0)
	if pressed {
		pressedInt = C.int(1)
	}

	if C.neru_ei_button(client, C.int(button), pressedInt) == 0 { //nolint:nlreturn
		return derrors.New(derrors.CodeActionFailed, "libei failed to emit button event")
	}

	return nil
}

func libeiScroll(axis, delta int) error {
	err := globalLibeiState.tryAcquire()
	if err != nil {
		return err
	}
	defer globalLibeiState.mu.Unlock()

	client := globalLibeiState.client

	if C.neru_ei_scroll(client, C.int(axis), C.int(delta)) == 0 { //nolint:nlreturn
		return derrors.New(derrors.CodeActionFailed, "libei failed to emit scroll event")
	}

	return nil
}

func libeiKey(keycode int, pressed bool) error {
	err := globalLibeiState.tryAcquire()
	if err != nil {
		return err
	}
	defer globalLibeiState.mu.Unlock()

	client := globalLibeiState.client

	pressedInt := C.int(0)
	if pressed {
		pressedInt = C.int(1)
	}

	if C.neru_ei_key(client, C.int(keycode), pressedInt) == 0 { //nolint:nlreturn
		return derrors.New(
			derrors.CodeNotSupported,
			"libei keyboard injection unavailable; the RemoteDesktop portal "+
				"session did not grant a keyboard device",
		)
	}

	return nil
}
