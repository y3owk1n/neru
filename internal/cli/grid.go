package cli

import (
	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/core/domain"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
)

var gridCmd = &cobra.Command{
	Use:   "grid",
	Short: "Launch grid mode",
	Long:  `Activate grid mode for mouseless navigation.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		logger.Debug("Launching grid mode")

		action, _ := cmd.Flags().GetString("action")
		if action != "" {
			// Validate action
			if !domain.IsKnownActionName(domain.ActionName(action)) {
				return derrors.Newf(
					derrors.CodeInvalidInput,
					"invalid action: %s. Supported actions: %s",
					action,
					domain.SupportedActionsString,
				)
			}
		}

		var params []string
		params = append(params, "grid")
		if action != "" {
			params = append(params, action)
		}

		return sendCommand("grid", params)
	},
}

func init() {
	gridCmd.Flags().
		StringP("action", "a", "", "Action to perform on grid selection (left_click, right_click, middle_click, mouse_up, mouse_down)")
	rootCmd.AddCommand(gridCmd)
}
