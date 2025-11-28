package cli

import (
	"github.com/spf13/cobra"
)

var launchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch the neru program",
	Long:  `Launch the neru program. Same as running 'neru' without any subcommand.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		launchProgram(cmd, configPath)

		return nil
	},
}

func init() {
	launchCmd.Flags().StringVar(&configPath, "config", "", "config file path")

	rootCmd.AddCommand(launchCmd)
}
