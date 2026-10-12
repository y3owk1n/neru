package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/cli/cliutil"
	"github.com/y3owk1n/neru/internal/domain"
)

// WatchCmd is the CLI watch command.
var WatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Stream Neru's status and events as JSON lines",
	Long: `Print Neru's status, then one JSON object per line for every lifecycle
event as it happens: a mode opening or closing, another app coming to the
front, a pause or resume, a config reload, Mission Control opening or closing (macOS).

The first line is the status as neru status --json prints it, under "status",
with the event "snapshot". Every line carries a "seq" that rises by one per
event. A reader that falls behind gets a fresh snapshot in place of the
events it missed, carrying their count in "dropped".

With --event, only the events named are printed, along with every snapshot.
"seq" then skips the events left out. Repeat --event or separate names with
commas.

The stream runs until the daemon exits, and then neru watch exits 0. It also
runs while Neru is stopped. --timeout applies to connecting only.

Examples:
  neru watch
  neru watch --event mode_enter,mode_exit
  neru watch | jq --unbuffered -r 'select(.event == "mode_enter").mode'`,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		out := cmd.OutOrStdout()

		events, flagErr := cmd.Flags().GetStringArray("event")
		if flagErr != nil {
			return flagErr
		}

		args := make([]string, 0, len(events))
		for _, value := range events {
			args = append(args, "--event="+value)
		}

		response, err := ipc.NewClient().Stream(
			ipc.Command{Action: domain.CommandWatch, Args: args},
			time.Duration(timeoutSec)*time.Second,
			func(line json.RawMessage) error {
				_, writeErr := fmt.Fprintf(out, "%s\n", line)

				return writeErr
			},
		)
		if err != nil {
			return err
		}

		if !response.Success {
			return cliutil.NewIPCCommunicator(timeoutSec).HandleResponse(cmd, response)
		}

		return nil
	},
}

func init() {
	WatchCmd.Flags().StringArray("event", nil,
		"Print only these events, comma-separated, such as mode_enter,mode_exit. Repeat to add more")

	RootCmd.AddCommand(WatchCmd)
}
