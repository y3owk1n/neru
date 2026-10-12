package modes

import (
	"context"
	"image"
	"strconv"

	"github.com/y3owk1n/neru/internal/domain/element"
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// labelTargets labels the targets an activation supplied, in place of a scan.
// A target carries no accessibility reference, so like a vision element it is
// acted on at its coordinates.
func (h *handlerState) labelTargets(
	ctx context.Context,
	targets []modecmd.Target,
	bundleID string,
	labelDirection string,
) ([]*domainHint.Interface, error) {
	elements := make([]*element.Element, 0, len(targets))

	for idx, target := range targets {
		elem, err := element.NewElement(
			element.ID("target-"+strconv.Itoa(idx)),
			image.Rect(target.X, target.Y, target.X+target.Width, target.Y+target.Height),
			element.Role(target.Role),
			element.WithTitle(target.Title),
			element.WithDescription(target.Description),
			element.WithValue(target.Value),
			element.WithVisionOnly(),
		)
		if err != nil {
			return nil, err
		}

		elements = append(elements, elem)
	}

	return h.hintService.LabelElements(ctx, elements, bundleID, labelDirection)
}
