package app

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform"
)

// noCGOLinuxUnavailable is what a CGO_ENABLED=0 Linux daemon cannot do. Every
// entry is a `CodeNotSupported` mirror of a backend that needs native linkage,
// so the list is the boundary itself rather than a summary of it.
var noCGOLinuxUnavailable = []string{
	"cursor position and movement",
	"clicks and drags",
	"scrolling",
	"global hotkeys",
	"keyboard capture",
	"overlay drawing",
	"screen enumeration and display-hotplug events",
	"the focused application",
	"key injection (`neru key`)",
	"the vision hint strategy",
}

// announceBuildBoundary says once, at startup, that this build sits outside
// the parity boundary — and says nothing at all when it does not.
//
// ADR 0013 puts the CGO_ENABLED=0 Linux build outside that boundary: it is a
// distribution convenience for a configuration macOS does not offer, and it
// starts perfectly well before failing feature by feature. ADR 0012's
// criterion is that the first hour must not lie, and a daemon that reports
// itself running while every keystroke reaches a stub is the lie in its purest
// form. So the boundary is stated up front, with what it costs and how to
// leave it.
//
// It is a warning rather than an error because the build is a supported
// artifact, and it is emitted from exactly one place so the blessed stack
// never hears it: a warning every ordinary run prints is one people learn to
// scroll past.
func announceBuildBoundary(logger *zap.Logger, targetOS platform.OS, cgoEnabled bool) {
	if targetOS != platform.Linux || cgoEnabled {
		return
	}

	logger.Warn(
		"This is a CGO_ENABLED=0 Linux build: it runs, but nothing that reaches the display server does",
		zap.Strings("unavailable", noCGOLinuxUnavailable),
		zap.String(
			"remedy",
			"install the Linux build dependencies (docs/LINUX_SETUP.md) and rebuild with CGO_ENABLED=1",
		),
	)
}
