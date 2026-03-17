package cli

import (
	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/config"
)

// configInitCmd is the CLI config init command for creating a default configuration file.
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default configuration file",
	Long: `Create a default configuration file.
The config is written to $XDG_CONFIG_HOME/neru/config.toml when
XDG_CONFIG_HOME is set, otherwise ~/.config/neru/config.toml.
Use the global --config flag to write to a custom path instead.
This copies the fully-commented default configuration to get you started.
If a config file already exists, use --force to overwrite it.
After running this command, start Neru with 'neru launch' and try:
  Cmd+Shift+C      Recursive Grid mode (recommended)
  Cmd+Shift+Space   Hints mode
  Cmd+Shift+S       Scroll mode
  Escape            Exit any mode`,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		force, _ := cmd.Flags().GetBool("force")

		return runConfigInit(cmd, force)
	},
}

func init() {
	configInitCmd.Flags().BoolP("force", "f", false, "Overwrite existing config file")
	configCmd.AddCommand(configInitCmd)
}

func runConfigInit(cmd *cobra.Command, force bool) error {
	var cfgPath string

	if configPath != "" {
		cfgPath = configPath
	} else {
		path, err := config.DefaultConfigPath()
		if err != nil {
			return err
		}

		cfgPath = path
	}

	err := config.WriteDefaultConfig(cfgPath, force)
	if err != nil {
		return err
	}

	cmd.Println("Created config at " + cfgPath)
	cmd.Println("")
	cmd.Println("Quick start:")
	cmd.Println("  1. Start Neru:          neru launch")
	cmd.Println("  2. Try Recursive Grid:  Cmd+Shift+C")
	cmd.Println("  3. Try Hints mode:      Cmd+Shift+Space")
	cmd.Println("  4. Try Scroll mode:     Cmd+Shift+S")
	cmd.Println("  5. Exit any mode:       Escape")
	cmd.Println("")
	cmd.Println("Edit the config file to customize hotkeys, colors, and behavior.")
	cmd.Println("Full reference: https://github.com/y3owk1n/neru/blob/main/docs/CONFIGURATION.md")

	return nil
}
