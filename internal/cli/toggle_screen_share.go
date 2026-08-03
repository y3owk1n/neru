package cli

import (
	"github.com/y3owk1n/neru/internal/domain"
)

// ToggleScreenShareCmd toggles overlay visibility in screen sharing.
var ToggleScreenShareCmd = BuildToggleCommand(
	"toggle-screen-share",
	"Toggle overlay visibility in screen sharing",
	`Toggle whether the overlay is visible during screen sharing (Zoom, Google Meet, etc.).
When hidden, the overlay will not appear in shared screens but remains visible locally.

Pass --state on to hide the overlay from screen sharing and --state off to show
it, matching the hidden_for_screen_share field of "neru status --json".`,
	domain.CommandToggleScreenShare,
)

func init() {
	RootCmd.AddCommand(ToggleScreenShareCmd)
}
