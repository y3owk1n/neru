package badge

import (
	"image"
	"math"
	"strconv"
	"strings"
)

const (
	hexOpaqueWhite = 0xFFFFFFFF
	hexRepeatCount = 2
	hexLenShort    = 3
	hexLenNoAlpha  = 6
	hexLenFull     = 8

	autoPaddingHorizontalMultiplier = 0.6
	autoPaddingVerticalMultiplier   = 0.35
	autoPaddingMinHorizontal        = 6
	autoPaddingMinVertical          = 4
	textWidthMultiplier             = 0.7
	textHeightMultiplier            = 1.4
	paddingSideCount                = 2

	fallbackFontSize = 14

	halfDivisor = 2
)

// ParseHexARGB converts a "#RGB", "#RRGGBB" or "#AARRGGBB" color to packed
// ARGB, returning opaque white for anything it cannot read.
func ParseHexARGB(value string) uint32 {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")

	switch len(value) {
	case hexLenShort:
		value = "FF" + strings.Repeat(string(value[0]), hexRepeatCount) +
			strings.Repeat(string(value[1]), hexRepeatCount) +
			strings.Repeat(string(value[2]), hexRepeatCount)
	case hexLenNoAlpha:
		value = "FF" + value
	case hexLenFull:
	default:
		return hexOpaqueWhite
	}

	parsed, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return hexOpaqueWhite
	}

	return uint32(parsed)
}

// AutoPadding resolves a configured padding value against the font size.
// Non-negative values are used as-is; negative values select an automatic
// padding proportional to the font size with a floor.
func AutoPadding(fontSize float64, padding int, horizontal bool) int {
	if padding >= 0 {
		return padding
	}

	if horizontal {
		return max(int(fontSize*autoPaddingHorizontalMultiplier), autoPaddingMinHorizontal)
	}

	return max(int(fontSize*autoPaddingVerticalMultiplier), autoPaddingMinVertical)
}

// EstimateTextWidth estimates the rendered width of text at the given font
// size, using the same rune-count heuristic on every platform.
func EstimateTextWidth(text string, fontSize float64) int {
	return int(math.Ceil(float64(len([]rune(text))) * fontSize * textWidthMultiplier))
}

// EstimateTextHeight estimates the rendered line height at the given font size.
func EstimateTextHeight(fontSize float64) int {
	return int(math.Ceil(fontSize * textHeightMultiplier))
}

// Size returns the outer badge width and height for text at the given font
// size, applying auto padding on both axes.
func Size(text string, fontSize float64, paddingX, paddingY int) (int, int) {
	resolvedX := AutoPadding(fontSize, paddingX, true)
	resolvedY := AutoPadding(fontSize, paddingY, false)

	width := EstimateTextWidth(text, fontSize) + resolvedX*paddingSideCount
	height := EstimateTextHeight(fontSize) + resolvedY*paddingSideCount

	return width, height
}

// Bounds returns the badge rectangle anchored at (posX+offsetX, posY+offsetY)
// and sized by Size. A non-positive font size falls back to a readable
// default.
func Bounds(
	posX, posY, offsetX, offsetY int,
	text string,
	fontSize float64,
	paddingX, paddingY int,
) image.Rectangle {
	if fontSize <= 0 {
		fontSize = fallbackFontSize
	}

	width, height := Size(text, fontSize, paddingX, paddingY)

	return image.Rect(
		posX+offsetX,
		posY+offsetY,
		posX+offsetX+width,
		posY+offsetY+height,
	)
}

// CenteredIn returns a width x height rectangle centered inside container.
//
// The centering is integer arithmetic and truncates toward the container's
// origin on an odd container or badge dimension, so a label plate lands on the
// same pixel on every backend. A badge larger than its container overhangs it
// rather than being clamped: the callers draw label plates, and a plate that
// outgrew its cell is the autohide threshold's problem, not this function's.
func CenteredIn(container image.Rectangle, width, height int) image.Rectangle {
	centerX := container.Min.X + container.Dx()/halfDivisor
	centerY := container.Min.Y + container.Dy()/halfDivisor

	return CenteredOn(image.Pt(centerX, centerY), width, height)
}

// CenteredOn returns a width x height rectangle centered on a point.
func CenteredOn(center image.Point, width, height int) image.Rectangle {
	originX := center.X - width/halfDivisor
	originY := center.Y - height/halfDivisor

	return image.Rect(originX, originY, originX+width, originY+height)
}

// BorderRadius resolves a configured border-radius value for the given
// rectangle. Negative values select an automatic radius: autoCap limits the
// auto-radius for badge-style corners (e.g. 6 px for hint badges); pass 0 for
// a full pill shape (label backgrounds). Zero means sharp corners. Positive
// values are clamped to half the smaller dimension.
func BorderRadius(configured int, bounds image.Rectangle, autoCap float64) float64 {
	maxRadius := float64(min(bounds.Dx(), bounds.Dy())) / halfDivisor

	if configured < 0 {
		if autoCap > 0 {
			return min(maxRadius, autoCap)
		}

		return maxRadius
	}

	if configured == 0 {
		return 0
	}

	return min(float64(configured), maxRadius)
}
