//go:build unit

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

func TestBuildSimpleCommand(t *testing.T) {
	cmd := cli.BuildSimpleCommand("test", "short desc", "long desc", "action")

	if cmd == nil {
		t.Fatal("BuildSimpleCommand returned nil")
	}

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

	if cmd == nil {
		t.Fatal("BuildActionCommand returned nil")
	}

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
		"start":   false,
		"stop":    false,
		"idle":    false,
		"hints":   false,
		"grid":    false,
		"scroll":  false,
		"action":  false,
		"status":  false,
		"doctor":  false,
		"metrics": false,
		"profile": false,
		"launch":  false,
	}

	for _, cmd := range cli.RootCmd.Commands() {
		if _, ok := expectedCommands[cmd.Use]; ok {
			expectedCommands[cmd.Use] = true
		}
	}

	for name, found := range expectedCommands {
		if !found {
			t.Errorf("command %s not found in RootCmd", name)
		}
	}

	// Test action subcommands
	expectedActionSubcommands := map[string]bool{
		"left_click":   false,
		"right_click":  false,
		"mouse_up":     false,
		"mouse_down":   false,
		"middle_click": false,
	}

	for _, cmd := range cli.ActionCmd.Commands() {
		if _, ok := expectedActionSubcommands[cmd.Use]; ok {
			expectedActionSubcommands[cmd.Use] = true
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
		{"status", getCmd("status"), true},
		{"doctor", getCmd("doctor"), true},
		{"metrics", getCmd("metrics"), false}, // Metrics just prints status, doesn't need daemon
		{"profile", getCmd("profile"), false}, // Profile just prints help, doesn't need daemon
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
