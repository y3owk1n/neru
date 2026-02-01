package cli

// GridCmd is the CLI grid command.
var GridCmd = BuildModeCommand(ModeConfig{
	Name:       "grid",
	Short:      "Launch grid mode",
	Long:       `Activate grid mode for mouseless navigation.`,
	ActionDesc: "grid selection",
})

func init() {
	RootCmd.AddCommand(GridCmd)
}
