package cli

// StopCmd is the CLI stop command.
var StopCmd = BuildSimpleCommand(
	"stop",
	"Pause Neru (daemon stays running)",
	`Pause the Neru daemon. All navigation modes and actions are disabled,
but the daemon process remains running in the background.

Use 'neru start' to resume functionality.
Use 'neru status' to check whether Neru is active or paused.`,
	"stop",
)

func init() {
	RootCmd.AddCommand(StopCmd)
}
