package cli_test

import (
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/cli"
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

func TestBuildSimpleCommand(t *testing.T) {
	cmd := cli.BuildSimpleCommand("test", "short desc", "long desc", cliTestAction)

	if cmd.Use != "test" {
		t.Errorf("expected Use='test', got %q", cmd.Use)
	}

	if cmd.Short != "short desc" {
		t.Errorf("expected Short='short desc', got %q", cmd.Short)
	}

	if cmd.Long != "long desc" {
		t.Errorf("expected Long='long desc', got %q", cmd.Long)
	}

	// Test that PreRunE and RunE are set
	if cmd.PreRunE == nil {
		t.Error("PreRunE should be set")
	}

	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}

func TestBuildActionCommand(t *testing.T) {
	cmd := cli.BuildActionCommand("test", "short desc", "long desc", []string{"arg1"}, true)

	if cmd.Use != "test" {
		t.Errorf("expected Use='test', got %q", cmd.Use)
	}

	if cmd.Short != "short desc" {
		t.Errorf("expected Short='short desc', got %q", cmd.Short)
	}

	if cmd.Long != "long desc" {
		t.Errorf("expected Long='long desc', got %q", cmd.Long)
	}

	// Test that PreRunE and RunE are set
	if cmd.PreRunE == nil {
		t.Error("PreRunE should be set")
	}

	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}

func TestCommandInitialization(t *testing.T) {
	// Test that global commands are properly initialized
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
		"doctor":                         false,
		cliTestRoles:                     false,
		"launch":                         false,
		"docs":                           false,
		"config":                         false,
		"services":                       false,
		"toggle-screen-share":            false,
		"toggle-cursor-follow-selection": false,
		"toggle-scroll-invert":           false,
		"recursive_grid":                 false,
		"monitor_select":                 false,
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

	// Test action subcommands
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
		"cycle_hint":          false,
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

func TestCommandExecutionWithoutDaemon(t *testing.T) {
	// Test that all CLI commands execute without panicking when no daemon is running
	// Commands that require IPC should return errors, while utility commands should work
	tests := []struct {
		name      string
		cmd       *cobra.Command
		expectErr bool
	}{
		{cliTestStart, getCmd(cliTestStart), true},
		{cliTestStop, getCmd(cliTestStop), true},
		{cliTestIdle, getCmd(cliTestIdle), true},
		{cliTestHints, getCmd(cliTestHints), true},
		{cliTestGrid, getCmd(cliTestGrid), true},
		{"scroll", getCmd("scroll"), true},
		{cliTestAction, getCmd(cliTestAction), true},
		{"action_left_click", getActionCmd(cliTestLeftClick), true},
		{"action_right_click", getActionCmd("right_click"), true},
		{"action_mouse_up", getActionCmd("mouse_up"), true},
		{"action_mouse_down", getActionCmd("mouse_down"), true},
		{"action_middle_click", getActionCmd("middle_click"), true},
		{"action_move_mouse", getActionCmd("move_mouse"), true},
		{"action_move_mouse_relative", getActionCmd("move_mouse_relative"), true},
		{"action_feed", getActionCmd("feed <key> [key...]"), true},
		{"action_reset", getActionCmd("reset"), true},
		{"action_backspace", getActionCmd("backspace"), true},
		{"action_wait_for_mode_exit", getActionCmd("wait_for_mode_exit"), true},
		{"action_save_cursor_pos", getActionCmd("save_cursor_pos"), true},
		{"action_restore_cursor_pos", getActionCmd("restore_cursor_pos"), true},
		{"action_scroll_up", getActionCmd("scroll_up"), true},
		{"action_scroll_down", getActionCmd("scroll_down"), true},
		{"action_scroll_left", getActionCmd("scroll_left"), true},
		{"action_scroll_right", getActionCmd("scroll_right"), true},
		{"action_go_top", getActionCmd("go_top"), true},
		{"action_go_bottom", getActionCmd("go_bottom"), true},
		{"action_page_up", getActionCmd("page_up"), true},
		{"action_page_down", getActionCmd("page_down"), true},
		{"action_move_monitor", getActionCmd("move_monitor"), true},
		{"action_hide_cursor", getActionCmd("hide_cursor"), true},
		{"action_show_cursor", getActionCmd("show_cursor"), true},
		{cliTestStatus, getCmd(cliTestStatus), true},
		{
			"doctor",
			getCmd("doctor"),
			runtime.GOOS != "windows",
		}, // Windows runs client-side doctor when daemon is down
		{"toggle-screen-share", getCmd("toggle-screen-share"), true},
		{"toggle-cursor-follow-selection", getCmd("toggle-cursor-follow-selection"), true},
		{"recursive_grid", getCmd("recursive_grid"), true},
		{"config_dump", getSubCmd("config", "dump"), true},
		{"config_reload", getSubCmd("config", "reload"), true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s panicked: %v", testCase.name, r)
				}
			}()

			err := testCase.cmd.RunE(testCase.cmd, []string{})
			if testCase.expectErr && err == nil {
				t.Errorf("expected error for %s when no daemon is running, got nil", testCase.name)
			}

			if !testCase.expectErr && err != nil {
				t.Errorf("unexpected error for %s: %v", testCase.name, err)
			}
		})
	}

	// NOTE: services subcommands (install, uninstall, start, stop,
	// restart, status) are intentionally excluded from this test.
	// On macOS they invoke launchctl and may succeed, fail, or cause
	// real side-effects (e.g. install writes a plist and loads a
	// launchd service). On other platforms they return
	// CodeNotSupported. Their registration is already verified by
	// TestCommandInitialization.
}

func TestModeCommand_OnExitRequiresAction(t *testing.T) {
	for _, name := range []string{cliTestHints, cliTestGrid, cliTestRecursive} {
		t.Run(name, func(t *testing.T) {
			cmd := getCmd(name)
			if cmd == nil {
				t.Fatalf("command %q not found", name)
			}

			setErr := cmd.Flags().Set("on-exit", "exec notify-send done")
			if setErr != nil {
				t.Fatalf("failed to set --on-exit: %v", setErr)
			}
			// Reset the shared flag so later tests are unaffected.
			defer func() {
				resetErr := cmd.Flags().Set("on-exit", "")
				if resetErr != nil {
					t.Fatalf("failed to reset --on-exit: %v", resetErr)
				}
			}()

			err := cmd.RunE(cmd, []string{})
			if err == nil || !strings.Contains(err.Error(), "--on-exit requires --action") {
				t.Fatalf("expected --on-exit requires --action error, got %v", err)
			}
		})
	}
}

func TestLaunchCommandExecution(t *testing.T) {
	// Note: This test modifies global LaunchFunc and is not parallel-safe
	// Save original LaunchFunc
	originalLaunchFunc := cli.LaunchFunc

	// Set a mock LaunchFunc for testing
	cli.LaunchFunc = func(configPath string) {
		// Mock launch - do nothing
	}

	defer func() {
		// Restore original
		cli.LaunchFunc = originalLaunchFunc

		if r := recover(); r != nil {
			t.Errorf("launchCmd.RunE panicked: %v", r)
		}
	}()

	// Launch command should work with mock LaunchFunc
	launchCmd := getCmd("launch")

	err := launchCmd.RunE(launchCmd, []string{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildClickActionCommand_HasButtonPhaseFlags(t *testing.T) {
	cmd := cli.BuildClickActionCommand("test", "short desc", "long desc", []string{"left_click"})

	for _, flagName := range []string{"state", "toggle", "modifier", "selection", "bare"} {
		if cmd.Flags().Lookup(flagName) == nil {
			t.Errorf("BuildClickActionCommand() missing --%s flag", flagName)
		}
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

func TestBuildActionCommand_HasNoButtonPhaseFlags(t *testing.T) {
	cmd := cli.BuildActionCommand("test", "short desc", "long desc", []string{"arg1"}, true)

	for _, flagName := range []string{"state", "toggle"} {
		if cmd.Flags().Lookup(flagName) != nil {
			t.Errorf("BuildActionCommand() should not define --%s", flagName)
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

		if cmd.Flags().Lookup("state") == nil {
			t.Errorf("action %s is missing the --state flag", cmd.Use)
		}

		if cmd.Flags().Lookup("toggle") == nil {
			t.Errorf("action %s is missing the --toggle flag", cmd.Use)
		}
	}

	for name, found := range clickCommands {
		if !found {
			t.Errorf("click action %s not registered on ActionCmd", name)
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
