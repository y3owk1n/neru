package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// GridCmd is the CLI grid command.
var GridCmd = &cobra.Command{
	Use:   "grid",
	Short: "Launch grid mode",
	Long:  `Activate grid mode for mouseless navigation.`,
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
		params = append(params, "grid")
		if actionFlag != "" {
			params = append(params, actionFlag)
		}

		return sendCommand(cmd, "grid", params)
	},
}

func init() {
	GridCmd.Flags().
		StringP(
			"action",
			"a",
			"",
			fmt.Sprintf("Action to perform on grid selection (%s)", action.SupportedNamesString()),
		)
	RootCmd.AddCommand(GridCmd)
}
