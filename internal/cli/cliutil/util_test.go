//nolint:testpackage // Tests unexported CLI formatter helpers directly.
package cliutil

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const testCgoRequired = "cgo_required"

func TestIsHealthyHealthStatus(t *testing.T) {
	testCases := []struct {
		name         string
		componentKey string
		status       string
		want         bool
	}{
		{name: "ok status", componentKey: "event_tap", status: "ok (idle)", want: true},
		{
			name:         "supported capability",
			componentKey: "capability.overlay",
			status:       "supported",
			want:         true,
		},
		{
			name:         "platform metadata",
			componentKey: "capability.platform",
			status:       "darwin",
			want:         true,
		},
		{name: "stub capability", componentKey: "capability.overlay", status: "stub", want: false},
		{name: "error status", componentKey: "config", status: "not loaded", want: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := isHealthyHealthStatus(testCase.componentKey, testCase.status)
			if got != testCase.want {
				t.Fatalf("isHealthyHealthStatus() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestMaxComponentWidth(t *testing.T) {
	keys := []string{"config", "capability.dark_mode_detection", "event_tap"}

	got := maxComponentWidth(keys)
	if got != len("capability.dark_mode_detection") {
		t.Fatalf("maxComponentWidth() = %d, want %d", got, len("capability.dark_mode_detection"))
	}
}

func TestPrintProfile(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer

	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	printProfile(cmd, map[string]any{
		"primary_modifier":            "cmd",
		"display_server":              "cocoa",
		"accessibility_backend":       "axuielement",
		"accessibility_build_mode":    testCgoRequired,
		"hotkeys_backend":             "carbon-hotkeys",
		"hotkeys_build_mode":          testCgoRequired,
		"keyboard_capture_backend":    "quartz-event-tap",
		"keyboard_capture_build_mode": testCgoRequired,
		"overlay_backend":             "cocoa-overlay-window",
		"overlay_build_mode":          testCgoRequired,
		"notifications_backend":       "usernotifications/nsalert",
		"notifications_build_mode":    testCgoRequired,
	})

	got := output.String()

	expectedLines := []string{
		"  Primary:  cmd",
		"  Display:  cocoa",
		"  Accessibility: axuielement (" + testCgoRequired + ")",
		"  Hotkeys: carbon-hotkeys (" + testCgoRequired + ")",
		"  Keyboard: quartz-event-tap (" + testCgoRequired + ")",
		"  Overlay: cocoa-overlay-window (" + testCgoRequired + ")",
		"  Notifications: usernotifications/nsalert (" + testCgoRequired + ")",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(got, expectedLine) {
			t.Fatalf("PrintProfile output missing %q in:\n%s", expectedLine, got)
		}
	}
}
