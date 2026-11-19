//go:build windows

package cli

import (
	"strings"
	"testing"
)

func TestRenderServiceTask_EscapesThePathAndNamesTheUser(t *testing.T) {
	rendered := renderServiceTask(`C:\Program Files\A&B <x>\neru.exe`, "S-1-5-21-1-2-3-1001")

	for _, want := range []string{
		`<Command>C:\Program Files\A&amp;B &lt;x&gt;\neru.exe</Command>`,
		`<Arguments>launch</Arguments>`,
		`<UserId>S-1-5-21-1-2-3-1001</UserId>`,
		`<LogonType>InteractiveToken</LogonType>`,
		`<ExecutionTimeLimit>PT0S</ExecutionTimeLimit>`,
		serviceTaskMarker,
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("renderServiceTask() lacks %q:\n%s", want, rendered)
		}
	}

	if strings.Contains(rendered, "NERU_") {
		t.Errorf("renderServiceTask() left a placeholder unfilled:\n%s", rendered)
	}
}

func TestTaskStateWord_UsesTheSchedulersOwnWords(t *testing.T) {
	testCases := []struct {
		state int32
		want  string
	}{
		{state: taskStateRunning, want: "running"},
		{state: taskStateReady, want: "ready (not running)"},
		{state: taskStateQueued, want: "queued"},
		{state: taskStateDisabled, want: "disabled"},
		{state: taskStateUnknown, want: "unknown"},
		{state: 99, want: taskWordUnknown},
	}

	for _, testCase := range testCases {
		if got := taskStateWord(testCase.state); got != testCase.want {
			t.Errorf("taskStateWord(%d) = %q, want %q", testCase.state, got, testCase.want)
		}
	}
}

func TestDescribeServiceStatus_ReadsAsAStatementNotAnError(t *testing.T) {
	testCases := []struct {
		name  string
		state serviceTaskState
		want  []string
	}{
		{
			name:  "not installed names the path and the next step",
			state: serviceTaskState{path: serviceTaskPath},
			want:  []string{"not installed", `\Neru`, "neru services install"},
		},
		{
			name: "running and enabled",
			state: serviceTaskState{
				installed:      true,
				path:           serviceTaskPath,
				state:          taskStateRunning,
				enabled:        true,
				triggerEnabled: true,
			},
			want: []string{"Service installed", taskWordRunning, "enabled at login"},
		},
		{
			name: "ready but disabled",
			state: serviceTaskState{
				installed:      true,
				path:           serviceTaskPath,
				state:          taskStateReady,
				triggerEnabled: true,
			},
			want: []string{taskWordReady, "disabled at login"},
		},
		{
			name: "task enabled but its logon trigger switched off",
			state: serviceTaskState{
				installed: true,
				path:      serviceTaskPath,
				state:     taskStateReady,
				enabled:   true,
			},
			want: []string{"disabled at login"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := describeServiceStatus(testCase.state)

			for _, want := range testCase.want {
				if !strings.Contains(got, want) {
					t.Errorf("describeServiceStatus() = %q, want it to contain %q", got, want)
				}
			}
		})
	}
}

// TestReportServiceStatus_SetsTheFlagsAScriptReads pins the booleans
// `neru services status --json` prints for each gathered state.
func TestReportServiceStatus_SetsTheFlagsAScriptReads(t *testing.T) {
	testCases := []struct {
		name  string
		state serviceTaskState
		want  serviceStatus
	}{
		{
			name:  "not installed",
			state: serviceTaskState{path: serviceTaskPath},
			want:  serviceStatus{Supported: true},
		},
		{
			name: "running and enabled",
			state: serviceTaskState{
				installed:      true,
				state:          taskStateRunning,
				enabled:        true,
				triggerEnabled: true,
			},
			want: serviceStatus{
				Supported:     true,
				Installed:     true,
				Running:       true,
				StartsAtLogin: true,
			},
		},
		{
			name: "ready with its logon trigger switched off",
			state: serviceTaskState{
				installed: true,
				state:     taskStateReady,
				enabled:   true,
			},
			want: serviceStatus{Supported: true, Installed: true},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := reportServiceStatus(testCase.state)
			got.Message = ""

			if got != testCase.want {
				t.Errorf("reportServiceStatus() = %+v, want %+v", got, testCase.want)
			}
		})
	}
}

func TestLogonTriggerEnabled_ReadsTheTriggersOwnFlag(t *testing.T) {
	testCases := []struct {
		name string
		xml  string
		want bool
	}{
		{
			name: "rendered template",
			xml:  renderServiceTask(`C:\neru.exe`, "S-1-5-21-1"),
			want: true,
		},
		{name: "no logon trigger", xml: "<Task><Triggers></Triggers></Task>", want: false},
		{
			name: "trigger switched off",
			xml:  "<Task><Triggers><LogonTrigger><Enabled>false</Enabled></LogonTrigger></Triggers></Task>",
			want: false,
		},
		{
			name: "another trigger off, logon on",
			xml: "<Task><Triggers><BootTrigger><Enabled>false</Enabled></BootTrigger>" +
				"<LogonTrigger><Enabled>true</Enabled></LogonTrigger></Triggers></Task>",
			want: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := logonTriggerEnabled(testCase.xml); got != testCase.want {
				t.Errorf("logonTriggerEnabled() = %v, want %v", got, testCase.want)
			}
		})
	}
}
