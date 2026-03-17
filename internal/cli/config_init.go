package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/configs"
	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
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

	// Respect the global --config flag when explicitly provided.
	if configPath != "" {
		cfgPath = configPath
	} else {
		configDir, err := config.DefaultConfigDir()
		if err != nil {
			return derrors.Wrap(
				err,
				derrors.CodeConfigIOFailed,
				"failed to determine config directory",
			)
		}

		cfgPath = filepath.Join(configDir, "config.toml")
	}

	// Check if config already exists
	_, statErr := os.Stat(cfgPath)
	if statErr == nil && !force {
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"config file already exists at %s (use --force to overwrite)",
			cfgPath,
		)
	}

	if statErr != nil && !os.IsNotExist(statErr) {
		return derrors.Wrap(statErr, derrors.CodeConfigIOFailed, "failed to check config file")
	}

	// Create directory
	mkdirErr := os.MkdirAll(filepath.Dir(cfgPath), config.DefaultDirPerms)
	if mkdirErr != nil {
		return derrors.Wrap(
			mkdirErr,
			derrors.CodeConfigIOFailed,
			"failed to create config directory",
		)
	}

	const filePerm = 0o644
	// Write embedded default config
	writeErr := os.WriteFile(
		cfgPath,
		configs.DefaultConfig,
		filePerm,
	)
	if writeErr != nil {
		return derrors.Wrap(writeErr, derrors.CodeConfigIOFailed, "failed to write config file")
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
