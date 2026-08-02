//go:build windows

package native //nolint:testpackage // exercises unexported defaultClickableRoles

import (
	"maps"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
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

// TestResolveRoles_WindowsNativeEntriesPassThrough covers the escape hatch that
// makes neru usable in legacy Win32 and WinForms applications, whose controls
// surface as Pane, Custom or Document and have no semantic role.
func TestResolveRoles_WindowsNativeEntriesPassThrough(t *testing.T) {
	t.Parallel()

	resolution := element.ResolveRoles([]string{"button", "uia:Custom", "uia:Pane"}, "windows")
	if resolution.HasFatal() {
		t.Fatalf("unexpected fatal diagnostics: %v", resolution.FatalMessages())
	}

	for _, want := range []string{uiaControlButton, uiaControlSplitButton, uiaControlCustom, uiaControlPane} {
		if !slices.Contains(resolution.Native, want) {
			t.Errorf("resolution %v missing %q", resolution.Native, want)
		}
	}
}
