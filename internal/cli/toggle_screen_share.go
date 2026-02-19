package cli

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

// ToggleScreenShareCmd toggles overlay visibility in screen sharing.
var ToggleScreenShareCmd = BuildSimpleCommand(
	"toggle-screen-share",
	"Toggle overlay visibility in screen sharing",
	`Toggle whether the overlay is visible during screen sharing (Zoom, Google Meet, etc.).
When hidden, the overlay will not appear in shared screens but remains visible locally.`,
	domain.CommandToggleScreenShare,
)

func init() {
	RootCmd.AddCommand(ToggleScreenShareCmd)
}
