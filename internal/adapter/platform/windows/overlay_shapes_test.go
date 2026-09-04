//go:build windows

package windows

import (
	"image"
	"testing"
)

// alphaAt returns the alpha channel of the pixel at (x, y) in a buffer of the
// given width, as the rasterizer wrote it.
func alphaAt(pixels []byte, bufW, x, y int) byte {
	return pixels[(y*bufW+x)*bytesPerPixel+3]
}

// TestFillRoundedRect_QueuesTheUnclippedRectangle pins that a rounded fill
// hanging off the surface keeps its own geometry.
//
// The distance field is derived from the rectangle's center and half-extents,
// so queueing the visible part instead would round the corners of that part and
// redraw the shape smaller — while the stroke and the label, which are not
// clipped on the way in, stayed where the real one was. A hint label placed
// below an element near the bottom of the screen is exactly that rectangle.
func TestFillRoundedRect_QueuesTheUnclippedRectangle(t *testing.T) {
	t.Parallel()

	window := &OverlayWindow{width: 100, height: 100}

	hangingOff := image.Rect(10, 80, 60, 120)
	window.FillRoundedRect(hangingOff, 6, 0xFF0000FF)

	if len(window.cmds) != 1 {
		t.Fatalf("queued %d commands, want 1", len(window.cmds))
	}

	if got := window.cmds[0].rect; got != hangingOff {
		t.Errorf("queued rect = %v, want the unclipped %v", got, hangingOff)
	}
}

// TestFillRoundedRect_DropsAFullyOffscreenRectangle pins the other half: the
// surface bounds still decide whether there is anything to draw at all.
func TestFillRoundedRect_DropsAFullyOffscreenRectangle(t *testing.T) {
	t.Parallel()

	window := &OverlayWindow{width: 100, height: 100}
	window.FillRoundedRect(image.Rect(200, 200, 260, 240), 6, 0xFF0000FF)

	if len(window.cmds) != 0 {
		t.Errorf("queued %d commands for an offscreen rect, want none", len(window.cmds))
	}
}

// TestClear_DrainsEveryCommandQueue pins that nothing queued before a Clear
// survives it, and that the clear itself is what the next frame carries: a
// shape queued before the Clear painted onto a surface the Clear emptied would
// be a shape with whatever it belonged to gone.
func TestClear_DrainsEveryCommandQueue(t *testing.T) {
	t.Parallel()

	window := &OverlayWindow{width: 100, height: 100}

	window.FillRect(image.Rect(1, 1, 9, 9), 0xFF0000FF)
	window.FillRoundedRect(image.Rect(1, 1, 9, 9), 2, 0xFF0000FF)
	window.FillTriangle(image.Pt(1, 1), image.Pt(5, 9), image.Pt(9, 1), 0xFF0000FF)
	window.StrokeRect(image.Rect(1, 1, 9, 9), 0xFF0000FF, 1)
	window.DrawTextCentered("a", image.Rect(1, 1, 9, 9), "Segoe UI", 10, 0xFF0000FF)

	window.Clear()

	if count := len(window.cmds); count != 0 {
		t.Errorf("%d commands survived Clear, want the queue drained", count)
	}

	if !window.clearPending {
		t.Error("Clear did not mark the surface to be emptied on the next frame")
	}
}

// TestAlphaFillTriangle_CoversTheInteriorAndNotTheOutside is the shape check
// the hint connector arrow depends on: a solid triangle paints its middle,
// leaves the corners of its bounding box alone, and stays inside the buffer.
func TestAlphaFillTriangle_CoversTheInteriorAndNotTheOutside(t *testing.T) {
	t.Parallel()

	const (
		bufW = 24
		bufH = 24
	)

	pixels := make([]byte, bufW*bufH*bytesPerPixel)

	// A downward-pointing arrow: base along y=4 from x=6 to x=16, tip at (11, 14).
	alphaFillTriangle(pixels, bufW, bufH, [triangleVertices]image.Point{
		image.Pt(6, 4), image.Pt(11, 14), image.Pt(16, 4),
	}, 0xFF0000FF)

	if got := alphaAt(pixels, bufW, 11, 6); got != 0xFF {
		t.Errorf("interior alpha = %#x, want a fully covered pixel", got)
	}

	// The bottom-left corner of the bounding box is well outside the triangle.
	if got := alphaAt(pixels, bufW, 6, 13); got != 0 {
		t.Errorf("outside-the-triangle alpha = %#x, want 0", got)
	}

	// Nothing above the base is touched.
	if got := alphaAt(pixels, bufW, 11, 2); got != 0 {
		t.Errorf("alpha above the base = %#x, want 0", got)
	}
}

// TestAlphaFillTriangle_AntiAliasesItsSlantedEdges pins the reason this is
// sampled rather than filled by scanline: the diagonal edges have to come out
// partially covered, or the arrow reads as a staircase next to the
// anti-aliased badge it hangs off.
func TestAlphaFillTriangle_AntiAliasesItsSlantedEdges(t *testing.T) {
	t.Parallel()

	const (
		bufW = 24
		bufH = 24
	)

	pixels := make([]byte, bufW*bufH*bytesPerPixel)

	alphaFillTriangle(pixels, bufW, bufH, [triangleVertices]image.Point{
		image.Pt(4, 4), image.Pt(12, 20), image.Pt(20, 4),
	}, 0xFF0000FF)

	partial := false

	for y := range bufH {
		for x := range bufW {
			if alpha := alphaAt(pixels, bufW, x, y); alpha > 0 && alpha < 0xFF {
				partial = true
			}
		}
	}

	if !partial {
		t.Error("no partially covered pixel; the slanted edges are not anti-aliased")
	}
}

// TestAlphaFillTriangle_DegenerateAndTransparentDrawNothing pins the two inputs
// that would otherwise paint the whole bounding box: three collinear points,
// where every coverage test reads as inside, and a fully transparent color.
func TestAlphaFillTriangle_DegenerateAndTransparentDrawNothing(t *testing.T) {
	t.Parallel()

	const (
		bufW = 16
		bufH = 16
	)

	tests := []struct {
		name     string
		vertices [triangleVertices]image.Point
		color    uint32
	}{
		{
			name: "collinear points",
			vertices: [triangleVertices]image.Point{
				image.Pt(2, 2), image.Pt(6, 6), image.Pt(10, 10),
			},
			color: 0xFF0000FF,
		},
		{
			name: "transparent color",
			vertices: [triangleVertices]image.Point{
				image.Pt(2, 2), image.Pt(6, 12), image.Pt(10, 2),
			},
			color: 0x000000FF,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			pixels := make([]byte, bufW*bufH*bytesPerPixel)
			alphaFillTriangle(pixels, bufW, bufH, testCase.vertices, testCase.color)

			for _, value := range pixels {
				if value != 0 {
					t.Fatal("the buffer was painted, want it left untouched")
				}
			}
		})
	}
}

// TestAlphaFillTriangle_ClipsToTheBuffer pins that a triangle hanging off the
// edge of the surface is clipped rather than skipped or indexed past the pixel
// slice — a hint against the top or bottom row of the screen puts one there.
// The run itself is half the assertion: an unclipped write panics.
func TestAlphaFillTriangle_ClipsToTheBuffer(t *testing.T) {
	t.Parallel()

	const (
		bufW = 12
		bufH = 12
	)

	tests := []struct {
		name     string
		vertices [triangleVertices]image.Point
		inside   image.Point
	}{
		{
			name: "base above the surface, tip on it",
			vertices: [triangleVertices]image.Point{
				image.Pt(-8, -8), image.Pt(4, 6), image.Pt(20, -8),
			},
			inside: image.Pt(4, 4),
		},
		{
			name: "base below the surface, tip on it",
			vertices: [triangleVertices]image.Point{
				image.Pt(-4, 20), image.Pt(6, 4), image.Pt(24, 20),
			},
			inside: image.Pt(6, 6),
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			pixels := make([]byte, bufW*bufH*bytesPerPixel)
			alphaFillTriangle(pixels, bufW, bufH, testCase.vertices, 0xFF0000FF)

			if got := alphaAt(
				pixels, bufW, testCase.inside.X, testCase.inside.Y,
			); got != 0xFF {
				t.Errorf(
					"alpha at %v = %#x, want the on-surface part of the triangle painted",
					testCase.inside, got,
				)
			}
		})
	}
}

// TestDrawPointerGlyph_CentersTheGlyphOnThePoint pins the primitive the grid
// and recursive-grid pointer stand-ins share: one text command, boxed around
// the selection point, drawn with the configured char and color.
func TestDrawPointerGlyph_CentersTheGlyphOnThePoint(t *testing.T) {
	t.Parallel()

	window := &OverlayWindow{width: 100, height: 100}
	window.DrawPointerGlyph(image.Pt(40, 50), 20, "x", "Segoe UI", 0xFF00FF00)

	if len(window.cmds) != 1 || window.cmds[0].kind != drawText {
		t.Fatalf("queued %d commands, want one text", len(window.cmds))
	}

	got := window.cmds[0]
	if want := image.Rect(30, 40, 50, 60); got.rect != want {
		t.Errorf("glyph rect = %v, want %v", got.rect, want)
	}

	if got.text != "x" || got.color != 0xFF00FF00 || got.fontSize != 20 {
		t.Errorf("queued %q size %v color %#x, want the configured char, size and color",
			got.text, got.fontSize, got.color)
	}
}

// TestDrawPointerGlyph_TinySizeStillDraws pins the floor on the box: a pointer
// configured to size 0 or 1 keeps a rectangle rather than vanishing into an
// empty one, and an empty char falls back to the default disc.
func TestDrawPointerGlyph_TinySizeStillDraws(t *testing.T) {
	t.Parallel()

	for _, size := range []int{0, 1} {
		window := &OverlayWindow{width: 100, height: 100}
		window.DrawPointerGlyph(image.Pt(10, 10), size, "", "", 0xFF0000FF)

		if len(window.cmds) != 1 || window.cmds[0].kind != drawText {
			t.Fatalf("size %d queued %d commands, want one text", size, len(window.cmds))
		}

		got := window.cmds[0]
		if want := image.Rect(9, 9, 11, 11); got.rect != want {
			t.Errorf("size %d glyph rect = %v, want %v", size, got.rect, want)
		}

		if got.text != pointerGlyphDefault {
			t.Errorf("size %d drew %q, want the default disc", size, got.text)
		}
	}
}
