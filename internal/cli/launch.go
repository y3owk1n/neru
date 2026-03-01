package cli

import (
	"github.com/spf13/cobra"
)

// LaunchCmd is the CLI launch command.
var LaunchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch the neru program",
	Long:  `Launch the neru program. Same as running 'neru' without any subcommand.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		launchProgram(cmd, configPath)

		return nil
	},
}

func init() {
	LaunchCmd.Flags().StringVarP(&configPath, "config", "c", "", "config file path")

	RootCmd.AddCommand(LaunchCmd)
}
