package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/cli/cliutil"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Ask the daemon what it sees right now",
	Long: `Print what the running daemon sees right now, for scripts and other tools
that drive Neru.

Positions and sizes are in the coordinates 'neru action move_mouse' takes,
so a script can pass them straight back.

Subcommands:
  displays   Every connected display, with its position, size and scale
  cursor     Where the cursor is, and which display holds it
  window     The focused window's position and size
  app        The focused application, as per-app config names it

Each takes --json to print a JSON object instead.`,
}

var queryDisplaysCmd = &cobra.Command{
	Use:   "displays",
	Short: "List the connected displays",
	Long: `List every connected display with its position, size and scale, in the
order the platform enumerates them.

The name is the one 'neru action move_monitor --name' takes. Two identical
monitors can share a name, so the position in the list is what tells them
apart.

Examples:
  neru query displays
  neru query displays --json`,
	Args: cobra.NoArgs,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runQuery(cmd, domain.CommandQueryDisplays, printDisplays)
	},
}

var queryCursorCmd = &cobra.Command{
	Use:   "cursor",
	Short: "Print the cursor position",
	Long: `Print where the cursor is, and which display holds it.

The display is named as 'neru query displays' lists it, by name and by its
position in that list. Both are null in --json when no display holds the
cursor.

Examples:
  neru query cursor
  neru query cursor --json`,
	Args: cobra.NoArgs,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runQuery(cmd, domain.CommandQueryCursor, printCursor)
	},
}

var queryWindowCmd = &cobra.Command{
	Use:   "window",
	Short: "Print the focused window's bounds",
	Long: `Print the focused window's position and size.

When no window has focus, such as when the desktop does, it says so, and
--json prints null.

Examples:
  neru query window
  neru query window --json`,
	Args: cobra.NoArgs,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runQuery(cmd, domain.CommandQueryWindow, printWindow)
	},
}

var queryAppCmd = &cobra.Command{
	Use:   "app",
	Short: "Print the focused application's bundle_id",
	Long: `Print the focused application as per-app config names it. This is the
exact string to write as bundle_id in [[app_configs]], excluded_apps and every
per-mode app_configs table.

Examples:
  neru query app
  neru query app --json`,
	Args: cobra.NoArgs,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runQuery(cmd, domain.CommandQueryApp, printApp)
	},
}

// runQuery sends one query and prints its payload, as JSON with --json and
// through render otherwise.
func runQuery[T any](
	cmd *cobra.Command,
	action string,
	render func(*cobra.Command, T),
) error {
	asJSON, flagErr := cmd.Flags().GetBool("json")
	if flagErr != nil {
		return flagErr
	}

	communicator := cliutil.NewIPCCommunicator(timeoutSec)

	ipcResponse, err := communicator.SendCommand(action, []string{})
	if err != nil {
		return err
	}

	if !ipcResponse.Success {
		return communicator.HandleResponse(cmd, ipcResponse)
	}

	// The payload arrives decoded into maps, so it is read back into its
	// type, which keeps the JSON keys in the order the type declares.
	var data T

	decodeErr := decodePayload(ipcResponse.Data, &data)
	if decodeErr != nil {
		return decodeErr
	}

	if asJSON {
		return formatter.PrintJSON(cmd, data)
	}

	render(cmd, data)

	return nil
}

// decodePayload reads a response payload into out.
func decodePayload(data any, out any) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return derrors.Wrap(err, derrors.CodeSerializationFailed, "failed to read query response")
	}

	err = json.Unmarshal(encoded, out)
	if err != nil {
		return derrors.Wrap(err, derrors.CodeSerializationFailed, "failed to read query response")
	}

	return nil
}

func printDisplays(cmd *cobra.Command, data ipc.DisplaysData) {
	for idx, display := range data.Displays {
		cmd.Printf("%d: %s\n", idx, display.Name)
		cmd.Printf("  Position: %d,%d\n", display.X, display.Y)
		cmd.Printf("  Size: %dx%d\n", display.Width, display.Height)
		cmd.Printf("  Scale: %g\n", display.Scale)
	}
}

func printCursor(cmd *cobra.Command, data ipc.CursorData) {
	cmd.Printf("Position: %d,%d\n", data.X, data.Y)

	if data.Display == nil || data.DisplayIndex == nil {
		cmd.Println("Display: none")

		return
	}

	cmd.Printf("Display: %d: %s\n", *data.DisplayIndex, *data.Display)
}

func printWindow(cmd *cobra.Command, data *ipc.WindowData) {
	if data == nil {
		cmd.Println("No window has focus")

		return
	}

	cmd.Printf("Position: %d,%d\n", data.X, data.Y)
	cmd.Printf("Size: %dx%d\n", data.Width, data.Height)
}

func printApp(cmd *cobra.Command, data ipc.AppData) {
	cmd.Println("bundle_id: " + data.BundleID)
}

func init() {
	for _, sub := range []*cobra.Command{
		queryDisplaysCmd,
		queryCursorCmd,
		queryWindowCmd,
		queryAppCmd,
	} {
		sub.Flags().Bool("json", false, "Print the answer as a JSON object")
		queryCmd.AddCommand(sub)
	}

	RootCmd.AddCommand(queryCmd)
}
