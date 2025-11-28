package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/core/domain"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

var gridCmd = &cobra.Command{
	Use:   "grid",
	Short: "Launch grid mode",
	Long:  `Activate grid mode for mouseless navigation.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		action, _ := cmd.Flags().GetString("action")
		if action != "" {
			// Validate action
			if !domain.IsKnownActionName(domain.ActionName(action)) {
				return derrors.Newf(
					derrors.CodeInvalidInput,
					"invalid action: %s. Supported actions: %s",
					action,
					domain.SupportedActionsString(),
				)
			}
		}

		var params []string
		params = append(params, "grid")
		if action != "" {
			params = append(params, action)
		}

		return sendCommand(cmd, "grid", params)
	},
}

func init() {
	gridCmd.Flags().
		StringP(
			"action",
			"a",
			"",
			fmt.Sprintf("Action to perform on grid selection (%s)", domain.SupportedActionsString()),
		)
	rootCmd.AddCommand(gridCmd)
}
