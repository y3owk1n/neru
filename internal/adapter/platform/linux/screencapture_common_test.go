//go:build linux

package linux

import (
	"image"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
)

// TestResolveCaptureRegion covers the one piece of capture logic that is not
// native: turning the caller's request into the concrete rectangle the backends
// receive. An empty rectangle is the port's CaptureScreen contract ("the
// current screen"), anything else is a region the caller wants honored —
// grabbing the whole display for a focused window is the difference between
// usable and not on a 4K screen.
func TestResolveCaptureRegion(t *testing.T) {
	screen := image.Rect(0, 0, 3840, 2160)
	errNoOutputs := derrors.New(derrors.CodeActionFailed, "no outputs")

	tests := []struct {
		name      string
		region    image.Rectangle
		bounds    image.Rectangle
		boundsErr error
		want      image.Rectangle
		wantErr   bool
	}{
		{
			name:   "an explicit region is passed through untouched",
			region: image.Rect(100, 200, 500, 700),
			bounds: screen,
			want:   image.Rect(100, 200, 500, 700),
		},
		{
			name:   "an empty region means the whole active screen",
			region: image.Rectangle{},
			bounds: screen,
			want:   screen,
		},
		{
			name:   "an inverted region is canonicalized rather than rejected",
			region: image.Rect(500, 700, 100, 200),
			bounds: screen,
			want:   image.Rect(100, 200, 500, 700),
		},
		{
			name:   "a zero-height region falls back to the whole screen",
			region: image.Rect(10, 10, 200, 10),
			bounds: screen,
			want:   screen,
		},
		{
			name:      "a screen-bounds failure is reported, not guessed around",
			region:    image.Rectangle{},
			boundsErr: errNoOutputs,
			wantErr:   true,
		},
		{
			name:    "an empty screen leaves nothing to capture",
			region:  image.Rectangle{},
			bounds:  image.Rectangle{},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			bounds := func() (image.Rectangle, error) {
				return testCase.bounds, testCase.boundsErr
			}

			got, err := resolveCaptureRegion(testCase.region, bounds)

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("resolveCaptureRegion(%v) = %v, want an error", testCase.region, got)
				}

				return
			}

			if err != nil {
				t.Fatalf("resolveCaptureRegion(%v) returned %v", testCase.region, err)
			}

			if got != testCase.want {
				t.Errorf(
					"resolveCaptureRegion(%v) = %v, want %v",
					testCase.region,
					got,
					testCase.want,
				)
			}
		})
	}
}

// TestCaptureScreenRegion_UnknownBackendReportsNotSupported pins the loud-stub
// rule for the one axis capture cannot answer: a display server with no capture
// mechanism Neru implements. The error has to name the backend, because "screen
// capture failed" on GNOME and "screen capture failed" on a session with no
// display server send a user to two different places.
func TestCaptureScreenRegion_UnknownBackendReportsNotSupported(t *testing.T) {
	tests := []struct {
		name    string
		backend string
		want    string
	}{
		{name: "gnome", backend: "wayland-gnome", want: "wayland-gnome"},
		{name: "other wayland", backend: "wayland-other", want: "wayland-other"},
		{name: "no backend detected", backend: "", want: "no display backend"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			img, err := CaptureScreenRegion(testCase.backend, image.Rect(0, 0, 10, 10))
			if err == nil {
				t.Fatal("CaptureScreenRegion returned nil error for a backend with no capture path")
			}

			if img != nil {
				t.Error("CaptureScreenRegion returned an image alongside its error")
			}

			if !derrors.IsNotSupported(err) {
				t.Errorf(
					"CaptureScreenRegion returned code %q, want CodeNotSupported",
					derrors.GetCode(err),
				)
			}

			if !strings.Contains(err.Error(), testCase.want) {
				t.Errorf(
					"CaptureScreenRegion error %q does not name %q",
					err.Error(),
					testCase.want,
				)
			}
		})
	}
}

// TestCaptureScreenRegion_HeadlessSessionFailsLoudly checks the implemented
// backends refuse rather than return an empty image when there is no session to
// capture. This is what the CI runner exercises: no DISPLAY, no
// WAYLAND_DISPLAY.
func TestCaptureScreenRegion_HeadlessSessionFailsLoudly(t *testing.T) {
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")

	for _, backend := range []string{"x11", "wayland-wlroots", "wayland-kde"} {
		t.Run(backend, func(t *testing.T) {
			img, err := CaptureScreenRegion(backend, image.Rect(0, 0, 10, 10))
			if err == nil {
				t.Fatal("CaptureScreenRegion succeeded with no display server")
			}

			if img != nil {
				t.Error("CaptureScreenRegion returned an image alongside its error")
			}
		})
	}
}
