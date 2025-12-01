package cli

import (
	"github.com/spf13/cobra"
)

// ProfileCmd is the CLI profile command.
var ProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Show profiling information",
	Long:  `Display information about enabling and accessing Go pprof profiling.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.Println("Neru Profiling:")
		cmd.Println("  To enable profiling, set the NERU_PPROF environment variable:")
		cmd.Println("    export NERU_PPROF=:6060")
		cmd.Println("  Then restart Neru:")
		cmd.Println("    neru stop && NERU_PPROF=:6060 neru launch")
		cmd.Println("  Access profiles at: http://localhost:6060/debug/pprof/")
		cmd.Println("  Heap profile: http://localhost:6060/debug/pprof/heap")
		cmd.Println("  CPU profile: http://localhost:6060/debug/pprof/profile?seconds=30")
		cmd.Println("  Use 'go tool pprof' to analyze profiles")

		return nil
	},
}

func init() {
	RootCmd.AddCommand(ProfileCmd)
}
