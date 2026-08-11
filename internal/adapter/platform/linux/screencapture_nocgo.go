//go:build linux && !cgo

package linux

import (
	"image"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The cgo twins live in screencapture_x11_cgo.go and
// screencapture_wayland_cgo.go. Both capture backends are native — XGetImage
// and a wl_shm screencopy client — so a pure-Go build has no capture at all and
// says so rather than handing back an empty image.

func x11CaptureRegion(region image.Rectangle) (*image.RGBA, error) {
	_ = region

	return nil, derrors.New(
		derrors.CodeNotSupported,
		"X11 screen capture requires CGO-enabled Linux builds",
	)
}

func wlrootsCaptureRegion(region image.Rectangle, backend string) (*image.RGBA, error) {
	_, _ = region, backend

	return nil, derrors.New(
		derrors.CodeNotSupported,
		"wlroots screen capture requires CGO-enabled Linux builds",
	)
}
