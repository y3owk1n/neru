package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/core/domain"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the health of Neru components",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if !ipc.IsServerRunning() {
			cmd.Println("❌ Neru is not running")

			return nil
		}

		ipcClient := ipc.NewClient()
		ipcResponse, ipcResponseErr := ipcClient.Send(ipc.Command{Action: domain.CommandHealth})
		if ipcResponseErr != nil {
			return derrors.Wrap(ipcResponseErr, derrors.CodeIPCFailed, "failed to check health")
		}

		if ipcResponse.Success {
			cmd.Println("✅ All systems operational")

			return nil
		}

		// Parse error details if available
		var data map[string]string
		if ipcResponse.Data != nil {
			ipcResponseData, ipcResponseDataErr := json.Marshal(ipcResponse.Data)
			if ipcResponseDataErr != nil {
				return derrors.Wrap(
					ipcResponseDataErr,
					derrors.CodeSerializationFailed,
					"failed to marshal error data",
				)
			}
			ipcResponseDataErr = json.Unmarshal(ipcResponseData, &data)
			if ipcResponseDataErr != nil {
				return derrors.Wrap(
					ipcResponseDataErr,
					derrors.CodeSerializationFailed,
					"failed to parse error data",
				)
			}
		}

		cmd.Println("⚠️  Some components are unhealthy:")
		for key, value := range data {
			status := "OK"
			if value != "" {
				status = value
			}
			if status == "OK" {
				cmd.Printf("  ✅ %s: %s\n", key, status)
			} else {
				cmd.Printf("  ❌ %s: %s\n", key, status)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
