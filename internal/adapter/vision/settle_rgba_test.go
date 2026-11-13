//go:build linux || windows

package vision

import (
	"image"
	"image/color"
	"testing"
)

// TestBandOf_MatchesACaptureOfTheBandAlone pins the comparison the watch
// makes. The band of the last read's whole capture has to equal a capture of
// that band by itself when nothing moved, at 1x and on a HiDPI display, or a
// region that never changed would never be seen as unchanged.
func TestBandOf_MatchesACaptureOfTheBandAlone(t *testing.T) {
	region := image.Rect(100, 50, 500, 450)
	band := probeBand(region)

	for _, scale := range []int{1, 2} {
		screen := func(r image.Rectangle) *image.RGBA {
			img := image.NewRGBA(image.Rect(0, 0, r.Dx()*scale, r.Dy()*scale))
			for y := range img.Rect.Dy() {
				for x := range img.Rect.Dx() {
					gx, gy := r.Min.X*scale+x, r.Min.Y*scale+y
					img.SetRGBA(
						x,
						y,
						color.RGBA{R: uint8(gx), G: uint8(gy), B: uint8(gx ^ gy), A: 0xFF},
					)
				}
			}

			return img
		}

		if !sameFrame(bandOf(screen(region), region, band), screen(band)) {
			t.Fatalf(
				"at %dx the band of the whole capture differs from a capture of the band",
				scale,
			)
		}
	}

	if band.Dy() >= region.Dy() || !band.In(region) {
		t.Fatalf("band %v is not a strip inside region %v", band, region)
	}
}

// TestMaskOut_LeavesTheOverlayOutOfTheBandComparison pins how the probes,
// which leave the overlay up, compare with the last read's capture, taken with
// it off. With what the overlay drew masked out the two match, at 1x and on a
// HiDPI display, and a change of the page beside it still shows.
func TestMaskOut_LeavesTheOverlayOutOfTheBandComparison(t *testing.T) {
	band := image.Rect(100, 200, 500, 250)
	cursor := image.Rect(240, 210, 260, 240)

	for _, scale := range []int{1, 2} {
		capture := func(overlay bool, changedAt image.Point) *image.RGBA {
			img := image.NewRGBA(image.Rect(0, 0, band.Dx()*scale, band.Dy()*scale))
			for row := range img.Rect.Dy() {
				for col := range img.Rect.Dx() {
					global := image.Pt(band.Min.X+col/scale, band.Min.Y+row/scale)
					pixel := color.RGBA{R: uint8(global.X), G: uint8(global.Y), A: 0xFF}

					if overlay && global.In(cursor) {
						pixel = color.RGBA{R: 0xFF, B: 0xFF, A: 0xFF}
					}

					if global == changedAt {
						pixel.B = 0x80
					}

					img.SetRGBA(col, row, pixel)
				}
			}

			return img
		}

		before, probe := capture(false, image.Point{}), capture(true, image.Point{})
		maskOut(before, band, []image.Rectangle{cursor})
		maskOut(probe, band, []image.Rectangle{cursor})

		if !sameFrame(before, probe) {
			t.Fatalf(
				"at %dx the probe with the cursor masked differs from the capture without it",
				scale,
			)
		}

		moved := capture(true, image.Pt(300, 220))
		maskOut(moved, band, []image.Rectangle{cursor})

		if sameFrame(before, moved) {
			t.Fatalf("at %dx a change of the page beside the masked cursor went unseen", scale)
		}
	}
}
