//go:build integration && windows

package windows_test

import (
	"context"
	"strings"
	"testing"

	winplatform "github.com/y3owk1n/neru/internal/adapter/platform/windows"
	"github.com/y3owk1n/neru/internal/derrors"
)

// Real Win32 integration tests for the Windows system adapter.
// Does not run in default CI; execute on WIN-VM with:
// go test -tags=integration ./internal/adapter/platform/windows/...
func skipIfHeadlessSession(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		return
	}

	msg := err.Error()
	if strings.Contains(msg, "interactive window station") ||
		derrors.IsCode(err, derrors.CodeElementNotFound) {
		t.Skipf("skipping: headless or non-interactive session (%v)", err)
	}
}

func TestSystemAdapterScreenAndCursorIntegration(t *testing.T) {
	t.Parallel()

	adapter := winplatform.NewSystemAdapter()
	ctx := context.Background()

	bounds, err := adapter.ScreenBounds(ctx)
	if err != nil {
		t.Fatalf("ScreenBounds: %v", err)
	}

	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Fatalf("ScreenBounds = %v, expected positive dimensions", bounds)
	}

	screens, err := adapter.Screens(ctx)
	if err != nil {
		t.Fatalf("Screens: %v", err)
	}

	if len(screens) == 0 {
		t.Fatal("Screens returned no monitors")
	}

	seen := make(map[string]bool, len(screens))
	for _, screen := range screens {
		if screen.Name == "" {
			t.Fatalf("Screens returned an unnamed screen: %v", screen)
		}

		if seen[screen.Name] {
			t.Fatalf("Screens returned %q twice; names must be unique", screen.Name)
		}

		seen[screen.Name] = true

		if screen.Bounds.Dx() <= 0 || screen.Bounds.Dy() <= 0 {
			t.Fatalf("Screens = %v, expected positive dimensions", screen.Bounds)
		}
	}

	cursor, err := adapter.CursorPosition(ctx)
	skipIfHeadlessSession(t, err)

	if err != nil {
		t.Fatalf("CursorPosition: %v", err)
	}

	err = adapter.MoveCursorToPoint(ctx, cursor, true)
	if err != nil {
		t.Fatalf("MoveCursorToPoint: %v", err)
	}
}

func TestSystemAdapterProcessIntegration(t *testing.T) {
	t.Parallel()

	adapter := winplatform.NewSystemAdapter()
	ctx := context.Background()

	pid, err := adapter.FocusedApplicationPID(ctx)
	skipIfHeadlessSession(t, err)

	if err != nil {
		t.Fatalf("FocusedApplicationPID: %v", err)
	}

	if pid <= 0 {
		t.Fatalf("FocusedApplicationPID = %d, want > 0", pid)
	}

	name, err := adapter.ApplicationNameByPID(ctx, pid)
	if err != nil {
		t.Fatalf("ApplicationNameByPID: %v", err)
	}

	if name == "" {
		t.Fatal("ApplicationNameByPID returned empty name")
	}

	bundleID, err := adapter.ApplicationBundleIDByPID(ctx, pid)
	if err != nil {
		t.Fatalf("ApplicationBundleIDByPID: %v", err)
	}

	if bundleID == "" {
		t.Fatal("ApplicationBundleIDByPID returned empty path")
	}
}
