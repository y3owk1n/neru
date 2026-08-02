//go:build integration && windows

// internal/adapter/accessibility/uia_windows_integration_test.go
// Real IUIAutomation enumeration test against the live foreground window.
// Does not run in default CI; execute on WIN-VM with a GUI app focused:
// go test -tags=integration ./internal/adapter/accessibility/...

package windows //nolint:testpackage // exercises unexported enumerateClickableElements directly

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestEnumerateClickableElementsIntegration(t *testing.T) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	getForegroundWindow := user32.NewProc("GetForegroundWindow")

	hwnd, _, _ := getForegroundWindow.Call()
	if hwnd == 0 {
		t.Skip("skipping: no foreground window (headless session)")
	}

	// A nil role set falls back to the shipped defaults, which is what the
	// hints path uses when no roles are configured.
	elements := enumerateClickableElements(hwnd, nil)
	if len(elements) == 0 {
		t.Skip("skipping: foreground window exposed no clickable elements")
	}

	clickableCount := 0
	for idx, elem := range elements {
		if elem.role == "" {
			t.Errorf("element %d has empty role", idx)
		}

		// Roles must be the native UIA control-type names the config vocabulary
		// resolves to, never the AX-style names neru used to synthesize here.
		if _, ok := defaultClickableRoles[elem.role]; !ok {
			t.Errorf(
				"element %d has role %q, which is not in the default role set",
				idx, elem.role,
			)
		}

		if elem.bounds.Dx() <= 0 || elem.bounds.Dy() <= 0 {
			t.Errorf("element %d has non-positive bounds %v", idx, elem.bounds)
		}

		if elem.clickable {
			clickableCount++
		}
	}

	if clickableCount == 0 {
		t.Fatalf(
			"enumerateClickableElements returned %d elements but none clickable",
			len(elements),
		)
	}

	t.Logf(
		"enumerated %d elements (%d clickable) from foreground window",
		len(elements),
		clickableCount,
	)
}
