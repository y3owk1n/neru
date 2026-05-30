package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/core/domain"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Neru configuration",
	Long: `Commands for managing the Neru configuration file and runtime settings.

Subcommands:
  dump       Print the currently active configuration as JSON
  reload     Reload configuration from disk without restarting
  init       Create a default configuration file to get started
  validate   Check a configuration file for errors

See 'neru config <subcommand> --help' for details on each.`,
}

var configDumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Print the active configuration as JSON",
	Long:  "Print the currently active Neru configuration (as resolved by the running daemon) as pretty-printed JSON. Useful for verifying that your config file is being parsed correctly.",
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		ipcClient := ipc.NewClient()

		ipcResponse, ipcResponseErr := ipcClient.Send(ipc.Command{Action: domain.CommandConfig})
		if ipcResponseErr != nil {
			return derrors.Wrap(
				ipcResponseErr,
				derrors.CodeIPCFailed,
				"failed to send config command",
			)
		}

		if !ipcResponse.Success {
			if ipcResponse.Code != "" {
				return derrors.Newf(
					derrors.CodeIPCFailed,
					"%s (code: %s)",
					ipcResponse.Message,
					ipcResponse.Code,
				)
			}

			return derrors.New(derrors.CodeIPCFailed, ipcResponse.Message)
		}

		// Marshal pretty JSON
		ipcResponseData, ipcResponseDataErr := json.MarshalIndent(ipcResponse.Data, "", "  ")
		if ipcResponseDataErr != nil {
			return derrors.Wrap(
				ipcResponseDataErr,
				derrors.CodeSerializationFailed,
				"failed to marshal config",
			)
		}

		cmd.Println(string(ipcResponseData))

		return nil
	},
}

var configReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload configuration from disk",
	Long:  "Reload the Neru configuration file from disk without restarting the running daemon. Changes to hotkeys, colors, and behavior take effect immediately.",
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		return sendCommand(cmd, domain.CommandReloadConfig, []string{})
	},
}

func init() {
	configCmd.AddCommand(configDumpCmd)
	configCmd.AddCommand(configReloadCmd)
	RootCmd.AddCommand(configCmd)
}
