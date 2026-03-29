//nolint:testpackage // Tests unexported CLI formatter helpers directly.
package cliutil

import "testing"

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
