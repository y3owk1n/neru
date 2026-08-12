//go:build linux

package linux

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The portal transport both KDE grants share: the object-path convention their
// replies arrive on, the bus dial they open on, and the two mappings the
// restore policy branches on. Nothing here is specific to RemoteDesktop or to
// ScreenCast, which is the point — a difference between them at this level
// would be a bug in one of them.

// TestPortalRequestPath_DerivesTheHandleFromTheSenderName pins the object-path
// convention the portal specification states, because getting it wrong is a
// silent hang rather than an error: the Response signal is emitted on the path
// derived from our unique bus name, and a listener on any other path simply
// never fires.
func TestPortalRequestPath_DerivesTheHandleFromTheSenderName(t *testing.T) {
	tests := []struct {
		name        string
		sender      string
		handleToken string
		want        dbus.ObjectPath
	}{
		{
			name:        "unique name loses its colon and gains underscores",
			sender:      ":1.42",
			handleToken: "neru_1",
			want:        "/org/freedesktop/portal/desktop/request/1_42/neru_1",
		},
		{
			name:        "every dot is replaced, not just the first",
			sender:      ":1.2.3",
			handleToken: "tok",
			want:        "/org/freedesktop/portal/desktop/request/1_2_3/tok",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := portalRequestPath(testCase.sender, testCase.handleToken)
			if got != testCase.want {
				t.Errorf("portalRequestPath(%q, %q) = %q, want %q",
					testCase.sender, testCase.handleToken, got, testCase.want)
			}
		})
	}
}

// TestPortalHandleToken_IsAValidObjectPathElement guards the other half of the
// same hazard: the token is pasted into an object path, and a character the
// D-Bus specification does not allow there makes the portal reject the whole
// call.
func TestPortalHandleToken_IsAValidObjectPathElement(t *testing.T) {
	valid := regexp.MustCompile(`^[A-Za-z0-9_]+$`)

	seen := make(map[string]bool)

	for range 32 {
		token := portalHandleToken()
		if !valid.MatchString(token) {
			t.Fatalf("portalHandleToken() = %q, want only [A-Za-z0-9_]", token)
		}

		if seen[token] {
			t.Fatalf("portalHandleToken() repeated %q; a reused handle would "+
				"collide with an in-flight request", token)
		}

		seen[token] = true
	}
}

// TestPortalCallFailed_KeepsTheBusErrorBodyOutOfTheMessage is a privacy
// guard, not a phrasing one. One of the options this code sends the portal is
// a restore token, and a backend that quotes a rejected option back in its
// error body would carry that credential into an error a caller may log. Only
// the error name is allowed through.
func TestPortalCallFailed_KeepsTheBusErrorBodyOutOfTheMessage(t *testing.T) {
	busError := dbus.Error{
		Name: "org.freedesktop.portal.Error.InvalidArgument",
		Body: []any{"restore_token 'secret-credential' is not valid"},
	}

	err := portalCallFailed(busError, "the portal refused SelectDevices")
	if err == nil {
		t.Fatal("portalCallFailed() = nil, want an error")
	}

	if strings.Contains(err.Error(), "secret-credential") {
		t.Errorf("error carries the bus error body: %q", err.Error())
	}

	if !strings.Contains(err.Error(), busError.Name) {
		t.Errorf("error = %q, want it to name %q", err.Error(), busError.Name)
	}
}

// TestPortalCallFailed_ReportsARanOutOfTimeCallAsATimeout matters because the
// restore policy branches on the code: a slow portal reported as a refusal
// would have the stored grant thrown away and the user prompted, for a failure
// that never reached the portal at all.
func TestPortalCallFailed_ReportsARanOutOfTimeCallAsATimeout(t *testing.T) {
	err := portalCallFailed(context.DeadlineExceeded, "the portal refused Start")

	if !derrors.IsCode(err, derrors.CodeTimeout) {
		t.Errorf("portalCallFailed(DeadlineExceeded) code = %q, want %q",
			derrors.GetCode(err), derrors.CodeTimeout)
	}

	if storedGrantPresumedDead(err) {
		t.Error("a timed-out call marks the stored grant dead, want it kept")
	}
}

// TestDialPortalBus_DoesNotStartASecondDialWhileOneIsStuck pins the rule the
// capability probes already follow: an uncancellable native-ish call that has
// wedged gets one outstanding attempt, not one per caller. Every mid-action
// input operation reaches this while no session is up, so restarting would buy
// an orphaned goroutine and a half-open connection per keypress.
func TestDialPortalBus_DoesNotStartASecondDialWhileOneIsStuck(t *testing.T) {
	dialer := &portalDialer{}

	first, err := dialer.start()
	if err != nil {
		t.Fatalf("first start() error = %v", err)
	}

	_, err = dialer.start()
	if err == nil {
		t.Fatal("second start() error = nil, want a refusal while one is outstanding")
	}

	if !derrors.IsCode(err, derrors.CodeTimeout) {
		t.Errorf("second start() code = %q, want %q", derrors.GetCode(err), derrors.CodeTimeout)
	}

	// Drain the real dial so the test leaves no goroutine behind, and close
	// whatever it produced — this machine may well have a session bus.
	result := <-first
	if result.conn != nil {
		_ = result.conn.Close()
	}
}

// TestPortalResponseError_SeparatesCancelationFromFailure pins the mapping the
// restore policy branches on. A canceled request is the user's own answer and
// must not be met with a second dialog; anything else is a failure the policy
// may fall back from.
func TestPortalResponseError_SeparatesCancelationFromFailure(t *testing.T) {
	tests := []struct {
		name     string
		code     uint32
		wantErr  bool
		canceled bool
	}{
		{name: "success", code: portalResponseSuccess},
		{name: "user canceled", code: portalResponseCanceled, wantErr: true, canceled: true},
		{name: "ended by other means", code: portalResponseEnded, wantErr: true},
		{name: "unknown code", code: 99, wantErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := portalResponseError(portalNameRemoteDesktop, testCase.code)

			if (err != nil) != testCase.wantErr {
				t.Fatalf(
					"portalResponseError(%d) = %v, wantErr %v",
					testCase.code,
					err,
					testCase.wantErr,
				)
			}

			if got := errors.Is(err, errPortalRequestCanceled); got != testCase.canceled {
				t.Errorf("errors.Is(err, errPortalRequestCanceled) = %v, want %v",
					got, testCase.canceled)
			}
		})
	}
}
