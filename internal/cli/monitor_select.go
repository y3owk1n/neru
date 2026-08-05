package cli

import "github.com/y3owk1n/neru/internal/domain"

// MonitorSelectCmd is the CLI command for interactive monitor picking.
var MonitorSelectCmd = BuildModeCommand(ModeConfig{
	Mode:  domain.ModeMonitorSelect,
	Short: "Launch monitor selection mode",
	Long: `Activate monitor_select mode for interactive display picking.

If only one monitor is available, the command is a no-op.

Keys:
  Type label    Select monitor immediately when unique

Examples:
  neru monitor_select
  neru monitor_select --toggle`,
})

func init() {
	RootCmd.AddCommand(MonitorSelectCmd)
}
