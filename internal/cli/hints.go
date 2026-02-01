package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/core/domain/action"
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
		actionFlag, err := cmd.Flags().GetString("action")
		if err != nil {
			return err
		}
		if actionFlag != "" {
			// Validate action
			if !action.IsKnownName(action.Name(actionFlag)) {
				return derrors.Newf(
					derrors.CodeInvalidInput,
					"invalid action: %s. Supported actions: %s",
					actionFlag,
					action.SupportedNamesString(),
				)
			}
		}

		var params []string
		params = append(params, "hints")
		if actionFlag != "" {
			params = append(params, actionFlag)
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
			fmt.Sprintf("Action to perform on hint selection (%s)", action.SupportedNamesString()),
		)
	RootCmd.AddCommand(HintsCmd)
}
