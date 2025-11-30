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

func TestStartCommandExecution(t *testing.T) {
	// Test that executing the start command doesn't panic
	// It should return an error since no daemon is running, but not crash
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("startCmd.RunE panicked: %v", r)
		}
	}()

	err := startCmd.RunE(startCmd, []string{})
	// We expect an error since no daemon is running
	if err == nil {
		t.Error("expected error when no daemon is running, got nil")
	}
}

func TestStopCommandExecution(t *testing.T) {
	// Test that executing the stop command doesn't panic
	// It should return an error since no daemon is running, but not crash
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("stopCmd.RunE panicked: %v", r)
		}
	}()

	err := stopCmd.RunE(stopCmd, []string{})
	// We expect an error since no daemon is running
	if err == nil {
		t.Error("expected error when no daemon is running, got nil")
	}
}

func TestIdleCommandExecution(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("idleCmd.RunE panicked: %v", r)
		}
	}()

	err := idleCmd.RunE(idleCmd, []string{})
	if err == nil {
		t.Error("expected error when no daemon is running, got nil")
	}
}

func TestHintsCommandExecution(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("hintsCmd.RunE panicked: %v", r)
		}
	}()

	err := hintsCmd.RunE(hintsCmd, []string{})
	if err == nil {
		t.Error("expected error when no daemon is running, got nil")
	}
}

func TestGridCommandExecution(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("gridCmd.RunE panicked: %v", r)
		}
	}()

	err := gridCmd.RunE(gridCmd, []string{})
	if err == nil {
		t.Error("expected error when no daemon is running, got nil")
	}
}

func TestScrollCommandExecution(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("scrollCmd.RunE panicked: %v", r)
		}
	}()

	err := scrollCmd.RunE(scrollCmd, []string{})
	if err == nil {
		t.Error("expected error when no daemon is running, got nil")
	}
}

func TestActionCommandExecution(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("actionCmd.RunE panicked: %v", r)
		}
	}()

	err := actionCmd.RunE(actionCmd, []string{})
	if err == nil {
		t.Error("expected error when no daemon is running, got nil")
	}
}

func TestActionLeftClickCommandExecution(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("actionLeftClickCmd.RunE panicked: %v", r)
		}
	}()

	err := actionLeftClickCmd.RunE(actionLeftClickCmd, []string{})
	if err == nil {
		t.Error("expected error when no daemon is running, got nil")
	}
}

func TestActionRightClickCommandExecution(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("actionRightClickCmd.RunE panicked: %v", r)
		}
	}()

	err := actionRightClickCmd.RunE(actionRightClickCmd, []string{})
	if err == nil {
		t.Error("expected error when no daemon is running, got nil")
	}
}

func TestActionMouseUpCommandExecution(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("actionMouseUpCmd.RunE panicked: %v", r)
		}
	}()

	err := actionMouseUpCmd.RunE(actionMouseUpCmd, []string{})
	if err == nil {
		t.Error("expected error when no daemon is running, got nil")
	}
}

func TestActionMouseDownCommandExecution(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("actionMouseDownCmd.RunE panicked: %v", r)
		}
	}()

	err := actionMouseDownCmd.RunE(actionMouseDownCmd, []string{})
	if err == nil {
		t.Error("expected error when no daemon is running, got nil")
	}
}

func TestActionMiddleClickCommandExecution(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("actionMiddleClickCmd.RunE panicked: %v", r)
		}
	}()

	err := actionMiddleClickCmd.RunE(actionMiddleClickCmd, []string{})
	if err == nil {
		t.Error("expected error when no daemon is running, got nil")
	}
}

func TestStatusCommandExecution(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("statusCmd.RunE panicked: %v", r)
		}
	}()

	err := statusCmd.RunE(statusCmd, []string{})
	if err == nil {
		t.Error("expected error when no daemon is running, got nil")
	}
}

func TestDoctorCommandExecution(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("doctorCmd.RunE panicked: %v", r)
		}
	}()

	err := doctorCmd.RunE(doctorCmd, []string{})
	if err == nil {
		t.Error("expected error when no daemon is running, got nil")
	}
}

func TestMetricsCommandExecution(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("metricsCmd.RunE panicked: %v", r)
		}
	}()

	// Metrics command checks if server is running first
	err := metricsCmd.RunE(metricsCmd, []string{})
	// Should not error even without daemon, just print message
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProfileCommandExecution(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("profileCmd.RunE panicked: %v", r)
		}
	}()

	// Profile command just prints help, doesn't need daemon
	err := profileCmd.RunE(profileCmd, []string{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLaunchCommandExecution(t *testing.T) {
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
