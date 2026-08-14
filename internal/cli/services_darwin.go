//go:build darwin

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/y3owk1n/neru/internal/adapter/logger"
)

const (
	serviceLabel    = "com.y3owk1n.neru"
	launchAgentsDir = "~/Library/LaunchAgents"
	plistFile       = launchAgentsDir + "/" + serviceLabel + ".plist"
)

// daemonStderrFileName is the file the login agent's standard error is
// redirected to, inside the per-user log directory the logger already owns.
//
// Standard output is deliberately not redirected: the logger's console core
// writes every log line there, so a redirect would duplicate app.log into a
// second, unrotated file. Standard error carries what app.log cannot — a panic,
// a native crash, or a startup failure raised before the file sink exists — so
// it is the half worth keeping.
const daemonStderrFileName = "daemon.err.log"

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.y3owk1n.neru</string>
    <key>ProgramArguments</key>
    <array>
        <string>NERU_BINARY_PATH</string>
        <string>launch</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardErrorPath</key>
    <string>NERU_STDERR_PATH</string>
    <key>ProcessType</key>
    <string>Interactive</string>
    <key>LimitLoadToSessionType</key>
    <string>Aqua</string>
    <key>Nice</key>
    <integer>-10</integer>
    <key>ThrottleInterval</key>
    <integer>10</integer>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin</string>
    </dict>
</dict>
</plist>`

var (
	errServiceAlreadyLoaded = errors.New(
		"service is already loaded; check for existing installations (e.g., nix-darwin, home-manager) and uninstall them first",
	)
	errPlistAlreadyExists = errors.New("plist file already exists")
)

// daemonStderrPath returns the absolute file the login agent's stderr is
// redirected to. The plist is read by launchd, which expands nothing, so this
// resolves the home directory rather than writing a "~" into it.
func daemonStderrPath() (string, error) {
	logDir, err := logger.DefaultLogDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(logDir, daemonStderrFileName), nil
}

// plistTextEscaper escapes what cannot appear literally inside a plist
// <string> element. A filesystem path is arbitrary text as far as XML is
// concerned — a directory called "A&B" is perfectly legal on macOS — and an
// unescaped one produces a plist launchctl refuses to load, leaving a file
// behind that the next install then refuses to write over.
var plistTextEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// renderPlist fills the launchd agent template in with the two absolute paths
// launchd cannot work out for itself: the binary to run, and the file its
// standard error is appended to.
func renderPlist(binPath, stderrPath string) string {
	return strings.NewReplacer(
		"NERU_BINARY_PATH", plistTextEscaper.Replace(binPath),
		"NERU_STDERR_PATH", plistTextEscaper.Replace(stderrPath),
	).Replace(plistTemplate)
}

func getBinaryPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	return filepath.EvalSymlinks(execPath)
}

func isServiceLoaded() bool {
	cmd := exec.CommandContext(context.Background(), "launchctl", "list", serviceLabel)

	return cmd.Run() == nil
}

func installService() error {
	// Check if service is already loaded
	if isServiceLoaded() {
		return errServiceAlreadyLoaded
	}

	binPath, err := getBinaryPath()
	if err != nil {
		return fmt.Errorf("failed to get binary path: %w", err)
	}

	// launchd opens the redirect itself, before neru runs, and it creates the
	// file but never the directory holding it — so the directory has to exist
	// by the time the agent is bootstrapped or the first launch's stderr, which
	// is the launch most likely to fail, goes nowhere.
	stderrPath, err := daemonStderrPath()
	if err != nil {
		return fmt.Errorf("failed to resolve the log directory: %w", err)
	}

	err = os.MkdirAll(filepath.Dir(stderrPath), logger.DefaultDirPerms)
	if err != nil {
		return fmt.Errorf("failed to create the log directory: %w", err)
	}

	plistContent := renderPlist(binPath, stderrPath)

	expandedDir, err := expandPath(launchAgentsDir)
	if err != nil {
		return fmt.Errorf("failed to expand LaunchAgents path: %w", err)
	}

	// Check if plist already exists
	expandedPlist := filepath.Join(expandedDir, serviceLabel+".plist")

	_, statErr := os.Stat(expandedPlist)
	if statErr == nil {
		return fmt.Errorf(
			"%w at %s; remove it manually or uninstall first",
			errPlistAlreadyExists,
			expandedPlist,
		)
	}

	// Ensure directory exists
	const dirPerm = 0o755

	err = os.MkdirAll(expandedDir, dirPerm)
	if err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	// Write plist
	const filePerm = 0o644

	err = os.WriteFile(expandedPlist, []byte(plistContent), filePerm)
	if err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Load service
	cmd := exec.CommandContext(
		context.Background(),
		"launchctl",
		"bootstrap",
		"gui/"+currentUser.Uid,
		expandedPlist,
	)

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to load service: %w", err)
	}

	return nil
}

func uninstallService() error {
	expandedPlist, err := expandPath(plistFile)
	if err != nil {
		return fmt.Errorf("failed to expand plist path: %w", err)
	}

	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Unload service if loaded
	cmd := exec.CommandContext(
		context.Background(),
		"launchctl",
		"bootout",
		"gui/"+currentUser.Uid+"/"+serviceLabel,
	)
	_ = cmd.Run() // Ignore error if not loaded

	err = os.Remove(expandedPlist)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist: %w", err)
	}

	return nil
}

func startService() error {
	cmd := exec.CommandContext(context.Background(), "launchctl", "start", serviceLabel)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

func stopService() error {
	cmd := exec.CommandContext(context.Background(), "launchctl", "stop", serviceLabel)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	return nil
}

func restartService() error {
	_ = stopService()

	// Always attempt to start
	return startService()
}

func statusService() string {
	cmd := exec.CommandContext(context.Background(), "launchctl", "list", serviceLabel)

	_, err := cmd.Output()
	if err != nil {
		return "Service not loaded"
	}

	return "Service loaded"
}

func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		return filepath.Join(home, path[1:]), nil
	}

	return path, nil
}
