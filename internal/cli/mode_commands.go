package cli

import (
	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// ModeConfig is what is bespoke about one mode command: the words it is
// documented with.
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
		if descriptor.AcceptedBy(config.Mode) {
			registerFlag(cmd, descriptor)
		}
	}
}

// registerFlag offers one flag in the shape its own rule reads it.
func registerFlag(cmd *cobra.Command, descriptor modecmd.Descriptor) {
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

	validateErr := modecmd.Validate(activation)
	if validateErr != nil {
		return modeRequest{}, validateErr
	}

	return modeRequest{
		action: domain.ModeString(config.Mode),
		args:   modecmd.Render(activation),
	}, nil
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
