package cli

import (
	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/config"
)

// configValidateCmd is the CLI config validate command.
var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long: `Validate the Neru configuration file without starting the daemon.
Checks for syntax errors, invalid values, and configuration conflicts.
Useful for verifying config changes before reloading.
By default, searches for config in standard locations:
  ~/.config/neru/config.toml
  ~/Library/Application Support/neru/config.toml
Use the global --config flag to validate a specific file.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runConfigValidate(cmd)
	},
}

func init() {
	configCmd.AddCommand(configValidateCmd)
}

func runConfigValidate(cmd *cobra.Command) error {
	svc := config.NewService(config.DefaultConfig(), "", nil, nil)

	path := configPath
	if path == "" {
		path = svc.FindConfigFile()
	}

	if path == "" {
		cmd.Println("No config file found. Neru will use default configuration.")
		cmd.Println("")
		cmd.Println("Create one with: neru config init")

		return nil
	}

	loadResult := svc.LoadWithValidation(path)
	if loadResult.ValidationError != nil {
		cmd.PrintErrln("Configuration validation failed:")
		cmd.PrintErrln("")
		cmd.PrintErrln("  " + loadResult.ValidationError.Error())
		cmd.PrintErrln("")
		cmd.PrintErrln("Config file: " + loadResult.ConfigPath)

		return loadResult.ValidationError
	}

	cmd.Println("Configuration is valid")
	cmd.Println("")
	cmd.Println("Config file: " + loadResult.ConfigPath)

	return nil
}
