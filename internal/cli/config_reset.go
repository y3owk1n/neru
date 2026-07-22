package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/core/domain"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

const minConfigResetArgs = 1

var configResetCmd = &cobra.Command{
	Use:   "reset <key>",
	Short: "Reset a config field to its default value",
	Long: `Remove a single config field from the override file, reverting it to the
value from your base config file (or the code default).

Use --no-reload to defer reloading when resetting multiple fields.
Run "neru config reload" afterward to apply all changes at once.

Examples:
  neru config reset recursive_grid.grid_cols
  neru config reset --no-reload recursive_grid.grid_rows
  neru config reset --no-reload recursive_grid.keys
  neru config reload`,
	Args: cobra.ExactArgs(minConfigResetArgs),
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		noReload, _ := cmd.Flags().GetBool(noReloadFlagName)

		ipcArgs := []string{"reset", key}
		if noReload {
			ipcArgs = append(ipcArgs, "--no-reload")
		}

		client := ipc.NewClient()

		resp, err := client.Send(ipc.Command{
			Action: domain.CommandConfig,
			Args:   ipcArgs,
		})
		if err != nil {
			return fmt.Errorf("failed to send config-reset command: %w", err)
		}

		if !resp.Success {
			if resp.Code != "" {
				return derrors.Newf(
					derrors.CodeActionFailed,
					"%s (code: %s)",
					resp.Message,
					resp.Code,
				)
			}

			return derrors.New(derrors.CodeActionFailed, resp.Message)
		}

		if resp.Message != "" {
			cmd.Println(resp.Message)
		}

		return nil
	},
}

func init() {
	configResetCmd.Flags().Bool(noReloadFlagName, false,
		"Skip reload. Use when resetting multiple fields; "+
			"run `neru config reload` afterwards to apply.")
}
