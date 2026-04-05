package app_test

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	"github.com/y3owk1n/neru/internal/core/ports"
	portmocks "github.com/y3owk1n/neru/internal/core/ports/mocks"
)

func TestIPCController_StatusIncludesCapabilities(t *testing.T) {
	cfg := config.DefaultConfig()
	appState := state.NewAppState()
	logger := zap.NewNop()
	configService := config.NewService(cfg, "", logger, nil)
	system := &portmocks.SystemMock{
		CapabilitiesFunc: func() ports.PlatformCapabilities {
			return ports.PlatformCapabilities{
				Platform: "testos",
				Overlay: ports.FeatureCapability{
					Status: ports.FeatureStatusStub,
					Detail: "not implemented",
				},
			}
		},
	}

	controller := app.NewIPCController(
		nil,
		nil,
		nil,
		nil,
		configService,
		appState,
		cfg,
		nil,
		system,
		nil,
		nil,
		nil,
		logger,
	)

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

	if capabilities["platform"] != "testos" {
		t.Fatalf("platform = %v, want testos", capabilities["platform"])
	}

	if capabilities["overlay"] != string(ports.FeatureStatusStub) {
		t.Fatalf("overlay capability = %v, want stub", capabilities["overlay"])
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
	configService := config.NewService(cfg, "", logger, nil)
	system := &portmocks.SystemMock{
		CapabilitiesFunc: func() ports.PlatformCapabilities {
			return ports.PlatformCapabilities{
				Platform: "testos",
				Process: ports.FeatureCapability{
					Status: ports.FeatureStatusStub,
					Detail: "not implemented",
				},
			}
		},
	}

	controller := app.NewIPCController(
		nil,
		nil,
		nil,
		nil,
		configService,
		appState,
		cfg,
		nil,
		system,
		nil,
		nil,
		nil,
		logger,
	)

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
