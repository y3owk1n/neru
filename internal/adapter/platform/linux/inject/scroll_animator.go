//go:build linux

package inject

import (
	eventtaplinux "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform/scrollanim"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// maxScrollUnitsPerRequest caps how many units one animated scroll travels on
// a granular backend, the same ceiling as x11ScrollAtCursor's maxClicks. A
// continuous backend reports granularity 0 and is never capped.
const maxScrollUnitsPerRequest = 50

// scrollBackendAvailable reports, synchronously and without side effects,
// whether an animated scroll can be injected at all, so the answer reaches
// whoever pressed the key.
//
// The animation runs on a worker goroutine where an error has nobody to go
// back to, and the backend enum alone cannot answer. A CGO_ENABLED=0 build
// reports the Wayland backend while every injection path is a CodeNotSupported
// stub. Without this check it would answer "scrolled" and move nothing, the
// silent no-op ADR 0013 exists to end. It answers "could this ever work". A
// failure once the animation runs goes unreported, as on darwin.
func scrollBackendAvailable() error {
	if currentLinuxBackend() == linuxBackendX11 {
		return x11ScrollBackendAvailable()
	}

	if currentLinuxBackend() == linuxBackendWayland {
		return waylandScrollBackendAvailable()
	}

	return derrors.New(
		derrors.CodeNotSupported,
		"animated scroll is not supported without an X11 or Wayland backend",
	)
}

// beginScrollSession opens a session on the live backend.
//
// The backend is read once here so one animation cannot straddle two, and the
// modifier press and release go to the same backend.
func beginScrollSession(modifiers action.Modifiers) (scrollanim.Session, error) {
	if currentLinuxBackend() == linuxBackendX11 {
		return newX11ScrollSession(modifiers)
	}

	if currentLinuxBackend() == linuxBackendWayland {
		// Hyprland needs its own session only to hold a modifier. Without one the
		// virtual pointer carries the chunks continuously, which beats whole notches.
		// Without uinput it falls back to the Wayland session.
		if modifiers != 0 &&
			hyprlandKeepsUinputScroll() &&
			eventtaplinux.IsUinputScrollAvailable() {
			return newHyprlandScrollSession(modifiers)
		}

		return newWaylandScrollSession(modifiers)
	}

	return nil, derrors.New(
		derrors.CodeNotSupported,
		"animated scroll is not supported without an X11 or Wayland backend",
	)
}

// scrollAnim spreads every animated scroll over time. Each request opens a
// session on the live backend, and a granular one travels at most
// maxScrollUnitsPerRequest notches.
var scrollAnim = scrollanim.New(beginScrollSession, maxScrollUnitsPerRequest)
