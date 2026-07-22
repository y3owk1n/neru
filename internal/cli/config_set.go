package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

// commandArgCount is the exact number of arguments expected by the config
// set command (key and value).
const commandArgCount = 2

// noReloadFlagName is the flag name for deferring hotkey re-registration.
const noReloadFlagName = "no-reload"

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value at runtime",
	Long: `Set a configuration value on the running daemon without restarting.
The key uses dotted TOML path notation matching your config file.

Supported types: string, integer, boolean, float, color (#RGB/#RRGGBB/#AARRGGBB),
array (comma-separated or JSON: "AXButton,AXLink" or '["AXButton","AXLink"]').

Use --no-reload to skip hotkey re-registration when setting multiple fields
in sequence. Run "neru config reload" afterward to apply all changes at once.

Examples:
  neru config set hints.hint_characters "asdfghjkl"
  neru config set hints.ui.font_size 14
  neru config set general.passthrough_unbounded_keys true
  neru config set hints.clickable_roles "AXButton,AXLink"
  neru config set scroll.scroll_step 50
  neru config set --no-reload recursive_grid.grid_cols 4
  neru config set --no-reload recursive_grid.grid_rows 3

Use "neru config reload" after setting multiple fields with --no-reload.
Use "neru config dump | jq" to explore all available keys.`,
	Args: cobra.ExactArgs(commandArgCount),
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		noReload, _ := cmd.Flags().GetBool(noReloadFlagName)

		var valErr error
		if noReload {
			valErr = config.SetField(config.DefaultConfig(), key, value)
		} else {
			valErr = config.ValidateConfigSetField(key, value)
		}

		if valErr != nil {
			typeHint := config.ConfigFieldType(key)

			return fmt.Errorf(
				"invalid config path or value: %w\n  Field %q type: %s",
				valErr,
				key,
				typeHint,
			)
		}

		client := ipc.NewClient()

		ipcArgs := []string{key, value}
		if noReload {
			ipcArgs = append(ipcArgs, "--no-reload")
		}

		resp, err := client.Send(ipc.Command{
			Action: domain.CommandConfigSet,
			Args:   ipcArgs,
		})
		if err != nil {
			return fmt.Errorf("failed to send config-set command: %w", err)
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
	configSetCmd.Flags().Bool(noReloadFlagName, false,
		"Skip hotkey re-registration. Use when setting multiple fields; "+
			"run `neru config reload` afterwards to apply.")
}
