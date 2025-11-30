//go:build unit

package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestBuildSimpleCommand(t *testing.T) {
	cmd := buildSimpleCommand("test", "short desc", "long desc", "action")

	if cmd == nil {
		t.Fatal("buildSimpleCommand returned nil")
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
	cmd := buildActionCommand("test", "short desc", "long desc", []string{"arg1"})

	if cmd == nil {
		t.Fatal("buildActionCommand returned nil")
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
	commands := map[string]*cobra.Command{
		"start":   startCmd,
		"stop":    stopCmd,
		"idle":    idleCmd,
		"hints":   hintsCmd,
		"grid":    gridCmd,
		"scroll":  scrollCmd,
		"action":  actionCmd,
		"status":  statusCmd,
		"doctor":  doctorCmd,
		"metrics": metricsCmd,
		"profile": profileCmd,
		"launch":  launchCmd,
	}

	for name, cmd := range commands {
		if cmd == nil {
			t.Errorf("%sCmd should not be nil", name)
		} else if cmd.Use != name {
			t.Errorf("%sCmd.Use = %q, want %q", name, cmd.Use, name)
		}
	}

	// Test action subcommands
	actionSubcommands := map[string]*cobra.Command{
		"left_click":   actionLeftClickCmd,
		"right_click":  actionRightClickCmd,
		"mouse_up":     actionMouseUpCmd,
		"mouse_down":   actionMouseDownCmd,
		"middle_click": actionMiddleClickCmd,
	}

	for name, cmd := range actionSubcommands {
		if cmd == nil {
			t.Errorf("action%sCmd should not be nil", name)
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
		{"start", startCmd, true},
		{"stop", stopCmd, true},
		{"idle", idleCmd, true},
		{"hints", hintsCmd, true},
		{"grid", gridCmd, true},
		{"scroll", scrollCmd, true},
		{"action", actionCmd, true},
		{"action_left_click", actionLeftClickCmd, true},
		{"action_right_click", actionRightClickCmd, true},
		{"action_mouse_up", actionMouseUpCmd, true},
		{"action_mouse_down", actionMouseDownCmd, true},
		{"action_middle_click", actionMiddleClickCmd, true},
		{"status", statusCmd, true},
		{"doctor", doctorCmd, true},
		{"metrics", metricsCmd, false}, // Metrics just prints status, doesn't need daemon
		{"profile", profileCmd, false}, // Profile just prints help, doesn't need daemon
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s panicked: %v", tt.name, r)
				}
			}()

			err := tt.cmd.RunE(tt.cmd, []string{})
			if tt.expectErr && err == nil {
				t.Errorf("expected error for %s when no daemon is running, got nil", tt.name)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error for %s: %v", tt.name, err)
			}
		})
	}
}

func TestLaunchCommandExecution(t *testing.T) {
	// Note: This test modifies global LaunchFunc and is not parallel-safe
	// Save original LaunchFunc
	originalLaunchFunc := LaunchFunc

	// Set a mock LaunchFunc for testing
	LaunchFunc = func(configPath string) {
		// Mock launch - do nothing
	}

	defer func() {
		// Restore original
		LaunchFunc = originalLaunchFunc

		if r := recover(); r != nil {
			t.Errorf("launchCmd.RunE panicked: %v", r)
		}
	}()

	// Launch command should work with mock LaunchFunc
	err := launchCmd.RunE(launchCmd, []string{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
