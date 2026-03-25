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
	ActionDesc string   // Description for the action flag (e.g., "hint selection" or "grid selection")
	Aliases    []string // Optional CLI aliases (e.g., "recursive-grid" for "recursive_grid")
}

// BuildModeCommand creates a CLI command for a navigation mode (hints, grid, etc.).
func BuildModeCommand(config ModeConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:     config.Name,
		Aliases: config.Aliases,
		Short:   config.Short,
		Long:    config.Long,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			actionFlag, err := cmd.Flags().GetString("action")
			if err != nil {
				return err
			}

			repeatFlag, err := cmd.Flags().GetBool("repeat")
			if err != nil {
				return err
			}

			if repeatFlag && actionFlag == "" {
				return derrors.New(
					derrors.CodeInvalidInput,
					"--repeat requires --action",
				)
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

				// Scroll sub-actions (scroll_up, page_down, etc.) are only
				// valid as standalone CLI/IPC commands, not as pending mode
				// actions. Reject them here so the user gets immediate feedback
				// instead of a silent failure when the mode completes.
				if action.IsScrollSubAction(actionFlag) {
					return derrors.Newf(
						derrors.CodeInvalidInput,
						"scroll sub-action %q cannot be used as a mode --action flag; use 'neru action %s' instead",
						actionFlag,
						actionFlag,
					)
				}

				if action.IsResetAction(actionFlag) || action.IsBackspaceAction(actionFlag) {
					return derrors.Newf(
						derrors.CodeInvalidInput,
						"%q cannot be used as a mode --action flag; use 'neru action %s' instead",
						actionFlag,
						actionFlag,
					)
				}
			}

			var params []string

			params = append(params, config.Name)
			if actionFlag != "" {
				params = append(params, actionFlag)
			}

			if repeatFlag {
				params = append(params, "--repeat")
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

	cmd.Flags().BoolP(
		"repeat",
		"r",
		false,
		"Re-activate mode after performing the action (requires --action)",
	)

	return cmd
}
