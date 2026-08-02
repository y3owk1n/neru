//nolint:testpackage // Tests private mode option parsing.
package app

import (
	"slices"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
)

const (
	// onExitTestFlag and onExitTestMode keep the repeated literals out of the
	// table below.
	onExitTestFlag = "--on-exit"
	onExitTestMode = "hints"
	onExitTestStep = "action sleep 0.2"
)

// --on-exit is repeatable so a mode can finish with a sequence rather than a
// single command; each occurrence appends one step, in order.
func TestExtractModeOptions_OnExitAccumulatesSteps(t *testing.T) {
	t.Parallel()

	handler := NewIPCControllerModes(nil, zap.NewNop())

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "space separated",
			args: []string{
				onExitTestMode,
				onExitTestFlag,
				onExitTestStep,
				onExitTestFlag,
				onExitTestMode,
			},
			want: []string{onExitTestStep, onExitTestMode},
		},
		{
			name: "equals form",
			args: []string{
				onExitTestMode,
				onExitTestFlag + "=" + onExitTestStep,
				onExitTestFlag + "=" + onExitTestMode,
			},
			want: []string{onExitTestStep, onExitTestMode},
		},
		{
			name: "mixed forms keep order",
			args: []string{onExitTestMode, onExitTestFlag + "=first", onExitTestFlag, "second"},
			want: []string{"first", "second"},
		},
		{
			name: "omitted stays nil",
			args: []string{onExitTestMode},
			want: nil,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			opts, errResp := handler.extractModeOptions(ipc.Command{
				Action: onExitTestMode,
				Args:   testCase.args,
			})
			if errResp != nil {
				t.Fatalf("extractModeOptions() error response: %+v", *errResp)
			}

			if !slices.Equal(opts.OnExit, testCase.want) {
				t.Fatalf("OnExit = %v, want %v", opts.OnExit, testCase.want)
			}
		})
	}
}

func TestExtractModeOptions_OnExitRequiresValue(t *testing.T) {
	t.Parallel()

	handler := NewIPCControllerModes(nil, zap.NewNop())

	_, errResp := handler.extractModeOptions(ipc.Command{
		Action: onExitTestMode,
		Args:   []string{onExitTestMode, onExitTestFlag},
	})

	if errResp == nil {
		t.Fatal("extractModeOptions() = nil error response, want a failure")
	}

	if errResp.Code != ipc.CodeInvalidInput {
		t.Fatalf("code = %q, want %q", errResp.Code, ipc.CodeInvalidInput)
	}
}
