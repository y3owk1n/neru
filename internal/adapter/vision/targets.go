package vision

import (
	"context"
	"fmt"
	"image"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// detectionFrame is one capture handed to a pixel detector: the frame, where
// its top-left corner sits on the global desktop, the region the caller asked
// for, and how many frame pixels make one logical pixel. The frame may be
// wider than the region (macOS falls back to capturing the whole display) but
// never narrower.
type detectionFrame struct {
	img    *image.RGBA
	origin image.Point
	region image.Rectangle
	scale  float64
}

// detectionRegion is the shared preamble of every pixel detector: honor a
// canceled context, and refuse an empty region, which a capture underneath
// would otherwise read as "the whole screen".
func detectionRegion(
	ctx context.Context,
	screenBounds image.Rectangle,
	detector string,
) (image.Rectangle, error) {
	select {
	case <-ctx.Done():
		return image.Rectangle{}, derrors.Wrap(
			ctx.Err(),
			derrors.CodeContextCanceled,
			"operation canceled",
		)
	default:
	}

	region := screenBounds.Canon()
	if region.Empty() {
		return image.Rectangle{}, derrors.Newf(
			derrors.CodeActionFailed,
			"%s detection needs a region to read; %v is empty",
			detector,
			screenBounds,
		)
	}

	return region, nil
}

// targetElements turns detector rectangles, which are logical pixels relative
// to the frame's top-left corner, into clickable vision-only buttons in global
// coordinates. Rectangles are clipped to the frame's region, so a frame wider
// than the region asked for yields nothing outside it, and a target straddling
// the edge cannot put its hint, and its click, outside.
func targetElements(
	frame detectionFrame,
	rects []image.Rectangle,
	source element.Source,
) []*element.Element {
	elements := make([]*element.Element, 0, len(rects))

	for _, rect := range rects {
		bounds := rect.Add(frame.origin).Intersect(frame.region)
		if bounds.Empty() {
			continue
		}

		elementID := element.ID(fmt.Sprintf(
			"%s-%d-%d-%d-%d",
			source,
			bounds.Min.X,
			bounds.Min.Y,
			bounds.Dx(),
			bounds.Dy(),
		))

		elem, err := element.NewElement(
			elementID,
			bounds,
			element.RoleButton,
			element.WithClickable(true),
			element.WithSource(source),
		)
		if err != nil {
			continue
		}

		elements = append(elements, elem)
	}

	return elements
}
