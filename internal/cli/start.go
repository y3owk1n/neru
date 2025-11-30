package cli

var startCmd = builder.BuildSimpleCommand(
	"start",
	"Start the neru program (resume if paused)",
	`Start or resume the neru program. This enables neru if it was previously stopped.`,
	"start",
)

func init() {
	rootCmd.AddCommand(startCmd)
}
