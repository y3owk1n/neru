package ipcctrl_test

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/ipcctrl"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

const testOS = "testos"

func TestIPCController_StatusIncludesCapabilities(t *testing.T) {
	cfg := config.DefaultConfig()
	appState := state.NewAppState()
	logger := zap.NewNop()
	configService := loader.NewService(cfg, "", logger, nil)
	system := &portmocks.MockSystemPort{
		CapabilitiesFunc: func() ports.PlatformCapabilities {
			return ports.PlatformCapabilities{
				Platform: testOS,
				Overlay: ports.FeatureCapability{
					Status: ports.FeatureStatusStub,
					Detail: "not implemented",
				},
			}
		},
	}

	controller := ipcctrl.New(ipcctrl.Deps{
		ConfigService: configService,
		AppState:      appState,
		Config:        cfg,
		System:        system,
		Logger:        logger,
	})

	resp := controller.HandleCommand(
		context.Background(),
		ipc.Command{Action: domain.CommandStatus},
	)
	if !resp.Success {
		t.Fatalf("HandleCommand(status) success = false, want true")
	}

	statusData, statusDataOK := resp.Data.(map[string]any)
	if !statusDataOK {
		t.Fatalf("status data type = %T, want map[string]any", resp.Data)
	}

	capabilities, capabilitiesOK := statusData["capabilities"].(map[string]any)
	if !capabilitiesOK {
		t.Fatalf("capabilities type = %T, want map[string]any", statusData["capabilities"])
	}

	profile, profileOK := statusData["profile"].(map[string]any)
	if !profileOK {
		t.Fatalf("profile type = %T, want map[string]any", statusData["profile"])
	}

	if capabilities["platform"] != testOS {
		t.Fatalf("platform = %v, want testos", capabilities["platform"])
	}

	if capabilities["overlay"] != string(ports.FeatureStatusStub) {
		t.Fatalf("overlay capability = %v, want stub", capabilities["overlay"])
	}

	// The IPC response must carry every registered capability, not a
	// hand-maintained subset. This is the seam that previously drifted: the
	// Windows doctor listed eight of ten capabilities and nothing noticed.
	for _, entry := range (ports.PlatformCapabilities{}).Entries() {
		if _, present := capabilities[string(entry.Key)]; !present {
			t.Errorf(
				"capability %q (field %s) is missing from the status response; "+
					"capabilitiesMap must iterate PlatformCapabilities.Entries()",
				entry.Key,
				entry.Field,
			)
		}
	}

	primaryMod, primaryModOK := profile["primary_modifier"].(string)
	if !primaryModOK || primaryMod == "" {
		t.Fatalf(
			"profile.primary_modifier = %v (%T), want non-empty string",
			profile["primary_modifier"],
			profile["primary_modifier"],
		)
	}

	displayServer, displayServerOK := profile["display_server"].(string)
	if !displayServerOK || displayServer == "" {
		t.Fatalf(
			"profile.display_server = %v (%T), want non-empty string",
			profile["display_server"],
			profile["display_server"],
		)
	}
}

func TestIPCController_HealthMarksStubCapabilitiesUnhealthy(t *testing.T) {
	cfg := config.DefaultConfig()
	appState := state.NewAppState()
	logger := zap.NewNop()
	configService := loader.NewService(cfg, "", logger, nil)
	system := &portmocks.MockSystemPort{
		CapabilitiesFunc: func() ports.PlatformCapabilities {
			return ports.PlatformCapabilities{
				Platform: testOS,
				Process: ports.FeatureCapability{
					Status: ports.FeatureStatusStub,
					Detail: "not implemented",
				},
			}
		},
	}

	controller := ipcctrl.New(ipcctrl.Deps{
		ConfigService: configService,
		AppState:      appState,
		Config:        cfg,
		System:        system,
		Logger:        logger,
	})

	resp := controller.HandleCommand(
		context.Background(),
		ipc.Command{Action: domain.CommandHealth},
	)
	if resp.Success {
		t.Fatalf("HandleCommand(health) success = true, want false")
	}

	healthData, healthDataOK := resp.Data.(map[string]any)
	if !healthDataOK {
		t.Fatalf("health data type = %T, want map[string]any", resp.Data)
	}

	components, componentsOK := healthData["components"].(map[string]string)
	if !componentsOK {
		t.Fatalf("components type = %T, want map[string]string", healthData["components"])
	}

	profile, profileOK := healthData["profile"].(map[string]any)
	if !profileOK {
		t.Fatalf("profile type = %T, want map[string]any", healthData["profile"])
	}

	if components["capability.process"] != string(ports.FeatureStatusStub) {
		t.Fatalf(
			"capability.process = %v, want stub",
			components["capability.process"],
		)
	}

	profileOS, profileOSOK := profile["os"].(string)
	if !profileOSOK || profileOS == "" {
		t.Fatalf("profile.os = %v (%T), want non-empty string", profile["os"], profile["os"])
	}
}

// TestIPCController_StatusCarriesTheFocusedAppReason checks that a stubbed
// process capability ships its reason, not just its status.
//
// On Linux the capability is live-probed, so "stub" is the answer both when
// focused-app inspection is genuinely missing and when the desktop merely has
// nothing focused. Those need opposite things from the user — nothing, or a
// working display server — and the status alone says neither. A detail written
// but never serialized is the failure mode this pins: the string existed and
// `neru doctor` printed the bare status regardless.
func TestIPCController_StatusCarriesTheFocusedAppReason(t *testing.T) {
	const reason = "focused-app inspection found no focused window on linux backend x11"

	cfg := config.DefaultConfig()
	logger := zap.NewNop()
	system := &portmocks.MockSystemPort{
		CapabilitiesFunc: func() ports.PlatformCapabilities {
			return ports.PlatformCapabilities{
				Platform: testOS,
				Process: ports.FeatureCapability{
					Status: ports.FeatureStatusStub,
					Detail: reason,
				},
			}
		},
	}

	controller := ipcctrl.New(ipcctrl.Deps{
		ConfigService: loader.NewService(cfg, "", logger, nil),
		AppState:      state.NewAppState(),
		Config:        cfg,
		System:        system,
		Logger:        logger,
	})

	resp := controller.HandleCommand(
		context.Background(),
		ipc.Command{Action: domain.CommandStatus},
	)

	statusData, statusDataOK := resp.Data.(map[string]any)
	if !statusDataOK {
		t.Fatalf("status data type = %T, want map[string]any", resp.Data)
	}

	capabilities, capabilitiesOK := statusData["capabilities"].(map[string]any)
	if !capabilitiesOK {
		t.Fatalf("capabilities type = %T, want map[string]any", statusData["capabilities"])
	}

	if got := capabilities["process_detail"]; got != reason {
		t.Errorf("process_detail = %v, want %q; without it `neru doctor` prints a bare "+
			"\"stub\" and an unfocused desktop is indistinguishable from a broken one",
			got, reason)
	}
}

// TestIPCController_StatusOmitsTheFocusedAppReasonWhenSupported is the other
// half: the sibling key is a reason a capability did not answer, so a working
// one must not carry a line explaining itself.
func TestIPCController_StatusOmitsTheFocusedAppReasonWhenSupported(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := zap.NewNop()
	system := &portmocks.MockSystemPort{
		CapabilitiesFunc: func() ports.PlatformCapabilities {
			return ports.PlatformCapabilities{
				Platform: testOS,
				Process: ports.FeatureCapability{
					Status: ports.FeatureStatusSupported,
					Detail: "_NET_ACTIVE_WINDOW / WM_CLASS",
				},
			}
		},
	}

	controller := ipcctrl.New(ipcctrl.Deps{
		ConfigService: loader.NewService(cfg, "", logger, nil),
		AppState:      state.NewAppState(),
		Config:        cfg,
		System:        system,
		Logger:        logger,
	})

	resp := controller.HandleCommand(
		context.Background(),
		ipc.Command{Action: domain.CommandStatus},
	)

	statusData, _ := resp.Data.(map[string]any)

	capabilities, capabilitiesOK := statusData["capabilities"].(map[string]any)
	if !capabilitiesOK {
		t.Fatalf("capabilities type = %T, want map[string]any", statusData["capabilities"])
	}

	if got, present := capabilities["process_detail"]; present {
		t.Errorf("process_detail = %v on a supported capability; the sibling key exists "+
			"to explain a capability that did not answer", got)
	}
}
