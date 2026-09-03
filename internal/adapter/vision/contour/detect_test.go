package contour_test

import (
	"image"
	"image/color"
	"image/draw"
	"math/rand/v2"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/vision/contour"
)

func TestDetect_NilOrEmpty(t *testing.T) {
	t.Parallel()

	res, err := contour.Detect(nil, 1.0)
	if err == nil || res != nil {
		t.Errorf("Detect(nil) = %v, %v; want nil and an error", res, err)
	}

	empty := image.NewRGBA(image.Rect(0, 0, 0, 0))

	res, err = contour.Detect(empty, 1.0)
	if err == nil || res != nil {
		t.Errorf("Detect(empty) = %v, %v; want nil and an error", res, err)
	}
}

func TestDetect_ButtonDetection(t *testing.T) {
	t.Parallel()

	w, h := 300, 200
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	// Fill white background
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	// Draw button with black border: rect at (30, 40) with size 80x30
	btnRect := image.Rect(30, 40, 110, 70)
	black := color.Black

	for x := btnRect.Min.X; x < btnRect.Max.X; x++ {
		img.Set(x, btnRect.Min.Y, black)
		img.Set(x, btnRect.Max.Y-1, black)
	}

	for y := btnRect.Min.Y; y < btnRect.Max.Y; y++ {
		img.Set(btnRect.Min.X, y, black)
		img.Set(btnRect.Max.X-1, y, black)
	}

	targets, err := contour.Detect(img, 1.0)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if len(targets) == 0 {
		t.Fatalf("expected at least 1 detected button target, got 0")
	}

	// Verify that the detected target covers the button area
	found := false
	for _, target := range targets {
		// Target should be close to (30, 40, 110, 70) with dilation padding
		if target.Min.X >= 25 && target.Max.X <= 115 && target.Min.Y >= 35 && target.Max.Y <= 75 {
			found = true
			break
		}
	}

	if !found {
		t.Errorf(
			"did not find target matching button rect %v in detected targets: %v",
			btnRect,
			targets,
		)
	}
}

func TestDetect_FiltersNoiseAndOversized(t *testing.T) {
	t.Parallel()

	w, h := 400, 300
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	black := color.Black

	// 1. Tiny noise: 2x2 dot at (50, 50)
	for y := 50; y < 52; y++ {
		for x := 50; x < 52; x++ {
			img.Set(x, y, black)
		}
	}

	// 2. Oversized panel: 300x180 box at (50, 80) (height >= 160 should be filtered)
	for x := 50; x < 350; x++ {
		img.Set(x, 80, black)
		img.Set(x, 259, black)
	}

	for y := 80; y < 260; y++ {
		img.Set(50, y, black)
		img.Set(349, y, black)
	}

	targets, err := contour.Detect(img, 1.0)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	for _, target := range targets {
		if target.Min.X >= 48 && target.Max.X <= 54 && target.Min.Y >= 48 && target.Max.Y <= 54 {
			t.Errorf("tiny noise dot should have been filtered out, got %v", target)
		}

		if target.Min.X >= 45 && target.Max.X <= 355 && target.Dy() >= 160 {
			t.Errorf(
				"oversized panel (height >= 160) should have been filtered out, got %v",
				target,
			)
		}
	}
}

func TestDetect_NotificationCardWithoutButtons(t *testing.T) {
	t.Parallel()

	w, h := 500, 300
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	black := color.Black

	// Draw an isolated notification card (350x75) at (50, 50) with no child buttons
	card := image.Rect(50, 50, 400, 125)
	for x := card.Min.X; x < card.Max.X; x++ {
		img.Set(x, card.Min.Y, black)
		img.Set(x, card.Max.Y-1, black)
	}

	for y := card.Min.Y; y < card.Max.Y; y++ {
		img.Set(card.Min.X, y, black)
		img.Set(card.Max.X-1, y, black)
	}

	targets, err := contour.Detect(img, 1.0)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if len(targets) == 0 {
		t.Fatalf("expected notification card to be detected, got 0 targets")
	}

	foundCard := false
	for _, target := range targets {
		if target.Min.X >= 45 && target.Max.X <= 405 && target.Min.Y >= 45 && target.Max.Y <= 130 {
			foundCard = true
		}
	}

	if !foundCard {
		t.Errorf(
			"did not find target matching notification card %v in detected targets: %v",
			card,
			targets,
		)
	}
}

func TestDetect_ButtonInsideEnclosingContainer(t *testing.T) {
	t.Parallel()

	w, h := 500, 400
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	black := color.Black

	// Draw an enclosing dialog/popup box (250x120) at (50, 50) -> height 120 >= 50
	dialog := image.Rect(50, 50, 300, 170)
	for x := dialog.Min.X; x < dialog.Max.X; x++ {
		img.Set(x, dialog.Min.Y, black)
		img.Set(x, dialog.Max.Y-1, black)
	}

	for y := dialog.Min.Y; y < dialog.Max.Y; y++ {
		img.Set(dialog.Min.X, y, black)
		img.Set(dialog.Max.X-1, y, black)
	}

	// Draw an "OK" button (70x28) inside the dialog at (150, 110)
	btn := image.Rect(150, 110, 220, 138)
	for x := btn.Min.X; x < btn.Max.X; x++ {
		img.Set(x, btn.Min.Y, black)
		img.Set(x, btn.Max.Y-1, black)
	}

	for y := btn.Min.Y; y < btn.Max.Y; y++ {
		img.Set(btn.Min.X, y, black)
		img.Set(btn.Max.X-1, y, black)
	}

	targets, err := contour.Detect(img, 1.0)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if len(targets) == 0 {
		t.Fatalf("expected button inside enclosing dialog to be detected, got 0 targets")
	}

	foundButton := false
	for _, target := range targets {
		// Button should be close to (150, 110, 220, 138) with dilation padding
		if target.Min.X >= 145 && target.Max.X <= 225 && target.Min.Y >= 105 &&
			target.Max.Y <= 143 {
			foundButton = true
		}
		// The outer dialog box itself (height 120 >= 50) should not be returned as a clickable target
		if target.Dy() >= 50 {
			t.Errorf("enclosing dialog box should not be returned as target, got %v", target)
		}
	}

	if !foundButton {
		t.Errorf(
			"did not find target matching button rect %v in detected targets: %v",
			btn,
			targets,
		)
	}
}

func BenchmarkDetect_FullScreenFrame(b *testing.B) {
	img := syntheticFrame(rand.New(rand.NewPCG(3, 4)), 2560, 1440)

	for b.Loop() {
		_, _ = contour.Detect(img, 2)
	}
}

// syntheticFrame draws a light background with random dark-bordered boxes,
// some nested, plus speckle noise: a stand-in for a busy desktop.
func syntheticFrame(rng *rand.Rand, width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(img.Pix); i += 4 {
		v := uint8(230 + rng.IntN(20))
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = v, v, v, 255
	}

	box := func(rect image.Rectangle) {
		rect = rect.Intersect(img.Bounds())
		for x := rect.Min.X; x < rect.Max.X; x++ {
			img.Set(x, rect.Min.Y, image.Black)
			img.Set(x, rect.Max.Y-1, image.Black)
		}

		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			img.Set(rect.Min.X, y, image.Black)
			img.Set(rect.Max.X-1, y, image.Black)
		}
	}

	for range 5 + rng.IntN(20) {
		boxW := 4 + rng.IntN(300)
		boxH := 2 + rng.IntN(120)
		x := rng.IntN(max(width-boxW, 1))
		y := rng.IntN(max(height-boxH, 1))
		rect := image.Rect(x, y, x+boxW, y+boxH)
		box(rect)

		if rng.IntN(2) == 0 && boxW > 30 && boxH > 20 {
			box(rect.Inset(rng.IntN(4) + 2))
		}
	}

	for range width * height / 200 {
		img.Set(rng.IntN(width), rng.IntN(height), image.Black)
	}

	return img
}
