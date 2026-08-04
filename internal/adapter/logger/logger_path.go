package logger

import (
	"os"
	"path/filepath"
	"runtime"
)

// defaultLogFilePath returns the platform default neru log file path when
// [logging].log_file is empty.
func defaultLogFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	var logDir string

	switch runtime.GOOS {
	case "darwin":
		logDir = filepath.Join(homeDir, "Library", "Logs", "neru")
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(homeDir, "AppData", "Local")
		}

		logDir = filepath.Join(localAppData, "neru", "log")
	default:
		// The rest are Linux, BSD, etc.: honor $XDG_STATE_HOME per the XDG
		// Base Directory spec (absolute values only), matching the Linux
		// system adapter's LogDir.
		stateHome := os.Getenv("XDG_STATE_HOME")
		if !filepath.IsAbs(stateHome) {
			stateHome = filepath.Join(homeDir, ".local", "state")
		}

		logDir = filepath.Join(stateHome, "neru", "log")
	}

	return filepath.Join(logDir, "app.log"), nil
}
