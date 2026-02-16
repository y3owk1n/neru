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
	Long:  "Commands for managing Neru configuration including dumping and reloading.",
}

var configDumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Dump effective config",
	Long:  "Print the currently active Neru configuration as JSON.",
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
	Short: "Reload configuration",
	Long:  "Reload the Neru configuration from disk without restarting the application.",
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
