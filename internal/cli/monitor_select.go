package cli

import "github.com/spf13/cobra"

// MonitorSelectCmd is the CLI command for interactive monitor picking.
var MonitorSelectCmd = &cobra.Command{
	Use:   "monitor_select",
	Short: "Launch monitor selection mode",
	Long: `Activate monitor_select mode for interactive display picking.

If only one monitor is available, the command is a no-op.

Keys:
  Type label    Select monitor immediately when unique

Examples:
  neru monitor_select
  neru monitor_select --toggle`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		toggleFlag, err := cmd.Flags().GetBool("toggle")
		if err != nil {
			return err
		}

		params := []string{}
		if toggleFlag {
			params = append(params, "--toggle")
		}

		return sendCommand(cmd, "monitor_select", params)
	},
}

func init() {
	MonitorSelectCmd.Flags().BoolP(
		"toggle", "t", false,
		"Toggle monitor_select mode on/off (exit to idle if already active)",
	)

	RootCmd.AddCommand(MonitorSelectCmd)
}
