package cliutil

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

// ErrUnhealthy is returned by PrintHealth when one or more components are unhealthy.
var ErrUnhealthy = errors.New("unhealthy components detected")

// IPCCommunicator handles IPC communication with the Neru daemon.
type IPCCommunicator struct {
	timeoutSec int
}

// NewIPCCommunicator creates a new IPC communicator.
func NewIPCCommunicator(timeoutSec int) *IPCCommunicator {
	return &IPCCommunicator{
		timeoutSec: timeoutSec,
	}
}

// SetTimeout updates the timeout for IPC operations.
func (c *IPCCommunicator) SetTimeout(timeoutSec int) {
	c.timeoutSec = timeoutSec
}

// SendCommand sends a command to the running Neru daemon.
func (c *IPCCommunicator) SendCommand(action string, args []string) (ipc.Response, error) {
	ipcClient := ipc.NewClient()

	ipcResponse, ipcResponseErr := ipcClient.SendWithTimeout(
		ipc.Command{Action: action, Args: args},
		time.Duration(c.timeoutSec)*time.Second,
	)
	if ipcResponseErr != nil {
		return ipc.Response{}, derrors.Wrap(
			ipcResponseErr,
			derrors.CodeIPCFailed,
			"failed to send command",
		)
	}

	return ipcResponse, nil
}

// HandleResponse processes an IPC response and handles success/error cases.
func (c *IPCCommunicator) HandleResponse(cmd *cobra.Command, ipcResponse ipc.Response) error {
	if !ipcResponse.Success {
		if ipcResponse.Code != "" {
			code := derrors.CodeIPCFailed
			if ipcResponse.Code == ipc.CodeVersionMismatch {
				code = derrors.CodeVersionMismatch
			}

			return derrors.Newf(
				code,
				"%s (code: %s)",
				ipcResponse.Message,
				ipcResponse.Code,
			)
		}

		return derrors.New(derrors.CodeIPCFailed, ipcResponse.Message)
	}

	cmd.Println(ipcResponse.Message)

	return nil
}

// SendAndHandle combines sending a command and handling the response.
func (c *IPCCommunicator) SendAndHandle(cmd *cobra.Command, action string, args []string) error {
	ipcResponse, err := c.SendCommand(action, args)
	if err != nil {
		return err
	}

	return c.HandleResponse(cmd, ipcResponse)
}

// OutputFormatter handles formatted output for CLI commands.
type OutputFormatter struct{}

// NewOutputFormatter creates a new output formatter.
func NewOutputFormatter() *OutputFormatter {
	return &OutputFormatter{}
}

// PrintStatus prints status information in a formatted way.
func (f *OutputFormatter) PrintStatus(cmd *cobra.Command, data any) error {
	cmd.Println("Neru Status:")

	// Try to parse as structured status data
	if statusData, ok := data.(map[string]any); ok {
		if enabled, ok := statusData["enabled"].(bool); ok {
			status := "stopped"
			if enabled {
				status = "running"
			}

			cmd.Println("  Status: " + status)
		}

		if mode, ok := statusData["mode"].(string); ok {
			cmd.Println("  Mode: " + mode)
		}

		if configPath, ok := statusData["config"].(string); ok {
			cmd.Println("  Config: " + configPath)
		}

		if capabilities, ok := statusData["capabilities"].(map[string]any); ok &&
			len(capabilities) > 0 {
			cmd.Println("  Platform: " + stringValue(capabilities["platform"]))
		}

		printProfile(cmd, statusData["profile"])
	} else {
		// Fallback to JSON output
		jsonData, jsonDataErr := json.MarshalIndent(data, "  ", "  ")
		if jsonDataErr != nil {
			return derrors.Wrap(
				jsonDataErr,
				derrors.CodeSerializationFailed,
				"failed to marshal status data",
			)
		}

		cmd.Println(string(jsonData))
	}

	return nil
}

// PrintHealth prints health check results.
func (f *OutputFormatter) PrintHealth(cmd *cobra.Command, success bool, data any) error {
	healthData, ok := data.(map[string]any)
	if !ok {
		// Fallback for unexpected data shape
		if success {
			cmd.Println("✅ All systems operational")

			return nil
		}

		cmd.Println("  ⚠️  Health check returned errors")

		return ErrUnhealthy
	}
	// Print metadata header
	cmd.Println("Daemon status:")

	if version, ok := healthData["version"].(string); ok && version != "" {
		cmd.Println("  Version:  " + version)
	}

	if configPath, ok := healthData["config"].(string); ok && configPath != "" {
		cmd.Println("  Config:   " + configPath)
	}

	if mode, ok := healthData["mode"].(string); ok && mode != "" {
		cmd.Println("  Mode:     " + mode)
	}

	if capabilities, ok := healthData["capabilities"].(map[string]any); ok {
		if platform := stringValue(capabilities["platform"]); platform != "" {
			cmd.Println("  Platform: " + platform)
		}
	}

	printProfile(cmd, healthData["profile"])

	cmd.Println()
	// Print component checks
	components, hasComponents := healthData["components"].(map[string]any)
	if !hasComponents {
		if success {
			cmd.Println("  ✅ All systems operational")

			return nil
		}

		cmd.Println("  ⚠️  Health check returned errors")

		return ErrUnhealthy
	}

	if success {
		cmd.Println("  ✅ All components healthy")
	} else {
		cmd.Println("  ⚠️  Some components are unhealthy")
	}

	cmd.Println()
	// Sort keys for deterministic output
	keys := sortedKeys(components)

	componentWidth := maxComponentWidth(keys)
	for _, key := range keys {
		value := components[key]

		status := "ok"
		if strVal, ok := value.(string); ok {
			status = strVal
		}

		if isHealthyHealthStatus(key, status) {
			cmd.Printf("  ✅ %-*s %s\n", componentWidth, key, status)
		} else {
			cmd.Printf("  ❌ %-*s %s\n", componentWidth, key, status)
		}
	}

	if !success {
		return ErrUnhealthy
	}

	return nil
}

// sortedKeys returns the keys of a map sorted alphabetically.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}

	return ""
}

func maxComponentWidth(keys []string) int {
	width := 24
	for _, key := range keys {
		if len(key) > width {
			width = len(key)
		}
	}

	return width
}

func isHealthyHealthStatus(componentKey, status string) bool {
	if strings.HasPrefix(status, "ok") || status == "supported" || status == "headless" {
		return true
	}

	if componentKey == "capability.platform" {
		switch status {
		case "darwin", "linux", "windows":
			return true
		}
	}

	return false
}

func printProfile(cmd *cobra.Command, rawProfile any) {
	profile, ok := rawProfile.(map[string]any)
	if !ok || len(profile) == 0 {
		return
	}

	if primaryModifier := stringValue(profile["primary_modifier"]); primaryModifier != "" {
		cmd.Println("  Primary:  " + primaryModifier)
	}

	if displayServer := stringValue(profile["display_server"]); displayServer != "" {
		cmd.Println("  Display:  " + displayServer)
	}

	profileLines := []string{
		profileBackendLine("accessibility", profile),
		profileBackendLine("hotkeys", profile),
		profileBackendLine("keyboard_capture", profile),
		profileBackendLine("overlay", profile),
		profileBackendLine("notifications", profile),
	}

	for _, profileLine := range profileLines {
		if profileLine == "" {
			continue
		}

		cmd.Println("  " + profileLine)
	}
}

func profileBackendLine(name string, profile map[string]any) string {
	backend := stringValue(profile[name+"_backend"])

	buildMode := stringValue(profile[name+"_build_mode"])
	if backend == "" && buildMode == "" {
		return ""
	}

	label := profileLabel(name)
	if buildMode == "" {
		return label + ": " + backend
	}

	if backend == "" {
		return label + ": " + buildMode
	}

	return label + ": " + backend + " (" + buildMode + ")"
}

func profileLabel(name string) string {
	switch name {
	case "":
		return ""
	case "keyboard_capture":
		return "Keyboard"
	default:
		return strings.ToUpper(name[:1]) + strings.ReplaceAll(name[1:], "_", " ")
	}
}

// ErrorHandler provides consistent error handling for CLI commands.
type ErrorHandler struct{}

// NewErrorHandler creates a new error handler.
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{}
}

// HandleIPCError wraps IPC errors with consistent formatting.
func (e *ErrorHandler) HandleIPCError(err error, context string) error {
	return derrors.Wrap(err, derrors.CodeIPCFailed, context)
}

// HandleSerializationError wraps serialization errors.
func (e *ErrorHandler) HandleSerializationError(err error, context string) error {
	return derrors.Wrap(err, derrors.CodeSerializationFailed, context)
}
