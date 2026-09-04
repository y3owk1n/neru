//go:build windows && !amd64

package windows

import (
	"errors"

	"golang.org/x/sys/windows"
)

// DirectComposition is not reachable from this build. Direct2D takes floats
// and Go's stdcall shim mirrors integer arguments into the floating-point
// registers on amd64 only (overlay_dcomp_amd64.go), so on every other architecture
// the GDI surface is the overlay. docs/CROSS_PLATFORM.md owns that status.

var errDCompUnavailable = errors.New(
	"directcomposition overlay is only built for windows/amd64",
)

func dcompAvailable() bool { return false }

func newDCompSurface(windows.HWND, int, int) (overlaySurface, error) {
	return nil, errDCompUnavailable
}
