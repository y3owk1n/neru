package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/config"
)

// errConfigValidationFailed is returned when config validation fails.
// The detailed error is already printed to stderr by runConfigValidate.
var errConfigValidationFailed = errors.New("config validation failed")

// configValidateCmd is the CLI config validate command.
var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long: `Validate the Neru configuration file without starting the daemon.
Checks for syntax errors, invalid values, and configuration conflicts.
Useful for verifying config changes before reloading.
By default, searches for config in standard locations:
  $XDG_CONFIG_HOME/neru/config.toml
  ~/.config/neru/config.toml
  ~/.neru.toml
Use the global --config flag to validate a specific file.`,
	SilenceErrors: true,
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

		return &silentError{err: errConfigValidationFailed}
	}

	cmd.Println("Configuration is valid")
	cmd.Println("")
	cmd.Println("Config file: " + loadResult.ConfigPath)

	return nil
}
