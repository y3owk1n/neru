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
)

const (
	serviceLabel    = "com.y3owk1n.neru"
	launchAgentsDir = "~/Library/LaunchAgents"
	plistFile       = launchAgentsDir + "/" + serviceLabel + ".plist"
)

// plistTemplateAppBundle is used when the binary is inside a .app bundle.
// It launches via /usr/bin/open so macOS associates accessibility permissions
// with the app bundle rather than the raw binary.
const plistTemplateAppBundle = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.y3owk1n.neru</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/bin/open</string>
        <string>-W</string>
        <string>-a</string>
        <string>NERU_APP_PATH</string>
        <string>--args</string>
        <string>launch</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
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

// plistTemplateBinary is used when the binary is a standalone executable
// (not inside a .app bundle). It invokes the binary directly.
const plistTemplateBinary = `<?xml version="1.0" encoding="UTF-8"?>
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
    <key>StandardOutPath</key>
    <string>/tmp/neru.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/neru.err</string>
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

func getBinaryPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	return filepath.EvalSymlinks(execPath)
}

func getAppPath(binPath string) string {
	// The binary lives at <app>/Contents/MacOS/Neru; walk up 3 levels.
	return filepath.Dir(filepath.Dir(filepath.Dir(binPath)))
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

	// When running from an app bundle, use `open -W -a` so macOS associates
	// accessibility permissions with the bundle. Otherwise invoke the binary directly.
	var plistContent string
	if isRunningFromAppBundle() {
		appPath := getAppPath(binPath)
		plistContent = strings.ReplaceAll(plistTemplateAppBundle, "NERU_APP_PATH", appPath)
	} else {
		plistContent = strings.ReplaceAll(plistTemplateBinary, "NERU_BINARY_PATH", binPath)
	}

	// Expand launchAgentsDir
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

	// Remove plist
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
	// Stop the service (ignore errors if already stopped)
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
