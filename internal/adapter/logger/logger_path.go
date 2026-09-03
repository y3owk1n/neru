package logger

import (
	"os"
	"path/filepath"
	"runtime"
)

// defaultLogFilePath returns the platform default neru log file path when
// [logging].log_file is empty.
func defaultLogFilePath() (string, error) {
	logDir, err := DefaultLogDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(logDir, "app.log"), nil
}

// DefaultLogDir returns the per-user directory neru writes its logs to. It is
// exported because the daemon's log file is not the only thing that belongs
// there: a service definition redirecting the daemon's stderr needs the same
// directory, and a second spelling of it would be a second answer to where
// neru's logs live.
func DefaultLogDir() (string, error) {
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

	return logDir, nil
}
