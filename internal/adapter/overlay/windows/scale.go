//go:build windows

package windows

import (
	"image"
	"math"
)

// The window's pixels are physical, so this backend multiplies every size
// from config by the window's monitor scale (winplatform.OverlayWindow.Scale),
// as the X11 backend does with Xft.dpi. Positions and bounds are physical
// pixels already and never go through here. Fonts, paddings, line widths and
// radii do.

// scaledInt is size at scale, rounded to a pixel.
func scaledInt(size int, scale float64) int {
	return int(math.Round(float64(size) * scale))
}

// scaledPoint is an offset configured in points at scale. Unlike a size, a
// negative offset is a direction rather than "auto", so it scales too.
func scaledPoint(x, y int, scale float64) image.Point {
	return image.Pt(scaledInt(x, scale), scaledInt(y, scale))
}

// scaledConfig is a configured pixel value at scale. A negative value is the
// "auto" sentinel badge.AutoPadding and badge.BorderRadius resolve from the
// (already scaled) font size, and it is passed through as it is.
func scaledConfig(value int, scale float64) int {
	if value < 0 {
		return value
	}

	return scaledInt(value, scale)
}
