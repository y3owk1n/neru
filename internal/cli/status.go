package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
	derrors "github.com/y3owk1n/neru/internal/errors"
	"github.com/y3owk1n/neru/internal/infra/ipc"
	"github.com/y3owk1n/neru/internal/infra/logger"
	"go.uber.org/zap"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show neru status",
	Long:  `Display the current status of the neru program.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(_ *cobra.Command, _ []string) error {
		logger.Debug("Fetching status")
		ipcClient := ipc.NewClient()
		ipcResponse, ipcResponseErr := ipcClient.Send(ipc.Command{Action: "status"})
		if ipcResponseErr != nil {
			return derrors.Wrap(
				ipcResponseErr,
				derrors.CodeIPCFailed,
				"failed to send status command",
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

		logger.Info("Neru Status:")
		var statusData ipc.StatusData
		ipcResponseData, ipcResponseDataErr := json.Marshal(ipcResponse.Data)
		if ipcResponseDataErr == nil {
			ipcResponseDataUnmarshalErr := json.Unmarshal(ipcResponseData, &statusData)
			if ipcResponseDataUnmarshalErr == nil {
				status := "stopped"
				if statusData.Enabled {
					status = "running"
				}
				logger.Info("  Status: " + status)
				logger.Info("  Mode: " + statusData.Mode)
				logger.Info("  Config: " + statusData.Config)
			} else {
				// Fallback to previous behavior
				if data, ok := ipcResponse.Data.(map[string]any); ok {
					if enabled, ok := data["enabled"].(bool); ok {
						status := "stopped"
						if enabled {
							status = "running"
						}
						logger.Info("  Status: " + status)
					}
					if mode, ok := data["mode"].(string); ok {
						logger.Info("  Mode: " + mode)
					}
					if configPath, ok := data["config"].(string); ok {
						logger.Info("  Config: " + configPath)
					}
				} else {
					jsonData, jsonDataErr := json.MarshalIndent(ipcResponse.Data, "  ", "  ")
					if jsonDataErr != nil {
						logger.Error("Failed to marshal status data to JSON", zap.Error(jsonDataErr))

						return derrors.Wrap(jsonDataErr, derrors.CodeSerializationFailed, "failed to marshal status data")
					}
					logger.Info(string(jsonData))
				}
			}
		} else {
			jsonData, jsonDataErr := json.MarshalIndent(ipcResponse.Data, "  ", "  ")
			if jsonDataErr != nil {
				logger.Error("Failed to marshal status data to JSON", zap.Error(jsonDataErr))

				return derrors.Wrap(jsonDataErr, derrors.CodeSerializationFailed, "failed to marshal status data")
			}
			logger.Info(string(jsonData))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
