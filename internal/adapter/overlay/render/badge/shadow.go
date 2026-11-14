package badge

import (
	"image"
	"math"
)

// A hint's shadow is its badge outline, connector arrow included, blurred the
// way a browser blurs a CSS box-shadow: a Gaussian whose standard deviation is
// half the blur radius. The Linux and Windows overlays paint the mask built
// here in the shadow color. The macOS overlay hands the same outline to Core
// Animation, whose shadowRadius is that standard deviation, so a configured
// blur draws the same shadow on all three.
const (
	// shadowSigmaPerBlur is the CSS relation between a blur radius and the
	// standard deviation of the Gaussian it stands for.
	shadowSigmaPerBlur = 0.5
	// shadowKernelSigmas is how far the kernel reaches, in standard
	// deviations. Past three, a tap weighs under half a percent.
	shadowKernelSigmas = 3
	// maxCachedShadows bounds the masks a ShadowCache keeps. Labels of one
	// length mostly measure alike, so a screen of hints needs a handful.
	maxCachedShadows = 128
	// arrowSamplesPerAxis supersamples the arrow, whose slanted edges have
	// no distance field as cheap as the rounded rectangle's.
	arrowSamplesPerAxis = 4
	// maxAlpha is a fully covered mask pixel.
	maxAlpha = 255
	// pixelCenter is the offset from a pixel's corner to its center.
	pixelCenter = 0.5
)

// ShadowMask is a hint badge's outline blurred into an alpha mask, in device
// pixels.
type ShadowMask struct {
	// Origin is where the mask's top-left pixel lands relative to the
	// badge rectangle's top-left corner, before the shadow is offset.
	Origin image.Point
	// Width and Height are the mask's size in pixels.
	Width, Height int
	// Alpha holds Width*Height coverage values row by row, top row first,
	// from 0 (no shadow) to 255 (the shadow color at full strength).
	Alpha []byte
}

// Rect is where the mask lands for a badge whose rectangle starts at badgeMin,
// with the shadow moved by offset.
func (m *ShadowMask) Rect(badgeMin, offset image.Point) image.Rectangle {
	topLeft := badgeMin.Add(m.Origin).Add(offset)

	return image.Rectangle{Min: topLeft, Max: topLeft.Add(image.Pt(m.Width, m.Height))}
}

// ShadowCache builds hint shadow masks and keeps them, one per badge shape, so
// a redraw on every keystroke blurs nothing it has blurred before. It is not
// safe for concurrent use. Each overlay keeps its own and reaches it under the
// lock its draws already hold.
type ShadowCache struct {
	masks map[shadowKey]*ShadowMask
}

// shadowKey is everything a mask depends on. The arrow is relative to the
// badge's top-left corner, so a badge moved across the screen hits its entry.
type shadowKey struct {
	width, height int
	radius        float64
	arrow         HintArrow
	blur          float64
}

// HintShadow returns the shadow mask for a badge of badgeRect's size, with the
// corner radius it is drawn with and, when hasArrow, the connector arrow
// PlaceHint gave it. blur is the CSS blur radius in device pixels, and 0
// draws a hard-edged shadow.
func (c *ShadowCache) HintShadow(
	badgeRect image.Rectangle,
	radius float64,
	arrow HintArrow,
	hasArrow bool,
	blur float64,
) *ShadowMask {
	key := shadowKey{
		width:  badgeRect.Dx(),
		height: badgeRect.Dy(),
		radius: radius,
		blur:   math.Max(blur, 0),
	}
	if hasArrow {
		key.arrow = HintArrow{
			BaseLeft:  arrow.BaseLeft.Sub(badgeRect.Min),
			Tip:       arrow.Tip.Sub(badgeRect.Min),
			BaseRight: arrow.BaseRight.Sub(badgeRect.Min),
		}
	}

	if mask, ok := c.masks[key]; ok {
		return mask
	}

	if c.masks == nil || len(c.masks) >= maxCachedShadows {
		c.masks = make(map[shadowKey]*ShadowMask)
	}

	mask := buildHintShadow(key, hasArrow)
	c.masks[key] = mask

	return mask
}

// buildHintShadow rasterizes the outline a key describes and blurs it.
func buildHintShadow(key shadowKey, hasArrow bool) *ShadowMask {
	sigma := key.blur * shadowSigmaPerBlur
	reach := int(math.Ceil(shadowKernelSigmas * sigma))

	shape := image.Rect(0, 0, key.width, key.height)
	if hasArrow {
		shape = shape.Union(triangleBounds(key.arrow))
	}

	mask := &ShadowMask{
		Origin: shape.Min.Sub(image.Pt(reach, reach)),
		Width:  shape.Dx() + 2*reach,
		Height: shape.Dy() + 2*reach,
	}

	coverage := outlineCoverage(key, hasArrow, mask)
	if reach > 0 {
		blurSeparable(coverage, mask.Width, mask.Height, gaussianKernel(sigma, reach))
	}

	mask.Alpha = make([]byte, len(coverage))
	for i, value := range coverage {
		mask.Alpha[i] = byte(math.Round(math.Min(math.Max(value, 0), 1) * maxAlpha))
	}

	return mask
}

// outlineCoverage is how much of each mask pixel the badge outline covers,
// from 0 to 1. The rounded rectangle is anti-aliased through its distance
// field and the arrow by supersampling, and where they meet the larger
// coverage wins, so the seam between them is not counted twice.
func outlineCoverage(key shadowKey, hasArrow bool, mask *ShadowMask) []float64 {
	coverage := make([]float64, mask.Width*mask.Height)

	halfW := float64(key.width) / halfDivisor
	halfH := float64(key.height) / halfDivisor
	// Cairo clamps the radius the same way when it builds the outline.
	radius := math.Max(math.Min(key.radius, math.Min(halfW, halfH)), 0)

	for row := range mask.Height {
		centerY := float64(mask.Origin.Y+row) + pixelCenter - halfH

		for col := range mask.Width {
			centerX := float64(mask.Origin.X+col) + pixelCenter - halfW
			dist := roundedBoxDistance(centerX, centerY, halfW, halfH, radius)
			coverage[row*mask.Width+col] = math.Min(math.Max(pixelCenter-dist, 0), 1)
		}
	}

	if hasArrow {
		addTriangleCoverage(coverage, mask, key.arrow)
	}

	return coverage
}

// roundedBoxDistance is the signed distance from a point to a rounded
// rectangle centered on the origin: negative inside, positive outside.
func roundedBoxDistance(pointX, pointY, halfW, halfH, radius float64) float64 {
	distX := math.Abs(pointX) - halfW + radius
	distY := math.Abs(pointY) - halfH + radius
	outside := math.Hypot(math.Max(distX, 0), math.Max(distY, 0)) - radius
	inside := math.Min(math.Max(distX, distY), 0)

	return outside + inside
}

// addTriangleCoverage raises each pixel the arrow touches to the share of its
// samples inside the arrow.
func addTriangleCoverage(coverage []float64, mask *ShadowMask, arrow HintArrow) {
	area := triangleBounds(
		arrow,
	).Sub(mask.Origin).
		Intersect(image.Rect(0, 0, mask.Width, mask.Height))
	step := 1.0 / arrowSamplesPerAxis

	for row := area.Min.Y; row < area.Max.Y; row++ {
		for col := area.Min.X; col < area.Max.X; col++ {
			inside := 0

			for sampleY := range arrowSamplesPerAxis {
				pointY := float64(mask.Origin.Y+row) + (float64(sampleY)+pixelCenter)*step

				for sampleX := range arrowSamplesPerAxis {
					pointX := float64(mask.Origin.X+col) + (float64(sampleX)+pixelCenter)*step
					if insideTriangle(pointX, pointY, arrow) {
						inside++
					}
				}
			}

			share := float64(inside) / (arrowSamplesPerAxis * arrowSamplesPerAxis)
			idx := row*mask.Width + col
			coverage[idx] = math.Max(coverage[idx], share)
		}
	}
}

// insideTriangle reports whether a point lies inside the arrow, whichever way
// its vertices wind.
func insideTriangle(pointX, pointY float64, arrow HintArrow) bool {
	vertices := [3]image.Point{arrow.BaseLeft, arrow.Tip, arrow.BaseRight}

	var negative, positive bool

	for i, from := range vertices {
		to := vertices[(i+1)%len(vertices)]
		cross := float64(to.X-from.X)*(pointY-float64(from.Y)) -
			float64(to.Y-from.Y)*(pointX-float64(from.X))
		negative = negative || cross < 0
		positive = positive || cross > 0
	}

	return !negative || !positive
}

// triangleBounds is the smallest rectangle holding the arrow's vertices.
func triangleBounds(arrow HintArrow) image.Rectangle {
	bounds := image.Rectangle{Min: arrow.BaseLeft, Max: arrow.BaseLeft}
	for _, vertex := range []image.Point{arrow.Tip, arrow.BaseRight} {
		bounds.Min.X = min(bounds.Min.X, vertex.X)
		bounds.Min.Y = min(bounds.Min.Y, vertex.Y)
		bounds.Max.X = max(bounds.Max.X, vertex.X)
		bounds.Max.Y = max(bounds.Max.Y, vertex.Y)
	}

	return bounds
}

// gaussianKernel returns the normalized weights of a Gaussian of standard
// deviation sigma, from -reach to reach.
func gaussianKernel(sigma float64, reach int) []float64 {
	kernel := make([]float64, 2*reach+1)
	total := 0.0

	for i := range kernel {
		offset := float64(i - reach)
		kernel[i] = math.Exp(-offset * offset / (2 * sigma * sigma))
		total += kernel[i]
	}

	for i := range kernel {
		kernel[i] /= total
	}

	return kernel
}

// blurSeparable convolves values with kernel along rows and then along
// columns, which is the 2D Gaussian blur because a Gaussian separates. Pixels
// past the edge count as empty, and the mask is padded so the outline never
// reaches one.
func blurSeparable(values []float64, width, height int, kernel []float64) {
	reach := len(kernel) / halfDivisor
	line := make([]float64, max(width, height))

	for row := range height {
		start := row * width
		copy(line, values[start:start+width])

		for col := range width {
			sum := 0.0

			for tap, weight := range kernel {
				if src := col + tap - reach; src >= 0 && src < width {
					sum += line[src] * weight
				}
			}

			values[start+col] = sum
		}
	}

	for col := range width {
		for row := range height {
			line[row] = values[row*width+col]
		}

		for row := range height {
			sum := 0.0

			for tap, weight := range kernel {
				if src := row + tap - reach; src >= 0 && src < height {
					sum += line[src] * weight
				}
			}

			values[row*width+col] = sum
		}
	}
}
