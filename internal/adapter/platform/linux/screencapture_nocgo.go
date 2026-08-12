//go:build linux && !cgo

package linux

import (
	"image"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The cgo twins live in screencapture_x11_cgo.go,
// screencapture_wayland_cgo.go and screencapture_wayland_kde_cgo.go. Every
// capture backend is native — XGetImage, a wl_shm screencopy client, and a
// PipeWire stream off the portal's ScreenCast session — so a pure-Go build has
// no capture at all and says so rather than handing back an empty image.

func x11CaptureRegion(region image.Rectangle) (*image.RGBA, error) {
	_ = region

	return nil, derrors.New(
		derrors.CodeNotSupported,
		"X11 screen capture requires CGO-enabled Linux builds",
	)
}

func wlrootsCaptureRegion(region image.Rectangle) (*image.RGBA, error) {
	_ = region

	return nil, derrors.New(
		derrors.CodeNotSupported,
		"wlroots screen capture requires CGO-enabled Linux builds",
	)
}

// pipewireCaptureNode is unreachable on this build — kdeCaptureRegion refuses
// before it ever negotiates a session, so no descriptor is opened to leak here.
// It exists so the KDE path compiles, and it refuses in the same words.
func pipewireCaptureNode(
	remoteFD int,
	nodeID uint32,
	crop image.Rectangle,
	logicalWidth int,
	logicalHeight int,
	budgetMS int,
) (*image.RGBA, error) {
	_, _, _ = remoteFD, nodeID, crop
	_, _, _ = logicalWidth, logicalHeight, budgetMS

	return nil, derrors.New(
		derrors.CodeNotSupported,
		"KDE screen capture requires CGO-enabled Linux builds",
	)
}
