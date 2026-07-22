package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/y3owk1n/neru/configs"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// linuxHotkeysBlock matches the default [hotkeys] section in default-config.toml.
// It is replaced with a commented-out version on Linux to avoid terminal collisions.
const linuxHotkeysBlock = `[hotkeys]
"Primary+Shift+Space" = "hints"
"Primary+Shift+G" = "grid"
"Primary+Shift+C" = "recursive_grid"
"Primary+Shift+S" = "scroll"`

const linuxHotkeysBlockComment = `# Global hotkeys are disabled by default on Linux to avoid conflicts with
# terminal and application shortcuts (e.g. Ctrl+Shift+C / copy).
# Uncomment to enable, or bind \` + "`neru <mode>`" + ` in your DE/WM.
# [hotkeys]
# "Primary+Shift+Space" = "hints"
# "Primary+Shift+G" = "grid"
# "Primary+Shift+C" = "recursive_grid"
# "Primary+Shift+S" = "scroll"`

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

	content := string(cfg)

	if runtime.GOOS == "windows" {
		content = strings.ReplaceAll(content,
			`exec_shell = "/bin/bash"`,
			`exec_shell = "C:\\Windows\\System32\\cmd.exe"`,
		)
		content = strings.ReplaceAll(content,
			`exec_shell_args = ["-lc"]`,
			`exec_shell_args = ["/c"]`,
		)
	}

	if runtime.GOOS == "linux" {
		content = strings.ReplaceAll(content,
			linuxHotkeysBlock,
			linuxHotkeysBlockComment,
		)
	}

	cfg = []byte(content)

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
