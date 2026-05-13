package accessibility

import (
	"slices"
	"strings"

	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// MatchesFilter checks if an element matches the given filter criteria.
func (a *Adapter) MatchesFilter(
	elem *element.Element,
	filter ports.ElementFilter,
) bool {
	// Check minimum size
	bounds := elem.Bounds()
	if bounds.Dx() < filter.MinSize.X || bounds.Dy() < filter.MinSize.Y {
		return false
	}

	// Check role inclusion
	if len(filter.Roles) > 0 {
		found := slices.Contains(filter.Roles, elem.Role())
		if !found {
			return false
		}
	}

	// Check role exclusion
	if slices.Contains(filter.ExcludeRoles, elem.Role()) {
		return false
	}

	// Check title contains filter
	titleMatched := false
	if filter.TitleContains != "" {
		title := elem.Title()
		if title != "" &&
			strings.Contains(strings.ToLower(title), strings.ToLower(filter.TitleContains)) {
			titleMatched = true
		}
	}

	// Check description contains filter
	descMatched := false
	if filter.DescriptionContains != "" {
		description := elem.Description()
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
		value := textForFilter(elem)
		if value != "" &&
			strings.Contains(strings.ToLower(value), strings.ToLower(filter.ValueContains)) {
			valueMatched = true
		}
	}

	// Check any of the additional text substrings match (OR logic)
	textListMatched := false
	if len(filter.TextContainsList) > 0 {
		title := elem.Title()
		description := elem.Description()

		value := textForFilter(elem)
		for _, text := range filter.TextContainsList {
			if text == "" {
				continue
			}

			textLower := strings.ToLower(text)
			if (title != "" && strings.Contains(strings.ToLower(title), textLower)) ||
				(description != "" && strings.Contains(strings.ToLower(description), textLower)) ||
				(value != "" && strings.Contains(strings.ToLower(value), textLower)) {
				textListMatched = true

				break
			}
		}
	}

	// Match if any of title, description, or value matches (OR logic)
	if filter.TitleContains != "" || filter.DescriptionContains != "" ||
		filter.ValueContains != "" || len(filter.TextContainsList) > 0 {
		if !titleMatched && !descMatched && !valueMatched && !textListMatched {
			return false
		}
	}

	return true
}

func textForFilter(elem *element.Element) string {
	value := elem.Value()

	searchText := elem.SearchText()
	if searchText == "" {
		return value
	}

	if value == "" {
		return searchText
	}

	return value + " " + searchText
}
