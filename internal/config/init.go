package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/y3owk1n/neru/configs"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// WriteDefaultConfig writes the default configuration to the specified path.
// If force is false and the file already exists, it returns an error.
func WriteDefaultConfig(cfgPath string, force bool) error {
	_, statErr := os.Stat(cfgPath)
	if statErr == nil && !force {
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"config file already exists at %s",
			cfgPath,
		)
	}

	if statErr != nil && !os.IsNotExist(statErr) {
		return derrors.Wrap(statErr, derrors.CodeConfigIOFailed, "failed to check config file")
	}

	mkdirErr := os.MkdirAll(filepath.Dir(cfgPath), DefaultDirPerms)
	if mkdirErr != nil {
		return derrors.Wrap(
			mkdirErr,
			derrors.CodeConfigIOFailed,
			"failed to create config directory",
		)
	}

	writeErr := os.WriteFile(cfgPath, platformDefaultConfig(), DefaultFilePerms)
	if writeErr != nil {
		return derrors.Wrap(writeErr, derrors.CodeConfigIOFailed, "failed to write config file")
	}

	return nil
}

// platformDefaultConfig returns the embedded default config with platform-specific
// adjustments applied (e.g. exec_shell path on Windows).
func platformDefaultConfig() []byte {
	cfg := configs.DefaultConfig
	if runtime.GOOS == "windows" {
		content := string(cfg)
		content = strings.ReplaceAll(content,
			`exec_shell = "/bin/bash"`,
			`exec_shell = "C:\\Windows\\System32\\cmd.exe"`,
		)
		content = strings.ReplaceAll(content,
			`exec_shell_args = ["-lc"]`,
			`exec_shell_args = ["/c"]`,
		)
		cfg = []byte(content)
	}

	return cfg
}

// DefaultConfigPath returns the default configuration file path.
func DefaultConfigPath() (string, error) {
	configDir, err := DefaultConfigDir()
	if err != nil {
		return "", derrors.Wrap(
			err,
			derrors.CodeConfigIOFailed,
			"failed to determine config directory",
		)
	}

	return filepath.Join(configDir, "config.toml"), nil
}
