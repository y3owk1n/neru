package segment

import (
	"image"

	pixels "github.com/y3owk1n/neru/internal/adapter/vision/frame"
)

// Texture. A frosted panel over a photo, a dithered fill or a grainy
// background steps by a few units on every pixel, enough to clear inkStep
// and fill a region with stray blobs that each become a target. Each tile's
// floor (the step a quarter of its pixels stay under) says how textured it
// is: flat UI, even dense text, leaves a quarter of a tile flat and has a
// floor of zero, while texture has no flat quarter. A textured tile only
// counts steps well above its floor as ink, so its noise drops out while text
// and controls drawn on it, which step by far more, stay.
const (
	// noiseTile is the side of a texture tile, in frame pixels.
	noiseTile = 16
	// noiseQuantile: the floor is the step under which 1/noiseQuantile of a
	// tile's pixels fall.
	noiseQuantile = 4
	// noiseFactor is how far above its floor a step must be to count as ink.
	noiseFactor = 3
	// noiseBins caps the floor's histogram. A floor past it is texture either
	// way, and the threshold stops at overlayStep below it.
	noiseBins = 64
)

// stepValues returns, for each pixel, the largest channel difference to its
// right or lower neighbor, the one number every mask here is drawn from.
// Comparing channels rather than luma keeps the edge between two colors of
// equal brightness, which a grayscale pass erases.
//
//nolint:varnamelen // pixel kernel: coordinates read as the algorithm writes them
func stepValues(img *image.RGBA, width, height int) []uint8 {
	steps := make([]uint8, width*height)

	pixels.Rows(height, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			row := img.Pix[y*img.Stride : y*img.Stride+width*4]

			var next []uint8
			if y+1 < height {
				next = img.Pix[(y+1)*img.Stride : (y+1)*img.Stride+width*4]
			}

			out := steps[y*width : (y+1)*width]

			for x := range width {
				i := x * 4 //nolint:mnd // four bytes per RGBA pixel
				r, g, b := int(row[i]), int(row[i+1]), int(row[i+2])
				step := 0

				if x+1 < width {
					step = max(abs(r-int(row[i+4])), abs(g-int(row[i+5])), abs(b-int(row[i+6])))
				}

				if next != nil {
					step = max(
						step,
						abs(r-int(next[i])),
						abs(g-int(next[i+1])),
						abs(b-int(next[i+2])),
					)
				}

				out[x] = uint8(step)
			}
		}
	})

	return steps
}

// stepMask marks every pixel stepping by at least inkStep.
func stepMask(steps []uint8) []bool {
	mask := make([]bool, len(steps))

	for i, v := range steps {
		mask[i] = v >= inkStep
	}

	return mask
}

// noiseMask marks a pixel when it steps by at least its tile's threshold, the
// tile's floor times noiseFactor, never under inkStep and never over
// overlayStep. It reports whether any tile's threshold rose above inkStep. A
// frame with none gets exactly stepMask's mask, and the caller keeps that.
//
//nolint:varnamelen // pixel kernel: coordinates read as the algorithm writes them
func noiseMask(steps []uint8, width, height int) ([]bool, bool) {
	mask := make([]bool, width*height)
	cols, rows := (width+noiseTile-1)/noiseTile, (height+noiseTile-1)/noiseTile
	raisedRows := make([]bool, rows)

	pixels.Rows(rows, func(t0, t1 int) {
		var histogram [noiseBins]int

		for ty := t0; ty < t1; ty++ {
			y0, y1 := ty*noiseTile, min((ty+1)*noiseTile, height)

			for tx := range cols {
				x0, x1 := tx*noiseTile, min((tx+1)*noiseTile, width)
				histogram = [noiseBins]int{}

				for y := y0; y < y1; y++ {
					for _, v := range steps[y*width+x0 : y*width+x1] {
						histogram[min(int(v), noiseBins-1)]++
					}
				}

				quarter, seen, floor := (y1-y0)*(x1-x0)/noiseQuantile, 0, 0
				for v, n := range histogram {
					seen += n
					if seen > quarter {
						floor = v

						break
					}
				}

				threshold := uint8(min(max(inkStep, noiseFactor*floor), overlayStep))
				if threshold > inkStep {
					raisedRows[ty] = true
				}

				for y := y0; y < y1; y++ {
					in, out := steps[y*width+x0:y*width+x1], mask[y*width+x0:y*width+x1]
					for k, v := range in {
						out[k] = v >= threshold
					}
				}
			}
		}
	})

	for _, raised := range raisedRows {
		if raised {
			return mask, true
		}
	}

	return mask, false
}

// densePictures returns the boxes, in mask coordinates, of blobs dense enough
// to be pictures and small enough to be one target: an avatar, a thumbnail.
//
//nolint:varnamelen // blob sides read as the geometry writes them
func densePictures(blobs []blob, lim limits) []image.Rectangle {
	var regions []image.Rectangle

	for i := range blobs {
		b := &blobs[i]
		w, h := b.width(), b.height()

		if w >= lim.minPictureSide && h >= lim.minPictureSide &&
			w <= lim.maxPictureSide && h <= lim.maxPictureSide &&
			float64(b.count) >= pictureFill*float64(w*h) {
			regions = append(regions, image.Rect(b.minX, b.minY, b.maxX+1, b.maxY+1))
		}
	}

	return regions
}

// keepWithin copies src into dst over each region, so a picture texture
// would otherwise break up stays whole.
func keepWithin(dst, src []bool, width int, regions []image.Rectangle) {
	for _, region := range regions {
		for y := region.Min.Y; y < region.Max.Y; y++ {
			copy(
				dst[y*width+region.Min.X:y*width+region.Max.X],
				src[y*width+region.Min.X:y*width+region.Max.X],
			)
		}
	}
}
