//go:build linux

package linux_test

import (
	"context"
	"image"
	"os"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/derrors"
)

// unimplementedBackend is a backend name no dispatch branch recognizes, so
// every method falls through to its stub.
const unimplementedBackend = "test-unimplemented-backend"

// The backend labels the adapter is built with, spelled as
// platform.LinuxBackend.String() produces them.
const (
	x11Backend     = "x11"
	wlrootsBackend = "wayland-wlroots"
	kdeBackend     = "wayland-kde"
)

// stubCall names a SystemAdapter method and invokes it, discarding any
// non-error results so only the error contract is under test.
type stubCall struct {
	name string
	call func(context.Context, *linux.SystemAdapter) error
}

func stubCalls() []stubCall {
	return []stubCall{
		{
			name: "FocusedApplicationPID",
			call: func(ctx context.Context, a *linux.SystemAdapter) error {
				_, err := a.FocusedApplicationPID(ctx)

				return err
			},
		},
		{
			name: "ScreenBounds",
			call: func(ctx context.Context, a *linux.SystemAdapter) error {
				_, err := a.ScreenBounds(ctx)

				return err
			},
		},
		{
			name: "ScreenBoundsByName",
			call: func(ctx context.Context, a *linux.SystemAdapter) error {
				_, _, err := a.ScreenBoundsByName(ctx, "DP-1")

				return err
			},
		},
		{
			name: "ScreenNames",
			call: func(ctx context.Context, a *linux.SystemAdapter) error {
				_, err := a.ScreenNames(ctx)

				return err
			},
		},
		{
			name: "FocusedWindowBounds",
			call: func(ctx context.Context, a *linux.SystemAdapter) error {
				_, _, err := a.FocusedWindowBounds(ctx)

				return err
			},
		},
		{
			name: "CursorPosition",
			call: func(ctx context.Context, a *linux.SystemAdapter) error {
				_, err := a.CursorPosition(ctx)

				return err
			},
		},
	}
}

// TestSystemAdapter_UnimplementedBackendReportsNotSupported pins the stub
// contract: on a backend with no implementation, every method reports
// CodeNotSupported. Callers branch on IsNotSupported to degrade — a bare error
// would surface as a real failure, and a nil error with a zero value would
// have hints placed at (0,0) on a 0x0 screen. The adapter is built with an
// unrecognized backend name, exactly what a new compositor produces, so
// nothing needs to be running. Implementing a method should fail its case
// here and prompt a capability-matrix update.
func TestSystemAdapter_UnimplementedBackendReportsNotSupported(t *testing.T) {
	adapter := linux.NewSystemAdapter(unimplementedBackend)
	ctx := context.Background()

	for _, testCase := range stubCalls() {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.call(ctx, adapter)
			if err == nil {
				t.Fatalf(
					"%s on an unimplemented backend returned nil; callers cannot tell the "+
						"feature is missing and will act on a zero value",
					testCase.name,
				)
			}

			if !derrors.IsNotSupported(err) {
				t.Errorf(
					"%s returned %v (code %q), want CodeNotSupported so callers can degrade "+
						"via derrors.IsNotSupported",
					testCase.name, err, derrors.GetCode(err),
				)
			}
		})
	}
}

// TestSystemAdapter_NotSupportedErrorNamesTheBackend checks the message carries
// the backend name. This is the string a user sees in `neru doctor` output and
// in logs; without it, "not yet implemented on linux backend" gives no clue
// which compositor was detected, which is the first thing needed to act on it.
func TestSystemAdapter_NotSupportedErrorNamesTheBackend(t *testing.T) {
	adapter := linux.NewSystemAdapter(unimplementedBackend)
	ctx := context.Background()

	for _, testCase := range stubCalls() {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.call(ctx, adapter)
			if err == nil {
				t.Fatalf("%s returned nil, expected a NotSupported error", testCase.name)
			}

			if !strings.Contains(err.Error(), unimplementedBackend) {
				t.Errorf("%s error %q does not name the backend %q",
					testCase.name, err.Error(), unimplementedBackend)
			}
		})
	}
}

// TestSystemAdapter_StubsDoNotPanicOnACanceledContext makes sure the stub path
// is reached without touching the context in a way that panics. Config reload
// and shutdown both cancel mid-flight, so these are called with dead contexts
// in practice.
func TestSystemAdapter_StubsDoNotPanicOnACanceledContext(t *testing.T) {
	adapter := linux.NewSystemAdapter(unimplementedBackend)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for _, testCase := range stubCalls() {
		t.Run(testCase.name, func(t *testing.T) {
			// A panic here fails the subtest; the assertion is that some error
			// comes back rather than a zero value with nil.
			err := testCase.call(ctx, adapter)
			if err == nil {
				t.Errorf("%s with a canceled context returned nil", testCase.name)
			}
		})
	}
}

// TestSystemAdapter_PlatformLabelReflectsTheBackend pins the label used in
// startup notices and diagnostics, including the empty-backend fallback.
func TestSystemAdapter_PlatformLabelReflectsTheBackend(t *testing.T) {
	tests := []struct {
		backend string
		want    string
	}{
		{unimplementedBackend, "linux/" + unimplementedBackend},
		{x11Backend, "linux/" + x11Backend},
		{wlrootsBackend, "linux/" + wlrootsBackend},
		{"", "linux"},
	}

	for _, testCase := range tests {
		t.Run(testCase.want, func(t *testing.T) {
			got := linux.NewSystemAdapter(testCase.backend).PlatformLabel()
			if got != testCase.want {
				t.Errorf("PlatformLabel() with backend %q = %q, want %q",
					testCase.backend, got, testCase.want)
			}
		})
	}
}

// TestSystemAdapter_CapabilitiesCarryTheBackendSuffix checks the capability
// surface reports which backend is live, since that is what makes `neru doctor`
// actionable on Linux.
func TestSystemAdapter_CapabilitiesCarryTheBackendSuffix(t *testing.T) {
	capabilities := linux.NewSystemAdapter(unimplementedBackend).Capabilities()

	want := "linux/" + unimplementedBackend
	if capabilities.Platform != want {
		t.Errorf("Capabilities().Platform = %q, want %q", capabilities.Platform, want)
	}

	// An adapter with no detected backend still reports the bare platform
	// rather than a dangling separator.
	if got := linux.NewSystemAdapter("").Capabilities().Platform; got != "linux" {
		t.Errorf("Capabilities().Platform with no backend = %q, want %q", got, "linux")
	}
}

// TestSystemAdapter_CapabilitiesMatchBackendBehavior is the Linux half of the
// capability cross-check, built directly for a named backend so it runs with
// no display or compositor — the platform-level test skips headless. The two
// directions are asymmetric because capabilities downgrade on any probe
// failure: supported must not answer CodeNotSupported, and stub must return
// some non-nil error — a runtime failure on an implemented backend counts,
// since only a nil-error zero value would break the surface.
func TestSystemAdapter_CapabilitiesMatchBackendBehavior(t *testing.T) {
	// Every backend name that reaches the dispatch, including ones with no
	// implementation, so both sides of the contract are exercised.
	backends := []string{unimplementedBackend, "", x11Backend, wlrootsBackend, kdeBackend}

	ctx := context.Background()

	for _, backend := range backends {
		name := backend
		if name == "" {
			name = "(undetected)"
		}

		t.Run(name, func(t *testing.T) {
			adapter := linux.NewSystemAdapter(backend)
			capabilities := adapter.Capabilities()

			checks := []struct {
				key      string
				declared func() bool
				call     func() error
			}{
				{
					key:      "process",
					declared: func() bool { return capabilities.Process.Supported() },
					call: func() error {
						_, err := adapter.FocusedApplicationPID(ctx)

						return err
					},
				},
				{
					key:      "screen",
					declared: func() bool { return capabilities.Screen.Supported() },
					call: func() error {
						_, err := adapter.ScreenBounds(ctx)

						return err
					},
				},
				{
					key:      "cursor",
					declared: func() bool { return capabilities.Cursor.Supported() },
					call: func() error {
						_, err := adapter.CursorPosition(ctx)

						return err
					},
				},
			}

			for _, check := range checks {
				err := check.call()
				notSupported := derrors.IsNotSupported(err)

				switch {
				case check.declared() && notSupported:
					t.Errorf(
						"backend %q declares %q supported but the adapter returned "+
							"CodeNotSupported: %v",
						backend, check.key, err,
					)

				case !check.declared() && err == nil:
					// A stubbed capability that silently succeeds breaks caller
					// fallback: the caller acts on a zero value it believes is
					// real. A stub may report CodeNotSupported (unimplemented) or
					// a live failure (an implemented backend wedged at runtime) —
					// both are honest — but it must never return nil.
					t.Errorf(
						"backend %q declares %q stubbed but the adapter succeeded; "+
							"a stub must return an error rather than a zero value with nil",
						backend, check.key,
					)
				}
			}
		})
	}
}

// TestSystemAdapter_DowngradedCapabilitiesExplainWhy checks that when a
// capability is downgraded from the static Linux preset, the detail says what
// to do about it. "stub" alone tells a user their feature is missing but not
// whether a CGO build, a different compositor, or nothing at all would fix it.
func TestSystemAdapter_DowngradedCapabilitiesExplainWhy(t *testing.T) {
	capabilities := linux.NewSystemAdapter(unimplementedBackend).Capabilities()

	downgraded := []struct {
		key  string
		read func() (string, bool)
	}{
		{"process", func() (string, bool) {
			return capabilities.Process.Detail, capabilities.Process.Supported()
		}},
		{"screen", func() (string, bool) {
			return capabilities.Screen.Detail, capabilities.Screen.Supported()
		}},
		{"cursor", func() (string, bool) {
			return capabilities.Cursor.Detail, capabilities.Cursor.Supported()
		}},
	}

	for _, entry := range downgraded {
		detail, supported := entry.read()

		if supported {
			t.Errorf("capability %q is reported supported on an unimplemented backend", entry.key)

			continue
		}

		if detail == "" {
			t.Errorf("capability %q was downgraded with no explanation", entry.key)

			continue
		}

		// The detail must name the backend or the missing prerequisite, not
		// merely restate that the feature is unavailable.
		if !strings.Contains(detail, unimplementedBackend) &&
			!strings.Contains(detail, "CGO") {
			t.Errorf(
				"capability %q detail %q neither names the backend nor the missing "+
					"prerequisite, so it is not actionable",
				entry.key, detail,
			)
		}
	}
}

// TestSystemAdapter_FocusedWindowBoundsRefusesWithNoGeometrySource extends the
// stub contract to a backend that is otherwise implemented.
//
// River and Wayfire are wlroots compositors with no IPC Neru can query, and
// answering "no focused window" there is indistinguishable from a desktop with
// nothing focused — so every caller silently scoped itself to the whole active
// screen and nothing said why. Refusing is what lets a caller tell the two
// apart, and the message has to name the backend for the same reason every
// other refusal here does.
func TestSystemAdapter_FocusedWindowBoundsRefusesWithNoGeometrySource(t *testing.T) {
	// A compositor socket inherited from another session must not stand in for
	// a source this one has.
	for _, name := range []string{"NIRI_SOCKET", "SWAYSOCK", "HYPRLAND_INSTANCE_SIGNATURE"} {
		t.Setenv(name, "")
	}

	adapter := linux.NewSystemAdapter(wlrootsBackend)

	_, found, err := adapter.FocusedWindowBounds(context.Background())
	if found {
		t.Fatal("FocusedWindowBounds reported a window with no compositor to ask")
	}

	if !derrors.IsNotSupported(err) {
		t.Fatalf("FocusedWindowBounds error = %v (code %q), want CodeNotSupported so "+
			"callers can tell a missing source from an unfocused desktop",
			err, derrors.GetCode(err))
	}

	if !strings.Contains(err.Error(), wlrootsBackend) {
		t.Errorf("error %q does not name the backend %q", err.Error(), wlrootsBackend)
	}
}

// TestSystemAdapter_StartsNoCompositorBridgeOffKDE pins the other half of
// #1430 for the platform adapter: building an adapter for a session KWin does
// not run must touch nothing outside the process. The KDE adapter installs a
// KWin script at construction so the first caller is not answered from a cold
// cache; every other backend must leave $XDG_RUNTIME_DIR alone.
func TestSystemAdapter_StartsNoCompositorBridgeOffKDE(t *testing.T) {
	for _, backend := range []string{x11Backend, wlrootsBackend, unimplementedBackend} {
		t.Run(backend, func(t *testing.T) {
			runtimeDir := t.TempDir()
			t.Setenv("XDG_RUNTIME_DIR", runtimeDir)

			linux.NewSystemAdapter(backend)

			entries, readErr := os.ReadDir(runtimeDir)
			if readErr != nil {
				t.Fatalf("ReadDir(%s) error = %v", runtimeDir, readErr)
			}

			for _, entry := range entries {
				t.Errorf(
					"building the %s adapter wrote %s into XDG_RUNTIME_DIR; no compositor "+
						"bridge may start on a backend that did not identify it",
					backend, entry.Name(),
				)
			}
		})
	}
}

// TestSystemAdapter_MoveCursorToPointReportsNotSupported covers the one mutating
// method worth pinning here. It is safe on an unimplemented backend precisely
// because nothing is wired up to move — which is the behavior being asserted.
func TestSystemAdapter_MoveCursorToPointReportsNotSupported(t *testing.T) {
	adapter := linux.NewSystemAdapter(unimplementedBackend)

	err := adapter.MoveCursorToPoint(context.Background(), image.Point{X: 10, Y: 10}, true)
	if err == nil {
		t.Fatal("MoveCursorToPoint on an unimplemented backend returned nil")
	}

	if !derrors.IsNotSupported(err) {
		t.Errorf("MoveCursorToPoint returned %v (code %q), want CodeNotSupported",
			err, derrors.GetCode(err))
	}
}
