package cli

import (
	"strings"

	"github.com/spf13/cobra"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// ActionFeedCmd feeds one or more keys or key chords directly to the operating system.
var ActionFeedCmd = &cobra.Command{
	Use:   "feed <key> [key...]",
	Short: "Feed keys directly to the operating system",
	Long: `Feed one or more keys or key chords to the operating system or to Neru's
own mode system.

By default, keys are posted directly to the OS. Use --mode to route keys
through Neru's active mode/action pipeline instead.

Examples:
  neru action feed o
  neru action feed ctrl+c
  neru action feed Cmd+Shift+P
  neru action feed h e l o return
  neru action feed --mode o
  neru action feed --mode Escape
  neru action feed --mode Cmd+Shift+p`,
	Args: validateActionFeedArgs,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		toMode, _ := cmd.Flags().GetBool("mode")

		actionName := "feed"
		if toMode {
			actionName = "feed-mode"
		}

		actionArgs := make([]string, 0, len(args)+1)

		actionArgs = append(actionArgs, actionName)
		for _, arg := range args {
			actionArgs = append(actionArgs, strings.TrimSpace(arg))
		}

		return sendCommand(cmd, "action", actionArgs)
	},
}

func init() {
	ActionFeedCmd.Flags().Bool("mode", false,
		"Feed keys into Neru's own mode system instead of the OS")
}

func validateActionFeedArgs(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return derrors.New(
			derrors.CodeInvalidInput,
			"feed requires at least one key (e.g., neru action feed o, neru action feed ctrl+c)",
		)
	}

	for _, arg := range args {
		if strings.TrimSpace(arg) == "" {
			return derrors.New(
				derrors.CodeInvalidInput,
				"feed keys cannot be empty",
			)
		}
	}

	return nil
}
