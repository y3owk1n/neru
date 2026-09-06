package cli

import (
	"errors"

	"github.com/spf13/cobra"
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
By default, searches for config in standard locations for your platform
(e.g. %APPDATA%/neru/config.toml then ~/.config/neru/config.toml on Windows,
or $XDG_CONFIG_HOME/neru/config.toml on Unix).
Use the global --config flag to validate a specific file.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runConfigValidate(cmd)
	},
}

func init() {
	configCmd.AddCommand(configValidateCmd)
}

func runConfigValidate(cmd *cobra.Command) error {
	svc := clientConfigLoader()

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

	printValidationWarnings(cmd, loadResult.Warnings)

	cmd.Println("")
	cmd.Println("Config file: " + loadResult.ConfigPath)

	return nil
}

// printValidationWarnings says what loaded and will not do what it says.
//
// This is the whole reason a warning is worth telling apart from a refusal: it
// does not stop the configuration loading, so without a line here it would
// exist only in the daemon's log, invisible to the command people run to check
// their configuration.
func printValidationWarnings(cmd *cobra.Command, warnings []string) {
	if len(warnings) == 0 {
		cmd.Println("Configuration is valid")

		return
	}

	cmd.Println("Configuration is valid, with warnings:")
	cmd.Println("")

	for _, warning := range warnings {
		cmd.Println("  " + warning)
	}

	cmd.Println("")
	cmd.Println("These parts of the configuration load and will not take effect.")
}
