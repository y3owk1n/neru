//go:build windows

package windows

import (
	"image"
	"math"
)

// Software rasterizer for the GDI surface: every primitive composites straight
// into a premultiplied BGRA buffer. The DirectComposition surface paints the
// same commands on the GPU and never reaches this file.

// paintCommand rasterizes one command into pixels and returns the rectangle it
// touched, clipped to the buffer. Text is painted through text, which owns the
// GDI objects the glyphs come from.
func paintCommand(
	pixels []byte,
	bufW, bufH int,
	cmd drawCmd,
	text *gdiTextRenderer,
) image.Rectangle {
	switch cmd.kind {
	case drawFill:
		alphaFillRect(pixels, bufW, bufH, cmd.rect, cmd.color)
	case drawRoundedFill:
		alphaFillRoundedRect(pixels, bufW, bufH, cmd.rect, cmd.radius, cmd.color)
	case drawStroke:
		alphaStrokeRect(pixels, bufW, bufH, cmd.rect, cmd.color, cmd.width)
	case drawRoundedStroke:
		alphaStrokeRoundedRect(pixels, bufW, bufH, cmd.rect, cmd.radius, cmd.color, cmd.width)
	case drawTriangle:
		alphaFillTriangle(pixels, bufW, bufH, cmd.vertices, cmd.color)
	case drawText:
		text.paint(pixels, bufW, bufH, cmd)
	}

	return cmd.bounds().Intersect(image.Rect(0, 0, bufW, bufH))
}

// zeroRect clears a rectangle of the buffer to transparent black.
func zeroRect(pixels []byte, bufW, bufH int, rect image.Rectangle) {
	rect = rect.Intersect(image.Rect(0, 0, bufW, bufH))
	if rect.Empty() {
		return
	}

	rowBytes := rect.Dx() * bytesPerPixel
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		start := (y*bufW + rect.Min.X) * bytesPerPixel
		clear(pixels[start : start+rowBytes])
	}
}

// alphaFillRect composites a semi-transparent ARGB fill over the pixel buffer.
//
// An opaque fill is a row copy: the first row is composed pixel by pixel and
// every row below it copies that one, which is what a grid of several hundred
// cells spends most of its paint on.
func alphaFillRect(pixels []byte, bufW, bufH int, rect image.Rectangle, color uint32) {
	colA := color >> alphaShift
	if colA == 0 {
		return
	}

	startY := clamp(rect.Min.Y, bufH)
	endY := clamp(rect.Max.Y, bufH)
	startX := clamp(rect.Min.X, bufW)
	endX := clamp(rect.Max.X, bufW)

	if startX >= endX || startY >= endY {
		return
	}

	if colA == alphaMax {
		fillOpaqueRows(pixels, bufW, startX, endX, startY, endY, color)

		return
	}

	colR := (color >> redShift) & byteMask
	colG := (color >> greenShift) & byteMask
	colB := color & byteMask

	// Pre-multiply the source color by its alpha.
	srcR := colR * colA
	srcG := colG * colA
	srcB := colB * colA

	invA := alphaMax - colA

	for y := startY; y < endY; y++ {
		row := y * bufW * bytesPerPixel
		for x := startX; x < endX; x++ {
			idx := row + x*bytesPerPixel
			dstB := uint32(pixels[idx])
			dstG := uint32(pixels[idx+1])
			dstR := uint32(pixels[idx+2])
			dstA := uint32(pixels[idx+3])

			pixels[idx] = byte((srcB + dstB*invA) / alphaMax)
			pixels[idx+1] = byte((srcG + dstG*invA) / alphaMax)
			pixels[idx+2] = byte((srcR + dstR*invA) / alphaMax)
			pixels[idx+3] = byte(colA + (dstA*invA)/alphaMax)
		}
	}
}

// fillOpaqueRows writes an opaque color over the rows [startY, endY) between
// columns [startX, endX): the first row pixel by pixel, the rest by copying it.
func fillOpaqueRows(pixels []byte, bufW, startX, endX, startY, endY int, color uint32) {
	colB := byte(color)
	colG := byte(color >> greenShift)
	colR := byte(color >> redShift)

	first := (startY*bufW + startX) * bytesPerPixel
	rowBytes := (endX - startX) * bytesPerPixel
	firstRow := pixels[first : first+rowBytes]

	for i := 0; i < rowBytes; i += bytesPerPixel {
		firstRow[i] = colB
		firstRow[i+1] = colG
		firstRow[i+2] = colR
		firstRow[i+3] = alphaMax
	}

	for y := startY + 1; y < endY; y++ {
		start := (y*bufW + startX) * bytesPerPixel
		copy(pixels[start:start+rowBytes], firstRow)
	}
}

// alphaStrokeRect composites a stroked rectangle border over the pixel buffer:
// two horizontal bands and two vertical ones, each lineWidth deep, so the
// corners are painted once.
func alphaStrokeRect(
	pixels []byte,
	bufW, bufH int,
	rect image.Rectangle,
	color uint32,
	lineWidth int,
) {
	if lineWidth < 1 {
		return
	}

	// A stroke wider than the rectangle is a fill.
	if lineWidth*2 >= rect.Dx() || lineWidth*2 >= rect.Dy() {
		alphaFillRect(pixels, bufW, bufH, rect, color)

		return
	}

	// Top and bottom bands, full width.
	alphaFillRect(pixels, bufW, bufH,
		image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+lineWidth), color)
	alphaFillRect(pixels, bufW, bufH,
		image.Rect(rect.Min.X, rect.Max.Y-lineWidth, rect.Max.X, rect.Max.Y), color)
	// Left and right bands, between them.
	alphaFillRect(
		pixels,
		bufW,
		bufH,
		image.Rect(
			rect.Min.X,
			rect.Min.Y+lineWidth,
			rect.Min.X+lineWidth,
			rect.Max.Y-lineWidth,
		),
		color,
	)
	alphaFillRect(
		pixels,
		bufW,
		bufH,
		image.Rect(
			rect.Max.X-lineWidth,
			rect.Min.Y+lineWidth,
			rect.Max.X,
			rect.Max.Y-lineWidth,
		),
		color,
	)
}

// triangleCoverage returns how much of the pixel at (col, row) the triangle
// covers, as a fraction of 1, by testing a triangleSamples x triangleSamples
// grid of points inside it against the triangle's three edges.
//
// A point is inside when it is on the same side of all three directed edges;
// the winding is unknown, so both all-non-negative and all-non-positive count.
// A degenerate triangle (three collinear points) has every cross product zero
// and would read as fully covered, so the caller drops it before sampling.
func triangleCoverage(vertices [triangleVertices]image.Point, col, row int) float64 {
	step := 1.0 / float64(triangleSamples)
	inside := 0

	for sampleY := range triangleSamples {
		pointY := float64(row) + (float64(sampleY)+pixelHalf)*step

		for sampleX := range triangleSamples {
			pointX := float64(col) + (float64(sampleX)+pixelHalf)*step

			negative, positive := false, false

			for edge := range triangleVertices {
				from := vertices[edge]
				to := vertices[(edge+1)%triangleVertices]

				cross := (float64(to.X-from.X))*(pointY-float64(from.Y)) -
					(float64(to.Y-from.Y))*(pointX-float64(from.X))

				if cross < 0 {
					negative = true
				}

				if cross > 0 {
					positive = true
				}
			}

			if !negative || !positive {
				inside++
			}
		}
	}

	return float64(inside) / float64(triangleSamples*triangleSamples)
}

// alphaFillTriangle composites an anti-aliased filled triangle over the pixel
// buffer. It is what draws the connector arrow between an offset hint badge and
// the element it labels; the Cairo and Quartz backends get the same shape from
// a path primitive they already have.
func alphaFillTriangle(
	pixels []byte,
	bufW, bufH int,
	vertices [triangleVertices]image.Point,
	color uint32,
) {
	colA := color >> alphaShift
	if colA == 0 {
		return
	}

	// A triangle with no area covers nothing; sampling it would read every
	// point as "inside" because all three cross products are zero.
	area := (vertices[1].X-vertices[0].X)*(vertices[2].Y-vertices[0].Y) -
		(vertices[1].Y-vertices[0].Y)*(vertices[2].X-vertices[0].X)
	if area == 0 {
		return
	}

	colR := (color >> redShift) & byteMask
	colG := (color >> greenShift) & byteMask
	colB := color & byteMask

	bounds := image.Rectangle{Min: vertices[0], Max: vertices[0]}
	for _, vertex := range vertices {
		bounds.Min.X = min(bounds.Min.X, vertex.X)
		bounds.Min.Y = min(bounds.Min.Y, vertex.Y)
		bounds.Max.X = max(bounds.Max.X, vertex.X)
		bounds.Max.Y = max(bounds.Max.Y, vertex.Y)
	}

	startY := clamp(bounds.Min.Y, bufH)
	endY := clamp(bounds.Max.Y+1, bufH)
	startX := clamp(bounds.Min.X, bufW)
	endX := clamp(bounds.Max.X+1, bufW)

	for y := startY; y < endY; y++ {
		row := y * bufW * bytesPerPixel

		for col := startX; col < endX; col++ {
			coverage := triangleCoverage(vertices, col, y)
			if coverage == 0 {
				continue
			}

			pixelAlpha := uint32(coverage * float64(colA))
			if pixelAlpha == 0 {
				continue
			}

			blendPixel(pixels, row+col*bytesPerPixel, colR, colG, colB, pixelAlpha)
		}
	}
}

// blendPixel composites one straight-alpha source pixel over the premultiplied
// destination at idx.
func blendPixel(pixels []byte, idx int, colR, colG, colB, pixelAlpha uint32) {
	invA := alphaMax - pixelAlpha
	dstB := uint32(pixels[idx])
	dstG := uint32(pixels[idx+1])
	dstR := uint32(pixels[idx+2])
	dstA := uint32(pixels[idx+3])

	pixels[idx] = byte((colB*pixelAlpha + dstB*invA) / alphaMax)
	pixels[idx+1] = byte((colG*pixelAlpha + dstG*invA) / alphaMax)
	pixels[idx+2] = byte((colR*pixelAlpha + dstR*invA) / alphaMax)
	pixels[idx+3] = byte(pixelAlpha + (dstA*invA)/alphaMax)
}

// sdRoundedBox computes the signed distance from a point (px, py) to a rounded
// rectangle centered at the origin with half-extents (halfW, halfH) and corner
// radius r.  Negative inside, positive outside, zero at the boundary.
func sdRoundedBox(ptX, ptY, halfW, halfH, radius float64) float64 {
	distX := math.Abs(ptX) - halfW + radius
	distY := math.Abs(ptY) - halfH + radius

	insideX := math.Max(distX, 0)
	insideY := math.Max(distY, 0)
	outside := math.Sqrt(insideX*insideX+insideY*insideY) - radius

	inside := math.Min(math.Max(distX, distY), 0)

	return outside + inside
}

// alphaFillRoundedRect composites an anti-aliased rounded rectangle fill
// using signed-distance-function edge smoothing.
//
// Only the corner squares are evaluated through the distance field: the
// middle of every row, and every row between the corner bands, is a plain
// rectangle fill.
func alphaFillRoundedRect(
	pixels []byte,
	bufW, bufH int,
	rect image.Rectangle,
	radius float64,
	color uint32,
) {
	colA := color >> alphaShift
	if colA == 0 {
		return
	}

	corner := int(math.Ceil(radius)) + 1
	if corner*2 >= rect.Dx() || corner*2 >= rect.Dy() {
		// Too small for a flat middle; evaluate everything.
		alphaFillRoundedRegion(pixels, bufW, bufH, rect, rect, radius, color)

		return
	}

	// The band between the corner rows is a rectangle.
	alphaFillRect(pixels, bufW, bufH,
		image.Rect(rect.Min.X, rect.Min.Y+corner, rect.Max.X, rect.Max.Y-corner), color)
	// The top and bottom bands, between the corners, are rectangles too.
	alphaFillRect(pixels, bufW, bufH,
		image.Rect(rect.Min.X+corner, rect.Min.Y, rect.Max.X-corner, rect.Min.Y+corner), color)
	alphaFillRect(pixels, bufW, bufH,
		image.Rect(rect.Min.X+corner, rect.Max.Y-corner, rect.Max.X-corner, rect.Max.Y), color)

	// The four corner squares go through the distance field.
	corners := [4]image.Rectangle{
		image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+corner, rect.Min.Y+corner),
		image.Rect(rect.Max.X-corner, rect.Min.Y, rect.Max.X, rect.Min.Y+corner),
		image.Rect(rect.Min.X, rect.Max.Y-corner, rect.Min.X+corner, rect.Max.Y),
		image.Rect(rect.Max.X-corner, rect.Max.Y-corner, rect.Max.X, rect.Max.Y),
	}
	for _, region := range corners {
		alphaFillRoundedRegion(pixels, bufW, bufH, rect, region, radius, color)
	}
}

// alphaFillRoundedRegion evaluates the rounded rectangle rect through its
// distance field over the pixels of region only.
func alphaFillRoundedRegion(
	pixels []byte,
	bufW, bufH int,
	rect, region image.Rectangle,
	radius float64,
	color uint32,
) {
	colA := color >> alphaShift
	colR := (color >> redShift) & byteMask
	colG := (color >> greenShift) & byteMask
	colB := color & byteMask

	halfW := float64(rect.Dx()) / 2.0 //nolint:mnd // simple arithmetic
	halfH := float64(rect.Dy()) / 2.0 //nolint:mnd // simple arithmetic
	centerX := float64(rect.Min.X) + halfW
	centerY := float64(rect.Min.Y) + halfH

	startY := clamp(region.Min.Y, bufH)
	endY := clamp(region.Max.Y, bufH)
	startX := clamp(region.Min.X, bufW)
	endX := clamp(region.Max.X, bufW)

	for y := startY; y < endY; y++ {
		row := y * bufW * bytesPerPixel
		floatY := float64(y) + pixelHalf

		for col := startX; col < endX; col++ {
			floatX := float64(col) + pixelHalf

			dist := sdRoundedBox(floatX-centerX, floatY-centerY, halfW, halfH, radius)
			if dist > 1 {
				continue
			}

			pixelAlpha := uint32(math.Max(0, math.Min(1, 1.0-dist)) * float64(colA))
			if pixelAlpha == 0 {
				continue
			}

			blendPixel(pixels, row+col*bytesPerPixel, colR, colG, colB, pixelAlpha)
		}
	}
}

// alphaStrokeRoundedRect composites an anti-aliased rounded rectangle stroke
// using signed-distance-function edge smoothing at both outer and inner edges.
//
// Only the ring of the stroke is walked: the interior beyond the line width
// is skipped row by row rather than tested pixel by pixel.
func alphaStrokeRoundedRect(
	pixels []byte,
	bufW, bufH int,
	rect image.Rectangle,
	radius float64,
	color uint32,
	lineWidth int,
) {
	if lineWidth < 1 {
		return
	}

	colA := color >> alphaShift
	if colA == 0 {
		return
	}

	colR := (color >> redShift) & byteMask
	colG := (color >> greenShift) & byteMask
	colB := color & byteMask

	halfW := float64(rect.Dx()) / 2.0 //nolint:mnd // simple arithmetic
	halfH := float64(rect.Dy()) / 2.0 //nolint:mnd // simple arithmetic
	centerX := float64(rect.Min.X) + halfW
	centerY := float64(rect.Min.Y) + halfH

	strokeW := float64(lineWidth)
	innerRadius := math.Max(radius-strokeW, 0)
	innerHalfW := math.Max(halfW-strokeW, 0)
	innerHalfH := math.Max(halfH-strokeW, 0)

	// Rows and columns further than this from the edge are wholly inside the
	// hole, beyond the anti-aliasing fringe of the inner edge.
	band := lineWidth + int(math.Ceil(radius)) + 2 //nolint:mnd // fringe

	startY := clamp(rect.Min.Y, bufH)
	endY := clamp(rect.Max.Y, bufH)
	startX := clamp(rect.Min.X, bufW)
	endX := clamp(rect.Max.X, bufW)

	for y := startY; y < endY; y++ {
		row := y * bufW * bytesPerPixel
		relY := float64(y) + pixelHalf - centerY

		// A row through the middle only has stroke at its two ends.
		middleRow := y-rect.Min.Y >= band && rect.Max.Y-1-y >= band

		for col := startX; col < endX; col++ {
			if middleRow && col-rect.Min.X >= band && rect.Max.X-1-col >= band {
				col = rect.Max.X - band - 1

				continue
			}

			relX := float64(col) + pixelHalf - centerX

			dOuter := sdRoundedBox(relX, relY, halfW, halfH, radius)
			if dOuter > 1 {
				continue
			}

			dInner := sdRoundedBox(relX, relY, innerHalfW, innerHalfH, innerRadius)
			if dInner < -1 {
				continue // inside inner hole, not part of stroke
			}

			outerAlpha := math.Max(0, math.Min(1, 1.0-dOuter))
			innerAlpha := math.Max(0, math.Min(1, 1.0-dInner))

			pixelAlpha := uint32(outerAlpha * (1.0 - innerAlpha) * float64(colA))
			if pixelAlpha == 0 {
				continue
			}

			blendPixel(pixels, row+col*bytesPerPixel, colR, colG, colB, pixelAlpha)
		}
	}
}

// alphaCompositeTextAt composites a text coverage bitmap of size (texW x texH)
// with the given row stride at position (offX, offY) in the main pixel buffer
// using the given ARGB color. Coverage is read from the red channel, which is
// where GDI put the white glyphs.
func alphaCompositeTextAt(
	pixels []byte,
	bufW, bufH int,
	textPixels []byte,
	texW, texH, texStride, offX, offY int,
	color uint32,
) {
	textA := (color >> alphaShift) & byteMask
	if textA == 0 {
		return
	}

	textR := (color >> redShift) & byteMask
	textG := (color >> greenShift) & byteMask
	textB := color & byteMask

	for texY := range texH {
		dstY := offY + texY
		if dstY < 0 || dstY >= bufH {
			continue
		}

		srcRow := texY * texStride
		dstRow := dstY * bufW * bytesPerPixel

		for texX := range texW {
			dstX := offX + texX
			if dstX < 0 || dstX >= bufW {
				continue
			}

			coverage := uint32(textPixels[srcRow+texX*bytesPerPixel+2])
			if coverage == 0 {
				continue
			}

			srcA := coverage * textA / alphaMax
			blendPixel(pixels, dstRow+dstX*bytesPerPixel, textR, textG, textB, srcA)
		}
	}
}
