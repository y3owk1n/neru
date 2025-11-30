package cliutil

import (
	"encoding/json"
	"time"

	"github.com/spf13/cobra"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

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
			return derrors.Newf(
				derrors.CodeIPCFailed,
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

// CommandBuilder helps build common CLI command patterns.
type CommandBuilder struct {
	communicator *IPCCommunicator
}

// NewCommandBuilder creates a new command builder.
func NewCommandBuilder(communicator *IPCCommunicator) *CommandBuilder {
	return &CommandBuilder{
		communicator: communicator,
	}
}

// BuildSimpleCommand creates a simple command that sends an action to the daemon.
func (b *CommandBuilder) BuildSimpleCommand(use, short, long string, action string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return b.CheckRunningInstance()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return b.communicator.SendAndHandle(cmd, action, args)
		},
	}
}

// BuildActionCommand creates a command that sends an action with parameters.
func (b *CommandBuilder) BuildActionCommand(
	use, short, long string,
	action string,
	params []string,
) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return b.CheckRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return b.communicator.SendAndHandle(cmd, action, params)
		},
	}
}

// CheckRunningInstance verifies that a Neru instance is running.
func (b *CommandBuilder) CheckRunningInstance() error {
	if !ipc.IsServerRunning() {
		return derrors.New(
			derrors.CodeIPCServerNotRunning,
			"neru is not running. Start it first with 'neru' or 'neru launch'",
		)
	}

	return nil
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
	} else {
		// Fallback to JSON output
		jsonData, jsonDataErr := json.MarshalIndent(data, "  ", "  ")
		if jsonDataErr != nil {
			return derrors.Wrap(jsonDataErr, derrors.CodeSerializationFailed, "failed to marshal status data")
		}

		cmd.Println(string(jsonData))
	}

	return nil
}

// PrintHealth prints health check results.
func (f *OutputFormatter) PrintHealth(cmd *cobra.Command, success bool, data any) error {
	if success {
		cmd.Println("✅ All systems operational")

		return nil
	}

	cmd.Println("⚠️  Some components are unhealthy:")

	if healthData, ok := data.(map[string]any); ok {
		for key, value := range healthData {
			status := "OK"
			if strVal, ok := value.(string); ok && strVal != "" {
				status = strVal
			}

			if status == "OK" {
				cmd.Printf("  ✅ %s: %s\n", key, status)
			} else {
				cmd.Printf("  ❌ %s: %s\n", key, status)
			}
		}
	}

	return nil
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
