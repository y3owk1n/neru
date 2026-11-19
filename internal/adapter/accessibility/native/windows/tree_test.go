//go:build windows

package windows

import (
	"maps"
	"slices"
	"testing"
)

// TestDefaultClickableRoles_ResolvesToUIANames checks the enumeration fallback
// used when a caller supplies no role filter. It must resolve to real UIA
// control-type names, otherwise hints are blank whenever the role set is empty.
func TestDefaultClickableRoles_ResolvesToUIANames(t *testing.T) {
	t.Parallel()

	if len(defaultClickableRoles) == 0 {
		t.Fatal("defaultClickableRoles is empty; enumeration would find nothing")
	}

	names := slices.Collect(maps.Values(controlTypeNames))

	for role := range defaultClickableRoles {
		if !slices.Contains(names, role) {
			t.Errorf("defaultClickableRoles contains %q, which is not a UIA control type", role)
		}
	}

	for _, want := range []string{uiaControlButton, uiaControlEdit, uiaControlHyperlink} {
		if _, ok := defaultClickableRoles[want]; !ok {
			t.Errorf("defaultClickableRoles missing %q", want)
		}
	}
}
