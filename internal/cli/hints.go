package cli

// HintsCmd is the CLI hints command.
var HintsCmd = BuildModeCommand(ModeConfig{
	Name:       "hints",
	Short:      "Launch hints mode",
	Long:       `Activate hint mode for direct clicking on UI elements.`,
	ActionDesc: "hint selection",
})

func init() {
	RootCmd.AddCommand(HintsCmd)
}
