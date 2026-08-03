package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/buildinfo"
	"github.com/y3owk1n/neru/internal/cli/cliutil"
	"github.com/y3owk1n/neru/internal/derrors"
)

const (
	// DefaultIPCTimeoutSeconds is the default IPC timeout in seconds.
	//
	// This is the client-side ceiling for a full request/response round-trip
	// and MUST stay comfortably larger than the daemon's own per-request work
	// budget — notably the hints element scan (modes.HintTimeout, 5s) plus
	// activation overhead. If the client gives up first, it disconnects and the
	// daemon logs a spurious "broken pipe" when it finally writes the response
	// (which the client is no longer there to read). The headroom keeps a slow
	// (but not wedged) accessibility backend from tripping that race.
	DefaultIPCTimeoutSeconds = 10
)

var (
	configPath string
	// LaunchFunc is set by main to handle daemon launch.
	LaunchFunc func(configPath string)
	// timeoutSec is overridden by the --timeout flag (default
	// DefaultIPCTimeoutSeconds); kept in sync so any pre-parse use matches.
	timeoutSec = DefaultIPCTimeoutSeconds

	// CLI utilities.
	formatter *cliutil.OutputFormatter
)

// RootCmd is the root CLI command for Neru.
var RootCmd = &cobra.Command{
	Use:   "neru",
	Short: "Neru - Keyboard-driven navigation tool",
	Long: `Neru is a keyboard-driven navigation tool that provides
vim-like navigation capabilities across applications using accessibility APIs.`,
	Example: `  neru launch                        Start the Neru daemon
  neru status                        Show daemon status
  neru hints --action left_click     Activate hints mode with pending click
  neru action scroll_down --steps 3  Scroll down 3 steps`,
	SilenceErrors: true,
	Version:       buildinfo.Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsRunningFromAppBundle() && len(args) == 0 {
			launchProgram(cmd, configPath)

			return nil
		}

		return cmd.Help()
	},
}

// silentError wraps an error whose message has already been printed to stderr.
// Execute() recognizes this type and skips duplicate output.
type silentError struct{ err error }

func (e *silentError) Error() string { return e.err.Error() }
func (e *silentError) Unwrap() error { return e.err }

// setSilenceUsage walks the command tree and installs a PersistentPreRunE
// wrapper on every command that sets SilenceUsage = true.  Because the
// wrapper is attached directly to each command it cannot be silently
// shadowed by a subcommand that defines its own PersistentPreRun(E) —
// unlike a single PersistentPreRun on the root which Cobra would skip
// whenever a child overrides the hook.
func setSilenceUsage(cmd *cobra.Command) {
	origE := cmd.PersistentPreRunE
	origNonE := cmd.PersistentPreRun

	// Clear the non-E variant so Cobra never double-calls it; the
	// wrapper below invokes it when needed.
	cmd.PersistentPreRun = nil

	cmd.PersistentPreRunE = func(_cmd *cobra.Command, args []string) error {
		_cmd.SilenceUsage = true
		if origE != nil {
			return origE(_cmd, args)
		}
		// Preserve a non-E hook if one was set instead.
		if origNonE != nil {
			origNonE(_cmd, args)
		}

		return nil
	}
	for _, child := range cmd.Commands() {
		setSilenceUsage(child)
	}
}

// Execute initializes and runs the CLI application.
func Execute() {
	setSilenceUsage(RootCmd)

	executeErr := RootCmd.Execute()
	if executeErr != nil {
		// If the command already printed detailed output, don't repeat it.
		if _, ok := errors.AsType[*silentError](executeErr); !ok {
			fmt.Fprintln(os.Stderr, executeErr)
		}

		os.Exit(1)
	}
}

func init() {
	// Set the build version for IPC version validation so both the CLI
	// client and the daemon (which also imports this package) agree on
	// the expected version.
	ipc.SetBuildVersion(buildinfo.Version)

	// Override Cobra's default OutOrStderr() for cmd.Println so that
	// primary command output goes to stdout (pipeable) while errors
	// via cmd.PrintErrln still correctly go to stderr.
	RootCmd.SetOut(os.Stdout)

	// Initialize CLI utilities.
	formatter = cliutil.NewOutputFormatter()

	RootCmd.SetVersionTemplate(
		fmt.Sprintf(
			"Neru version %s\nGit commit: %s\nBuild date: %s\n",
			buildinfo.Version,
			buildinfo.GitCommit,
			buildinfo.BuildDate,
		),
	)

	RootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to config file")
	RootCmd.PersistentFlags().
		IntVar(&timeoutSec, "timeout", DefaultIPCTimeoutSeconds, "IPC timeout in seconds")
}

// IsRunningFromAppBundle reports whether the executable is running from a
// platform app bundle. On macOS this checks for ".app/Contents/MacOS".
// On other platforms this always returns false (no bundle concept).
// Implementation is in root_darwin.go / root_other.go.
func IsRunningFromAppBundle() bool {
	return isRunningFromAppBundle()
}

func launchProgram(cmd *cobra.Command, cfgPath string) {
	if ipc.IsServerRunning() {
		cmd.Println("Neru is already running")
		os.Exit(0)
	}

	// From here on this process is the daemon rather than a CLI client. On
	// Windows that means giving up a console the shell allocated for us, so a
	// Start Menu or autostart launch does not leave a window behind. No-op
	// elsewhere, and no-op when a terminal owns the console.
	detachConsoleIfOwned()

	if LaunchFunc != nil {
		LaunchFunc(cfgPath)
	} else {
		cmd.PrintErrln("Error: Launch function not initialized")
		os.Exit(1)
	}
}

// sendCommand transmits a command to the running Neru daemon via IPC.
func sendCommand(cmd *cobra.Command, action string, args []string) error {
	communicator := cliutil.NewIPCCommunicator(timeoutSec)

	return communicator.SendAndHandle(cmd, action, args)
}

func requiresRunningInstance() error {
	if !ipc.IsServerRunning() {
		return derrors.New(
			derrors.CodeIPCServerNotRunning,
			"neru is not running. Start it first with 'neru' or 'neru launch'",
		)
	}

	return nil
}
