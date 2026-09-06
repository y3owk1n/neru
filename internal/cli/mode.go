package cli

import "github.com/y3owk1n/neru/internal/domain"

// ModeCmd is the CLI command that enters a user-declared mode.
var ModeCmd = BuildModeCommand(ModeConfig{
	Mode:  domain.ModeCustom,
	Short: "Enter a mode declared in your config",
	Long: `Activate a mode you declared under [modes.<name>] in config.toml.

A declared mode has no logic of its own: it captures the keyboard, shows its
indicator, and answers every key from its own [modes.<name>.hotkeys] table.
Escape returns to idle unless the table rebinds it.

Examples:
  neru mode window           Enter the mode declared as [modes.window]
  neru mode window --toggle  Toggle it on/off`,
})

func init() {
	ModeCmd.Use = domain.ModeNameCustom + " <name>"

	RootCmd.AddCommand(ModeCmd)
}
