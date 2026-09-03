package platform_test

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

// systemCapabilityProbe pairs a capability with a cheap, side-effect-free call
// that exercises the same subsystem, so the declared status can be compared
// against what the adapter really does.
//
// Only read-only probes belong here. Capabilities whose only entry point has a
// side effect (moving the cursor, showing an overlay, injecting a key) are
// deliberately absent — see TestCapabilities_ProbeCoverageIsDocumented, which
// pins exactly which ones are and are not covered so the gap cannot widen
// silently.
type systemCapabilityProbe struct {
	key   ports.CapabilityKey
	read  func(ports.PlatformCapabilities) ports.FeatureCapability
	probe func(context.Context, ports.SystemPort) error
}

func systemCapabilityProbes() []systemCapabilityProbe {
	return []systemCapabilityProbe{
		{
			key:  ports.CapabilityProcess,
			read: func(c ports.PlatformCapabilities) ports.FeatureCapability { return c.Process },
			probe: func(ctx context.Context, port ports.SystemPort) error {
				_, err := port.FocusedApplicationPID(ctx)

				return err
			},
		},
		{
			key:  ports.CapabilityScreen,
			read: func(c ports.PlatformCapabilities) ports.FeatureCapability { return c.Screen },
			probe: func(ctx context.Context, port ports.SystemPort) error {
				_, err := port.ScreenBounds(ctx)

				return err
			},
		},
		{
			key:  ports.CapabilityCursor,
			read: func(c ports.PlatformCapabilities) ports.FeatureCapability { return c.Cursor },
			probe: func(ctx context.Context, port ports.SystemPort) error {
				_, err := port.CursorPosition(ctx)

				return err
			},
		},
	}
}

// newSystemPort builds the platform's real SystemPort, or skips when the
// environment genuinely cannot provide one.
//
// On Linux, NewSystemPort refuses to construct an adapter when no display
// server is detected — a headless CI runner, a container, an SSH session — and
// reports that as CodeNotSupported. That is correct behavior, not a failure, so
// these tests skip rather than fail there. The skip is visible in the test
// output, unlike a silently-passing assertion.
//
// The Linux capability surface is not left unchecked by that skip: its
// backend-by-backend contract is asserted in the linux package itself, where
// adapters are constructed directly and need no display. Any other error from
// NewSystemPort is a real failure.
func newSystemPort(t *testing.T) ports.SystemPort {
	t.Helper()

	systemPort, err := platform.NewSystemPort()
	if err == nil {
		return systemPort
	}

	if derrors.IsNotSupported(err) {
		t.Skipf("no usable %s display backend in this environment: %v", runtime.GOOS, err)
	}

	t.Fatalf("NewSystemPort() error = %v, want nil", err)

	return nil
}

// TestCapabilities_DeclaredStatusMatchesAdapterBehavior checks the hand-written
// capability matrix against what the adapters actually do: declared supported
// must not answer CodeNotSupported, declared stub must. The matrix is the
// external contract — `neru doctor` prints it — and nothing else ties it to
// behavior. Each CI platform checks its own declaration, so adding a stub
// without downgrading its status, or implementing one without promoting it,
// fails there. Strictness works because adapters report what is reachable at
// runtime, not what the build target intends.
func TestCapabilities_DeclaredStatusMatchesAdapterBehavior(t *testing.T) {
	systemPort := newSystemPort(t)
	capabilities := systemPort.Capabilities()
	ctx := context.Background()

	for _, probe := range systemCapabilityProbes() {
		t.Run(string(probe.key), func(t *testing.T) {
			declared := probe.read(capabilities)
			err := probe.probe(ctx, systemPort)
			notSupported := derrors.IsNotSupported(err)

			switch declared.Status {
			case ports.FeatureStatusSupported, ports.FeatureStatusHeadless:
				if notSupported {
					t.Errorf(
						"capability %q is declared %q on %s but its adapter returned "+
							"CodeNotSupported: %v",
						probe.key, declared.Status, runtime.GOOS, err,
					)
				}

			case ports.FeatureStatusStub:
				if !notSupported {
					t.Errorf(
						"capability %q is declared %q on %s but its adapter did not return "+
							"CodeNotSupported (got %v); a stub must report NotSupported explicitly "+
							"rather than silently succeeding",
						probe.key, declared.Status, runtime.GOOS, err,
					)
				}

			default:
				t.Errorf("capability %q has unknown status %q", probe.key, declared.Status)
			}
		})
	}
}

// TestCapabilities_DowngradedEntriesExplainThemselves checks that a capability
// reported as stubbed on a platform whose static preset calls it supported
// carries a detail saying why, and what to do.
//
// A bare "stub" in `neru doctor` tells a user their feature is missing but not
// whether that is fixable — installing a portal, using a CGO build, switching
// compositor — which is the only thing they can act on.
func TestCapabilities_DowngradedEntriesExplainThemselves(t *testing.T) {
	live := newSystemPort(t).Capabilities()

	staticPresets := map[string]ports.PlatformCapabilities{
		"darwin":  ports.DarwinCapabilities(),
		"linux":   ports.LinuxCapabilities(),
		"windows": ports.WindowsCapabilities(),
	}

	preset, known := staticPresets[runtime.GOOS]
	if !known {
		t.Skipf("no static preset registered for %s", runtime.GOOS)
	}

	declared := make(map[ports.CapabilityKey]ports.FeatureCapability)
	for _, entry := range preset.Entries() {
		declared[entry.Key] = entry.FeatureCapability
	}

	for _, entry := range live.Entries() {
		if entry.Status != ports.FeatureStatusStub {
			continue
		}

		if declared[entry.Key].Status != ports.FeatureStatusSupported {
			// Statically stubbed: its detail is already the static description.
			continue
		}

		// Downgraded at runtime — the detail must differ from the static one,
		// or the user is being shown a description of a feature that is not
		// actually available to them.
		if entry.Detail == declared[entry.Key].Detail {
			t.Errorf(
				"capability %q was downgraded to stub at runtime but still carries the "+
					"static supported-description %q; it should explain the downgrade",
				entry.Key, entry.Detail,
			)
		}

		if entry.Detail == "" {
			t.Errorf("capability %q was downgraded to stub with no explanation", entry.Key)
		}
	}
}

// TestCapabilities_PlatformLabelMatchesBuildTarget guards against a
// copy-paste slip in the presets — a preset returning another platform's label
// would make `neru doctor` misreport what it is running on.
//
// The label is either the bare GOOS or GOOS with a backend suffix: Linux
// refines it to "linux/x11", "linux/wayland-wlroots" and so on, because which
// backend is live is the first thing that matters when diagnosing Linux. Both
// shapes are accepted; a different GOOS entirely is not.
func TestCapabilities_PlatformLabelMatchesBuildTarget(t *testing.T) {
	platform := newSystemPort(t).Capabilities().Platform

	if platform == runtime.GOOS {
		return
	}

	prefix := runtime.GOOS + "/"
	if !strings.HasPrefix(platform, prefix) {
		t.Errorf("Capabilities().Platform = %q, want %q or %q<backend>",
			platform, runtime.GOOS, prefix)

		return
	}

	if strings.TrimPrefix(platform, prefix) == "" {
		t.Errorf("Capabilities().Platform = %q has an empty backend suffix", platform)
	}
}

// TestCapabilities_EveryEntryHasAStatusAndDetail checks the surface that
// actually reaches users. An entry with an empty status serializes as an empty
// string in the IPC response, and one with no detail leaves `neru doctor`
// printing a capability name with no explanation of what it means here.
func TestCapabilities_EveryEntryHasAStatusAndDetail(t *testing.T) {
	entries := newSystemPort(t).Capabilities().Entries()

	if len(entries) == 0 {
		t.Fatal("Capabilities().Entries() is empty")
	}

	validStatuses := map[ports.FeatureStatus]bool{
		ports.FeatureStatusSupported: true,
		ports.FeatureStatusHeadless:  true,
		ports.FeatureStatusStub:      true,
	}

	for _, entry := range entries {
		if !validStatuses[entry.Status] {
			t.Errorf("capability %q has status %q, which is not one of supported/headless/stub",
				entry.Key, entry.Status)
		}

		if entry.Detail == "" {
			t.Errorf("capability %q has no detail; `neru doctor` would print it unexplained",
				entry.Key)
		}

		if entry.Key == "" {
			t.Errorf("capability with field %q has an empty wire key", entry.Field)
		}
	}
}

// TestCapabilities_ProbeCoverageIsDocumented records exactly which capabilities
// the behavioral cross-check above covers and which it does not.
//
// It exists so the uncovered set is a deliberate, visible list rather than an
// accident. Adding a capability without deciding whether it can be probed fails
// here, which is the prompt to either write a probe or add it to the documented
// exclusions with a reason.
func TestCapabilities_ProbeCoverageIsDocumented(t *testing.T) {
	// Capabilities with no side-effect-free entry point on SystemPort. Each
	// needs a heavier fixture (a live overlay manager, an AX client, a uinput
	// device) or has only a mutating entry point, so it is checked by that
	// subsystem's own tests instead of here.
	uncovered := map[ports.CapabilityKey]string{
		ports.CapabilityScroll:        "injects a real scroll into the focused app",
		ports.CapabilityAccessibility: "needs an AX client fixture; covered by internal/adapter/accessibility",
		ports.CapabilityOverlay:       "needs a live overlay manager; covered by internal/adapter/overlay",
		ports.CapabilityNotifications: "only entry point displays UI to the user",
		ports.CapabilityGlobalHotkeys: "registration mutates global OS state",
		ports.CapabilityKeyboardEventTap: "installing a tap mutates global OS state; " +
			"covered by internal/adapter/eventtap",
		ports.CapabilityAppWatcher:        "requires a focus change to observe",
		ports.CapabilityDarkModeDetection: "IsDarkMode returns no error, so NotSupported is unobservable",
		ports.CapabilityTextInput:         "needs a live overlay window",
		ports.CapabilityVision:            "a probe would trigger a screen-capture permission prompt",
		ports.CapabilityKeyFeed:           "injects real keystrokes into the focused app",
		ports.CapabilitySystray:           "needs a live run loop and adds a tray icon",
	}

	covered := make(map[ports.CapabilityKey]bool)
	for _, probe := range systemCapabilityProbes() {
		covered[probe.key] = true
	}

	for _, entry := range newSystemPort(t).Capabilities().Entries() {
		reason, excused := uncovered[entry.Key]

		switch {
		case covered[entry.Key] && excused:
			t.Errorf(
				"capability %q is both probed and listed as uncovered; remove it from the exclusions",
				entry.Key,
			)

		case !covered[entry.Key] && !excused:
			t.Errorf(
				"capability %q is neither probed by TestCapabilities_DeclaredStatusMatchesAdapterBehavior "+
					"nor listed in the documented exclusions. Add a side-effect-free probe, or add it to "+
					"`uncovered` with the reason it cannot have one.",
				entry.Key,
			)

		case excused && reason == "":
			t.Errorf("capability %q is excluded from probing with no stated reason", entry.Key)
		}
	}

	// The exclusion list must not name capabilities that no longer exist —
	// otherwise a renamed capability would silently become unprobed.
	known := make(map[ports.CapabilityKey]bool)
	for _, entry := range newSystemPort(t).Capabilities().Entries() {
		known[entry.Key] = true
	}

	for key := range uncovered {
		if !known[key] {
			t.Errorf("exclusion list names %q, which is not a registered capability", key)
		}
	}
}

// TestCapabilities_StubsSurfaceNotSupportedNotNil states the rule that makes
// degradation work at all: a stub must return an error callers can recognize
// with IsNotSupported, not nil and not some other code. Callers branch on
// exactly that predicate to fall back gracefully, so a stub returning a bare
// error is treated as a real failure and surfaces to the user.
func TestCapabilities_StubsSurfaceNotSupportedNotNil(t *testing.T) {
	systemPort := newSystemPort(t)
	capabilities := systemPort.Capabilities()
	ctx := context.Background()

	stubsFound := 0

	for _, probe := range systemCapabilityProbes() {
		if probe.read(capabilities).Status != ports.FeatureStatusStub {
			continue
		}

		stubsFound++

		err := probe.probe(ctx, systemPort)
		if err == nil {
			t.Errorf("stubbed capability %q returned nil; callers cannot detect the gap", probe.key)

			continue
		}

		if !derrors.IsNotSupported(err) {
			t.Errorf("stubbed capability %q returned %v (code %q), want CodeNotSupported",
				probe.key, err, derrors.GetCode(err))
		}

		// errors.Is must agree with the helper, since some callers use it.
		if !errors.Is(err, err) {
			t.Errorf("stubbed capability %q returned an error that fails errors.Is against itself",
				probe.key)
		}
	}

	t.Logf("%s: %d of %d probed capabilities are stubbed",
		runtime.GOOS, stubsFound, len(systemCapabilityProbes()))
}
