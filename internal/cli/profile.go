package cli

import (
	"github.com/spf13/cobra"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Show profiling information",
	Long:  `Display information about enabling and accessing Go pprof profiling.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		logger.Info("Neru Profiling:")
		logger.Info("  To enable profiling, set the NERU_PPROF environment variable:")
		logger.Info("    export NERU_PPROF=:6060")
		logger.Info("  Then restart Neru:")
		logger.Info("    neru stop && NERU_PPROF=:6060 neru launch")
		logger.Info("  Access profiles at: http://localhost:6060/debug/pprof/")
		logger.Info("  Heap profile: http://localhost:6060/debug/pprof/heap")
		logger.Info("  CPU profile: http://localhost:6060/debug/pprof/profile?seconds=30")
		logger.Info("  Use 'go tool pprof' to analyze profiles")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(profileCmd)
}
