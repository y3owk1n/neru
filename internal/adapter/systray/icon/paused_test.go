package icon_test

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/systray/icon"
)

// decodeTile decodes an embedded or derived tray asset.
func decodeTile(t *testing.T, name string, data []byte) image.Image {
	t.Helper()

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("%s is not a decodable PNG: %v", name, err)
	}

	return img
}

// TestBrandPaused pins what the paused tray tile owes a user: it is a real PNG
// of the running tile's size, it is visibly not the running tile, and it is
// still there — a transform that erased the artwork would leave the tray
// showing nothing at all, which is the invisible-state bug in a new costume.
func TestBrandPaused(t *testing.T) {
	paused := icon.BrandPaused()

	if bytes.Equal(paused, icon.Brand) {
		t.Fatal(
			"paused tile is byte-identical to the running tile; the paused state stays invisible",
		)
	}

	running := decodeTile(t, "Brand", icon.Brand)
	pausedImg := decodeTile(t, "BrandPaused", paused)

	if pausedImg.Bounds() != running.Bounds() {
		t.Errorf(
			"paused tile bounds = %v, want the running tile's %v",
			pausedImg.Bounds(),
			running.Bounds(),
		)
	}

	var opaque int

	bounds := pausedImg.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := pausedImg.At(x, y).RGBA()
			if a > 0 {
				opaque++
			}
		}
	}

	if opaque*4 < bounds.Dx()*bounds.Dy() {
		t.Errorf(
			"paused tile has %d visible pixels of %d; the artwork was erased rather than dimmed",
			opaque,
			bounds.Dx()*bounds.Dy(),
		)
	}
}

// TestBrandPaused_IsStable pins that the derivation is computed once and the
// same bytes come back on every call: the tray sets an icon on every state
// change, and a fresh encode per call would put PNG work on that path.
func TestBrandPaused_IsStable(t *testing.T) {
	first := icon.BrandPaused()

	second := icon.BrandPaused()

	if &first[0] != &second[0] {
		t.Error("BrandPaused re-derived the tile; callers expect a cached asset")
	}
}
