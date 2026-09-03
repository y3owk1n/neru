package icon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"sync"
)

// Coefficients of the Rec. 601 luma formula, the perceptual weighting a
// desaturated tile needs so the glyph keeps the contrast it had against the
// tile behind it. A flat channel average would sink the blue tile and the
// white glyph towards each other.
const (
	lumaRed   = 0.299
	lumaGreen = 0.587
	lumaBlue  = 0.114

	// neutralGray is what the desaturated tile is mixed towards, and
	// pausedContrast is how much of its own contrast survives the mix. Flat is
	// the tray's oldest idiom for "this is not running": three fifths leaves
	// the glyph readable against the tile while putting both far enough from
	// the running colors to tell apart at 22 pixels.
	//
	// Contrast rather than opacity, deliberately. A translucent tile would
	// depend on the host compositing it correctly, and the Win32 notification
	// area is handed these bytes with straight alpha where GDI's icon path
	// wants it premultiplied — a latent difference that today only touches the
	// tile's anti-aliased corners, and that fading the whole tile would put on
	// screen. This transform leaves every alpha value exactly as the running
	// tile has it.
	neutralGray    = 128.0
	pausedContrast = 0.6
)

// BrandPaused returns the Brand tile as it appears while Neru is paused:
// desaturated and flattened, the same artwork at the same size.
//
// It is derived from Brand rather than shipped as a second asset so the two
// can never drift — the paused tile is the running tile by construction, and a
// new brand tile needs no second export. macOS answers this question with a
// pair of hand-drawn template glyphs instead (see trayicon_darwin.go); those
// are white-on-transparent and would be invisible in a tray host that renders
// icon bytes literally, which is the whole reason this derivation exists.
//
// The derivation runs at most once, on the first paused state of the process:
// the tray sets an icon on every toggle, and a decode-transform-encode per
// toggle would put PNG work on a path the user is waiting on. Its input is an
// embedded asset and TestBrandPaused runs the transform over it, so the
// fallback below is unreachable short of a corrupt binary — and it hands back
// the running tile rather than nothing, because a tray showing the wrong state
// is still better than a tray with no icon in it.
func BrandPaused() []byte {
	return brandPaused()
}

// brandPaused derives the paused tile once and remembers it.
var brandPaused = sync.OnceValue(func() []byte {
	flattened, err := flattenToPaused(Brand)
	if err != nil {
		return Brand
	}

	return flattened
})

// flattenToPaused decodes a PNG tile, desaturates and flattens it, and
// re-encodes it.
func flattenToPaused(tile []byte) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(tile))
	if err != nil {
		return nil, err
	}

	bounds := src.Bounds()
	dst := image.NewNRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.Set(x, y, pausedPixel(src.At(x, y)))
		}
	}

	var out bytes.Buffer

	err = png.Encode(&out, dst)
	if err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// pausedPixel desaturates one pixel and flattens its contrast, leaving its
// alpha untouched.
func pausedPixel(c color.Color) color.NRGBA {
	nrgba := color.NRGBAModel.Convert(c).(color.NRGBA) //nolint:forcetypeassert // NRGBAModel converts to color.NRGBA by definition.

	luma := lumaRed*float64(nrgba.R) + lumaGreen*float64(nrgba.G) + lumaBlue*float64(nrgba.B)
	gray := uint8(luma*pausedContrast + neutralGray*(1-pausedContrast))

	return color.NRGBA{
		R: gray,
		G: gray,
		B: gray,
		A: nrgba.A,
	}
}
