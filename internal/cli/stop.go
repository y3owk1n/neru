package cli

// StopCmd is the CLI stop command.
var StopCmd = BuildSimpleCommand(
	"stop",
	"Pause the neru program (does not quit)",
	`Pause the neru program. This disables neru functionality but keeps it running in the background.`,
	"stop",
)

func init() {
	RootCmd.AddCommand(StopCmd)
}
