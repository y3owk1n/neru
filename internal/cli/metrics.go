package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/infra/ipc"
	"github.com/y3owk1n/neru/internal/infra/metrics"
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Show application metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !ipc.IsServerRunning() {
			fmt.Println("❌ Neru is not running")
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
			fmt.Println("No metrics available")
			return nil
		}

		// Decode metrics
		var snapshot []metrics.Metric
		b, err := json.Marshal(resp.Data)
		if err != nil {
			return fmt.Errorf("failed to marshal metrics data: %w", err)
		}
		if err := json.Unmarshal(b, &snapshot); err != nil {
			return fmt.Errorf("failed to unmarshal metrics: %w", err)
		}

		if len(snapshot) == 0 {
			fmt.Println("No metrics recorded yet")
			return nil
		}

		// Sort metrics by name
		sort.Slice(snapshot, func(i, j int) bool {
			return snapshot[i].Name < snapshot[j].Name
		})

		fmt.Println("📊 Application Metrics:")
		fmt.Println("-----------------------")

		for _, m := range snapshot {
			switch m.Type {
			case metrics.TypeCounter:
				fmt.Printf("%-40s %d\n", m.Name, int(m.Value))
			case metrics.TypeGauge:
				fmt.Printf("%-40s %.2f\n", m.Name, m.Value)
			case metrics.TypeHistogram:
				fmt.Printf("%-40s %.4fs\n", m.Name, m.Value)
			}
		}
		fmt.Printf("\nLast updated: %s\n", time.Now().Format(time.RFC1123))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(metricsCmd)
}
