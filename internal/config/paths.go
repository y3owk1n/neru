package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// FindNormalizedMapKey returns the existing map key in m whose normalized form
// matches the normalized form of rawKey. If no match is found it returns rawKey
// itself so callers can use the result directly.
func FindNormalizedMapKey[V any](m map[string]V, rawKey string) string {
	norm := NormalizeKeyForComparison(rawKey)
	for k := range m {
		if NormalizeKeyForComparison(k) == norm {
			return k
		}
	}

	return rawKey
}

// DefaultConfigDir returns the preferred directory for the Neru config file.
// On Windows it uses %APPDATA%/neru (falling back to ~/AppData/Roaming/neru);
// on Unix it checks $XDG_CONFIG_HOME/neru first, falling back to ~/.config/neru.
// The $XDG_CONFIG_HOME environment variable is also respected on Windows when set,
// for users of cross-platform shell environments.
// This is the single source of truth for the primary config location,
// used by both FindConfigFile and config init. It is where a config file is
// written; FindConfigFile additionally reads ~/.config/neru on Windows, so a
// config carried over from a Unix dotfiles repo is still picked up.
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
