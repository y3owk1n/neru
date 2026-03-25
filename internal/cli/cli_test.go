package cli_test

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/cli"
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
	cmd := cli.BuildSimpleCommand("test", "short desc", "long desc", "action")

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
	cmd := cli.BuildActionCommand("test", "short desc", "long desc", []string{"arg1"})

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
		"start":               false,
		"stop":                false,
		"idle":                false,
		"hints":               false,
		"grid":                false,
		"scroll":              false,
		"action":              false,
		"status":              false,
		"doctor":              false,
		"launch":              false,
		"docs":                false,
		"config":              false,
		"services":            false,
		"toggle-screen-share": false,
		"recursive_grid":      false,
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
		"left_click":          false,
		"right_click":         false,
		"mouse_up":            false,
		"mouse_down":          false,
		"middle_click":        false,
		"move_mouse":          false,
		"move_mouse_relative": false,
		"reset":               false,
		"backspace":           false,
		"wait_for_mode_exit":  false,
		"save_cursor_pos":     false,
		"restore_cursor":      false,
		"scroll_up":           false,
		"scroll_down":         false,
		"scroll_left":         false,
		"scroll_right":        false,
		"go_top":              false,
		"go_bottom":           false,
		"page_up":             false,
		"page_down":           false,
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
		{"start", getCmd("start"), true},
		{"stop", getCmd("stop"), true},
		{"idle", getCmd("idle"), true},
		{"hints", getCmd("hints"), true},
		{"grid", getCmd("grid"), true},
		{"scroll", getCmd("scroll"), true},
		{"action", getCmd("action"), true},
		{"action_left_click", getActionCmd("left_click"), true},
		{"action_right_click", getActionCmd("right_click"), true},
		{"action_mouse_up", getActionCmd("mouse_up"), true},
		{"action_mouse_down", getActionCmd("mouse_down"), true},
		{"action_middle_click", getActionCmd("middle_click"), true},
		{"action_move_mouse", getActionCmd("move_mouse"), true},
		{"action_move_mouse_relative", getActionCmd("move_mouse_relative"), true},
		{"action_reset", getActionCmd("reset"), true},
		{"action_backspace", getActionCmd("backspace"), true},
		{"action_wait_for_mode_exit", getActionCmd("wait_for_mode_exit"), true},
		{"action_save_cursor_pos", getActionCmd("save_cursor_pos"), true},
		{"action_restore_cursor", getActionCmd("restore_cursor"), true},
		{"action_scroll_up", getActionCmd("scroll_up"), true},
		{"action_scroll_down", getActionCmd("scroll_down"), true},
		{"action_scroll_left", getActionCmd("scroll_left"), true},
		{"action_scroll_right", getActionCmd("scroll_right"), true},
		{"action_go_top", getActionCmd("go_top"), true},
		{"action_go_bottom", getActionCmd("go_bottom"), true},
		{"action_page_up", getActionCmd("page_up"), true},
		{"action_page_down", getActionCmd("page_down"), true},
		{"status", getCmd("status"), true},
		{"doctor", getCmd("doctor"), true}, // doctor returns silentError when daemon is down
		{"toggle-screen-share", getCmd("toggle-screen-share"), true},
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
