package accessibility

import (
	"slices"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/domain/element"
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
	return !slices.Contains(filter.ExcludeRoles, element.Role())
}
