package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// FindNormalizedMapKey returns the existing map key in m whose normalized form
// matches the normalized form of rawKey, with Primary resolved by primary. If
// no match is found it returns rawKey itself so callers can use the result
// directly.
func FindNormalizedMapKey[V any](
	m map[string]V,
	rawKey string,
	primary PrimaryModifierConfig,
) string {
	norm := primary.NormalizeKey(rawKey)
	for k := range m {
		if primary.NormalizeKey(k) == norm {
			return k
		}
	}

	return rawKey
}

// DefaultConfigDir returns the preferred directory for the Neru config file.
// $XDG_CONFIG_HOME/neru wins on every platform when the variable is set, for
// users of cross-platform shell environments. Otherwise Windows uses
// %APPDATA%/neru, falling back to ~/AppData/Roaming/neru, and Unix uses
// ~/.config/neru. FindConfigFile and config init both use it, and it is where
// a config file is written. FindConfigFile also reads ~/.config/neru on
// Windows, so a config carried over from a Unix dotfiles repo is still picked up.
func DefaultConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "neru"), nil
	}

	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}

			appData = filepath.Join(home, "AppData", "Roaming")
		}

		return filepath.Join(appData, "neru"), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".config", "neru"), nil
}
