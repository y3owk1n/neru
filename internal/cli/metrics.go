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

		ipcClient := ipc.NewClient()
		ipcResponse, ipcResponseErr := ipcClient.Send(ipc.Command{Action: domain.CommandMetrics})
		if ipcResponseErr != nil {
			return fmt.Errorf("failed to get metrics: %w", ipcResponseErr)
		}

		if !ipcResponse.Success {
			return fmt.Errorf("failed to get metrics: %s", ipcResponse.Message)
		}

		if ipcResponse.Data == nil {
			cmd.Println("No metrics recorded yet")

			return nil
		}

		// Decode metrics
		ipcResponseData, ipcResponseDataErr := json.Marshal(ipcResponse.Data)
		if ipcResponseDataErr != nil {
			return fmt.Errorf("failed to marshal metrics data: %w", ipcResponseDataErr)
		}

		var snapshot struct {
			Metrics []struct {
				Name  string  `json:"name"`
				Value float64 `json:"value"`
				Type  string  `json:"type"`
			} `json:"metrics"`
		}

		ipcResponseDataErr = json.Unmarshal(ipcResponseData, &snapshot)
		if ipcResponseDataErr != nil {
			return fmt.Errorf("failed to parse metrics: %w", ipcResponseDataErr)
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

		for _, metric := range snapshot.Metrics {
			switch metric.Type {
			case "counter":
				cmd.Printf("%-40s %d\n", metric.Name, int(metric.Value))
			case "gauge":
				cmd.Printf("%-40s %.2f\n", metric.Name, metric.Value)
			default: // Assuming histogram or other time-based metrics
				cmd.Printf("%-40s %.4fs\n", metric.Name, metric.Value)
			}
		}

		cmd.Printf("\nLast updated: %s\n", time.Now().Format(time.RFC1123))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(metricsCmd)
}
