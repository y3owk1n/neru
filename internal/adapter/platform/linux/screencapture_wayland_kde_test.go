//go:build linux

package linux

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

// TestPipewireCaptureError_DoesNotBorrowTheWlrootsSentences guards the one way
// this backend's failures could mislead. captureError's vocabulary was written
// for a compositor Neru talks to directly, so its "no display server" and "does
// not implement wlr-screencopy-unstable-v1" sentences would send a KDE user to
// look at KWin for a failure that belongs to the portal or to PipeWire — and
// they would contradict the capability matrix, which now reports KDE capture as
// working.
func TestPipewireCaptureError_DoesNotBorrowTheWlrootsSentences(t *testing.T) {
	tests := []struct {
		name          string
		status        captureStatus
		wantSubstring string
		wantAbsent    string
	}{
		{
			name:          "no PipeWire connection",
			status:        captureStatusNoDisplay,
			wantSubstring: "pipewire",
			wantAbsent:    "display server",
		},
		{
			name:          "PipeWire refused the stream",
			status:        captureStatusNoProtocol,
			wantSubstring: "PipeWire refused",
			wantAbsent:    "wlr-screencopy",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := pipewireCaptureError(testCase.status)
			if err == nil {
				t.Fatal("pipewireCaptureError() = nil, want an error")
			}

			if !strings.Contains(err.Error(), testCase.wantSubstring) {
				t.Errorf("error = %q, which does not mention %q",
					err.Error(), testCase.wantSubstring)
			}

			if strings.Contains(err.Error(), testCase.wantAbsent) {
				t.Errorf("error = %q, which still borrows %q",
					err.Error(), testCase.wantAbsent)
			}
		})
	}
}

// TestPipewireCaptureError_NamesKDEForTheSharedFailures pins the other half:
// everything a capture can fail with that means the same thing on all three
// backends keeps the shared sentence, and says which display server it is
// about.
func TestPipewireCaptureError_NamesKDEForTheSharedFailures(t *testing.T) {
	for _, status := range []captureStatus{
		captureStatusFailed,
		captureStatusTimeout,
	} {
		err := pipewireCaptureError(status)
		if err == nil {
			t.Fatalf("pipewireCaptureError(%d) = nil, want an error", status)
		}

		if !strings.Contains(err.Error(), captureLabelKDE) {
			t.Errorf("error = %q, which does not name %q", err.Error(), captureLabelKDE)
		}
	}
}

// TestPipewireCaptureError_TreatsEveryPipewireFailureAsRetriable is the code
// half of the same point. captureError reports "no display server", "no
// protocol" and "unreadable pixel format" as CodeNotSupported, which makes
// callers degrade permanently — right for a compositor that will never
// implement a protocol, wrong here, where all three describe one frame or one
// connection on a stack that granted the session a moment ago.
func TestPipewireCaptureError_TreatsEveryPipewireFailureAsRetriable(t *testing.T) {
	for _, status := range []captureStatus{
		captureStatusNoDisplay,
		captureStatusNoProtocol,
		captureStatusFormat,
	} {
		err := pipewireCaptureError(status)
		if derrors.IsNotSupported(err) {
			t.Errorf(
				"pipewireCaptureError(%d) is CodeNotSupported, want a retriable failure: %v",
				status,
				err,
			)
		}
	}
}

// TestKdeCaptureRegion_RefusesBeforeAskingThePortalWithNoStoredGrant is the
// contract that keeps a consent dialog off the capture path. A hint refresh
// runs while the user is waiting for hints, and the mode handler holds its lock
// across it; a portal prompt there is both a several-second stall and a
// question asked at the worst possible moment. With nothing stored to restore
// there is nothing this path may do but refuse and name the consent it needs.
func TestKdeCaptureRegion_RefusesBeforeAskingThePortalWithNoStoredGrant(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	img, err := kdeCaptureRegion(context.Background(), twoMonitorStreams()[0].bounds)
	if err == nil {
		t.Fatal("kdeCaptureRegion() error = nil, want a refusal with no grant stored")
	}

	if img != nil {
		t.Error("kdeCaptureRegion returned an image alongside its error")
	}

	if !derrors.IsNotSupported(err) {
		t.Errorf("error code = %q, want %q", derrors.GetCode(err), derrors.CodeNotSupported)
	}
}

// TestSystemAdapter_CheckScreenCapturePermission_IsHonestPerBackend pins the
// half of this change that is not about pixels at all. X11 and the wlroots
// family have no screen-recording gate, and reporting one would have callers
// prompt forever for a consent that is already granted; KDE reads the screen
// through a portal session that is exactly such a gate, and reporting it open
// unconditionally is what made the preflight useless on the one Linux backend
// that has a permission.
func TestSystemAdapter_CheckScreenCapturePermission_IsHonestPerBackend(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	tests := []struct {
		name    string
		backend string
		want    bool
	}{
		{name: "X11 has no gate", backend: backendX11, want: true},
		{name: "wlroots has no gate", backend: backendWaylandWlroots, want: true},
		{
			name:    "KDE reports the portal's consent",
			backend: backendWaylandKDE,
			// nativeBackendsCompiledIn is false on a CGO_ENABLED=0 build, where
			// there is no capture to gate at all and the refusal belongs to the
			// capture. With cgo, no session has been established here.
			want: !nativeBackendsCompiledIn,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			adapter := NewSystemAdapter(testCase.backend)

			got := adapter.CheckScreenCapturePermission(context.Background())
			if got != testCase.want {
				t.Errorf("CheckScreenCapturePermission() = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestSystemAdapter_RequestScreenCapturePermission_GrantsWhereThereIsNoGate
// keeps the two ungated backends from showing anything or dialing anything.
// The port contract says a platform with no permission gate returns granted
// without showing anything, and a Request that reached the portal on X11 would
// be a consent prompt invented out of nothing.
func TestSystemAdapter_RequestScreenCapturePermission_GrantsWhereThereIsNoGate(t *testing.T) {
	for _, backend := range []string{backendX11, backendWaylandWlroots} {
		t.Run(backend, func(t *testing.T) {
			adapter := NewSystemAdapter(backend)

			got := adapter.RequestScreenCapturePermission(context.Background())
			if got != ports.ScreenCaptureGranted {
				t.Errorf("RequestScreenCapturePermission() = %v, want granted", got)
			}
		})
	}
}

// TestRequestScreenCastConsent_ReportsCanceledOnACanceledContext pins that the
// caller's cancelation reaches the handshake. The mode handler hands this
// h.ctx, which carries no deadline and is canceled at shutdown; a handshake
// built from context.Background() would keep KDE's picker up, and this mutex
// held, for the full two minutes a human was given to answer it.
func TestRequestScreenCastConsent_ReportsCanceledOnACanceledContext(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := requestScreenCastConsent(ctx)
	if got != ports.ScreenCaptureCanceled {
		t.Errorf("requestScreenCastConsent() = %v, want canceled", got)
	}
}

// TestScreenCastConsentHeld_AnswersWhileTheSessionMutexIsHeld is the
// concurrency regression, and it is a port-contract one rather than a
// performance one. ports.SystemPort requires a Granted consent to be followed
// by a true permission check; the handler enforces that at
// resumeHintActivationAfterPermission and drops the activation when it fails.
// An implementation that took the session mutex here would answer false for as
// long as any other goroutine held it — including the moment right after the
// consent was granted — and throw away the activation the user had just
// approved. It runs under the mode handler's lock too, so it must not block at
// all.
func TestScreenCastConsentHeld_AnswersWhileTheSessionMutexIsHeld(t *testing.T) {
	state := globalScreenCastState

	state.mu.Lock()

	t.Cleanup(func() {
		state.ready.Store(false)
		state.mu.Unlock()
	})

	if screenCastConsentHeld() {
		t.Error("consent reported held with no session established")
	}

	// What a granted consent does, from inside the hold that granting it takes.
	state.ready.Store(true)

	answered := make(chan bool, 1)

	go func() { answered <- screenCastConsentHeld() }()

	select {
	case held := <-answered:
		if !held {
			t.Error("consent reported not held while the mutex was busy, " +
				"which would drop the activation the user just approved")
		}
	case <-time.After(time.Second):
		t.Fatal("screenCastConsentHeld blocked on the session mutex; it runs under " +
			"the mode handler's lock and must never wait")
	}
}
