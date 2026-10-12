package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/cli/cliutil"
	"github.com/y3owk1n/neru/internal/domain"
)

var (
	errDaemonUnreachable = errors.New("daemon unreachable")
	// errClickableRolesUnusable marks a configuration that loads but selects no
	// accessibility role on this platform, so hints would find nothing.
	errClickableRolesUnusable = errors.New("no clickable roles apply on this platform")
)

// DoctorCmd is the CLI doctor command.
var DoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run comprehensive diagnostics",
	Long: `Run a comprehensive health check of the Neru system.

This command performs client-side checks (IPC socket, config) first,
then queries the running daemon for component-level health status
(accessibility permissions, overlay state, input monitoring).

The platform_support row is a different question from the component
health below it: it names the options, actions and mode flags your
configuration writes that do nothing on this platform. They load and
the daemon runs, so it never fails the check.

Runs client-side checks even when the daemon is not running, so you
can use it to verify accessibility permissions before launching.

Use --json to print every check as one JSON object. The exit status is
the same either way.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		asJSON, flagErr := cmd.Flags().GetBool("json")
		if flagErr != nil {
			return flagErr
		}

		if asJSON {
			return runDoctorJSON(cmd)
		}

		cmd.Println("Neru Doctor — pre-flight checks")
		cmd.Println()
		// --- client-side checks (no daemon needed) --------------------------
		loadResult := doctorConfigLoad()

		rolesUsable := printClickableRolesCheck(cmd, loadResult)

		printPlatformSupportCheck(cmd, loadResult)

		// The endpoint is named after the user it belongs to, so on a system
		// that will not say who that is there is no path to print — and
		// nowhere for the daemon to listen either.
		endpointPath := ipc.SocketPath()
		if endpointPath == "" {
			endpointPath = "cannot be determined for this user"
		}

		if !ipc.IsServerRunning() {
			cmd.Printf("  ❌ %-24s %s\n", "ipc_endpoint", "not reachable: "+endpointPath)

			err := printClientDoctorWithoutDaemon(cmd)
			if err != nil {
				return err
			}

			// Not every platform treats a stopped daemon as a failure, so the
			// role check still has to be able to fail this branch.
			if !rolesUsable {
				return &silentError{err: errClickableRolesUnusable}
			}

			return nil
		}

		cmd.Printf("  ✅ %-24s %s\n", "ipc_endpoint", endpointPath)
		cmd.Println()
		// --- daemon-side checks (via IPC) -----------------------------------
		cmd.Println("Querying daemon...")
		cmd.Println()

		communicator := cliutil.NewIPCCommunicator(timeoutSec)

		ipcResponse, err := communicator.SendCommand(domain.CommandHealth, []string{})
		if err != nil {
			cmd.Printf("  ❌ %-24s %s\n", "daemon", "unreachable")
			cmd.Println()
			cmd.Println("The daemon endpoint exists but is not responding.")
			cmd.Println("Try restarting: neru launch")

			return &silentError{err: errDaemonUnreachable}
		}

		err = formatter.PrintHealth(cmd, ipcResponse.Success, ipcResponse.Data)

		if errors.Is(err, cliutil.ErrUnhealthy) {
			return &silentError{err: err}
		}

		if err != nil {
			return err
		}

		// A healthy daemon running a config that selects no roles still cannot
		// produce a hint, so the client-side failure has to reach the exit
		// status or a scripted health check would treat it as fine.
		if !rolesUsable {
			return &silentError{err: errClickableRolesUnusable}
		}

		return nil
	},
}

// doctorReport is `neru doctor --json`.
type doctorReport struct {
	Healthy         bool                `json:"healthy"`
	Config          doctorConfig        `json:"config"`
	ClickableRoles  clickableRolesCheck `json:"clickable_roles"`  //nolint:tagliatelle // snake_case like every key neru prints.
	PlatformSupport *doctorInert        `json:"platform_support"` //nolint:tagliatelle // snake_case like every key neru prints.
	IPCEndpoint     doctorEndpoint      `json:"ipc_endpoint"`     //nolint:tagliatelle // snake_case like every key neru prints.
	Daemon          any                 `json:"daemon"`
}

// doctorConfig is the configuration the checks judged.
type doctorConfig struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// doctorInert is the platform_support check.
type doctorInert struct {
	Inert []inertSetting `json:"inert"`
}

// doctorEndpoint is where the daemon listens and whether it answers there.
type doctorEndpoint struct {
	Path      string `json:"path"`
	Reachable bool   `json:"reachable"`
}

// runDoctorJSON runs the checks the human form runs and prints them as one
// object, then fails the command when the object says it is unhealthy.
func runDoctorJSON(cmd *cobra.Command) error {
	loadResult := doctorConfigLoad()

	report := doctorReport{
		Config:         doctorConfig{Path: loadResult.ConfigPath},
		ClickableRoles: checkClickableRoles(loadResult),
		IPCEndpoint:    doctorEndpoint{Path: ipc.SocketPath(), Reachable: ipc.IsServerRunning()},
	}

	if loadResult.ValidationError != nil {
		report.Config.Error = loadResult.ValidationError.Error()
	}

	if inert := inertSettings(loadResult); inert != nil {
		report.PlatformSupport = &doctorInert{Inert: inert}
	}

	daemonHealthy := !stoppedDaemonFailsDoctor

	if report.IPCEndpoint.Reachable {
		communicator := cliutil.NewIPCCommunicator(timeoutSec)

		ipcResponse, err := communicator.SendCommand(domain.CommandHealth, []string{})
		if err == nil {
			report.Daemon = ipcResponse.Data
		}

		daemonHealthy = err == nil && ipcResponse.Success
	}

	report.Healthy = daemonHealthy && report.ClickableRoles.Usable

	err := formatter.PrintJSON(cmd, report)
	if err != nil {
		return err
	}

	if !report.Healthy {
		return &silentError{err: cliutil.ErrUnhealthy}
	}

	return nil
}

func init() {
	DoctorCmd.Flags().Bool("json", false, "Print the checks as a JSON object")

	RootCmd.AddCommand(DoctorCmd)
}
