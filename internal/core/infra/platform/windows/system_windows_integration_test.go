//go:build integration && windows

// internal/core/infra/platform/windows/system_windows_integration_test.go
// Real Win32 integration tests for the Windows system adapter.
// Does not run in default CI; execute on WIN-VM with:
// go test -tags=integration ./internal/core/infra/platform/windows/...

package windows_test

import (
	"context"
	"strings"
	"testing"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	winplatform "github.com/y3owk1n/neru/internal/core/infra/platform/windows"
)

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

	names, err := adapter.ScreenNames(ctx)
	if err != nil {
		t.Fatalf("ScreenNames: %v", err)
	}

	if len(names) == 0 {
		t.Fatal("ScreenNames returned no monitors")
	}

	foundBounds, ok, err := adapter.ScreenBoundsByName(ctx, names[0])
	if err != nil {
		t.Fatalf("ScreenBoundsByName: %v", err)
	}

	if !ok {
		t.Fatalf("ScreenBoundsByName did not find %q", names[0])
	}

	if foundBounds.Dx() <= 0 || foundBounds.Dy() <= 0 {
		t.Fatalf("ScreenBoundsByName = %v, expected positive dimensions", foundBounds)
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
