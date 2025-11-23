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

		client := ipc.NewClient()
		resp, err := client.Send(ipc.Command{Action: domain.CommandHealth})
		if err != nil {
			return fmt.Errorf("failed to check health: %w", err)
		}

		if !resp.Success {
			cmd.Println("⚠️  Some components are unhealthy:")
		} else {
			cmd.Println("✅ All systems operational")
		}
		if resp.Success {
			cmd.Println("✅ All systems operational")
			return nil
		}

		// Parse error details if available
		var data map[string]string
		if resp.Data != nil {
			b, err := json.Marshal(resp.Data)
			if err != nil {
				return fmt.Errorf("failed to marshal error data: %w", err)
			}
			err = json.Unmarshal(b, &data)
			if err != nil {
				return fmt.Errorf("failed to parse error data: %w", err)
			}
		}

		cmd.Println("⚠️  Some components are unhealthy:")
		for k, v := range data {
			status := "OK"
			if v != "" {
				status = v
			}
			if status == "OK" {
				cmd.Printf("  ✅ %s: %s\n", k, status)
			} else {
				cmd.Printf("  ❌ %s: %s\n", k, status)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
