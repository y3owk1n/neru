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
	RunE: func(cmd *cobra.Command, args []string) error {
		if !ipc.IsServerRunning() {
			fmt.Println("❌ Neru is not running")
			return nil
		}

		client := ipc.NewClient()
		resp, err := client.Send(ipc.Command{Action: domain.CommandHealth})
		if err != nil {
			return fmt.Errorf("failed to check health: %w", err)
		}

		if !resp.Success {
			fmt.Println("⚠️  Some components are unhealthy:")
		} else {
			fmt.Println("✅ All systems operational")
		}

		if resp.Data != nil {
			// Convert map[string]interface{} to map[string]string for display
			// JSON decoding unmarshals to map[string]interface{}
			data, ok := resp.Data.(map[string]interface{})
			if !ok {
				// Try to re-marshal and unmarshal if it's not the expected type
				// This handles cases where the type info is lost
				b, _ := json.Marshal(resp.Data)
				json.Unmarshal(b, &data)
			}

			for k, v := range data {
				status := fmt.Sprintf("%v", v)
				if status == "healthy" {
					fmt.Printf("  ✅ %s: %s\n", k, status)
				} else {
					fmt.Printf("  ❌ %s: %s\n", k, status)
				}
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
