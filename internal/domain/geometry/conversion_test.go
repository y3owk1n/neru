package geometry_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/geometry"
)

const testNegativeOffsetScreen = "negative offset screen"

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
			result := geometry.NormalizeToLocalCoordinates(testCase.screenBounds)
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
			name:         "multi-monitor extended screen",
			localPoint:   image.Point{X: 100, Y: 200},
			screenBounds: image.Rect(1920, 0, 3840, 1080), // Second monitor
			expected:     image.Point{X: 2020, Y: 200},
		},
		{
			name:         testNegativeOffsetScreen,
			localPoint:   image.Point{X: 50, Y: 75},
			screenBounds: image.Rect(-1920, -1080, 0, 0),
			expected:     image.Point{X: -1870, Y: -1005},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := geometry.ConvertToAbsoluteCoordinates(
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
			result := geometry.ClampInt(testCase.value, testCase.minVal, testCase.maxVal)
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
	localHintPos := geometry.ConvertToLocalCoordinates(screenHintPos, screenBounds)

	expectedLocalPos := image.Point{X: 960, Y: 540} // 2880-1920=960, 540-0=540

	if localHintPos != expectedLocalPos {
		t.Errorf("Local coordinate conversion failed: got %v, expected %v",
			localHintPos, expectedLocalPos)
	}

	// Verify that converting back to absolute works
	absolutePos := geometry.ConvertToAbsoluteCoordinates(localHintPos, screenBounds)
	if absolutePos != screenHintPos {
		t.Errorf("Round-trip conversion failed: got %v, expected %v",
			absolutePos, screenHintPos)
	}
}

func TestConvertToLocalCoordinates(t *testing.T) {
	tests := []struct {
		name         string
		screenPoint  image.Point
		screenBounds image.Rectangle
		expected     image.Point
	}{
		{
			name:         "origin screen",
			screenPoint:  image.Point{X: 100, Y: 200},
			screenBounds: image.Rect(0, 0, 1920, 1080),
			expected:     image.Point{X: 100, Y: 200},
		},
		{
			name:         "multi-monitor extended screen",
			screenPoint:  image.Point{X: 2020, Y: 200},
			screenBounds: image.Rect(1920, 0, 3840, 1080), // Second monitor
			expected:     image.Point{X: 100, Y: 200},
		},
		{
			name:         testNegativeOffsetScreen,
			screenPoint:  image.Point{X: -1870, Y: -1005},
			screenBounds: image.Rect(-1920, -1080, 0, 0),
			expected:     image.Point{X: 50, Y: 75},
		},
		{
			name:         "center of second monitor",
			screenPoint:  image.Point{X: 2880, Y: 540},
			screenBounds: image.Rect(1920, 0, 3840, 1080),
			expected:     image.Point{X: 960, Y: 540},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := geometry.ConvertToLocalCoordinates(
				testCase.screenPoint,
				testCase.screenBounds,
			)
			if result != testCase.expected {
				t.Errorf("ConvertToLocalCoordinates(%v, %v) = %v, expected %v",
					testCase.screenPoint, testCase.screenBounds, result, testCase.expected)
			}
		})
	}
}
