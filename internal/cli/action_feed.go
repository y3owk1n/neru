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
	Long:  "Feed one or more keys or key chords directly to the operating system through\nNeru's action IPC path. On macOS, the daemon posts the key back to macOS\nwithout routing it through Neru's active mode/action pipeline.\n\nExamples:\n  neru action feed o\n  neru action feed ctrl+c\n  neru action feed Cmd+Shift+P\n  neru action feed h e l o return",
	Args:  validateActionFeedArgs,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		actionArgs := make([]string, 0, len(args)+1)

		actionArgs = append(actionArgs, "feed")
		for _, arg := range args {
			actionArgs = append(actionArgs, strings.TrimSpace(arg))
		}

		return sendCommand(cmd, "action", actionArgs)
	},
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
