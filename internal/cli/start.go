package cli

// StartCmd is the CLI start command.
var StartCmd = BuildSimpleCommand(
	"start",
	"Resume Neru after being stopped",
	`Resume the Neru daemon after it was paused with 'neru stop'.

This re-enables all navigation modes and actions without restarting
the daemon process. Use 'neru stop' to pause.`,
	"start",
)

func init() {
	RootCmd.AddCommand(StartCmd)
}
