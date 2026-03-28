package cli

import "github.com/y3owk1n/neru/internal/core/domain"

// ToggleCursorFollowSelectionCmd toggles cursor-follow-selection for the active mode session.
var ToggleCursorFollowSelectionCmd = BuildSimpleCommand(
	"toggle-cursor-follow-selection",
	"Toggle cursor-follow-selection in the active mode",
	`Toggle whether the active hints, grid, or recursive-grid session follows the current selection with the real cursor.`,
	domain.CommandToggleCursorFollowSelection,
)

func init() {
	RootCmd.AddCommand(ToggleCursorFollowSelectionCmd)
}
