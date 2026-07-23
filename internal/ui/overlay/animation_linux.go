//go:build linux && cgo

package overlay

import "time"

const (
	animationFPS      = 120
	animationFrameDur = time.Second / animationFPS
)

// easeInOut applies a smoothstep ease-in-out interpolation.
// Matches the visual feel of kCAMediaTimingFunctionEaseInEaseOut on macOS.
func easeInOut(progress float64) float64 {
	const (
		smoothStep3 = 3
		smoothStep2 = 2
	)

	if progress <= 0 {
		return 0
	}

	if progress >= 1 {
		return 1
	}

	return progress * progress * (smoothStep3 - smoothStep2*progress)
}

// lerp linearly interpolates between a and b by t.
func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}
