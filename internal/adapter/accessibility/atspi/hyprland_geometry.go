//go:build linux

package atspi

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/compositorcli"
)

// Hyprland window-origin source. `hyprctl -j activewindow` reports the focused
// window's absolute position ("at") and size ("size"), which give the screen
// origin directly.
type hyprlandOriginSource struct {
	logger *zap.Logger
}

func newHyprlandOriginSource(logger *zap.Logger) *hyprlandOriginSource {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &hyprlandOriginSource{logger: logger.Named("accessibility.hyprland")}
}

func (h *hyprlandOriginSource) start() {}

// hyprlandWindow mirrors the fields of `hyprctl -j activewindow` we use.
// "at" is [x, y] and "size" is [w, h], both in absolute screen pixels.
type hyprlandWindow struct {
	At   []int `json:"at"`
	Size []int `json:"size"`
}

func (h *hyprlandOriginSource) originFor(frameW, frameH int) (image.Point, bool, error) {
	var win hyprlandWindow

	err := compositorcli.Query(&win, "hyprctl", "-j", "activewindow")
	if err != nil {
		return image.Point{}, false, err
	}

	origin, ok := hyprlandComputeOrigin(win, frameW, frameH, h.logger)

	return origin, ok, nil
}

// hyprlandComputeOrigin derives the focused window's screen origin from
// `hyprctl activewindow` data, rejecting a size mismatch with the AT-SPI frame.
func hyprlandComputeOrigin(
	win hyprlandWindow,
	frameW, frameH int,
	logger *zap.Logger,
) (image.Point, bool) {
	if len(win.At) < coordPairLen || len(win.Size) < coordPairLen {
		return image.Point{}, false
	}

	if absInt(win.Size[0]-frameW) > windowOriginSizeTolerance ||
		absInt(win.Size[1]-frameH) > windowOriginSizeTolerance {
		logger.Debug("hyprland origin rejected: window size does not match AT-SPI frame",
			zap.Ints("windowSize", win.Size),
			zap.Int("frameW", frameW), zap.Int("frameH", frameH))

		return image.Point{}, false
	}

	return image.Pt(win.At[0], win.At[1]), true
}
