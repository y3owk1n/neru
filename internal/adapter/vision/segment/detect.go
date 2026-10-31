package segment

import (
	"context"
	"image"
	"math"

	pixels "github.com/y3owk1n/neru/internal/adapter/vision/frame"
	"github.com/y3owk1n/neru/internal/derrors"
)

// inkStep is the smallest per-channel difference between neighboring pixels
// that counts as an edge. Screens carry no sensor noise, so it sits far below
// what a photograph needs: low enough for a 1px border a few shades off its
// background, which is how flat UI draws controls.
const inkStep = 8

// Detect returns the bounding boxes of likely hint targets in img, in logical
// pixels relative to the frame's top-left corner. targets is what to hint, one
// of the domain.SegmentTargets values: every word, phrases, or controls only.
// scale is frame pixels per logical pixel. A frame at two or more pixels per
// logical pixel is box-filtered down by the whole part of that first, and the
// size rules scale by the rest, so they mean the same thing on every display.
// An empty frame is refused, and a frame with nothing in it returns no
// rectangles and no error. The context is checked between stages, so a
// deadline set by the caller ends a slow frame early instead of after the
// fact.
func Detect(
	ctx context.Context,
	img *image.RGBA,
	scale float64,
	targets string,
) ([]image.Rectangle, error) {
	if img == nil || img.Rect.Dx() <= 0 || img.Rect.Dy() <= 0 {
		return nil, derrors.New(
			derrors.CodeInvalidInput,
			"segment detection needs a non-empty frame",
		)
	}

	if scale <= 0 {
		scale = 1
	}

	frame := img

	// Whole factors only, rounded down, so a 150% frame is read at full
	// resolution rather than shrunk below its logical size.
	factor := int(scale)
	if factor > 1 {
		frame = pixels.Downsample(img, factor)
		scale /= float64(factor)
	}

	width, height := frame.Rect.Dx(), frame.Rect.Dy()
	lim := newLimits(scale, targets)

	steps := stepValues(frame, width, height)
	mask := stepMask(steps)

	err := checkDeadline(ctx)
	if err != nil {
		return nil, err
	}

	comps, ink := components(mask, width, height)

	// Texture raised some tile's threshold, so read the frame again through the
	// quieter mask, keeping the plain one inside pictures so an avatar stays
	// whole. A dense blob too big to be one target is a photo, so read it again
	// for what is drawn on it.
	photos, anyPhoto := densePhotos(comps, lim)
	quiet, raised := noiseMask(steps, width, height)

	if raised {
		keepWithin(quiet, mask, width, densePictures(comps, lim))
		mask = quiet
	}

	if anyPhoto {
		remaskPhotos(steps, mask, width, ink, photos)
	}

	if raised || anyPhoto {
		comps, ink = components(mask, width, height)
	}

	err = checkDeadline(ctx)
	if err != nil {
		return nil, err
	}

	measure(frame, comps, ink)

	rects := classify(comps, lim)

	// Nothing leaves after the deadline, however far the pass got.
	err = checkDeadline(ctx)
	if err != nil {
		return nil, err
	}

	for i, rect := range rects {
		rects[i] = toLogical(rect, scale)
	}

	return rects, nil
}

func checkDeadline(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "segment detection canceled")
	default:
		return nil
	}
}

// toLogical divides a frame-pixel rectangle by scale, rounding outward so a
// target never shrinks below its ink.
func toLogical(rect image.Rectangle, scale float64) image.Rectangle {
	if scale == 1 {
		return rect
	}

	return image.Rect(
		int(math.Floor(float64(rect.Min.X)/scale)),
		int(math.Floor(float64(rect.Min.Y)/scale)),
		int(math.Ceil(float64(rect.Max.X)/scale)),
		int(math.Ceil(float64(rect.Max.Y)/scale)),
	)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}

	return v
}
