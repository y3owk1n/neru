package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	derrors "github.com/y3owk1n/neru/internal/errors"
	"github.com/y3owk1n/neru/internal/infra/logger"
	"go.uber.org/zap"
)

// LoadWithValidation loads configuration from the specified path and returns both
// the config and any validation error separately. This allows callers to decide
// how to handle validation failures (e.g., show alert and use default config).
func LoadWithValidation(path string) *LoadResult {
	configResult := &LoadResult{
		Config:     DefaultConfig(),
		ConfigPath: path,
	}

	if path == "" {
		configResult.ConfigPath = FindConfigFile()
	}

	logger.Info("Loading config from", zap.String("path", configResult.ConfigPath))

	_, statErr := os.Stat(configResult.ConfigPath)
	if os.IsNotExist(statErr) {
		logger.Info("Config file not found, using default configuration")

		return configResult
	}

	_, decodeErr := toml.DecodeFile(configResult.ConfigPath, configResult.Config)
	if decodeErr != nil {
		configResult.ValidationError = derrors.Wrap(
			decodeErr,
			derrors.CodeInvalidConfig,
			"failed to parse config file",
		)
		configResult.Config = DefaultConfig()

		return configResult
	}

	var raw map[string]map[string]any

	_, anotherDecodeErr := toml.DecodeFile(configResult.ConfigPath, &raw)
	if anotherDecodeErr == nil {
		if hot, ok := raw["hotkeys"]; ok {
			if len(hot) > 0 {
				// Clear default bindings when user provides hotkeys config
				configResult.Config.Hotkeys.Bindings = map[string]string{}
			}

			for key, value := range hot {
				str, ok := value.(string)
				if !ok {
					configResult.ValidationError = derrors.Newf(
						derrors.CodeInvalidConfig,
						"hotkeys.%s must be a string action",
						key,
					)
					configResult.Config = DefaultConfig()

					return configResult
				}

				configResult.Config.Hotkeys.Bindings[key] = str
			}
		}
	}

	validateErr := configResult.Config.Validate()
	if validateErr != nil {
		configResult.ValidationError = derrors.Wrap(
			validateErr,
			derrors.CodeInvalidConfig,
			"invalid configuration",
		)
		configResult.Config = DefaultConfig()

		return configResult
	}

	logger.Info("Configuration loaded successfully")

	return configResult
}

// FindConfigFile searches for a configuration file in standard locations.
// Returns the path to the config file, or an empty string if not found.
func FindConfigFile() string {
	// Try XDG config directory first
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		path := filepath.Join(xdgConfig, "neru", "config.toml")

		_, err := os.Stat(path)
		if err == nil {
			return path
		}
	}

	// Try standard config directory
	homeDir, homeErr := os.UserHomeDir()
	if homeErr == nil {
		// Try .config/neru/config.toml
		path := filepath.Join(homeDir, ".config", "neru", "config.toml")

		_, err := os.Stat(path)
		if err == nil {
			return path
		}

		// Try .neru.toml
		path = filepath.Join(homeDir, ".neru.toml")

		_, err = os.Stat(path)
		if err == nil {
			return path
		}
	}

	// Try current directory
	_, err := os.Stat("neru.toml")
	if err == nil {
		return "neru.toml"
	}

	// Try config.toml
	_, err = os.Stat("config.toml")
	if err == nil {
		return "config.toml"
	}

	return ""
}
