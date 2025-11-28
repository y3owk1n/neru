package cli

import (
	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/core/domain"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
)

var hintsCmd = &cobra.Command{
	Use:   "hints",
	Short: "Launch hints mode",
	Long:  `Activate hint mode for direct clicking on UI elements.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		logger.Debug("Launching hints mode")

		action, _ := cmd.Flags().GetString("action")
		if action != "" {
			// Validate action
			if !domain.IsKnownActionName(domain.ActionName(action)) {
				return derrors.Newf(
					derrors.CodeInvalidInput,
					"invalid action: %s. Supported actions: left_click, right_click, middle_click, mouse_up, mouse_down",
					action,
				)
			}
		}

		var params []string
		params = append(params, "hints")
		if action != "" {
			params = append(params, action)
		}

		return sendCommand("hints", params)
	},
}

func init() {
	hintsCmd.Flags().
		StringP("action", "a", "", "Action to perform on hint selection (left_click, right_click, middle_click, mouse_up, mouse_down)")
	rootCmd.AddCommand(hintsCmd)
}
