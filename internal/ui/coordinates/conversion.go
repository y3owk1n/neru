package coordinates

import (
	"image"
	"math"
)

// ComputeRestoredPosition calculates the restored cursor position when switching screens.
// It maintains the relative position of the cursor within the screen bounds.
func ComputeRestoredPosition(initPos image.Point, fromPoint, toPoint image.Rectangle) image.Point {
	// If screens are the same, no adjustment needed
	if fromPoint == toPoint {
		return initPos
	}

	// Validate screen bounds
	if fromPoint.Dx() == 0 || fromPoint.Dy() == 0 || toPoint.Dx() == 0 || toPoint.Dy() == 0 {
		return initPos
	}

	// Calculate relative position (0.0 to 1.0) within the original screen
	relativeX := float64(initPos.X-fromPoint.Min.X) / float64(fromPoint.Dx())
	relativeY := float64(initPos.Y-fromPoint.Min.Y) / float64(fromPoint.Dy())

	// Clamp relative positions to valid range
	relativeX = ClampFloat(relativeX, 0, 1)
	relativeY = ClampFloat(relativeY, 0, 1)

	// Calculate new position in target screen
	newX := toPoint.Min.X + int(math.Round(relativeX*float64(toPoint.Dx())))
	newY := toPoint.Min.Y + int(math.Round(relativeY*float64(toPoint.Dy())))

	// Clamp to target screen bounds
	newX = ClampInt(newX, toPoint.Min.X, toPoint.Max.X)
	newY = ClampInt(newY, toPoint.Min.Y, toPoint.Max.Y)

	return image.Point{X: newX, Y: newY}
}

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

// ClampFloat clamps a float64 value between minVal and maxVal.
func ClampFloat(value, minVal, maxVal float64) float64 {
	if value < minVal {
		return minVal
	}

	if value > maxVal {
		return maxVal
	}

	return value
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
