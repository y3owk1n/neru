package cli

import (
	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/core/domain/action"
)

var marksCmd = &cobra.Command{
	Use:   "marks",
	Short: "Manage cursor position marks",
	Long: `Save and recall named cursor positions (marks), similar to vim's mark system.

Marks let you name the current cursor position and jump back to it later.
They are stored in memory for the current session.

Subcommands:
  set <name>     Save the current cursor position as a named mark
  get <name>     Move the cursor to a named mark's position
  delete <name>  Delete a named mark
  clear          Delete all marks

Examples:
  neru marks set a          Mark the current position as 'a'
  neru marks get a          Jump to mark 'a'
  neru marks delete a       Delete mark 'a'
  neru marks clear          Clear all marks`,
}

var marksSetCmd = &cobra.Command{
	Use:   "set <name>",
	Short: "Save current cursor position as a named mark",
	Long: "Save the current cursor position and associate it with a name. " +
		"The mark can later be recalled with 'neru marks get <name>'.",
	Args: cobra.ExactArgs(1),
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return sendCommand(cmd, "action", []string{string(action.NameMarksSet), args[0]})
	},
}

var marksGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Move cursor to a named mark's position",
	Long: "Move the cursor to the position previously saved with 'neru marks set <name>'. " +
		"Returns an error if no mark with the given name exists.",
	Args: cobra.ExactArgs(1),
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return sendCommand(cmd, "action", []string{string(action.NameMarksGet), args[0]})
	},
}

var marksDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a named mark",
	Long:  "Delete a previously saved mark by its name. No error if the mark does not exist.",
	Args:  cobra.ExactArgs(1),
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return sendCommand(cmd, "action", []string{string(action.NameMarksDelete), args[0]})
	},
}

var marksClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete all marks",
	Long:  "Delete all saved cursor position marks.",
	Args:  cobra.NoArgs,
	PreRunE: func(_ *cobra.Command, _ []string) error {
		return requiresRunningInstance()
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		return sendCommand(cmd, "action", []string{string(action.NameMarksClear)})
	},
}

func init() {
	marksCmd.AddCommand(marksSetCmd)
	marksCmd.AddCommand(marksGetCmd)
	marksCmd.AddCommand(marksDeleteCmd)
	marksCmd.AddCommand(marksClearCmd)
	RootCmd.AddCommand(marksCmd)
}
