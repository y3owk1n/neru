package accessibility

import (
	"github.com/y3owk1n/neru/internal/adapter/accessibility/ax"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// convertToDomainElement converts an ax.Node to a domain Element.
func (a *Adapter) convertToDomainElement(node ax.Node) (*element.Element, error) {
	if node == nil {
		return nil, derrors.New(derrors.CodeInvalidInput, "node is nil")
	}

	elementID := element.ID(node.ID())
	bounds := node.Bounds()
	role := element.Role(node.Role())
	isClickable := node.IsClickable()

	searchText := ""
	if provider, ok := node.(interface{ SearchText() string }); ok {
		searchText = provider.SearchText()
	}

	// Create element with options
	domElement, elementErr := element.NewElement(
		elementID,
		bounds,
		role,
		element.WithClickable(isClickable),
		element.WithTitle(node.Title()),
		element.WithDescription(node.Description()),
		element.WithValue(node.Value()),
		element.WithSearchText(searchText),
	)
	if elementErr != nil {
		return nil, derrors.Wrap(
			elementErr,
			derrors.CodeAccessibilityFailed,
			"failed to create element",
		)
	}

	return domElement, nil
}
