//go:build linux

package linux

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
)

// TestX11BoundsForActiveWindow is the bounds half of the _NET_ACTIVE_WINDOW
// split. Every non-answer used to arrive as "no focused window, no error", so a
// display server that never answered and a user looking at their wallpaper both
// silently widened hint detection to the whole screen. Only one of those is an
// answer.
func TestX11BoundsForActiveWindow(t *testing.T) {
	tests := []struct {
		name         string
		result       x11ActiveWindowResult
		wantGeometry bool
		wantErr      bool
		wantCode     derrors.Code
	}{
		{
			name:         "a found window is worth asking about",
			result:       x11ActiveWindowFound,
			wantGeometry: true,
		},
		{
			// The one case that keeps the nil error: the display answered, and
			// widening to the active screen is obeying the answer rather than
			// guessing past a failure.
			name:   "an unfocused desktop reports no bounds and no failure",
			result: x11ActiveWindowNone,
		},
		{
			name:     "a display no live window manager owns is a failure",
			result:   x11ActiveWindowNoWindowManager,
			wantErr:  true,
			wantCode: derrors.CodeActionFailed,
		},
		{
			name:     "a failed property read is a failure",
			result:   x11ActiveWindowQueryFailed,
			wantErr:  true,
			wantCode: derrors.CodeActionFailed,
		},
		{
			name:     "a malformed property is a failure",
			result:   x11ActiveWindowMalformed,
			wantErr:  true,
			wantCode: derrors.CodeActionFailed,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			readGeometry, err := x11ShouldReadWindowGeometry(testCase.result)

			if readGeometry != testCase.wantGeometry {
				t.Errorf("x11ShouldReadWindowGeometry(%v) geometry = %v, want %v",
					testCase.result, readGeometry, testCase.wantGeometry)
			}

			if !testCase.wantErr {
				if err != nil {
					t.Fatalf("x11ShouldReadWindowGeometry(%v) = %v, want nil error",
						testCase.result, err)
				}

				return
			}

			if err == nil {
				t.Fatalf("x11ShouldReadWindowGeometry(%v) = nil, want %q",
					testCase.result, testCase.wantCode)
			}

			if got := derrors.GetCode(err); got != testCase.wantCode {
				t.Errorf("x11ShouldReadWindowGeometry(%v) code = %q, want %q",
					testCase.result, got, testCase.wantCode)
			}
		})
	}
}

// TestX11WindowGeometryError names the failure a bounds caller gets when the
// window was found and the X server would not describe it. It has to be a
// failure and it has to say what was asked, because the caller's own answer —
// the whole screen — looks identical to a window that filled it.
func TestX11WindowGeometryError(t *testing.T) {
	err := x11WindowGeometryError()
	if err == nil {
		t.Fatal("x11WindowGeometryError() = nil, want a failure")
	}

	if got := derrors.GetCode(err); got != derrors.CodeActionFailed {
		t.Errorf("x11WindowGeometryError() code = %q, want %q", got, derrors.CodeActionFailed)
	}

	if derrors.IsNotSupported(err) {
		t.Error("x11WindowGeometryError() reports CodeNotSupported, so a caller would " +
			"read a live X11 session as one that cannot answer at all")
	}

	for _, word := range []string{"XGetWindowAttributes", "XTranslateCoordinates"} {
		if !strings.Contains(err.Error(), word) {
			t.Errorf("x11WindowGeometryError() = %q, want it to mention %q", err.Error(), word)
		}
	}
}
