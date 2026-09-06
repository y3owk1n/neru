package cli

import (
	"slices"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// flagDebug asks for a probe rather than an activation.
//
// It is the one mode-command flag the grammar does not hold, and deliberately:
// a probe reports what hints mode would target and enters nothing, so it
// travels as its own request. --debug stays its command-line spelling, and the
// CLI is the only place that has to know that.
const (
	flagDebug      modecmd.Flag = "debug"
	flagDebugShort string       = "d"
)

// probeFlags are the flags a probe reads: the ones that decide which elements
// are collected. Everything else in the vocabulary describes an activation,
// which a probe is not.
var probeFlags = []modecmd.Flag{
	modecmd.FlagRole,
	modecmd.FlagText,
	modecmd.FlagStrategy,
	modecmd.FlagCaptureScope,
	modecmd.FlagSplitWord,
}

// ModeConfig is what is bespoke about one mode command: the words it is
// documented with, and whether it offers the probe.
//
// Which flags it accepts is not among them. That is the mode's answer, read
// from the grammar's descriptor table, so a flag a mode has no use for is one
// the command never offers.
type ModeConfig struct {
	// Mode is the mode this command enters, and the name it is invoked by.
	Mode domain.Mode

	Short string
	Long  string

	// Aliases are further spellings of the command name, such as
	// "recursive-grid" for recursive_grid.
	Aliases []string

	// SupportDebug offers --debug, which asks for a probe instead of an
	// activation.
	SupportDebug bool
}

// BuildModeCommand creates the CLI command for a navigation mode.
//
// Every mode goes through here, including the ones that enter no mode at all
// and the ones that take no flags: what separates them is what the grammar says
// their mode accepts, not a command written by hand.
func BuildModeCommand(config ModeConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:     domain.ModeString(config.Mode),
		Aliases: config.Aliases,
		Short:   config.Short,
		Long:    config.Long,
		// A mode command is its flags and nothing else. Refusing a stray
		// argument is what stops "neru idle left_click" from looking like it
		// asked for something. The custom mode command is the exception by
		// design: the one argument it takes is the declared mode it enters.
		Args: modeArgs(config.Mode),
		RunE: func(cmd *cobra.Command, args []string) error {
			request, err := readModeCommand(cmd, args, config)
			if err != nil {
				return err
			}

			// Reading the command before asking whether the daemon is up is
			// what makes a mistyped one fail the same way either way.
			runningErr := requiresRunningInstance()
			if runningErr != nil {
				return runningErr
			}

			return sendCommand(cmd, request.action, request.args)
		},
	}

	registerModeFlags(cmd, config)

	return cmd
}

// modeArgs is the positional shape a mode command takes: none, except for the
// custom mode command, whose single argument names the declared mode.
func modeArgs(mode domain.Mode) cobra.PositionalArgs {
	if mode == domain.ModeCustom {
		return cobra.ExactArgs(1)
	}

	return cobra.NoArgs
}

// registerModeFlags offers exactly the flags the mode accepts, spelled and
// explained as the grammar declares them.
//
// A flag is registered in the shape its own rule reads it: a presence-only flag
// as a boolean, a repeatable one as a list that keeps every occurrence, and the
// rest as a value. Their values travel back through the same table, so a flag
// the CLI offers is one the daemon acts on.
func registerModeFlags(cmd *cobra.Command, config ModeConfig) {
	for _, descriptor := range modecmd.All() {
		if !descriptor.AcceptedBy(config.Mode) {
			continue
		}

		name, short, usage := descriptor.Name().String(), descriptor.Short(), descriptor.Usage()

		switch descriptor.Kind() {
		case modecmd.KindPresence:
			cmd.Flags().BoolP(name, short, false, usage)
		case modecmd.KindList:
			cmd.Flags().StringArrayP(name, short, nil, usage)
		case modecmd.KindValue:
			cmd.Flags().StringP(name, short, "", usage)
		}
	}

	if config.SupportDebug {
		cmd.Flags().BoolP(
			flagDebug.String(),
			flagDebugShort,
			false,
			"Probe the focused window and print detected clickable elements without showing the overlay",
		)
	}
}

// modeRequest is what a mode command sends: the action naming it, and the
// arguments carrying the rest of what was asked for.
type modeRequest struct {
	action string
	args   []string
}

// readModeCommand reads what the user typed into the request it describes.
//
// The rules are not applied here. What was written becomes an activation, the
// grammar judges it, and the grammar writes it back out — so a command refused
// on the wire is refused here in the same words, and one that travels is
// spelled the way the daemon reads it.
func readModeCommand(cmd *cobra.Command, args []string, config ModeConfig) (modeRequest, error) {
	activation, err := readActivation(cmd, config.Mode)
	if err != nil {
		return modeRequest{}, err
	}

	if config.Mode == domain.ModeCustom {
		activation.Name = args[0]
	}

	probeWanted, err := probeRequested(cmd, config)
	if err != nil {
		return modeRequest{}, err
	}

	if !probeWanted {
		validateErr := modecmd.Validate(activation)
		if validateErr != nil {
			return modeRequest{}, validateErr
		}

		return modeRequest{
			action: domain.ModeString(config.Mode),
			args:   modecmd.Render(activation),
		}, nil
	}

	args, refused := probeRendering(activation)
	if refused != "" {
		return modeRequest{}, derrors.Newf(
			derrors.CodeInvalidInput,
			"%s cannot be combined with %s: a probe reports what would be targeted without entering the mode",
			flagDebug.Long(),
			refused.Long(),
		)
	}

	validateErr := modecmd.Validate(activation)
	if validateErr != nil {
		return modeRequest{}, validateErr
	}

	return modeRequest{action: domain.CommandHintsProbe, args: args}, nil
}

// readActivation builds the activation the typed flags describe.
//
// A flag left alone contributes nothing, which is how the activation says
// "inherit what the configuration says" rather than "override it with the zero
// value". A presence-only flag written off says the same as leaving it out: it
// asks the mode for nothing.
func readActivation(cmd *cobra.Command, mode domain.Mode) (modecmd.Activation, error) {
	activation := modecmd.Activation{Mode: mode}

	for _, descriptor := range modecmd.All() {
		if !descriptor.AcceptedBy(mode) || !cmd.Flags().Changed(descriptor.Name().String()) {
			continue
		}

		err := applyFlag(cmd, descriptor, &activation)
		if err != nil {
			return modecmd.Activation{}, err
		}
	}

	return activation, nil
}

// applyFlag reads one flag's typed value and hands it to the flag's own rule,
// which is what decides what the value means.
func applyFlag(
	cmd *cobra.Command,
	descriptor modecmd.Descriptor,
	activation *modecmd.Activation,
) error {
	name := descriptor.Name().String()

	switch descriptor.Kind() {
	case modecmd.KindPresence:
		on, err := cmd.Flags().GetBool(name)
		if err != nil || !on {
			return err
		}

		return descriptor.Apply(activation, "")

	case modecmd.KindList:
		values, err := cmd.Flags().GetStringArray(name)
		if err != nil {
			return err
		}

		for _, value := range values {
			applyErr := descriptor.Apply(activation, value)
			if applyErr != nil {
				return applyErr
			}
		}

		return nil

	case modecmd.KindValue:
		value, err := cmd.Flags().GetString(name)
		if err != nil {
			return err
		}

		return descriptor.Apply(activation, value)
	}

	// Unreachable while every kind is answered above, which the exhaustiveness
	// check enforces. Saying so out loud is the point: a shape read as nothing
	// is a flag accepted and dropped, which is what this whole path exists to
	// stop.
	return derrors.Newf(
		derrors.CodeInternal,
		"%s is written in a shape this command cannot read",
		descriptor.Name().Long(),
	)
}

// probeRequested reports whether --debug asked for a probe instead of an
// activation. A mode that does not offer it never asks for one.
func probeRequested(cmd *cobra.Command, config ModeConfig) (bool, error) {
	if !config.SupportDebug {
		return false, nil
	}

	return cmd.Flags().GetBool(flagDebug.String())
}

// probeRendering builds the argument list for a hints probe, and names the
// first flag given that a probe has no use for.
//
// A probe reports what the mode would target and then stops; it never selects
// anything, so there is nothing for an action to act on, nothing to repeat and
// nothing to run on exit. Answering with a summary while dropping the rest of
// what was asked for is what naming the flag avoids. An empty name means
// nothing outside the probe's own vocabulary was given.
//
// --debug itself is in neither list. It named the command, and the command is
// now its own.
//
// Whether a flag was given is read from its own rendering: a flag renders
// exactly what it was given, so rendering nothing is what absent means.
func probeRendering(activation modecmd.Activation) ([]string, modecmd.Flag) {
	var args []string

	for _, descriptor := range modecmd.All() {
		written := descriptor.Render(activation)
		if len(written) == 0 {
			continue
		}

		if !slices.Contains(probeFlags, descriptor.Name()) {
			return nil, descriptor.Name()
		}

		args = append(args, written...)
	}

	return args, ""
}
