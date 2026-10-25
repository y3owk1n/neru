// Package frame holds the pixel plumbing the pure-Go detectors and the capture
// backends share: splitting a frame's rows across cores and box-filtering a
// high-density frame down to logical pixels.
//
// A frame is screen content. Nothing here logs, copies past the call, or
// retains the pixels.
package frame

import (
	"image"
	"runtime"
	"sync"
)

const bytesPerPixel = 4

// Rows runs work over [0, height) split into one band of rows per core, and
// returns once every band is done.
func Rows(height int, work func(y0, y1 int)) {
	workers := max(min(runtime.GOMAXPROCS(0), height), 1)
	band := (height + workers - 1) / workers

	var bands sync.WaitGroup

	for y0 := 0; y0 < height; y0 += band {
		bands.Go(func() { work(y0, min(y0+band, height)) })
	}

	bands.Wait()
}

// Order names where red sits in a packed four-byte pixel. Green is always
// second and the fourth byte is ignored.
type Order int

// The two packed layouts screen captures arrive in.
const (
	RGBA Order = iota
	BGRA
)

// Downsample averages each factor x factor block of img into one opaque
// pixel. Blocks that run past the frame's edge average what they hold. A
// factor below two returns img itself.
func Downsample(img *image.RGBA, factor int) *image.RGBA {
	if factor < 2 { //nolint:mnd // one is no downsampling at all
		return img
	}

	return Resample(img.Pix, img.Stride, img.Rect.Dx(), img.Rect.Dy(), RGBA, factor)
}

// Resample reads width x height packed pixels in order from pix, rows stride
// bytes apart, and returns them as an opaque RGBA frame with each factor x
// factor block averaged into one pixel, in a single pass across cores. A
// factor of one is a plain copy. pix must hold every row it is told to.
//
//nolint:varnamelen // pixel kernel: coordinates read as the algorithm writes them
func Resample(pix []byte, stride, width, height int, order Order, factor int) *image.RGBA {
	factor = max(factor, 1)
	redAt, blueAt := 0, 2

	if order == BGRA {
		redAt, blueAt = 2, 0
	}

	dstW, dstH := (width+factor-1)/factor, (height+factor-1)/factor
	out := image.NewRGBA(image.Rect(0, 0, dstW, dstH))

	Rows(dstH, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			sy0, sy1 := y*factor, min((y+1)*factor, height)
			row := out.Pix[y*out.Stride:]

			for x := range dstW {
				sx0, sx1 := x*factor, min((x+1)*factor, width)

				var r, g, b, n int

				for sy := sy0; sy < sy1; sy++ {
					src := pix[sy*stride:]
					for sx := sx0; sx < sx1; sx++ {
						i := sx * bytesPerPixel
						r += int(src[i+redAt])
						g += int(src[i+1])
						b += int(src[i+blueAt])
						n++
					}
				}

				i := x * bytesPerPixel
				row[i] = uint8((r + n/2) / n)
				row[i+1] = uint8((g + n/2) / n)
				row[i+2] = uint8((b + n/2) / n)
				row[i+3] = 0xff
			}
		}
	})

	return out
}
