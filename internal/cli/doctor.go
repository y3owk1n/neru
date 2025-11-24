package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/infra/ipc"
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
			return fmt.Errorf("failed to check health: %w", ipcResponseErr)
		}

		if !ipcResponse.Success {
			cmd.Println("⚠️  Some components are unhealthy:")
		} else {
			cmd.Println("✅ All systems operational")
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
				return fmt.Errorf("failed to marshal error data: %w", ipcResponseDataErr)
			}
			ipcResponseDataErr = json.Unmarshal(ipcResponseData, &data)
			if ipcResponseDataErr != nil {
				return fmt.Errorf("failed to parse error data: %w", ipcResponseDataErr)
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
