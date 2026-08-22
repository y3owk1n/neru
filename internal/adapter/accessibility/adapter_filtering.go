package accessibility

import (
	"slices"
	"strings"

	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/ports"
)

// MatchesFilter checks if an element matches the given filter criteria.
// NOTE: Filter search strings (TitleContains, DescriptionContains, ValueContains,
// and TextContainsList) must be pre-lowercased by the caller before invoking MatchesFilter
// to ensure case-insensitive matching without redundant allocations.
func (a *Adapter) MatchesFilter(
	elem *element.Element,
	filter ports.ElementFilter,
) bool {
	bounds := elem.Bounds()
	if bounds.Dx() < filter.MinSize.X || bounds.Dy() < filter.MinSize.Y {
		return false
	}

	if len(filter.Roles) > 0 {
		found := slices.Contains(filter.Roles, elem.Role())
		if !found {
			return false
		}
	}

	if slices.Contains(filter.ExcludeRoles, elem.Role()) {
		return false
	}

	titleMatched := false
	if filter.TitleContains != "" {
		title := elem.Title()
		if title != "" &&
			strings.Contains(strings.ToLower(title), filter.TitleContains) {
			titleMatched = true
		}
	}

	descMatched := false
	if filter.DescriptionContains != "" {
		description := elem.Description()
		if description != "" &&
			strings.Contains(
				strings.ToLower(description),
				filter.DescriptionContains,
			) {
			descMatched = true
		}
	}

	valueMatched := false
	if filter.ValueContains != "" {
		value := textForFilter(elem)
		if value != "" &&
			strings.Contains(strings.ToLower(value), filter.ValueContains) {
			valueMatched = true
		}
	}

	textListMatched := false
	if len(filter.TextContainsList) > 0 {
		title := elem.Title()
		description := elem.Description()

		value := textForFilter(elem)
		for _, textLower := range filter.TextContainsList {
			if textLower == "" {
				continue
			}

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
