package cli

import "github.com/y3owk1n/neru/internal/domain"

// IdleCmd is the CLI idle command.
//
// Idle is the one mode that is a departure rather than an arrival, so it
// accepts no flags at all — and, going through the same builder as the rest,
// says so instead of forwarding what it was given to a daemon that would drop
// it.
var IdleCmd = BuildModeCommand(ModeConfig{
	Mode:  domain.ModeIdle,
	Short: "Exit the current navigation mode",
	Long: `Exit the current navigation mode (hints, grid, recursive-grid, scroll)
and return to idle state. Useful for scripting mode transitions.`,
})

func init() {
	RootCmd.AddCommand(IdleCmd)
}
