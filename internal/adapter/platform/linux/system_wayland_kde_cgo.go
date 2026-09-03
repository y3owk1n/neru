//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: libei-1.0 liboeffis-1.0
#include <stdlib.h>
#include "libei_client.h"
*/
import "C"

import (
	"context"
	"sync"
	"time"

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
//
// It is also the only place the prompt is expected to be paid at all: warm-up
// presents the stored restore token, so a restart normally establishes the
// session with no dialog and well inside this budget. The long timeout is what
// the first grant on a machine costs, not what every start costs.
const libeiWarmupTimeoutMs = 120000

// libeiState owns the libei/RemoteDesktop session used for input injection on
// compositors without zwlr_virtual_pointer_v1 (KWin/KDE). The session is
// established lazily on the first input operation so that read-only probes
// (screen bounds, `neru doctor`) never trigger the portal consent prompt.
type libeiState struct {
	mu     sync.Mutex
	client *C.NeruEiClient
	// closePortal ends the RemoteDesktop session this client injects through
	// and drops the bus connection holding it. It is nil when the session came
	// from the liboeffis fallback below, which owns its session inside the C
	// client and tears it down with neru_ei_disconnect.
	closePortal func()
	ready       bool
}

var globalLibeiState = &libeiState{}

// ensureLocked establishes the portal session on first use. The caller holds mu.
func (s *libeiState) ensureLocked() error {
	return s.ensureLockedTimeout(libeiConnectTimeoutMs)
}

// ensureLockedTimeout establishes the portal session with an explicit connect
// timeout. The caller holds mu.
//
// The whole budget is one deadline, shared by both attempts below, so a session
// never costs more wall clock than the caller allowed — which is what keeps the
// mid-action path from freezing the goroutine holding the keyboard grab even
// when the portal is unresponsive.
func (s *libeiState) ensureLockedTimeout(timeoutMs int) error {
	if s.ready {
		return nil
	}

	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)

	client, closePortal, portalErr := connectViaPortal(deadline)
	if portalErr == nil {
		s.client, s.closePortal, s.ready = client, closePortal, true

		return nil
	}

	// A refusal the user made themselves is final. The fallback below opens a
	// second session and would put the same dialog back in front of them, which
	// is the one outcome worse than having no session.
	if !promptingFallbackAllowed(portalErr) {
		return libeiSessionError(portalErr)
	}

	// Fall back to the liboeffis handshake inside the C client. It cannot reuse
	// a grant, so it costs the user a consent prompt, but a session that
	// prompts is worth far more than no session at all — and this is the path
	// every KDE install used before restore existed, so a portal Neru's own
	// handshake cannot drive still works exactly as well as it used to.
	remainingMs := int(time.Until(deadline).Milliseconds())
	if remainingMs <= 0 {
		return libeiSessionError(portalErr)
	}

	client = C.neru_ei_connect(C.int(remainingMs))
	if client == nil {
		return libeiSessionError(portalErr)
	}

	s.client, s.closePortal, s.ready = client, nil, true

	return nil
}

// connectViaPortal runs Neru's own RemoteDesktop handshake — the one that can
// present a stored restore token — and attaches libei to the EIS socket it
// yields. It returns the C client and the function that ends the session.
//
// The EIS file descriptor's ownership passes to neru_ei_connect_fd, which
// closes it on every path, so nothing here closes it: the portal session is
// what this function still has to unwind.
func connectViaPortal(deadline time.Time) (*C.NeruEiClient, func(), error) {
	store, err := newFileRestoreTokenStore(remoteDesktopTokenFileName)
	if err != nil {
		return nil, nil, derrors.Wrap(
			err,
			derrors.CodeActionFailed,
			"could not resolve where to keep the RemoteDesktop portal grant",
		)
	}

	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	grant, err := establishPortalGrant(ctx, store, openRemoteDesktopSession)
	if err != nil {
		return nil, nil, err
	}

	// Whatever is left of the budget, the socket is handed over rather than
	// short-circuited on: neru_ei_connect_fd is the only thing that closes the
	// descriptor, so a branch that skipped it would leak one per attempt. A
	// budget already spent buys a call that gives up immediately, which is what
	// was wanted anyway — and the floor is 1, because the C side reads a
	// non-positive timeout as "use the default", which is 30 seconds.
	remainingMs := max(int(time.Until(deadline).Milliseconds()), 1)

	client := C.neru_ei_connect_fd(C.int(grant.eisFD), C.int(remainingMs))
	if client == nil {
		grant.close()

		return nil, nil, derrors.Wrap(
			errPortalGrantYieldedNoDevices,
			derrors.CodeActionFailed,
			"the RemoteDesktop portal granted a session but no pointer device came "+
				"up on it",
		)
	}

	return client, grant.close, nil
}

// libeiSessionError phrases the failure for the user, carrying the portal's own
// reason as the cause. Nothing here can name the restore token: the token never
// reaches an error value, by construction (portal_restore_token.go).
func libeiSessionError(cause error) error {
	return derrors.Wrap(
		cause,
		derrors.CodeActionFailed,
		"could not establish a libei input session via the RemoteDesktop "+
			"portal; approve the one-time \"Remote Control\" consent prompt "+
			"(KDE Plasma routes input through xdg-desktop-portal because KWin "+
			"does not implement zwlr_virtual_pointer_v1)",
	)
}

// libeiEnsure establishes the portal session without injecting input. The
// daemon calls this at startup (via WarmWaylandInput) so the one-time consent
// prompt is handled before any action, instead of blocking the first action
// past the IPC timeout. This is the only path allowed to hold mu across the
// long consent wait; mid-action input uses tryAcquire so it never blocks here.
// libeiEnsure brings up the libei session this backend injects through.
//
// It is also where a stored grant is restored, which is why restoring belongs
// here and not on the first action: the handshake runs on the startup
// goroutine, before any keyboard grab exists, and by the time a mode can ask
// for input the session is already up. The mid-action path can still establish
// one — after a suspend/resume reset, say — but it does so under the short
// budget above, never the warm-up one.
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

	// libei first: it owns the EIS socket, and closing the portal session out
	// from under it would leave the client emitting into a dead transport for
	// as long as the teardown takes.
	if globalLibeiState.client != nil {
		C.neru_ei_disconnect(globalLibeiState.client)
		globalLibeiState.client = nil
	}

	if globalLibeiState.closePortal != nil {
		globalLibeiState.closePortal()
		globalLibeiState.closePortal = nil
	}

	globalLibeiState.ready = false
}
