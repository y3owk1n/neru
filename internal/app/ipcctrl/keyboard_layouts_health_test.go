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
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// XKB layout names the cases list.
const (
	layoutUS      = "English (US)"
	layoutRussian = "Russian"
	layoutDvorak  = "English (Dvorak)"
)

// layoutReportingTap is an event tap that lists keyboard layouts, as every
// platform's tap does.
type layoutReportingTap struct {
	portmocks.MockEventTapPort

	names     []string
	reference string
}

func (tap *layoutReportingTap) KeyboardLayouts() ([]string, string) {
	return tap.names, tap.reference
}

// TestIPCController_HealthListsKeyboardLayouts pins the keyboard_layouts row of
// neru doctor: the layouts a user can force, the one keys use, and a
// general.kb_layout_to_use naming none of them reported as unhealthy.
func TestIPCController_HealthListsKeyboardLayouts(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		layouts   []string
		reference string
		requested string
		want      string
		healthy   bool
	}{
		{
			name:      "the layouts and the one keys use",
			layouts:   []string{layoutUS, layoutRussian},
			reference: layoutUS,
			want:      "ok (English (US), Russian; keys use English (US))",
			healthy:   true,
		},
		{
			name:    "keys in whichever layout is active",
			layouts: []string{layoutRussian, "Greek"},
			want:    "ok (Russian, Greek; keys use the active layout)",
			healthy: true,
		},
		{
			name:    "a keymap not read yet",
			want:    "ok (no keyboard layouts read yet)",
			healthy: true,
		},
		{
			name:      "a forced layout with no keymap read yet",
			requested: layoutDvorak,
			want:      "unverified: English (Dvorak) (no keyboard layouts read yet)",
			healthy:   true,
		},
		{
			name:      "a forced layout matched regardless of case",
			layouts:   []string{layoutUS, layoutDvorak},
			reference: layoutDvorak,
			requested: "english (dvorak)",
			want:      "ok (English (US), English (Dvorak); keys use English (Dvorak))",
			healthy:   true,
		},
		{
			name:      "a forced layout that is not there",
			layouts:   []string{layoutUS, layoutRussian},
			reference: layoutUS,
			requested: "Klingon",
			want:      "not found: Klingon (layouts: English (US), Russian)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			cfg.General.KBLayoutToUse = test.requested

			logger := zap.NewNop()
			controller := ipcctrl.New(ipcctrl.Deps{
				ConfigService: loader.NewService(cfg, "", logger, nil),
				AppState:      state.NewAppState(),
				Config:        cfg,
				System:        &portmocks.MockSystemPort{},
				Logger:        logger,
			})
			controller.SetInfrastructure(
				&layoutReportingTap{names: test.layouts, reference: test.reference},
				&portmocks.MockIPCPort{},
			)

			resp := controller.HandleCommand(
				context.Background(),
				ipc.Command{Action: domain.CommandHealth},
			)

			healthData, _ := resp.Data.(map[string]any)
			components, _ := healthData["components"].(map[string]string)

			if got := components["keyboard_layouts"]; got != test.want {
				t.Errorf("keyboard_layouts = %q, want %q", got, test.want)
			}

			if !test.healthy && resp.Success {
				t.Errorf(
					"health succeeded with kb_layout_to_use = %q, which no layout matches",
					test.requested,
				)
			}
		})
	}
}
