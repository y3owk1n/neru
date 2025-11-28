package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Show profiling information",
	Long:  `Display information about enabling and accessing Go pprof profiling.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		fmt.Println("Neru Profiling:")
		fmt.Println("  To enable profiling, set the NERU_PPROF environment variable:")
		fmt.Println("    export NERU_PPROF=:6060")
		fmt.Println("  Then restart Neru:")
		fmt.Println("    neru stop && NERU_PPROF=:6060 neru launch")
		fmt.Println("  Access profiles at: http://localhost:6060/debug/pprof/")
		fmt.Println("  Heap profile: http://localhost:6060/debug/pprof/heap")
		fmt.Println("  CPU profile: http://localhost:6060/debug/pprof/profile?seconds=30")
		fmt.Println("  Use 'go tool pprof' to analyze profiles")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(profileCmd)
}
