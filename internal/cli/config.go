package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/infra/ipc"
	"github.com/y3owk1n/neru/internal/infra/logger"
	"go.uber.org/zap"
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
	RunE: func(_ *cobra.Command, _ []string) error {
		logger.Debug("Fetching config")
		ipcClient := ipc.NewClient()
		ipcResponse, ipcResponseErr := ipcClient.Send(ipc.Command{Action: domain.CommandConfig})
		if ipcResponseErr != nil {
			return fmt.Errorf("failed to send config command: %w", ipcResponseErr)
		}

		if !ipcResponse.Success {
			if ipcResponse.Code != "" {
				return fmt.Errorf("%s (code: %s)", ipcResponse.Message, ipcResponse.Code)
			}

			return fmt.Errorf("%s", ipcResponse.Message)
		}

		// Marshal pretty JSON
		ipcResponseData, ipcResponseDataErr := json.MarshalIndent(ipcResponse.Data, "", "  ")
		if ipcResponseDataErr != nil {
			logger.Error("Failed to marshal config to JSON", zap.Error(ipcResponseDataErr))

			return fmt.Errorf("failed to marshal config: %w", ipcResponseDataErr)
		}

		logger.Info(string(ipcResponseData))

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
	RunE: func(_ *cobra.Command, _ []string) error {
		logger.Debug("Reloading config")

		return sendCommand(domain.CommandReloadConfig, []string{})
	},
}

func init() {
	configCmd.AddCommand(configDumpCmd)
	configCmd.AddCommand(configReloadCmd)
	rootCmd.AddCommand(configCmd)
}
