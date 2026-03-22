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
	"strconv"
	"strings"
	"syscall"
	"time"
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
    <key>StandardErrorPath</key>
    <string>/tmp/neru.err.log</string>
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
    <string>/tmp/neru.err.log</string>
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

// findAppBundle locates a Neru.app bundle relative to the resolved binary path.
// It checks two locations:
//  1. The binary is inside the bundle: <prefix>/Neru.app/Contents/MacOS/neru
//     → walk up 3 levels.
//  2. The binary is a sibling: <prefix>/bin/neru with the bundle at
//     <prefix>/Applications/Neru.app (typical Nix / Homebrew layout).
//
// Returns the .app path and true if a valid bundle is found, or ("", false).
func findAppBundle(binPath string) (string, bool) {
	// Case 1: binary lives inside the .app bundle.
	ancestor := filepath.Dir(filepath.Dir(filepath.Dir(binPath)))
	if strings.HasSuffix(ancestor, ".app") && isValidAppBundle(ancestor) {
		return ancestor, true
	}

	// Case 2: binary is at <prefix>/bin/neru; bundle is at <prefix>/Applications/Neru.app.
	prefix := filepath.Dir(filepath.Dir(binPath)) // <prefix>/bin/neru → <prefix>

	sibling := filepath.Join(prefix, "Applications", "Neru.app")
	if isValidAppBundle(sibling) {
		return sibling, true
	}

	return "", false
}

// isValidAppBundle returns true when path ends with ".app" and contains
// a Contents/MacOS directory.
func isValidAppBundle(path string) bool {
	if !strings.HasSuffix(path, ".app") {
		return false
	}

	info, err := os.Stat(filepath.Join(path, "Contents", "MacOS"))

	return err == nil && info.IsDir()
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

	// When an .app bundle is available, use `open -W -a` so macOS associates
	// accessibility permissions with the bundle rather than the raw binary.
	// Otherwise invoke the binary directly.
	var plistContent string
	if appPath, ok := findAppBundle(binPath); ok {
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

// quitNeruApp gracefully terminates a running Neru app process.
// When the service uses `open -W -a`, launchd only manages the `open` wrapper;
// the actual Neru process is a separate Launch Services process that must be
// stopped explicitly.
func quitNeruApp() {
	// Try graceful quit via osascript first (respects the app's shutdown handler).
	quit := exec.CommandContext(context.Background(),
		"osascript", "-e", `tell application "Neru" to quit`)
	_ = quit.Run()

	// Give the app a moment to shut down gracefully, then force-kill if needed.
	time.Sleep(1 * time.Second)
	killNeruProcesses()
}

// killNeruProcesses sends SIGTERM to all processes named "neru" except the
// current process. This avoids the CLI killing itself when running commands
// like `neru service stop` or `neru service uninstall`.
func killNeruProcesses() {
	out, err := exec.CommandContext(context.Background(), "pgrep", "-x", "neru").Output()
	if err != nil {
		return
	}

	self := os.Getpid()
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || pid == self {
			continue
		}

		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
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

	// Ensure the Neru app process is also terminated (it may outlive `open -W`).
	quitNeruApp()

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

	// Ensure the Neru app process is also terminated (it may outlive `open -W`).
	quitNeruApp()

	return nil
}

func restartService() error {
	// Stop the service and the Neru app process.
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
