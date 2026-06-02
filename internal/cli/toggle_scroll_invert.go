package cli

import (
	"github.com/y3owk1n/neru/internal/core/domain"
)

// ToggleScrollInvertCmd toggles the scroll direction inversion setting.
var ToggleScrollInvertCmd = BuildSimpleCommand(
	"toggle-scroll-invert",
	"Toggle scroll direction inversion",
	`Toggle whether scroll direction is inverted at runtime.
Useful when using tools like Mos that reverse synthetic scroll events.
The change is immediate and does not persist across restarts.`,
	domain.CommandToggleScrollInvert,
)

func init() {
	RootCmd.AddCommand(ToggleScrollInvertCmd)
}
