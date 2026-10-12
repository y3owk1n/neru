package app_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/domain"
)

// TestSimulation_ConfigSetRefusesABadKeyOrValueAsInvalidInput pins that a
// mistyped key or a value that cannot be read is the caller's to fix on both
// paths of config set: it answers ERR_INVALID_INPUT whether or not the change
// is applied at once.
func TestSimulation_ConfigSetRefusesABadKeyOrValueAsInvalidInput(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	for _, args := range [][]string{
		{"hints.no_such_option", "1"},
		{"hints.no_such_option", "1", "--no-reload"},
		{"grid.enabled", "sometimes"},
		{"grid.enabled", "sometimes", "--no-reload"},
	} {
		resp := sim.app.HandleCommand(
			context.Background(),
			ipc.Command{Action: domain.CommandConfigSet, Args: args},
		)
		if resp.Success || resp.Code != ipc.CodeInvalidInput {
			t.Errorf(
				"config-set %v = %s (%s), want %s",
				args,
				resp.Code,
				resp.Message,
				ipc.CodeInvalidInput,
			)
		}
	}
}

// TestSimulation_HealthNamesTheDeclaredMode pins that health reports the mode
// under the name status gives it, the declared name in a declared mode.
func TestSimulation_HealthNamesTheDeclaredMode(t *testing.T) {
	sim := newSimHarness(t, simConfigDeclaringAMode(), nil)

	sim.pressHotkey(customModeHotkey)
	sim.waitMode(domain.ModeCustom)

	resp := sim.app.HandleCommand(context.Background(), ipc.Command{Action: domain.CommandHealth})

	data, _ := resp.Data.(map[string]any)
	if data["mode"] != customModeName {
		t.Errorf("health mode = %v, want %q as status reports it", data["mode"], customModeName)
	}
}
