//go:build integration && windows

// internal/core/infra/accessibility/uia_windows_integration_test.go
// Real IUIAutomation enumeration test against the live foreground window.
// Does not run in default CI; execute on WIN-VM with a GUI app focused:
// go test -tags=integration ./internal/core/infra/accessibility/...

package accessibility

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

	elements := enumerateClickableElements(hwnd)
	if len(elements) == 0 {
		t.Skip("skipping: foreground window exposed no clickable elements")
	}

	clickableCount := 0
	for i, el := range elements {
		if el.role == "" {
			t.Errorf("element %d has empty role", i)
		}

		if el.bounds.Dx() <= 0 || el.bounds.Dy() <= 0 {
			t.Errorf("element %d has non-positive bounds %v", i, el.bounds)
		}

		if el.clickable {
			clickableCount++
		}
	}

	if clickableCount == 0 {
		t.Fatalf("enumerateClickableElements returned %d elements but none clickable", len(elements))
	}

	t.Logf("enumerated %d elements (%d clickable) from foreground window", len(elements), clickableCount)
}
