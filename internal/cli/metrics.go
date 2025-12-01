package cli

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/core/domain"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

// MetricsCmd is the CLI metrics command.
var MetricsCmd = &cobra.Command{
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
			return derrors.Wrap(ipcResponseErr, derrors.CodeIPCFailed, "failed to get metrics")
		}

		if !ipcResponse.Success {
			return derrors.New(derrors.CodeIPCFailed, ipcResponse.Message)
		}

		if ipcResponse.Data == nil {
			cmd.Println("No metrics recorded yet")

			return nil
		}

		// Decode metrics
		ipcResponseData, ipcResponseDataErr := json.Marshal(ipcResponse.Data)
		if ipcResponseDataErr != nil {
			return derrors.Wrap(
				ipcResponseDataErr,
				derrors.CodeSerializationFailed,
				"failed to marshal metrics data",
			)
		}

		var metrics []struct {
			Name  string  `json:"name"`
			Value float64 `json:"value"`
			Type  int     `json:"type"`
		}

		ipcResponseDataErr = json.Unmarshal(ipcResponseData, &metrics)
		if ipcResponseDataErr != nil {
			return derrors.Wrap(
				ipcResponseDataErr,
				derrors.CodeSerializationFailed,
				"failed to parse metrics",
			)
		}

		if len(metrics) == 0 {
			cmd.Println("No metrics recorded yet")

			return nil
		}

		// Sort metrics by name
		sort.Slice(metrics, func(i, j int) bool {
			return metrics[i].Name < metrics[j].Name
		})

		cmd.Println("📊 Application Metrics:")
		cmd.Println("-----------------------")

		for _, metric := range metrics {
			switch metric.Type {
			case 0: // TypeCounter
				cmd.Printf("%-40s %d\n", metric.Name, int(metric.Value))
			case 1: // TypeGauge
				cmd.Printf("%-40s %.2f\n", metric.Name, metric.Value)
			default: // TypeHistogram
				cmd.Printf("%-40s %.4fs\n", metric.Name, metric.Value)
			}
		}

		cmd.Printf("\nLast updated: %s\n", time.Now().Format(time.RFC1123))

		return nil
	},
}

func init() {
	RootCmd.AddCommand(MetricsCmd)
}
