package contour

import (
	"fmt"
	"image"

	"github.com/y3owk1n/neru/internal/domain/element"
)

// Elements turns detector rectangles, which are logical pixels relative to the
// captured frame's top-left corner, into clickable vision-only buttons in
// global coordinates. origin is where the frame sits on the global desktop;
// only rectangles overlapping region are kept, so a frame wider than the
// region asked for (macOS captures the whole display) yields nothing outside
// it.
func Elements(
	origin image.Point,
	region image.Rectangle,
	rects []image.Rectangle,
) []*element.Element {
	elements := make([]*element.Element, 0, len(rects))

	for _, rect := range rects {
		bounds := rect.Add(origin)
		if !bounds.Overlaps(region) {
			continue
		}

		elementID := element.ID(
			fmt.Sprintf(
				"contour-%d-%d-%d-%d",
				bounds.Min.X,
				bounds.Min.Y,
				bounds.Dx(),
				bounds.Dy(),
			),
		)

		elem, err := element.NewElement(
			elementID,
			bounds,
			element.RoleButton,
			element.WithClickable(true),
			element.WithVisionOnly(),
		)
		if err != nil {
			continue
		}

		elements = append(elements, elem)
	}

	return elements
}
