package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/infra/ipc"
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Show application metrics",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if !ipc.IsServerRunning() {
			cmd.Println("❌ Neru is not running")
			return nil
		}

		client := ipc.NewClient()
		resp, err := client.Send(ipc.Command{Action: domain.CommandMetrics})
		if err != nil {
			return fmt.Errorf("failed to get metrics: %w", err)
		}

		if !resp.Success {
			return fmt.Errorf("failed to get metrics: %s", resp.Message)
		}

		if resp.Data == nil {
			cmd.Println("No metrics recorded yet")
			return nil
		}

		// Decode metrics
		b, err := json.Marshal(resp.Data)
		if err != nil {
			return fmt.Errorf("failed to marshal metrics data: %w", err)
		}

		var snapshot struct {
			Metrics []struct {
				Name  string  `json:"name"`
				Value float64 `json:"value"`
				Type  string  `json:"type"`
			} `json:"metrics"`
		}

		err = json.Unmarshal(b, &snapshot)
		if err != nil {
			return fmt.Errorf("failed to parse metrics: %w", err)
		}

		if len(snapshot.Metrics) == 0 {
			cmd.Println("No metrics recorded yet")
			return nil
		}

		// Sort metrics by name
		sort.Slice(snapshot.Metrics, func(i, j int) bool {
			return snapshot.Metrics[i].Name < snapshot.Metrics[j].Name
		})

		cmd.Println("📊 Application Metrics:")
		cmd.Println("-----------------------")

		for _, m := range snapshot.Metrics {
			switch m.Type {
			case "counter":
				cmd.Printf("%-40s %d\n", m.Name, int(m.Value))
			case "gauge":
				cmd.Printf("%-40s %.2f\n", m.Name, m.Value)
			default: // Assuming histogram or other time-based metrics
				cmd.Printf("%-40s %.4fs\n", m.Name, m.Value)
			}
		}

		cmd.Printf("\nLast updated: %s\n", time.Now().Format(time.RFC1123))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(metricsCmd)
}
