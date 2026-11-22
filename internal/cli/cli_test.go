package cli_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/cli"
	"github.com/y3owk1n/neru/internal/derrors"
)

const (
	cliTestStart     = "start"
	cliTestStop      = "stop"
	cliTestIdle      = "idle"
	cliTestHints     = "hints"
	cliTestGrid      = "grid"
	cliTestAction    = "action"
	cliTestStatus    = "status"
	cliTestLeftClick = "left_click"
	cliTestRecursive = "recursive_grid"
	cliTestConfig    = "config"
)

// Helper to get command by name from RootCmd.
func getCmd(name string) *cobra.Command {
	for _, cmd := range cli.RootCmd.Commands() {
		if cmd.Use == name {
			return cmd
		}
	}

	return nil
}

// Helper to get action subcommand from ActionCmd.
func getActionCmd(name string) *cobra.Command {
	for _, cmd := range cli.ActionCmd.Commands() {
		if cmd.Use == name {
			return cmd
		}
	}

	return nil
}

// Helper to get a subcommand of a named root command.
func getSubCmd(parentName, childName string) *cobra.Command {
	parent := getCmd(parentName)
	if parent == nil {
		return nil
	}

	for _, cmd := range parent.Commands() {
		if cmd.Use == childName {
			return cmd
		}
	}

	return nil
}

func TestCommandInitialization(t *testing.T) {
	expectedCommands := map[string]bool{
		cliTestStart:                     false,
		cliTestStop:                      false,
		cliTestIdle:                      false,
		cliTestHints:                     false,
		cliTestGrid:                      false,
		"scroll":                         false,
		cliTestAction:                    false,
		"run <step> [step...]":           false,
		"macro <name> [arg...]":          false,
		cliTestStatus:                    false,
		"watch":                          false,
		"query":                          false,
		"doctor":                         false,
		cliTestRoles:                     false,
		"launch":                         false,
		"docs":                           false,
		cliTestConfig:                    false,
		"services":                       false,
		"toggle-screen-share":            false,
		"toggle-cursor-follow-selection": false,
		"toggle-scroll-invert":           false,
		"recursive_grid":                 false,
		"bisect":                         false,
		"monitor_select":                 false,
		"vim":                            false,
		"mode <name>":                    false,
		"describe":                       false,
		"schema":                         false,
		"agent-guide":                    false,
	}

	for _, cmd := range cli.RootCmd.Commands() {
		if _, ok := expectedCommands[cmd.Use]; ok {
			expectedCommands[cmd.Use] = true
		} else {
			t.Errorf(
				"unexpected command %q registered on RootCmd but not in expectedCommands",
				cmd.Use,
			)
		}
	}

	for name, found := range expectedCommands {
		if !found {
			t.Errorf("command %s not found in RootCmd", name)
		}
	}

	expectedActionSubcommands := map[string]bool{
		cliTestLeftClick:      false,
		"right_click":         false,
		"mouse_up":            false,
		"mouse_down":          false,
		"left_mouse_down":     false,
		"left_mouse_up":       false,
		"left_mouse_toggle":   false,
		"right_mouse_down":    false,
		"right_mouse_up":      false,
		"right_mouse_toggle":  false,
		"middle_mouse_down":   false,
		"middle_mouse_up":     false,
		"middle_mouse_toggle": false,
		"middle_click":        false,
		"move_mouse":          false,
		"move_mouse_relative": false,
		"feed <key> [key...]": false,
		"reset":               false,
		"backspace":           false,
		"wait_for_mode_exit":  false,
		"save_cursor_pos":     false,
		"restore_cursor_pos":  false,
		"scroll_up":           false,
		"scroll_down":         false,
		"scroll_left":         false,
		"scroll_right":        false,
		"go_top":              false,
		"go_bottom":           false,
		"page_up":             false,
		"page_down":           false,
		"move_monitor":        false,
		"move_cell":           false,
		"bisect":              false,
		"cycle_hint":          false,
		"refresh_hints":       false,
		"session_action":      false,
		"copy_text":           false,
		"snap":                false,
		"hide_cursor":         false,
		"show_cursor":         false,
	}

	for _, cmd := range cli.ActionCmd.Commands() {
		if _, ok := expectedActionSubcommands[cmd.Use]; ok {
			expectedActionSubcommands[cmd.Use] = true
		} else {
			t.Errorf(
				"unexpected action subcommand %q registered on ActionCmd but not in expectedActionSubcommands",
				cmd.Use,
			)
		}
	}

	for name, found := range expectedActionSubcommands {
		if !found {
			t.Errorf("action subcommand %s not found in ActionCmd", name)
		}
	}
}

// isolateEndpoint points the IPC endpoint at a directory no daemon listens in,
// so a test that runs a command never reaches the user's own daemon. Windows
// names its pipe after the user instead, so there the test skips when a
// daemon is up.
func isolateEndpoint(t *testing.T) {
	t.Helper()

	//nolint:usetesting // t.TempDir names are too long for a Unix socket path,
	// and a dial on one fails with EINVAL rather than with the missing endpoint.
	dir, err := os.MkdirTemp("", "neru")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}

	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("TMPDIR", dir)

	if ipc.IsServerRunning() {
		t.Skip("a Neru daemon is running for this user")
	}
}

// TestCommandExecutionWithoutDaemon pins what each RunE path says when no
// daemon is listening. Most send and have to fail on the missing endpoint, not
// on a flag parse error or a refusal. Mode commands check for the daemon
// first, and a bare action refuses before it would send anything.
func TestCommandExecutionWithoutDaemon(t *testing.T) {
	isolateEndpoint(t)

	tests := []struct {
		name string
		cmd  *cobra.Command
		// wantCode is the refusal a path answers before it dials. Empty means
		// the path dials and must fail on the missing endpoint.
		wantCode derrors.Code
	}{
		{"simple command", getCmd(cliTestStart), ""},
		{"mode command", getCmd(cliTestHints), derrors.CodeIPCServerNotRunning},
		{"action without a subcommand", getCmd(cliTestAction), derrors.CodeInvalidInput},
		{"click action", getActionCmd(cliTestLeftClick), ""},
		{"plain action", getActionCmd("reset"), ""},
		{"move_mouse", getActionCmd("move_mouse"), ""},
		{"move_mouse_relative", getActionCmd("move_mouse_relative"), ""},
		{"feed", getActionCmd("feed <key> [key...]"), ""},
		{"wait_for_mode_exit", getActionCmd("wait_for_mode_exit"), ""},
		{"cursor slot action", getActionCmd("save_cursor_pos"), ""},
		{"scroll action", getActionCmd("scroll_up"), ""},
		{"move_monitor", getActionCmd("move_monitor"), ""},
		{cliTestStatus, getCmd(cliTestStatus), ""},
		{"toggle command", getCmd("toggle-screen-share"), ""},
		{"config dump", getSubCmd(cliTestConfig, "dump"), ""},
		{"config reload", getSubCmd(cliTestConfig, "reload"), ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.cmd.RunE(testCase.cmd, []string{})

			if testCase.wantCode != "" {
				if !derrors.IsCode(err, testCase.wantCode) {
					t.Errorf("RunE() error = %v, want %s", err, testCase.wantCode)
				}

				return
			}

			if !derrors.IsCode(err, derrors.CodeIPCFailed) || !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("RunE() error = %v, want a failed connection to a missing endpoint", err)
			}
		})
	}

	// Windows runs client-side doctor when the daemon is down, so only the
	// other platforms fail it.
	t.Run("doctor", func(t *testing.T) {
		doctor := getCmd("doctor")

		var output bytes.Buffer

		doctor.SetOut(&output)
		defer doctor.SetOut(nil)

		err := doctor.RunE(doctor, []string{})
		if (err != nil) != (runtime.GOOS != goosWindows) {
			t.Errorf("doctor RunE() error = %v on %s", err, runtime.GOOS)
		}

		if !strings.Contains(output.String(), "not reachable") {
			t.Errorf("doctor did not report the unreachable endpoint:\n%s", output.String())
		}
	})

	// NOTE: services subcommands (install, uninstall, start, stop,
	// restart, status) are intentionally excluded from this test.
	// On macOS they invoke launchctl and may succeed, fail, or cause
	// real side-effects (e.g. install writes a plist and loads a
	// launchd service). On other platforms they return
	// CodeNotSupported. Their registration is already verified by
	// TestCommandInitialization.
}

// TestLaunchCommandExecution pins that launch with no daemon running hands
// the --config path to the launch function, once.
func TestLaunchCommandExecution(t *testing.T) {
	isolateEndpoint(t)

	originalLaunchFunc := cli.LaunchFunc

	var launched []string

	cli.LaunchFunc = func(configPath string) {
		launched = append(launched, configPath)
	}

	defer func() { cli.LaunchFunc = originalLaunchFunc }()

	const wantPath = "/nonexistent/neru-test-config.toml"

	setErr := cli.RootCmd.PersistentFlags().Set("config", wantPath)
	if setErr != nil {
		t.Fatalf("set --config: %v", setErr)
	}

	defer func() { _ = cli.RootCmd.PersistentFlags().Set("config", "") }()

	launchCmd := getCmd("launch")

	err := launchCmd.RunE(launchCmd, []string{})
	if err != nil {
		t.Fatalf("launch RunE() error = %v", err)
	}

	if len(launched) != 1 || launched[0] != wantPath {
		t.Errorf("launch function calls = %q, want one with %q", launched, wantPath)
	}
}

// TestLaunchCommand_FailsWithoutALaunchFunction pins a launch that cannot start
// a daemon. It says so once on stderr and fails, instead of reporting success
// with nothing running.
func TestLaunchCommand_FailsWithoutALaunchFunction(t *testing.T) {
	isolateEndpoint(t)

	originalLaunchFunc := cli.LaunchFunc
	cli.LaunchFunc = nil

	defer func() { cli.LaunchFunc = originalLaunchFunc }()

	launchCmd := getCmd("launch")

	var stderr bytes.Buffer

	launchCmd.SetErr(&stderr)
	defer launchCmd.SetErr(nil)

	err := launchCmd.RunE(launchCmd, []string{})
	if !derrors.IsCode(err, derrors.CodeInternal) {
		t.Fatalf("launch without a launch function returned %v, want CodeInternal", err)
	}

	if got := strings.Count(stderr.String(), "Launch function not initialized"); got != 1 {
		t.Errorf("stderr reported the failure %d times, want once: %q", got, stderr.String())
	}
}

// TestBuildScrollActionCommand_HasModifierFlag pins the CLI half of issue
// #1448. The scroll subcommands are built here rather than by
// BuildActionCommand, so registering --modifier on the latter left every
// documented `neru action scroll_up --modifier ctrl` failing with "unknown
// flag" while the same string worked from a config binding.
func TestBuildScrollActionCommand_HasModifierFlag(t *testing.T) {
	for _, supportSteps := range []bool{true, false} {
		cmd := cli.BuildScrollActionCommand("scroll_up", "short desc", "long desc", supportSteps)

		for _, flagName := range []string{"modifier", "selection", "bare"} {
			if cmd.Flags().Lookup(flagName) == nil {
				t.Errorf(
					"BuildScrollActionCommand(supportSteps=%v) missing --%s flag",
					supportSteps,
					flagName,
				)
			}
		}
	}
}

func TestClickActionCommands_HaveButtonPhaseFlags(t *testing.T) {
	clickCommands := map[string]bool{
		"left_click":   false,
		"right_click":  false,
		"middle_click": false,
	}

	for _, cmd := range cli.ActionCmd.Commands() {
		if _, ok := clickCommands[cmd.Use]; !ok {
			continue
		}

		clickCommands[cmd.Use] = true

		for _, flagName := range []string{"state", "toggle", "modifier", "selection", "bare"} {
			if cmd.Flags().Lookup(flagName) == nil {
				t.Errorf("action %s is missing the --%s flag", cmd.Use, flagName)
			}
		}
	}

	for name, found := range clickCommands {
		if !found {
			t.Errorf("click action %s not registered on ActionCmd", name)
		}
	}

	reset := getActionCmd("reset")
	if reset == nil {
		t.Fatal("action reset not registered on ActionCmd")
	}

	for _, flagName := range []string{"state", "toggle"} {
		if reset.Flags().Lookup(flagName) != nil {
			t.Errorf("action reset should not define --%s, only click actions take it", flagName)
		}
	}
}

// TestButtonPhaseAliasCommands_AreHiddenAndComplete pins that every press,
// release, and toggle action name is reachable from the CLI (so a string that
// works in a config also works in a terminal) without cluttering help output.
func TestButtonPhaseAliasCommands_AreHiddenAndComplete(t *testing.T) {
	wantHidden := map[string]bool{
		"left_mouse_down":     false,
		"left_mouse_up":       false,
		"left_mouse_toggle":   false,
		"right_mouse_down":    false,
		"right_mouse_up":      false,
		"right_mouse_toggle":  false,
		"middle_mouse_down":   false,
		"middle_mouse_up":     false,
		"middle_mouse_toggle": false,
	}

	for _, cmd := range cli.ActionCmd.Commands() {
		if _, ok := wantHidden[cmd.Use]; !ok {
			continue
		}

		wantHidden[cmd.Use] = true

		if !cmd.Hidden {
			t.Errorf("action %s should be hidden so help output steers users to the flags", cmd.Use)
		}
	}

	for name, found := range wantHidden {
		if !found {
			t.Errorf("action %s not registered on ActionCmd", name)
		}
	}
}
