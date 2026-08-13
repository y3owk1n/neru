//go:build linux

package linux

import (
	"errors"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
)

// wmPIDProperty is the EWMH property every one of these answers comes from,
// and the word each message has to carry so a reader knows which query is being
// talked about.
const wmPIDProperty = "_NET_WM_PID"

// TestX11WindowPIDError is the _NET_WM_PID half of the split
// TestX11ActiveWindowQueryError covers one property up: a window that is alive
// and advertises no pid answered the question, and a window that closed under
// the query is a failure. Collapsing them — which is what the C returning a bare
// ok flag used to force — told a user focusing an older X client that the query
// had failed.
func TestX11WindowPIDError(t *testing.T) {
	tests := []struct {
		name     string
		result   x11WindowPIDResult
		wantErr  bool
		wantCode derrors.Code
		// wantWords are substrings the message must carry so a reader can tell
		// which of the answers happened without reading this file.
		wantWords []string
	}{
		{
			name:    "a pid that was read is not an error",
			result:  x11WindowPIDFound,
			wantErr: false,
		},
		{
			name:      "a window that advertises no pid is not supported, not failed",
			result:    x11WindowPIDAbsent,
			wantErr:   true,
			wantCode:  derrors.CodeNotSupported,
			wantWords: []string{wmPIDProperty, "no"},
		},
		{
			name:      "a window that closed names itself",
			result:    x11WindowPIDWindowGone,
			wantErr:   true,
			wantCode:  derrors.CodeActionFailed,
			wantWords: []string{wmPIDProperty, "closed"},
		},
		{
			name:      "a failed property read names itself",
			result:    x11WindowPIDQueryFailed,
			wantErr:   true,
			wantCode:  derrors.CodeActionFailed,
			wantWords: []string{"XGetWindowProperty", wmPIDProperty},
		},
		{
			name:      "a malformed property names itself",
			result:    x11WindowPIDMalformed,
			wantErr:   true,
			wantCode:  derrors.CodeActionFailed,
			wantWords: []string{wmPIDProperty, "malformed"},
		},
		{
			// The header can grow a sixth answer before this switch does. It
			// must land on the side callers surface, not the side they silently
			// degrade through.
			name:      "an answer nobody has classified is a failure",
			result:    x11WindowPIDMalformed + 1,
			wantErr:   true,
			wantCode:  derrors.CodeActionFailed,
			wantWords: []string{"unknown " + wmPIDProperty + " query result"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := x11WindowPIDError(testCase.result)

			if !testCase.wantErr {
				if err != nil {
					t.Fatalf("x11WindowPIDError(%v) = %v, want nil", testCase.result, err)
				}

				return
			}

			if err == nil {
				t.Fatalf(
					"x11WindowPIDError(%v) = nil, want %q",
					testCase.result,
					testCase.wantCode,
				)
			}

			if got := derrors.GetCode(err); got != testCase.wantCode {
				t.Errorf("x11WindowPIDError(%v) code = %q, want %q",
					testCase.result, got, testCase.wantCode)
			}

			for _, word := range testCase.wantWords {
				if !strings.Contains(err.Error(), word) {
					t.Errorf("x11WindowPIDError(%v) = %q, want it to mention %q",
						testCase.result, err.Error(), word)
				}
			}
		})
	}
}

// TestX11WindowPIDError_OnlyTheAbsentPropertyCarriesTheSentinel pins the half of
// the split the capability surface reads. A window that advertises no pid is not
// a broken session, and `neru doctor` tells the two apart by this sentinel — a
// failure wearing it would put the reassuring wording on a display that really
// did fail to answer.
//
// It is deliberately its own sentinel rather than errNoFocusedWindow: a window
// *does* have focus here, so the sentence that explains an unfocused desktop
// ("answers as soon as a window takes focus") would be false.
func TestX11WindowPIDError_OnlyTheAbsentPropertyCarriesTheSentinel(t *testing.T) {
	absent := x11WindowPIDError(x11WindowPIDAbsent)
	if !errors.Is(absent, errNoWindowPID) {
		t.Errorf("the absent-property error %v does not wrap errNoWindowPID", absent)
	}

	if errors.Is(absent, errNoFocusedWindow) {
		t.Errorf("the absent-property error %v claims nothing has focus, but a window "+
			"was found and only its pid was not", absent)
	}

	failures := []x11WindowPIDResult{
		x11WindowPIDWindowGone,
		x11WindowPIDQueryFailed,
		x11WindowPIDMalformed,
	}

	for _, result := range failures {
		err := x11WindowPIDError(result)
		if errors.Is(err, errNoWindowPID) {
			t.Errorf("failure %v wraps errNoWindowPID, so callers would read it as "+
				"a window that simply advertises no pid", result)
		}

		if derrors.IsNotSupported(err) {
			t.Errorf("failure %v reports CodeNotSupported, so callers would degrade "+
				"through a real failure", result)
		}
	}
}

// TestSystemAdapter_UnavailableDetail_SeparatesAWindowWithNoPIDFromAFailure
// covers what a user actually reads. `neru doctor` downgrades a capability its
// live probe refused, and the probe refuses on a window that advertises no pid
// as much as on a broken display — but telling someone "focused-app inspection
// is unavailable" because they focused an older X client sends them looking for
// a fix that does not exist.
func TestSystemAdapter_UnavailableDetail_SeparatesAWindowWithNoPIDFromAFailure(t *testing.T) {
	adapter := NewSystemAdapter(backendX11)
	feature := focusedAppFeature

	noPID := adapter.unavailableDetail(feature, x11WindowPIDError(x11WindowPIDAbsent))

	if strings.Contains(noPID, "unavailable") {
		t.Errorf("a window that advertises no pid is described as %q; it must not claim "+
			"the capability is unavailable", noPID)
	}

	for _, word := range []string{feature, backendX11, wmPIDProperty} {
		if !strings.Contains(noPID, word) {
			t.Errorf("the no-pid detail %q does not mention %q", noPID, word)
		}
	}

	// A window that closed under the query keeps the wording it had: something
	// went wrong, and the user is not being told a healthy session is fine.
	broken := adapter.unavailableDetail(feature, x11WindowPIDError(x11WindowPIDWindowGone))
	if !strings.Contains(broken, "unavailable") {
		t.Errorf("a failed query is described as %q; it must still read as unavailable", broken)
	}
}
