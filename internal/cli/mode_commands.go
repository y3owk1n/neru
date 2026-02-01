package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// ModeConfig holds configuration for creating a mode command.
type ModeConfig struct {
	Name       string
	Short      string
	Long       string
	ActionDesc string // Description for the action flag (e.g., "hint selection" or "grid selection")
}

// BuildModeCommand creates a CLI command for a navigation mode (hints, grid, etc.).
func BuildModeCommand(config ModeConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   config.Name,
		Short: config.Short,
		Long:  config.Long,
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

			params = append(params, config.Name)
			if actionFlag != "" {
				params = append(params, actionFlag)
			}

			return sendCommand(cmd, config.Name, params)
		},
	}

	cmd.Flags().StringP(
		"action",
		"a",
		"",
		fmt.Sprintf("Action to perform on %s (%s)", config.ActionDesc, action.SupportedNamesString()),
	)

	return cmd
}
