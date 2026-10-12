package cli_test

import (
	"encoding/json"
	"runtime"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
)

// goosWindows is the one platform where a stopped daemon passes the doctor.
const goosWindows = "windows"

// TestDoctorCmd_JSONWithoutADaemon pins `neru doctor --json` while no daemon
// runs: one object on stdout, no daemon section, and an exit status that
// fails exactly when the object says it is unhealthy.
func TestDoctorCmd_JSONWithoutADaemon(t *testing.T) {
	// Point the socket lookup at an empty directory, so a daemon running on
	// the machine cannot answer.
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	// Windows names its pipe after the user, so a daemon running there still
	// answers.
	if ipc.IsServerRunning() {
		t.Skip("a running daemon answers on this machine")
	}

	// Roles that belong to a platform other than the one running the test.
	foreign := map[string]string{
		"darwin":    `["atspi:push button", "uia:Button"]`,
		"linux":     `["ax:AXButton", "uia:Button"]`,
		goosWindows: `["ax:AXButton", "atspi:push button"]`,
	}

	tests := []struct {
		name        string
		roles       string
		wantUsable  bool
		wantHealthy bool
	}{
		{
			name:       "roles that apply here",
			roles:      `["button"]`,
			wantUsable: true,
			// Only Windows runs its client-side probes instead of failing.
			wantHealthy: runtime.GOOS == goosWindows,
		},
		{
			name:  "roles for other platforms only",
			roles: foreign[runtime.GOOS],
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.roles == "" {
				t.Skipf("no foreign role set defined for %s", runtime.GOOS)
			}

			path := writeRolesConfig(t, testCase.roles)

			stdout, stderr, err := executeRoot(t, "doctor", "--json", "--config", path)

			var report struct {
				Healthy bool `json:"healthy"`
				Config  struct {
					Path string `json:"path"`
				} `json:"config"`
				ClickableRoles struct {
					Usable bool `json:"usable"`
				} `json:"clickable_roles"` //nolint:tagliatelle // the keys neru doctor --json prints.
				IPCEndpoint struct {
					Reachable bool `json:"reachable"`
				} `json:"ipc_endpoint"` //nolint:tagliatelle // the keys neru doctor --json prints.
				Daemon any `json:"daemon"`
			}

			decodeErr := json.Unmarshal([]byte(stdout), &report)
			if decodeErr != nil {
				t.Fatalf("neru doctor --json is not a JSON object: %v\n%s", decodeErr, stdout)
			}

			if stderr != "" {
				t.Errorf("neru doctor --json wrote to stderr:\n%s", stderr)
			}

			if report.Config.Path != path {
				t.Errorf("config.path = %q, want %q", report.Config.Path, path)
			}

			if report.ClickableRoles.Usable != testCase.wantUsable {
				t.Errorf("clickable_roles.usable = %v, want %v",
					report.ClickableRoles.Usable, testCase.wantUsable)
			}

			if report.IPCEndpoint.Reachable || report.Daemon != nil {
				t.Errorf("ipc_endpoint.reachable = %v, daemon = %v, want false and null",
					report.IPCEndpoint.Reachable, report.Daemon)
			}

			if report.Healthy != testCase.wantHealthy {
				t.Errorf("healthy = %v, want %v", report.Healthy, testCase.wantHealthy)
			}

			if (err == nil) != report.Healthy {
				t.Errorf("exit error = %v with healthy = %v, want an error exactly when unhealthy",
					err, report.Healthy)
			}
		})
	}
}
