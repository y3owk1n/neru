package cliutil

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const (
	testCgoRequired    = "cgo_required"
	testPlatformDarwin = "darwin"
)

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
		"hotkeys_backend":             "cgeventtap-hotkeys",
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
		"  Hotkeys: cgeventtap-hotkeys (" + testCgoRequired + ")",
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

// The JSON form is what a script reads, so it must be the payload itself —
// parseable, and without the IPC envelope wrapped around it.
func TestPrintJSON(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer

	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	formatter := NewOutputFormatter()

	err := formatter.PrintJSON(cmd, map[string]any{
		"enabled": true,
		"mode":    "idle",
		"nested":  map[string]any{"platform": testPlatformDarwin},
	})
	if err != nil {
		t.Fatalf("PrintJSON() error = %v", err)
	}

	var decoded map[string]any

	decodeErr := json.Unmarshal(output.Bytes(), &decoded)
	if decodeErr != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", decodeErr, output.String())
	}

	if decoded["mode"] != "idle" || decoded["enabled"] != true {
		t.Fatalf("decoded = %v, want the payload verbatim", decoded)
	}

	if !strings.Contains(output.String(), "\n  \"mode\"") {
		t.Fatalf("output is not indented for reading:\n%s", output.String())
	}
}

// A payload that cannot be encoded has to surface as an error rather than as
// empty output a script would read as success.
func TestPrintJSON_ReportsUnencodablePayload(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})

	err := NewOutputFormatter().PrintJSON(cmd, make(chan int))
	if err == nil {
		t.Fatal("PrintJSON() = nil, want an error for an unencodable payload")
	}
}

func TestPrintToggles(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer

	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	printToggles(cmd, map[string]any{
		keyScrollInverted:        true,
		keyHiddenForScreenShare:  false,
		keyCursorFollowSelection: true,
	})

	got := output.String()

	expectedLines := []string{
		"  Scroll inverted: yes",
		"  Screen share hidden: no",
		"  Cursor follows selection: yes",
	}

	for _, expectedLine := range expectedLines {
		if !strings.Contains(got, expectedLine) {
			t.Fatalf("printToggles output missing %q in:\n%s", expectedLine, got)
		}
	}
}

// A toggle with no state is skipped, not printed as off: cursor-follow-selection
// is null while no mode is running, and "no" would name a state the daemon is
// not in.
func TestPrintToggles_SkipsAbsentState(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer

	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	printToggles(cmd, map[string]any{
		keyScrollInverted:        false,
		keyHiddenForScreenShare:  false,
		keyCursorFollowSelection: nil,
	})

	got := output.String()

	if strings.Contains(got, "Cursor follows selection") {
		t.Fatalf("printToggles printed a toggle that carries no state:\n%s", got)
	}

	if !strings.Contains(got, "  Scroll inverted: no") {
		t.Fatalf("printToggles output missing the toggles that do carry state:\n%s", got)
	}
}
