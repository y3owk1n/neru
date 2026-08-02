package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// MacroCmd is the CLI macro command for running a named sequence from [macros].
var MacroCmd = &cobra.Command{
	Use:   "macro <name> [arg...]",
	Short: "Run a named sequence from the [macros] config table",
	Long: `Run a macro defined in the [macros] table of the config file.

A macro is an ordered sequence of steps under a name, with positional
placeholders ($1, $2, ...) filled in by the arguments given here. The daemon
runs it exactly as it would when a hotkey binding invokes "macro <name>", so a
sequence written once is available to bindings and to external drivers (skhd,
Hammerspoon, shell scripts) alike.

Arguments are passed through as given, so one containing spaces needs no
quoting beyond what the shell already does:

  neru macro say_it "hello there"

The number of arguments must match the highest placeholder the macro uses.

Macros that block (sleeps, waits) can outlast the default IPC timeout. Raise it
for those: neru --timeout 60 macro slow_one

Examples:
  neru macro window_click
  neru macro zoom_click 3
  neru macro open_and_click Safari left_click`,
	Args: validateMacroArgs,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return sendCommand(cmd, domain.CommandMacro, args)
	},
}

// validateMacroArgs rejects a call the daemon could only reject later, so a
// typo fails here rather than after a round trip.
func validateMacroArgs(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return derrors.New(
			derrors.CodeInvalidInput,
			"macro requires a name (e.g., neru macro window_click)",
		)
	}

	if !config.IsValidMacroName(strings.TrimSpace(args[0])) {
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"invalid macro name %q: names start with a letter and may contain letters, digits, underscores, and dashes",
			args[0],
		)
	}

	return nil
}

func init() {
	RootCmd.AddCommand(MacroCmd)
}
