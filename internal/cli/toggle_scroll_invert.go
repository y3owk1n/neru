package cli

import (
	"github.com/y3owk1n/neru/internal/domain"
)

// ToggleScrollInvertCmd toggles the scroll direction inversion setting.
var ToggleScrollInvertCmd = BuildToggleCommand(
	"toggle-scroll-invert",
	"Toggle scroll direction inversion",
	`Toggle whether scroll direction is inverted at runtime.
Useful when using tools like Mos that reverse synthetic scroll events.
The change is immediate and does not persist across restarts.

Pass --state on or --state off to set the state instead of flipping it, and
read it back from the scroll_inverted field of "neru status --json".`,
	domain.CommandToggleScrollInvert,
)

func init() {
	RootCmd.AddCommand(ToggleScrollInvertCmd)
}
