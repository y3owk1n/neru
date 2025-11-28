package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show neru status",
	Long:  `Display the current status of the neru program.`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
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

		cmd.Println("Neru Status:")
		var statusData ipc.StatusData
		ipcResponseData, ipcResponseDataErr := json.Marshal(ipcResponse.Data)
		if ipcResponseDataErr == nil {
			ipcResponseDataUnmarshalErr := json.Unmarshal(ipcResponseData, &statusData)
			if ipcResponseDataUnmarshalErr == nil {
				status := "stopped"
				if statusData.Enabled {
					status = "running"
				}
				cmd.Println("  Status: " + status)
				cmd.Println("  Mode: " + statusData.Mode)
				cmd.Println("  Config: " + statusData.Config)
			} else {
				// Fallback to previous behavior
				if data, ok := ipcResponse.Data.(map[string]any); ok {
					if enabled, ok := data["enabled"].(bool); ok {
						status := "stopped"
						if enabled {
							status = "running"
						}
						cmd.Println("  Status: " + status)
					}
					if mode, ok := data["mode"].(string); ok {
						cmd.Println("  Mode: " + mode)
					}
					if configPath, ok := data["config"].(string); ok {
						cmd.Println("  Config: " + configPath)
					}
				} else {
					jsonData, jsonDataErr := json.MarshalIndent(ipcResponse.Data, "  ", "  ")
					if jsonDataErr != nil {
						return derrors.Wrap(jsonDataErr, derrors.CodeSerializationFailed, "failed to marshal status data")
					}
					cmd.Println(string(jsonData))
				}
			}
		} else {
			jsonData, jsonDataErr := json.MarshalIndent(ipcResponse.Data, "  ", "  ")
			if jsonDataErr != nil {
				return derrors.Wrap(jsonDataErr, derrors.CodeSerializationFailed, "failed to marshal status data")
			}
			cmd.Println(string(jsonData))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
