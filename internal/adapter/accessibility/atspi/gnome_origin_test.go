//go:build linux

package atspi

import (
	"errors"
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/gnomeshell"
	"github.com/y3owk1n/neru/internal/derrors"
)

const (
	gnomeTestAppID = "org.gnome.Nautilus"
	gnomeTestTitle = "Home"
)

var errGNOMEAbsent = errors.New("absent")

type fakeGNOMEGeometry struct {
	window gnomeshell.Window
	cached bool
	err    error
}

func (f *fakeGNOMEGeometry) EnsureStarted() {}

func (f *fakeGNOMEGeometry) Focused() (gnomeshell.Window, bool, error) {
	return f.window, f.cached, f.err
}

func TestGNOMEOriginSource_OriginFor(t *testing.T) {
	window := gnomeshell.Window{
		Rect:  image.Rect(100, 50, 900, 650),
		AppID: gnomeTestAppID,
		Title: gnomeTestTitle,
	}
	frame := windowFrame{
		Width:        800,
		Height:       600,
		FocusedAppID: gnomeTestAppID,
		FocusedTitle: gnomeTestTitle,
	}

	tests := []struct {
		name     string
		geometry fakeGNOMEGeometry
		frame    windowFrame
		want     image.Point
		wantOK   bool
		wantErr  bool
	}{
		{
			name:     "extension absent is a refusal",
			geometry: fakeGNOMEGeometry{err: errGNOMEAbsent},
			frame:    frame,
			wantErr:  true,
		},
		{name: "nothing cached is not found", geometry: fakeGNOMEGeometry{}, frame: frame},
		{
			name:     "matching window offsets",
			geometry: fakeGNOMEGeometry{window: window, cached: true},
			frame:    frame,
			want:     image.Pt(100, 50),
			wantOK:   true,
		},
		{
			name:     "another application is rejected",
			geometry: fakeGNOMEGeometry{window: window, cached: true},
			frame: windowFrame{
				Width:        800,
				Height:       600,
				FocusedAppID: "firefox",
				FocusedTitle: gnomeTestTitle,
			},
		},
		{
			name:     "another document is rejected",
			geometry: fakeGNOMEGeometry{window: window, cached: true},
			frame: windowFrame{
				Width:        800,
				Height:       600,
				FocusedAppID: gnomeTestAppID,
				FocusedTitle: "Downloads",
			},
		},
		{
			name:     "a different size is rejected",
			geometry: fakeGNOMEGeometry{window: window, cached: true},
			frame: windowFrame{
				Width:        400,
				Height:       600,
				FocusedAppID: gnomeTestAppID,
				FocusedTitle: gnomeTestTitle,
			},
		},
		{
			name:     "an identity nobody reported is not a mismatch",
			geometry: fakeGNOMEGeometry{window: gnomeshell.Window{Rect: window.Rect}, cached: true},
			frame:    windowFrame{Width: 800, Height: 600},
			want:     image.Pt(100, 50),
			wantOK:   true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			geometry := testCase.geometry
			source := &gnomeOriginSource{logger: zap.NewNop(), geometry: &geometry}

			got, found, err := source.originFor(testCase.frame)
			if testCase.wantErr {
				if !derrors.IsNotSupported(err) {
					t.Fatalf("originFor() error = %v, want CodeNotSupported", err)
				}

				return
			}

			if err != nil || found != testCase.wantOK || got != testCase.want {
				t.Fatalf(
					"originFor() = (%v, %v, %v), want (%v, %v, nil)",
					got,
					found,
					err,
					testCase.want,
					testCase.wantOK,
				)
			}
		})
	}
}
