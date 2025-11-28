package accessibility

import (
	"github.com/y3owk1n/neru/internal/core/domain/element"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// convertToDomainElement converts an AXNode to a domain Element.
func (a *Adapter) convertToDomainElement(node AXNode) (*element.Element, error) {
	if node == nil {
		return nil, derrors.New(derrors.CodeInvalidInput, "node is nil")
	}

	// Create element ID from unique identifier
	elementID := element.ID(node.ID())

	// Get bounds
	bounds := node.Bounds()

	// Convert role
	role := element.Role(node.Role())

	// Determine if clickable
	isClickable := node.IsClickable()

	// Create element with options
	element, elementErr := element.NewElement(
		elementID,
		bounds,
		role,
		element.WithClickable(isClickable),
		element.WithTitle(node.Title()),
		element.WithDescription(node.Description()),
	)
	if elementErr != nil {
		return nil, derrors.Wrap(
			elementErr,
			derrors.CodeAccessibilityFailed,
			"failed to create element",
		)
	}

	return element, nil
}
