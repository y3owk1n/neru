//go:build linux

package linux

import (
	"errors"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
)

// activeWindowProperty is the EWMH property every one of these answers comes
// from, and the word each failure message has to carry so a reader knows which
// query is being talked about.
const activeWindowProperty = "_NET_ACTIVE_WINDOW"

// TestX11ActiveWindowQueryError is the whole point of splitting the
// _NET_ACTIVE_WINDOW answers apart: a desktop with nothing focused is a state
// callers degrade through, and the three ways the query can fail are failures
// they surface. Collapsing them — which is what the C returning a bare 0 used
// to force — made clicking the desktop background look like a broken X server.
func TestX11ActiveWindowQueryError(t *testing.T) {
	tests := []struct {
		name     string
		result   x11ActiveWindowResult
		wantErr  bool
		wantCode derrors.Code
		// wantWords are substrings the message must carry so a reader can tell
		// which of the failures happened without reading this file.
		wantWords []string
	}{
		{
			name:    "a found window is not an error",
			result:  x11ActiveWindowFound,
			wantErr: false,
		},
		{
			name:      "nothing focused is not supported, not failed",
			result:    x11ActiveWindowNone,
			wantErr:   true,
			wantCode:  derrors.CodeNotSupported,
			wantWords: []string{"no window", "focus"},
		},
		{
			name:      "no EWMH window manager names itself",
			result:    x11ActiveWindowNoWindowManager,
			wantErr:   true,
			wantCode:  derrors.CodeActionFailed,
			wantWords: []string{activeWindowProperty, "window manager"},
		},
		{
			name:      "a failed property read names itself",
			result:    x11ActiveWindowQueryFailed,
			wantErr:   true,
			wantCode:  derrors.CodeActionFailed,
			wantWords: []string{"XGetWindowProperty", activeWindowProperty},
		},
		{
			name:      "a malformed property names itself",
			result:    x11ActiveWindowMalformed,
			wantErr:   true,
			wantCode:  derrors.CodeActionFailed,
			wantWords: []string{activeWindowProperty, "malformed"},
		},
		{
			// The header can grow a sixth answer before this switch does. It
			// must land on the side callers surface, not the side they
			// silently degrade through.
			name:      "an answer nobody has classified is a failure",
			result:    x11ActiveWindowMalformed + 1,
			wantErr:   true,
			wantCode:  derrors.CodeActionFailed,
			wantWords: []string{"unknown " + activeWindowProperty + " query result"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := x11ActiveWindowQueryError(testCase.result)

			if !testCase.wantErr {
				if err != nil {
					t.Fatalf("x11ActiveWindowQueryError(%v) = %v, want nil", testCase.result, err)
				}

				return
			}

			if err == nil {
				t.Fatalf(
					"x11ActiveWindowQueryError(%v) = nil, want %q",
					testCase.result,
					testCase.wantCode,
				)
			}

			if got := derrors.GetCode(err); got != testCase.wantCode {
				t.Errorf("x11ActiveWindowQueryError(%v) code = %q, want %q",
					testCase.result, got, testCase.wantCode)
			}

			for _, word := range testCase.wantWords {
				if !strings.Contains(err.Error(), word) {
					t.Errorf("x11ActiveWindowQueryError(%v) = %q, want it to mention %q",
						testCase.result, err.Error(), word)
				}
			}
		})
	}
}

// TestX11ActiveWindowQueryError_OnlyTheUnfocusedDesktopCarriesTheSentinel pins
// the half of the split the capability surface reads. `neru doctor` explains an
// unfocused desktop differently from a broken query, and it tells them apart by
// this sentinel — a failure wearing it would put the reassuring wording on a
// genuinely broken session.
func TestX11ActiveWindowQueryError_OnlyTheUnfocusedDesktopCarriesTheSentinel(t *testing.T) {
	unfocused := x11ActiveWindowQueryError(x11ActiveWindowNone)
	if !errors.Is(unfocused, errNoFocusedWindow) {
		t.Errorf("the unfocused-desktop error %v does not wrap errNoFocusedWindow", unfocused)
	}

	failures := []x11ActiveWindowResult{
		x11ActiveWindowNoWindowManager,
		x11ActiveWindowQueryFailed,
		x11ActiveWindowMalformed,
	}

	for _, result := range failures {
		err := x11ActiveWindowQueryError(result)
		if errors.Is(err, errNoFocusedWindow) {
			t.Errorf("failure %v wraps errNoFocusedWindow, so callers would read it as "+
				"an unfocused desktop", result)
		}

		if derrors.IsNotSupported(err) {
			t.Errorf("failure %v reports CodeNotSupported, so callers would degrade "+
				"through a real failure", result)
		}
	}
}

// TestSystemAdapter_UnavailableDetail_SeparatesAnUnfocusedDesktopFromAFailure
// covers what a user actually reads. `neru doctor` downgrades a capability its
// live probe refused, and the probe refuses on an unfocused desktop as much as
// on a broken one — but telling someone "focused-app inspection is unavailable"
// because they clicked their wallpaper sends them looking for a fix that does
// not exist. The status stays a stub, since that is what FocusedApplicationPID
// answers right now and the matrix has to agree with behavior; the wording is
// what has to stop claiming the subsystem is gone.
func TestSystemAdapter_UnavailableDetail_SeparatesAnUnfocusedDesktopFromAFailure(t *testing.T) {
	adapter := NewSystemAdapter(backendX11)
	feature := "focused-app inspection"

	unfocused := adapter.unavailableDetail(feature, x11ActiveWindowQueryError(x11ActiveWindowNone))

	if strings.Contains(unfocused, "unavailable") {
		t.Errorf("an unfocused desktop is described as %q; it must not claim the "+
			"capability is unavailable", unfocused)
	}

	for _, word := range []string{feature, backendX11, "focus"} {
		if !strings.Contains(unfocused, word) {
			t.Errorf("the unfocused-desktop detail %q does not mention %q", unfocused, word)
		}
	}

	// A real failure keeps the wording it had: something is wrong and the user
	// has something to do about it.
	broken := adapter.unavailableDetail(
		feature,
		x11ActiveWindowQueryError(x11ActiveWindowNoWindowManager),
	)
	if !strings.Contains(broken, "unavailable") {
		t.Errorf("a failed query is described as %q; it must still read as unavailable", broken)
	}
}
