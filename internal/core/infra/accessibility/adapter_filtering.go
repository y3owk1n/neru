package accessibility

import (
	"slices"
	"strings"

	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// MatchesFilter checks if an element matches the given filter criteria.
func (a *Adapter) MatchesFilter(
	element *element.Element,
	filter ports.ElementFilter,
) bool {
	// Check minimum size
	bounds := element.Bounds()
	if bounds.Dx() < filter.MinSize.X || bounds.Dy() < filter.MinSize.Y {
		return false
	}

	// Check role inclusion
	if len(filter.Roles) > 0 {
		found := slices.Contains(filter.Roles, element.Role())
		if !found {
			return false
		}
	}

	// Check role exclusion
	if slices.Contains(filter.ExcludeRoles, element.Role()) {
		return false
	}

	// Check title contains filter
	titleMatched := false
	if filter.TitleContains != "" {
		title := element.Title()
		if title != "" &&
			strings.Contains(strings.ToLower(title), strings.ToLower(filter.TitleContains)) {
			titleMatched = true
		}
	}

	// Check description contains filter
	descMatched := false
	if filter.DescriptionContains != "" {
		description := element.Description()
		if description != "" &&
			strings.Contains(
				strings.ToLower(description),
				strings.ToLower(filter.DescriptionContains),
			) {
			descMatched = true
		}
	}

	// Check value contains filter
	valueMatched := false
	if filter.ValueContains != "" {
		value := element.Value()
		if value != "" &&
			strings.Contains(strings.ToLower(value), strings.ToLower(filter.ValueContains)) {
			valueMatched = true
		}
	}

	// Match if any of title, description, or value matches (OR logic)
	if filter.TitleContains != "" || filter.DescriptionContains != "" ||
		filter.ValueContains != "" {
		if !titleMatched && !descMatched && !valueMatched {
			return false
		}
	}

	return true
}
