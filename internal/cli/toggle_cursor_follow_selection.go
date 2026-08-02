package cli

import "github.com/y3owk1n/neru/internal/domain"

// ToggleCursorFollowSelectionCmd toggles cursor-follow-selection for the active mode session.
var ToggleCursorFollowSelectionCmd = BuildToggleCommand(
	"toggle-cursor-follow-selection",
	"Toggle cursor-follow-selection in the active mode",
	`Toggle whether the active hints, grid, or recursive-grid session follows the current selection with the real cursor.

Pass --state on or --state off to set the state instead of flipping it. The
preference belongs to the running mode session, so the command fails when no
mode is active and "neru status --json" reports cursor_follow_selection as null.`,
	domain.CommandToggleCursorFollowSelection,
)

func init() {
	RootCmd.AddCommand(ToggleCursorFollowSelectionCmd)
}
