//go:build windows

// internal/cli/doctor_client_windows.go
// Client-side neru doctor checks when the Windows daemon is not running.
// Does not query overlay, hotkeys, or IPC-backed components.

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/core/infra/platform"
	"github.com/y3owk1n/neru/internal/core/ports"
)

func printClientDoctorWithoutDaemon(cmd *cobra.Command) error {
	cmd.Println()
	cmd.Println("Daemon not running. Showing client-side platform checks.")
	cmd.Println()

	adapter, err := platform.NewSystemPort()
	if err != nil {
		return err
	}
	ctx := context.Background()
	caps := adapter.Capabilities()
	profile := platform.CurrentProfile()

	cmd.Println("Platform capabilities:")
	printCapabilityLine(cmd, "platform", caps.Platform, "")
	printCapabilityLine(cmd, "process", string(caps.Process.Status), caps.Process.Detail)
	printCapabilityLine(cmd, "screen", string(caps.Screen.Status), caps.Screen.Detail)
	printCapabilityLine(cmd, "cursor", string(caps.Cursor.Status), caps.Cursor.Detail)
	printCapabilityLine(
		cmd,
		"accessibility",
		string(caps.Accessibility.Status),
		caps.Accessibility.Detail,
	)
	printCapabilityLine(cmd, "overlay", string(caps.Overlay.Status), caps.Overlay.Detail)
	printCapabilityLine(
		cmd,
		"global_hotkeys",
		string(caps.GlobalHotkeys.Status),
		caps.GlobalHotkeys.Detail,
	)
	printCapabilityLine(
		cmd,
		"keyboard_event_tap",
		string(caps.KeyboardEventTap.Status),
		caps.KeyboardEventTap.Detail,
	)

	cmd.Println()
	cmd.Println("Live probes:")
	probeScreen(ctx, cmd, adapter)
	probeCursor(ctx, cmd, adapter)
	probeProcess(ctx, cmd, adapter)

	cmd.Println()
	cmd.Printf("  Primary:  %s\n", profile.PrimaryModifier)
	cmd.Printf("  Display:  %s\n", profile.DisplayServer)
	cmd.Println()
	cmd.Println("Start the daemon with: neru launch")

	return nil
}

func printCapabilityLine(cmd *cobra.Command, name, status, detail string) {
	if detail != "" {
		cmd.Printf("  %-22s %s (%s)\n", name+":", status, detail)

		return
	}

	cmd.Printf("  %-22s %s\n", name+":", status)
}

func probeScreen(ctx context.Context, cmd *cobra.Command, adapter ports.SystemPort) {
	bounds, err := adapter.ScreenBounds(ctx)
	if err != nil {
		cmd.Printf("  %-22s error: %v\n", "screen_bounds:", err)

		return
	}

	cmd.Printf(
		"  %-22s %dx%d at (%d,%d)\n",
		"screen_bounds:",
		bounds.Dx(),
		bounds.Dy(),
		bounds.Min.X,
		bounds.Min.Y,
	)

	names, err := adapter.ScreenNames(ctx)
	if err != nil {
		cmd.Printf("  %-22s error: %v\n", "screen_names:", err)

		return
	}

	cmd.Printf("  %-22s %d monitor(s): %s\n", "screen_names:", len(names), fmt.Sprint(names))
}

func probeCursor(ctx context.Context, cmd *cobra.Command, adapter ports.SystemPort) {
	pos, err := adapter.CursorPosition(ctx)
	if err != nil {
		cmd.Printf(
			"  %-22s unavailable in this session (%v)\n",
			"cursor_position:",
			err,
		)

		return
	}

	cmd.Printf("  %-22s (%d,%d)\n", "cursor_position:", pos.X, pos.Y)
}

func probeProcess(ctx context.Context, cmd *cobra.Command, adapter ports.SystemPort) {
	pid, err := adapter.FocusedApplicationPID(ctx)
	if err != nil {
		cmd.Printf(
			"  %-22s unavailable in this session (%v)\n",
			"focused_app_pid:",
			err,
		)

		return
	}

	name, err := adapter.ApplicationNameByPID(ctx, pid)
	if err != nil {
		cmd.Printf("  %-22s pid=%d name error: %v\n", "focused_app:", pid, err)

		return
	}

	cmd.Printf("  %-22s pid=%d (%s)\n", "focused_app:", pid, name)
}
