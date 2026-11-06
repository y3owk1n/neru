//go:build linux && cgo

package linux

import (
	"image"
	"math"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/motion"
)

const (
	animationFPS      = 120
	animationFrameDur = time.Second / animationFPS

	// defaultMouseActionDuration is the mouse-action indicator's animation
	// length when the configured one is non-positive.
	defaultMouseActionDuration = 260 * time.Millisecond
)

// Easing curve names accepted by the mouse-action-indicator config.
const (
	easingLinear    = "linear"
	easingEaseIn    = "ease_in"
	easingEaseOut   = "ease_out"
	easingEaseInOut = "ease_in_out"
)

// ARGB (0xAARRGGBB) channel layout used when fading indicator colors.
const (
	alphaShift            = 24
	colorByteMask         = 0xFF
	rgbMask        uint32 = 0x00FFFFFF
	alphaRoundBias        = 0.5
)

// applyEasing maps linear progress in [0,1] through the named easing curve.
// Unknown names fall back to ease_out, the config default.
func applyEasing(easing string, progress float64) float64 {
	if progress <= 0 {
		return 0
	}

	if progress >= 1 {
		return 1
	}

	switch easing {
	case easingLinear:
		return progress
	case easingEaseIn:
		return progress * progress * progress
	case easingEaseInOut:
		return motion.EaseInOut(progress)
	case easingEaseOut:
		// Computed below, shared with the unknown-name fallback.
	}

	// ease_out (the config default) and any unrecognized name: 1 - (1-t)^3.
	inv := 1 - progress

	return 1 - inv*inv*inv
}

// applyOpacity scales the alpha of an ARGB (0xAARRGGBB) color by opacity in
// [0,1], leaving RGB untouched, to fade the mouse-action indicator.
func applyOpacity(color uint32, opacity float64) uint32 {
	if opacity <= 0 {
		return color & rgbMask
	}

	if opacity >= 1 {
		return color
	}

	alpha := float64((color>>alphaShift)&colorByteMask) * opacity
	scaled := uint32(alpha + alphaRoundBias)

	return (scaled << alphaShift) | (color & rgbMask)
}

// mouseActionIndicatorRect returns the square bounding box of a mouse-action
// indicator of the given diameter centered on point. A circle is drawn as a
// rounded rect with radius diameter/2 inside it.
//
// Not badge.CenteredOn, because the diameter animates as a float and each
// edge is rounded separately, so the box tracks a growing circle smoothly.
func mouseActionIndicatorRect(point image.Point, diameter float64) image.Rectangle {
	half := diameter / halfDivisor
	minX := int(math.Round(float64(point.X) - half))
	minY := int(math.Round(float64(point.Y) - half))
	maxX := int(math.Round(float64(point.X) + half))
	maxY := int(math.Round(float64(point.Y) + half))

	return image.Rect(minX, minY, maxX, maxY)
}
