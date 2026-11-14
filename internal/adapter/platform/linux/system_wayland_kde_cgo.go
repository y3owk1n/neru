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
// the libei/RemoteDesktop session. It MUST stay short, because mid-action calls
// run on the eventtap goroutine holding the keyboard grab, and the overlay
// hides the consent dialog anyway. Establishing the session is warm-up's job.
const libeiConnectTimeoutMs = 1500

// libeiWarmupTimeoutMs bounds the startup warm-up wait. It is long because the
// one-time "Remote Control" consent dialog appears here with no overlay up.
// Warm-up presents the stored restore token, so a restart normally needs no
// dialog, and only the first grant on a machine costs the long wait.
const libeiWarmupTimeoutMs = 120000

// libeiState owns the libei/RemoteDesktop session used for input injection on
// compositors without zwlr_virtual_pointer_v1 (KWin/KDE). It is established on
// first input, so read-only probes never trigger the consent prompt.
type libeiState struct {
	mu     sync.Mutex
	client *C.NeruEiClient
	// closePortal ends the RemoteDesktop session and its bus connection. It is
	// nil for the liboeffis fallback, whose C client owns its session and ends
	// it in neru_ei_disconnect.
	closePortal func()
	ready       bool
}

var globalLibeiState = &libeiState{}

// ensureLocked establishes the portal session on first use. The caller holds mu.
func (s *libeiState) ensureLocked() error {
	return s.ensureLockedTimeout(libeiConnectTimeoutMs)
}

// ensureLockedTimeout establishes the portal session with an explicit connect
// timeout. The caller holds mu. Both attempts share one deadline, so an
// unresponsive portal never costs more than the caller allowed.
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

	// A user's own refusal is final. The fallback would show the dialog again.
	if !promptingFallbackAllowed(portalErr) {
		return libeiSessionError(portalErr)
	}

	// Fall back to the C client's liboeffis handshake. It cannot reuse a
	// grant, so it prompts, but that beats no session.
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

// connectViaPortal runs Neru's own RemoteDesktop handshake, which can present
// a stored restore token, and attaches libei to the EIS socket it yields. It
// returns the C client and the function that ends the session. The EIS fd
// passes to neru_ei_connect_fd, which closes it on every path.
func connectViaPortal(deadline time.Time) (*C.NeruEiClient, func(), error) {
	store, err := newFileRestoreTokenStore(remoteDesktopTokenBase)
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

	// Always hand the socket over, even with no budget left, because only
	// neru_ei_connect_fd closes it. The floor is 1 because the C side reads a
	// non-positive timeout as its 30 second default.
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

// libeiSessionError phrases the failure for the user, carrying the portal's
// reason as the cause. The restore token never reaches an error value
// (portal_restore_token.go).
func libeiSessionError(cause error) error {
	return derrors.Wrap(
		cause,
		derrors.CodeActionFailed,
		"could not establish a libei input session via the RemoteDesktop "+
			"portal; approve the one-time \"Remote Control\" consent prompt "+
			"(KDE Plasma and COSMIC route input through xdg-desktop-portal because "+
			"KWin and cosmic-comp do not implement zwlr_virtual_pointer_v1; COSMIC "+
			"needs xdg-desktop-portal-cosmic 1.7 or later for its RemoteDesktop portal)",
	)
}

// libeiEnsure establishes the portal session without injecting input. The
// daemon calls it at startup (via WarmWaylandInput), before any keyboard grab
// exists, so the consent prompt and grant restore never block an action. It is
// the only path allowed to hold mu across the long consent wait. Mid-action
// input uses tryAcquire and the short budget, for example after a
// suspend/resume reset.
//
// This file is the KDE input slot, sibling to the wlroots one. libei_client.c
// is desktop-agnostic, so factor shared pieces out rather than copying them
// for another compositor. Runtime selection is in system_wayland_input.go.
func libeiEnsure() error {
	globalLibeiState.mu.Lock()
	defer globalLibeiState.mu.Unlock()

	return globalLibeiState.ensureLockedTimeout(libeiWarmupTimeoutMs)
}

// tryAcquire grabs mu without blocking and guarantees the session is ready, so
// mid-action input never stalls the eventtap goroutine behind warm-up's lock.
// On success the caller owns mu and must Unlock. On error mu is released.
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

	// The session may be stale after suspend. Reset and retry once.
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

// libeiScrollContinuous emits one scroll of an arbitrary pixel distance. KWin
// forwards ei_device_scroll_delta with a zero v120 step count, so a fraction
// of a notch reaches the client at once, which the smooth-scroll animator needs.
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

// libeiScrollDiscrete emits wheel clicks, in 120ths of a click.
func libeiScrollDiscrete(axis, value120 int) error {
	return globalLibeiState.execute(
		func(c *C.NeruEiClient) bool {
			return C.neru_ei_scroll_discrete(c, C.int(axis), C.int(value120)) != 0
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

// libeiHasKeyboard reports whether the granted session includes a keyboard,
// which the portal often omits, so callers fail fast rather than mid-sequence.
// It uses TryLock so `action feed` never blocks behind warm-up. When busy is
// true the caller should return a retriable error, not unsupported.
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

// LibeiReset tears down the libei/RemoteDesktop portal session, which the next
// input re-establishes. Call it after sleep/wake and after an evdev failure, so
// a stale portal connection cannot silently block injection.
func LibeiReset() {
	globalLibeiState.mu.Lock()
	defer globalLibeiState.mu.Unlock()

	if !globalLibeiState.ready {
		return
	}

	// libei first, because it owns the EIS socket and would otherwise emit
	// into a dead transport during teardown.
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
