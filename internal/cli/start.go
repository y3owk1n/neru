package cli

// StartCmd is the CLI start command.
var StartCmd = BuildSimpleCommand(
	"start",
	"Start the neru program (resume if paused)",
	`Start or resume the neru program. This enables neru if it was previously stopped.`,
	"start",
)

func init() {
	RootCmd.AddCommand(StartCmd)
}
