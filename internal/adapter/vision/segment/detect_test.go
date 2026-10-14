package segment_test

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"math/rand/v2"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/vision/segment"
	"github.com/y3owk1n/neru/internal/derrors"
)

var (
	white = color.RGBA{255, 255, 255, 255}
	black = color.RGBA{30, 30, 30, 255}
	blue  = color.RGBA{10, 70, 180, 255}
	gray  = color.RGBA{134, 134, 134, 255}
	dark  = color.RGBA{26, 29, 33, 255}
	snow  = color.RGBA{240, 240, 240, 255}
)

func canvas(w, h int, bg color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	return img
}

func fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(img, r, &image.Uniform{c}, image.Point{}, draw.Src)
}

func border(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	fill(img, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), c)
	fill(img, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), c)
	fill(img, image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), c)
	fill(img, image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y), c)
}

// word draws n glyph strokes 2px wide and 9px tall with 2px between them, a
// stand-in for one word of 13px text, and returns its box.
func word(img *image.RGBA, x, y, n int, c color.RGBA) image.Rectangle {
	for i := range n {
		fill(img, image.Rect(x+i*4, y, x+i*4+2, y+9), c)
	}

	return image.Rect(x, y, x+n*4-2, y+9)
}

// sentence draws words separated by a 4px space and returns each word's box.
func sentence(img *image.RGBA, left, top int, inks ...color.RGBA) []image.Rectangle {
	boxes := make([]image.Rectangle, 0, len(inks))

	for _, ink := range inks {
		box := word(img, left, top, 5, ink)
		boxes = append(boxes, box)
		left = box.Max.X + 4
	}

	return boxes
}

func detect(t *testing.T, img *image.RGBA, scale float64) []image.Rectangle {
	t.Helper()

	rects, err := segment.Detect(context.Background(), img, scale)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	return rects
}

// hitsOnce reports whether exactly one rectangle's center lies inside want.
func hitsOnce(rects []image.Rectangle, want image.Rectangle) bool {
	hits := 0

	for _, r := range rects {
		c := image.Pt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2)
		if c.In(want) {
			hits++
		}
	}

	return hits == 1
}

func covering(rects []image.Rectangle, pt image.Point) []image.Rectangle {
	var out []image.Rectangle

	for _, r := range rects {
		if pt.In(r) {
			out = append(out, r)
		}
	}

	return out
}

func TestDetect_RefusesEmptyFrame(t *testing.T) {
	t.Parallel()

	for name, img := range map[string]*image.RGBA{
		"nil":   nil,
		"empty": image.NewRGBA(image.Rect(0, 0, 0, 0)),
	} {
		rects, err := segment.Detect(context.Background(), img, 1)
		if err == nil || rects != nil {
			t.Errorf("%s: Detect() = %v, %v; want nil and an error", name, rects, err)
		}
	}
}

func TestDetect_BlankFrameFindsNothing(t *testing.T) {
	t.Parallel()

	if rects := detect(t, canvas(200, 100, white), 1); len(rects) != 0 {
		t.Errorf("Detect(blank) = %v, want none", rects)
	}
}

func TestDetect_CanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := segment.Detect(ctx, canvas(200, 100, white), 1)
	if code := derrors.GetCode(err); code != derrors.CodeContextCanceled {
		t.Errorf("Detect(canceled) code = %v, want %v", code, derrors.CodeContextCanceled)
	}
}

func TestDetect_HintsEachWordOnce(t *testing.T) {
	t.Parallel()

	img := canvas(400, 60, white)
	words := sentence(img, 20, 20, black, black, blue, blue, black)
	rects := detect(t, img, 1)

	for _, w := range words {
		if got := covering(rects, image.Pt(w.Min.X+1, 24)); len(got) != 1 || got[0] != w {
			t.Errorf("word %v = %v, want one target over exactly that word", w, got)
		}
	}
}

func TestDetect_SplitsTightLinkByColor(t *testing.T) {
	t.Parallel()

	img := canvas(300, 60, white)
	text := word(img, 20, 20, 5, black)
	link := word(img, text.Max.X+2, 20, 5, blue)
	rects := detect(t, img, 1)

	for name, w := range map[string]image.Rectangle{"text": text, "link": link} {
		if !hitsOnce(rects, w) {
			t.Errorf(
				"%s %v should be its own target despite the tight spacing, got %v",
				name,
				w,
				rects,
			)
		}
	}
}

func TestDetect_SeparatorSplitsLinks(t *testing.T) {
	t.Parallel()

	img := canvas(300, 60, white)
	left := word(img, 20, 20, 4, gray)
	fill(img, image.Rect(left.Max.X+3, 19, left.Max.X+4, 31), gray)
	right := word(img, left.Max.X+7, 20, 4, gray)
	rects := detect(t, img, 1)

	if !hitsOnce(rects, left) || !hitsOnce(rects, right) || len(rects) != 2 {
		t.Errorf(
			"Detect(left | right) = %v, want one target each for %v and %v, none for the bar",
			rects,
			left,
			right,
		)
	}
}

func TestDetect_KeepsLoneNarrowGlyph(t *testing.T) {
	t.Parallel()

	img := canvas(100, 40, white)
	stroke := image.Rect(40, 15, 42, 24)
	fill(img, stroke, blue)

	if rects := detect(t, img, 1); !hitsOnce(rects, stroke) {
		t.Errorf("Detect(lone I) = %v, want one target over %v: a one-letter link is not a separator", rects, stroke)
	}
}

func TestDetect_RoundKeyIsOneTarget(t *testing.T) {
	t.Parallel()

	img := canvas(120, 120, dark)
	key := image.Rect(30, 30, 90, 90)

	for y := key.Min.Y; y < key.Max.Y; y++ {
		for x := key.Min.X; x < key.Max.X; x++ {
			dx, dy := x-60, y-60
			if dx*dx+dy*dy <= 29*29 {
				img.SetRGBA(x, y, gray)
			}
		}
	}

	word(img, 52, 54, 3, snow)
	rects := detect(t, img, 1)

	if len(rects) != 1 || !hitsOnce(rects, key) {
		t.Errorf("Detect(round key) = %v, want one target over the key %v", rects, key)
	}
}

func TestDetect_FindsButtonOnPhoto(t *testing.T) {
	t.Parallel()

	img := canvas(800, 500, white)
	photo := image.Rect(20, 20, 780, 480)
	rng := rand.New(rand.NewPCG(5, 6))

	for y := photo.Min.Y; y < photo.Max.Y; y++ {
		for x := photo.Min.X; x < photo.Max.X; x++ {
			v := uint8(90 + rng.IntN(40))
			img.SetRGBA(x, y, color.RGBA{v, v + 10, v - 20, 255})
		}
	}

	button := image.Rect(100, 300, 260, 340)
	fill(img, button, white)
	word(img, 140, 315, 6, black)

	rects := detect(t, img, 1)

	if !hitsOnce(rects, button) {
		t.Errorf("Detect() = %v, want the button %v drawn on the photo", rects, button)
	}

	for _, r := range rects {
		if r.Dx() > 400 {
			t.Errorf("the photo itself came back as a target: %v", r)
		}
	}
}

func TestDetect_KeepsWhiteAndGrayTextApart(t *testing.T) {
	t.Parallel()

	img := canvas(300, 60, dark)
	words := sentence(img, 20, 20, snow, gray)
	rects := detect(t, img, 1)

	for _, w := range words {
		if !hitsOnce(rects, w) {
			t.Errorf("word %v should be its own label, got %v", w, rects)
		}
	}
}

func TestDetect_ButtonIsOneTarget(t *testing.T) {
	t.Parallel()

	cases := map[string]color.RGBA{
		"dark border":  black,
		"faint border": {234, 238, 242, 255},
		"chip fill":    {235, 236, 240, 255},
	}

	for name, edge := range cases {
		img := canvas(200, 80, white)
		button := image.Rect(30, 20, 130, 50)

		if name == "chip fill" {
			fill(img, button, edge)
		} else {
			border(img, button, edge)
		}

		word(img, 50, 31, 6, black)
		rects := detect(t, img, 1)

		if len(rects) != 1 || rects[0] != button {
			t.Errorf("%s: Detect() = %v, want only the button %v", name, rects, button)
		}
	}
}

func TestDetect_CardIsAContainer(t *testing.T) {
	t.Parallel()

	img := canvas(400, 260, white)
	card := image.Rect(20, 20, 380, 220)
	button := image.Rect(40, 160, 140, 190)

	border(img, card, black)
	border(img, button, black)
	word(img, 60, 171, 6, black)

	rects := detect(t, img, 1)

	if len(rects) != 1 || rects[0] != button {
		t.Errorf("Detect() = %v, want only the button %v and not the card", rects, button)
	}
}

func TestDetect_DropsSeparators(t *testing.T) {
	t.Parallel()

	img := canvas(400, 60, white)
	fill(img, image.Rect(10, 30, 390, 31), color.RGBA{220, 220, 220, 255})
	fill(img, image.Rect(200, 5, 201, 55), color.RGBA{220, 220, 220, 255})

	if rects := detect(t, img, 1); len(rects) != 0 {
		t.Errorf("Detect(separators) = %v, want none", rects)
	}
}

func TestDetect_KeepsTinyMenu(t *testing.T) {
	t.Parallel()

	img := canvas(100, 40, white)
	for i := range 3 {
		fill(img, image.Rect(40+i*4, 20, 42+i*4, 22), gray)
	}

	rects := detect(t, img, 1)
	if !hitsOnce(rects, image.Rect(39, 19, 51, 23)) {
		t.Errorf("Detect(⋯) = %v, want one target over the three dots", rects)
	}
}

func TestDetect_PictureBySize(t *testing.T) {
	t.Parallel()

	noise := func(img *image.RGBA, r image.Rectangle) {
		rng := rand.New(rand.NewPCG(1, 2))
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				img.SetRGBA(
					x,
					y,
					color.RGBA{
						uint8(rng.IntN(256)),
						uint8(rng.IntN(256)),
						uint8(rng.IntN(256)),
						255,
					},
				)
			}
		}
	}

	img := canvas(600, 400, white)
	avatar := image.Rect(20, 20, 56, 56)
	photo := image.Rect(100, 20, 500, 380)

	noise(img, avatar)
	noise(img, photo)

	rects := detect(t, img, 1)

	if len(rects) != 1 || rects[0] != avatar {
		t.Errorf(
			"Detect() = %v, want only the avatar %v: a photo is content, not a target",
			rects,
			avatar,
		)
	}
}

func TestDetect_ScaledFrameReportsLogicalPixels(t *testing.T) {
	t.Parallel()

	img := canvas(400, 160, white)
	border(img, image.Rect(60, 40, 260, 100), black)
	border(img, image.Rect(61, 41, 259, 99), black)

	rects := detect(t, img, 2)
	want := image.Rect(30, 20, 130, 50)

	if len(rects) != 1 || rects[0] != want {
		t.Errorf("Detect(scale 2) = %v, want %v in logical pixels", rects, want)
	}
}

// BenchmarkDetect_FullScreenFrame runs a 1920x1080 frame of bordered buttons
// and lines of two-color text, a stand-in for a busy desktop.
func BenchmarkDetect_FullScreenFrame(b *testing.B) {
	img := canvas(1920, 1080, white)

	for y := 10; y+30 < 1080; y += 36 {
		border(img, image.Rect(10, y, 130, y+28), black)
		word(img, 30, y+10, 8, black)
		sentence(img, 160, y+10, black, black, blue, black, black, black, gray, black, black, black,
			black, blue, blue, black, black, black, black, black, gray, gray, black, black)
	}

	for b.Loop() {
		_, _ = segment.Detect(context.Background(), img, 1)
	}
}
