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

	"github.com/y3owk1n/neru/internal/derrors"
)

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
// libeiEnsure brings up the libei session this backend injects through.
//
// This file is the KDE Plasma input slot, sibling to the wlroots one. KWin does
// not implement zwlr_virtual_pointer_v1, so input goes through libei via the
// org.freedesktop.portal.RemoteDesktop portal. The libei mechanism itself
// (libei_client.c) is desktop-agnostic: if another compositor later routes
// input through libei, factor the shared pieces out rather than copying them.
// Runtime selection happens in system_wayland_input.go.
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

// execute runs an input action against the active libei client. If the action
// fails (e.g. because the portal session went stale across system sleep), it
// resets the session, re-acquires a fresh session, and retries once.
func (s *libeiState) execute(
	clientFn func(client *C.NeruEiClient) bool,
	errorProvider func() error,
) error {
	err := s.tryAcquire()
	if err != nil {
		return err
	}

	client := s.client
	ok := clientFn(client)
	s.mu.Unlock()

	if ok {
		return nil
	}

	// Action failed on existing session; it might be stale after system suspend.
	// Reset and attempt a single reconnect + retry.
	LibeiReset()

	err = s.tryAcquire()
	if err != nil {
		return err
	}
	defer s.mu.Unlock()

	if !clientFn(s.client) {
		return errorProvider()
	}

	return nil
}

func libeiMoveAbs(posX, posY int) error {
	return globalLibeiState.execute(
		func(c *C.NeruEiClient) bool { return C.neru_ei_move_abs(c, C.int(posX), C.int(posY)) != 0 },
		func() error {
			return derrors.Newf(
				derrors.CodeActionFailed,
				"libei failed to move pointer to (%d, %d)",
				posX, posY,
			)
		},
	)
}

func libeiButton(button int, pressed bool) error {
	pressedInt := C.int(0)
	if pressed {
		pressedInt = C.int(1)
	}

	return globalLibeiState.execute(
		func(c *C.NeruEiClient) bool { return C.neru_ei_button(c, C.int(button), pressedInt) != 0 },
		func() error {
			return derrors.New(derrors.CodeActionFailed, "libei failed to emit button event")
		},
	)
}

func libeiScroll(axis, delta int) error {
	return libeiScrollContinuous(axis, float64(delta))
}

// libeiScrollContinuous emits one scroll of an arbitrary pixel distance,
// including a fraction of a wheel notch.
//
// ei_device_scroll_delta is pixel-precise and KWin forwards it with a zero
// v120 step count, so the fraction reaches the focused client as a plain
// wl_pointer.axis instead of being held back until a whole notch adds up. That
// is what the smooth-scroll animator needs, and libeiScroll is the same call
// with a whole number.
func libeiScrollContinuous(axis int, delta float64) error {
	return globalLibeiState.execute(
		func(c *C.NeruEiClient) bool {
			return C.neru_ei_scroll(c, C.int(axis), C.double(delta)) != 0
		},
		func() error {
			return derrors.New(derrors.CodeActionFailed, "libei failed to emit scroll event")
		},
	)
}

func libeiKey(keycode int, pressed bool) error {
	pressedInt := C.int(0)
	if pressed {
		pressedInt = C.int(1)
	}

	return globalLibeiState.execute(
		func(c *C.NeruEiClient) bool { return C.neru_ei_key(c, C.int(keycode), pressedInt) != 0 },
		func() error {
			return derrors.New(
				derrors.CodeNotSupported,
				"libei keyboard injection unavailable; the RemoteDesktop portal "+
					"session did not grant a keyboard device. Restart neru and "+
					"check \"Enable keyboard\" in the consent prompt; if the prompt "+
					"does not appear, run `flatpak permission-remove kde-authorized "+
					"remote-desktop \"\"` first to clear the saved grant",
			)
		},
	)
}

// libeiHasKeyboard reports whether the granted libei session includes a keyboard
// device. The portal often grants only pointer + absolute-pointer capability,
// so this check lets callers fail fast with a clear message rather than
// discovering the absence mid-sequence.
//
// It uses TryLock so that `action feed` never blocks behind the portal warm-up
// lock (held up to 120 s). If the lock is busy, busy is true and the caller
// should return a retriable error instead of a permanent unsupported result.
func libeiHasKeyboard() (bool, bool) {
	if !globalLibeiState.mu.TryLock() {
		return false, true
	}

	defer globalLibeiState.mu.Unlock()

	if !globalLibeiState.ready {
		return false, false
	}

	return C.neru_ei_has_keyboard(globalLibeiState.client) != 0, false
}

// LibeiReset tears down the libei/RemoteDesktop portal session. The next input
// operation re-establishes the session via tryAcquire. Call after sleep/wake and
// after a detected evdev failure so that stale portal connections don't silently
// block input injection.
func LibeiReset() {
	globalLibeiState.mu.Lock()
	defer globalLibeiState.mu.Unlock()

	if !globalLibeiState.ready {
		return
	}

	if globalLibeiState.client != nil {
		C.neru_ei_disconnect(globalLibeiState.client)
		globalLibeiState.client = nil
	}

	globalLibeiState.ready = false
}
