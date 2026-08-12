//go:build linux

package linux

import (
	"context"
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
			name:    "a degenerate region is refused, not widened to the whole screen",
			region:  image.Rect(10, 10, 200, 10),
			bounds:  screen,
			wantErr: true,
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
		// What DetectLinuxBackend actually returns for a session it could not
		// identify — the label the production caller passes, and the one an
		// earlier version of this code missed.
		{name: "detection answered unknown", backend: "unknown", want: "no display backend"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			img, err := CaptureScreenRegion(
				context.Background(),
				testCase.backend,
				image.Rect(0, 0, 10, 10),
			)
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
	// KDE reads its pixels through the portal rather than the display server, so
	// "headless" for that backend means no stored screen-sharing grant. Pointing
	// the state directory at a throwaway guarantees that, on a developer machine
	// that has one as much as on the CI runner that does not.
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	for _, backend := range []string{"x11", "wayland-wlroots", "wayland-kde"} {
		t.Run(backend, func(t *testing.T) {
			img, err := CaptureScreenRegion(context.Background(), backend, image.Rect(0, 0, 10, 10))
			if err == nil {
				t.Fatal("CaptureScreenRegion succeeded with no display server")
			}

			if img != nil {
				t.Error("CaptureScreenRegion returned an image alongside its error")
			}
		})
	}
}

// TestCaptureError covers the only part of a failed capture a user ever sees:
// the sentence. Each native status has to arrive as a distinguishable error,
// and the ones that mean "this display server will never do this" have to be
// CodeNotSupported so callers degrade instead of retrying.
//
// Nothing in this repository can run any of the three display servers these
// sentences describe, so this pins the sentence rather than the session.
func TestCaptureError(t *testing.T) {
	tests := []struct {
		name             string
		status           captureStatus
		what             string
		wantNotSupported bool
		wantSubstring    string
	}{
		{
			name:             "a compositor missing the protocol says which protocol",
			status:           captureStatusNoProtocol,
			what:             captureLabelCompositor,
			wantNotSupported: true,
			wantSubstring:    "wlr-screencopy-unstable-v1",
		},
		{
			name:             "no display server to connect to",
			status:           captureStatusNoDisplay,
			what:             captureLabelXServer,
			wantNotSupported: true,
			wantSubstring:    captureLabelXServer,
		},
		{
			name:             "an unreadable pixel format is not a transient failure",
			status:           captureStatusFormat,
			what:             captureLabelXServer,
			wantNotSupported: true,
			wantSubstring:    "pixel format",
		},
		{
			name:          "no output covers the region",
			status:        captureStatusNoOutput,
			what:          captureLabelCompositor,
			wantSubstring: "covers the requested region",
		},
		{
			name:          "an empty region",
			status:        captureStatusRegion,
			what:          captureLabelXServer,
			wantSubstring: "region is empty",
		},
		{
			name:          "the compositor never answered",
			status:        captureStatusTimeout,
			what:          captureLabelCompositor,
			wantSubstring: "in time",
		},
		{
			name:          "the copy failed",
			status:        captureStatusFailed,
			what:          captureLabelCompositor,
			wantSubstring: "failed to capture the screen",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := captureError(testCase.status, testCase.what)
			if err == nil {
				t.Fatal("captureError returned nil for a failure status")
			}

			if derrors.IsNotSupported(err) != testCase.wantNotSupported {
				t.Errorf(
					"captureError code is %q, want CodeNotSupported == %v",
					derrors.GetCode(err),
					testCase.wantNotSupported,
				)
			}

			if !strings.Contains(err.Error(), testCase.wantSubstring) {
				t.Errorf(
					"captureError said %q, which does not mention %q",
					err.Error(),
					testCase.wantSubstring,
				)
			}
		})
	}
}
