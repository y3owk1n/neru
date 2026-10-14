//go:build linux

package linux_test

import (
	"image"
	"testing"

	nativelinux "github.com/y3owk1n/neru/internal/adapter/accessibility/native/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// TestMouseActions_RefuseWithoutABackend pins that a click or drag with no
// display server to inject through says so. Each one answered nil, so a hint
// selection or `neru action left_click` reported success while nothing was
// clicked.
func TestMouseActions_RefuseWithoutABackend(t *testing.T) {
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", "")

	// The backend is detected once per process and cached, so a session that
	// has a display server makes this branch unreachable rather than wrong.
	if platform.DetectLinuxBackend() != platform.BackendUnknown {
		t.Skip("a display server backend was detected; the no-backend branch is unreachable here")
	}

	point := image.Pt(100, 100)

	tests := []struct {
		name string
		call func() error
	}{
		{"left click", func() error { return nativelinux.LeftClickAtPoint(point, false, 0) }},
		{"right click", func() error { return nativelinux.RightClickAtPoint(point, false, 0) }},
		{"middle click", func() error { return nativelinux.MiddleClickAtPoint(point, false, 0) }},
		{
			"mouse down",
			func() error { return nativelinux.MouseDownAtPoint(point, action.ButtonLeft, 0) },
		},
		{
			"mouse up at a point",
			func() error { return nativelinux.MouseUpAtPoint(point, action.ButtonLeft, 0) },
		},
		{"mouse up", func() error { return nativelinux.MouseUp(action.ButtonLeft) }},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.call()
			if !derrors.IsNotSupported(err) {
				t.Fatalf(
					"%s with no backend returned %v, want a CodeNotSupported error",
					testCase.name,
					err,
				)
			}
		})
	}
}
