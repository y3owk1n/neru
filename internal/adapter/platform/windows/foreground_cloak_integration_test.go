//go:build integration && windows

package windows

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/y3owk1n/neru/internal/derrors"
)

// dwmCloak is DWMWA_CLOAK, which a process may set on its own windows.
const dwmCloak = 13

// TestUsableWindowHandle_RefusesACloakedWindow pins that a window DWM keeps
// off the screen does not count as the focused window. The shell leaves its
// Search host in the foreground, cloaked, after it is dismissed, and vim took
// its frame for the window's, drawing its status bar along a window nobody
// could see. The test checks the handle, because Windows does not reliably
// let a test process take the foreground.
func TestUsableWindowHandle_RefusesACloakedWindow(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		cloak bool
	}{
		{"shown window is usable", false},
		{"cloaked window is not", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			hwnd := createTestWindow(t, "neru-cloak-"+scenario.name)

			if scenario.cloak {
				on := uint32(1)

				result, _, _ := procDwmSetWindowAttribute.Call(
					hwnd, dwmCloak, uintptr(unsafe.Pointer(&on)), unsafe.Sizeof(on),
				)
				if result != 0 {
					t.Skipf(
						"skipping: DWM cannot cloak a window in this session (HRESULT %#x)",
						result,
					)
				}
			}

			_, err := usableWindowHandle(windows.HWND(hwnd))

			switch {
			case scenario.cloak && !derrors.IsCode(err, derrors.CodeElementNotFound):
				t.Fatalf("usableWindowHandle(cloaked) = %v, want CodeElementNotFound", err)
			case !scenario.cloak && err != nil:
				t.Fatalf("usableWindowHandle(shown) = %v, want the handle", err)
			}
		})
	}
}
