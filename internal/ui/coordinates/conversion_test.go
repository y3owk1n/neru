//go:build unit

package coordinates_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/ui/coordinates"
)

func TestComputeRestoredPosition(t *testing.T) {
	tests := []struct {
		name     string
		initPos  image.Point
		fromRect image.Rectangle
		toRect   image.Rectangle
		expected image.Point
	}{
		{
			name:     "same screen",
			initPos:  image.Point{X: 100, Y: 200},
			fromRect: image.Rect(0, 0, 1920, 1080),
			toRect:   image.Rect(0, 0, 1920, 1080),
			expected: image.Point{X: 100, Y: 200},
		},
		{
			name:     "different screen same size",
			initPos:  image.Point{X: 100, Y: 200},
			fromRect: image.Rect(0, 0, 1920, 1080),
			toRect:   image.Rect(1920, 0, 3840, 1080),
			expected: image.Point{X: 2020, Y: 200},
		},
		{
			name:     "different screen different size",
			initPos:  image.Point{X: 960, Y: 540}, // center of 1920x1080
			fromRect: image.Rect(0, 0, 1920, 1080),
			toRect:   image.Rect(0, 0, 2560, 1440),
			expected: image.Point{X: 1280, Y: 720}, // center of 2560x1440
		},
		{
			name:     "clamp to bounds",
			initPos:  image.Point{X: 2000, Y: 1200}, // outside original screen
			fromRect: image.Rect(0, 0, 1920, 1080),
			toRect:   image.Rect(0, 0, 2560, 1440),
			expected: image.Point{X: 2560, Y: 1440}, // clamped to max
		},
		{
			name:     "zero width screen",
			initPos:  image.Point{X: 100, Y: 200},
			fromRect: image.Rect(0, 0, 0, 1080),
			toRect:   image.Rect(0, 0, 1920, 1080),
			expected: image.Point{X: 100, Y: 200}, // returns original position
		},
		{
			name:     "zero height screen",
			initPos:  image.Point{X: 100, Y: 200},
			fromRect: image.Rect(0, 0, 1920, 0),
			toRect:   image.Rect(0, 0, 1920, 1080),
			expected: image.Point{X: 100, Y: 200}, // returns original position
		},
		{
			name:     "relative position calculation",
			initPos:  image.Point{X: 480, Y: 270}, // quarter of 1920x1080
			fromRect: image.Rect(0, 0, 1920, 1080),
			toRect:   image.Rect(0, 0, 1280, 720),
			expected: image.Point{X: 320, Y: 180}, // quarter of 1280x720
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := coordinates.ComputeRestoredPosition(
				testCase.initPos,
				testCase.fromRect,
				testCase.toRect,
			)
			if result != testCase.expected {
				t.Errorf("ComputeRestoredPosition(%v, %v, %v) = %v, expected %v",
					testCase.initPos, testCase.fromRect, testCase.toRect, result, testCase.expected)
			}
		})
	}
}

func TestNormalizeToLocalCoordinates(t *testing.T) {
	tests := []struct {
		name         string
		screenBounds image.Rectangle
		expected     image.Rectangle
	}{
		{
			name:         "standard screen",
			screenBounds: image.Rect(0, 0, 1920, 1080),
			expected:     image.Rect(0, 0, 1920, 1080),
		},
		{
			name:         "offset screen",
			screenBounds: image.Rect(1920, 0, 3840, 1080),
			expected:     image.Rect(0, 0, 1920, 1080),
		},
		{
			name:         "small screen",
			screenBounds: image.Rect(100, 50, 300, 200),
			expected:     image.Rect(0, 0, 200, 150),
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := coordinates.NormalizeToLocalCoordinates(testCase.screenBounds)
			if result != testCase.expected {
				t.Errorf("NormalizeToLocalCoordinates(%v) = %v, expected %v",
					testCase.screenBounds, result, testCase.expected)
			}
		})
	}
}

func TestConvertToAbsoluteCoordinates(t *testing.T) {
	tests := []struct {
		name         string
		localPoint   image.Point
		screenBounds image.Rectangle
		expected     image.Point
	}{
		{
			name:         "origin screen",
			localPoint:   image.Point{X: 100, Y: 200},
			screenBounds: image.Rect(0, 0, 1920, 1080),
			expected:     image.Point{X: 100, Y: 200},
		},
		{
			name:         "offset screen",
			localPoint:   image.Point{X: 100, Y: 200},
			screenBounds: image.Rect(1920, 0, 3840, 1080),
			expected:     image.Point{X: 2020, Y: 200},
		},
		{
			name:         "negative offset screen",
			localPoint:   image.Point{X: 50, Y: 75},
			screenBounds: image.Rect(-1920, -1080, 0, 0),
			expected:     image.Point{X: -1870, Y: -1005},
		},
		{
			name:         "multi-monitor extended screen",
			localPoint:   image.Point{X: 100, Y: 200},
			screenBounds: image.Rect(1920, 0, 3840, 1080), // Second monitor
			expected:     image.Point{X: 2020, Y: 200},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := coordinates.ConvertToAbsoluteCoordinates(
				testCase.localPoint,
				testCase.screenBounds,
			)
			if result != testCase.expected {
				t.Errorf("ConvertToAbsoluteCoordinates(%v, %v) = %v, expected %v",
					testCase.localPoint, testCase.screenBounds, result, testCase.expected)
			}
		})
	}
}

func TestClampFloat(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		minVal   float64
		maxVal   float64
		expected float64
	}{
		{
			name:     "within range",
			value:    0.5,
			minVal:   0.0,
			maxVal:   1.0,
			expected: 0.5,
		},
		{
			name:     "below minimum",
			value:    -0.5,
			minVal:   0.0,
			maxVal:   1.0,
			expected: 0.0,
		},
		{
			name:     "above maximum",
			value:    1.5,
			minVal:   0.0,
			maxVal:   1.0,
			expected: 1.0,
		},
		{
			name:     "equal to minimum",
			value:    0.0,
			minVal:   0.0,
			maxVal:   1.0,
			expected: 0.0,
		},
		{
			name:     "equal to maximum",
			value:    1.0,
			minVal:   0.0,
			maxVal:   1.0,
			expected: 1.0,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := coordinates.ClampFloat(testCase.value, testCase.minVal, testCase.maxVal)
			if result != testCase.expected {
				t.Errorf("ClampFloat(%v, %v, %v) = %v, expected %v",
					testCase.value, testCase.minVal, testCase.maxVal, result, testCase.expected)
			}
		})
	}
}

func TestClampInt(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		minVal   int
		maxVal   int
		expected int
	}{
		{
			name:     "within range",
			value:    50,
			minVal:   0,
			maxVal:   100,
			expected: 50,
		},
		{
			name:     "below minimum",
			value:    -10,
			minVal:   0,
			maxVal:   100,
			expected: 0,
		},
		{
			name:     "above maximum",
			value:    150,
			minVal:   0,
			maxVal:   100,
			expected: 100,
		},
		{
			name:     "equal to minimum",
			value:    0,
			minVal:   0,
			maxVal:   100,
			expected: 0,
		},
		{
			name:     "equal to maximum",
			value:    100,
			minVal:   0,
			maxVal:   100,
			expected: 100,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := coordinates.ClampInt(testCase.value, testCase.minVal, testCase.maxVal)
			if result != testCase.expected {
				t.Errorf("ClampInt(%v, %v, %v) = %v, expected %v",
					testCase.value, testCase.minVal, testCase.maxVal, result, testCase.expected)
			}
		})
	}
}

func TestMultiMonitor_CoordinateConversion(t *testing.T) {
	// Test case: Hint on extended screen should be converted to local coordinates
	// Screen bounds: (1920, 0) to (3840, 1080) - second monitor in extended desktop
	screenBounds := image.Rect(1920, 0, 3840, 1080)

	// Hint position in screen coordinates (center of second monitor)
	screenHintPos := image.Point{X: 2880, Y: 540} // center of second monitor

	// Convert to local coordinates (relative to overlay window at screen origin)
	localHintPos := image.Point{
		X: screenHintPos.X - screenBounds.Min.X,
		Y: screenHintPos.Y - screenBounds.Min.Y,
	}

	expectedLocalPos := image.Point{X: 960, Y: 540} // 2880-1920=960, 540-0=540

	if localHintPos != expectedLocalPos {
		t.Errorf("Local coordinate conversion failed: got %v, expected %v",
			localHintPos, expectedLocalPos)
	}

	// Verify that converting back to absolute works
	absolutePos := coordinates.ConvertToAbsoluteCoordinates(localHintPos, screenBounds)
	if absolutePos != screenHintPos {
		t.Errorf("Round-trip conversion failed: got %v, expected %v",
			absolutePos, screenHintPos)
	}
}
