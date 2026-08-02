package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/core/domain"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// RunCmd is the CLI run command for executing a sequence of actions.
var RunCmd = &cobra.Command{
	Use:   "run <step> [step...]",
	Short: "Run a sequence of actions in one call",
	Long: `Run an ordered sequence of actions in a single call.

Each argument is one step, written exactly as it would be written in a hotkey
binding: an action ("action left_click"), a mode ("hints --action left_click"),
or a shell command ("exec open -a Safari").

The daemon runs the steps in order, so an external driver (skhd, Hammerspoon, a
shell script) can compose a workflow without spawning one process per step. A
step that asks the sequence to stop — "action wait_for_mode_exit --bail" after
the mode was canceled — ends it; any other failing step is reported while the
remaining steps still run.

To stop at a failure instead, end that step with --bail-on-error, or pass
--stop-on-error to make every step in the sequence fatal.

Sequences that block (sleeps, waits) can outlast the default IPC timeout. Raise
it for those: neru --timeout 60 run ...

Examples:
  neru run "action save_cursor_pos" hints
  neru run "action left_click" "action sleep 0.2" "action restore_cursor_pos"
  neru run "hints --action left_click" "action wait_for_mode_exit --bail" "exec say done"
  neru run "action left_click --bail-on-error" idle
  neru run --stop-on-error "action left_click" "action restore_cursor_pos"`,
	Args: validateRunArgs,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		stopOnError, err := cmd.Flags().GetBool("stop-on-error")
		if err != nil {
			return err
		}

		params := args
		if stopOnError {
			params = append([]string{"--stop-on-error"}, args...)
		}

		return sendCommand(cmd, domain.CommandRun, params)
	},
}

// validateRunArgs rejects an empty sequence and blank steps, so a quoting
// mistake fails at the CLI instead of silently running fewer steps.
func validateRunArgs(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return derrors.New(
			derrors.CodeInvalidInput,
			"run requires at least one action step (e.g., neru run 'action left_click' hints)",
		)
	}

	for _, arg := range args {
		if strings.TrimSpace(arg) == "" {
			return derrors.New(derrors.CodeInvalidInput, "run steps cannot be empty")
		}
	}

	return nil
}

func init() {
	RunCmd.Flags().Bool(
		"stop-on-error",
		false,
		"Stop the sequence at the first failing step, as if every step ended with --bail-on-error",
	)

	RootCmd.AddCommand(RunCmd)
}
