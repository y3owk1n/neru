package geometry

import (
	"image"
)

// NormalizeToLocalCoordinates converts screen-absolute coordinates to window-local coordinates.
// The overlay window is positioned at the screen origin, but the view uses local coordinates.
func NormalizeToLocalCoordinates(screenBounds image.Rectangle) image.Rectangle {
	return image.Rect(0, 0, screenBounds.Dx(), screenBounds.Dy())
}

// ConvertToAbsoluteCoordinates converts window-local coordinates to screen-absolute coordinates.
func ConvertToAbsoluteCoordinates(
	localPoint image.Point,
	screenBounds image.Rectangle,
) image.Point {
	return image.Point{
		X: localPoint.X + screenBounds.Min.X,
		Y: localPoint.Y + screenBounds.Min.Y,
	}
}

// ConvertToLocalCoordinates converts screen-absolute coordinates to window-local coordinates.
func ConvertToLocalCoordinates(
	screenPoint image.Point,
	screenBounds image.Rectangle,
) image.Point {
	return image.Point{
		X: screenPoint.X - screenBounds.Min.X,
		Y: screenPoint.Y - screenBounds.Min.Y,
	}
}

// ClampInt clamps an int value between minVal and maxVal.
func ClampInt(value, minVal, maxVal int) int {
	if value < minVal {
		return minVal
	}

	if value > maxVal {
		return maxVal
	}

	return value
}
