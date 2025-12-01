package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/core/domain"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// HintsCmd is the CLI hints command.
var HintsCmd = &cobra.Command{
	Use:   "hints",
	Short: "Launch hints mode",
	Long:  `Activate hint mode for direct clicking on UI elements.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		action, err := cmd.Flags().GetString("action")
		if err != nil {
			return err
		}
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
		params = append(params, "hints")
		if action != "" {
			params = append(params, action)
		}

		return sendCommand(cmd, "hints", params)
	},
}

func init() {
	HintsCmd.Flags().
		StringP(
			"action",
			"a",
			"",
			fmt.Sprintf("Action to perform on hint selection (%s)", domain.SupportedActionsString()),
		)
	RootCmd.AddCommand(HintsCmd)
}
