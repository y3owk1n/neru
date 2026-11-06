//go:build linux && !cgo

package linux

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Every capture backend is native (XGetImage, wl_shm screencopy, PipeWire), so
// a pure-Go build refuses rather than handing back an empty image.

func x11CaptureRegion(region image.Rectangle) (*image.RGBA, error) {
	_ = region

	return nil, derrors.New(
		derrors.CodeNotSupported,
		"X11 screen capture requires CGO-enabled Linux builds",
	)
}

func wlrootsCaptureRegion(region image.Rectangle, budgetMS int) (*image.RGBA, error) {
	_, _ = region, budgetMS

	return nil, derrors.New(
		derrors.CodeNotSupported,
		"wlroots screen capture requires CGO-enabled Linux builds",
	)
}

// pipewireCaptureNode is unreachable here, because kdeCaptureRegion refuses
// before negotiating a session. It exists so the KDE path compiles.
func pipewireCaptureNode(
	ctx context.Context,
	remoteFD int,
	nodeID uint32,
	crop image.Rectangle,
	logicalWidth int,
	logicalHeight int,
) (*image.RGBA, error) {
	_, _, _ = ctx, remoteFD, nodeID
	_, _, _ = crop, logicalWidth, logicalHeight

	return nil, derrors.New(
		derrors.CodeNotSupported,
		"portal screen capture requires CGO-enabled Linux builds",
	)
}
