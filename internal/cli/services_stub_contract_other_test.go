//go:build !darwin && !linux && !windows

package cli

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
)

// TestServicesStubsRefuseLoudly pins the fallback slot's half of the contract in
// internal/adapter/platform/AGENTS.md: a subcommand with no service manager
// behind it returns CodeNotSupported, never nil.
//
// Nothing else in the tree covers it — cli_test.go deliberately skips the
// services subcommands because on a real platform they have side effects — so
// without this a rewrite of the stubs could quietly start returning success.
func TestServicesStubsRefuseLoudly(t *testing.T) {
	testCases := []struct {
		name string
		call func() error
	}{
		{name: "install", call: installService},
		{name: "uninstall", call: uninstallService},
		{name: "start", call: startService},
		{name: "stop", call: stopService},
		{name: "restart", call: restartService},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.call()
			if !derrors.IsNotSupported(err) {
				t.Fatalf("%sService() error = %v, want %v",
					testCase.name, err, derrors.CodeNotSupported)
			}

			if !strings.Contains(err.Error(), testCase.name) {
				t.Errorf("%sService() message = %q, want it to name the subcommand",
					testCase.name, err.Error())
			}
		})
	}
}

// TestStatusServiceSaysItIsUnsupported covers the one subcommand whose signature
// carries no error, so the refusal has to be in the words.
func TestStatusServiceSaysItIsUnsupported(t *testing.T) {
	status := statusService()
	if !strings.Contains(status, "not supported") {
		t.Errorf("statusService() = %q, want it to say service management is unsupported", status)
	}
}
