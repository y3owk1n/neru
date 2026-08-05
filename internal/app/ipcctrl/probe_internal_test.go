package ipcctrl

import (
	"context"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/domain"
)

// probeCommand builds a probe request the way the CLI sends one: the action
// names the probe, and the arguments carry only the flags that decide which
// elements are collected.
func probeCommand(args ...string) ipc.Command {
	return ipc.Command{Action: domain.CommandHintsProbe, Args: args}
}

func TestExtractProbeOptions_ReadsEveryCollectionFlag(t *testing.T) {
	handler := &ModesHandler{}

	opts, resp := handler.extractProbeOptions(probeCommand(
		"--role=AXButton,AXLink",
		"--text=OK",
		"--strategy=vision",
		"--split-word",
	))
	if resp != nil {
		t.Fatalf("probe request was refused: %s", resp.Message)
	}

	if !slices.Equal(opts.FilterRoles, []string{"AXButton", "AXLink"}) {
		t.Errorf("FilterRoles = %v, want the two roles given", opts.FilterRoles)
	}

	if !slices.Equal(opts.FilterTextContains, []string{"OK"}) {
		t.Errorf("FilterTextContains = %v, want [OK]", opts.FilterTextContains)
	}

	if opts.Strategy != domain.StrategyVision {
		t.Errorf("Strategy = %q, want %q", opts.Strategy, domain.StrategyVision)
	}

	if !opts.SplitWord {
		t.Error("SplitWord did not take effect")
	}
}

// A probe reports and stops, so a flag describing an activation has nothing to
// act on. Refusing is what tells the caller that, rather than answering with a
// summary while dropping the rest of what they asked for.
func TestExtractProbeOptions_RefusesActivationFlags(t *testing.T) {
	handler := &ModesHandler{}

	for _, arg := range []string{
		"--action=left_click",
		"--repeat",
		flagToggle,
		"--on-exit=scroll",
		"--search",
		argLabelDirectionReverse,
		"--cursor-selection-mode=hold",
		leftClick,
	} {
		t.Run(arg, func(t *testing.T) {
			_, resp := handler.extractProbeOptions(probeCommand(arg))
			if resp == nil {
				t.Errorf("%s was accepted by a probe request; it describes an activation", arg)
			}
		})
	}
}

// The probe's own value flags have to report a missing value rather than
// swallowing whatever follows, exactly as the mode-command parser does.
func TestExtractProbeOptions_RefusesAMissingValue(t *testing.T) {
	handler := &ModesHandler{}

	for _, arg := range []string{"--role", "--text", "--strategy"} {
		t.Run(arg, func(t *testing.T) {
			_, resp := handler.extractProbeOptions(probeCommand(arg))
			if resp == nil {
				t.Errorf("%s with no value was accepted", arg)
			}
		})
	}
}

func TestExtractProbeOptions_RefusesAnInvalidStrategy(t *testing.T) {
	handler := &ModesHandler{}

	_, resp := handler.extractProbeOptions(probeCommand("--strategy=telepathy"))
	if resp == nil {
		t.Fatal("an unknown strategy was accepted")
	}

	if resp.Code != ipc.CodeInvalidInput {
		t.Errorf("code = %s, want %s", resp.Code, ipc.CodeInvalidInput)
	}
}

// The probe is registered under its own action, so a caller reaches it without
// spelling a mode command at all.
func TestRegisterHandlers_ExposesTheProbeAsItsOwnCommand(t *testing.T) {
	handler := &ModesHandler{}
	handlers := map[string]func(context.Context, ipc.Command) ipc.Response{}

	handler.RegisterHandlers(handlers)

	if _, registered := handlers[domain.CommandHintsProbe]; !registered {
		t.Errorf("%q is not registered as a command", domain.CommandHintsProbe)
	}
}
