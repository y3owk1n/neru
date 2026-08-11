//go:build integration && linux

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// requireSystemdMachine skips unless systemd booted this machine, which is the
// only configuration `neru services` claims on Linux.
func requireSystemdMachine(t *testing.T) {
	t.Helper()

	if !systemdIsInit(systemdRuntimeMarker) {
		t.Skip("skipping: this machine was not booted by systemd")
	}
}

// requireNoExistingUnit skips when the machine already has a neru.service, so a
// test never disturbs a real installation.
func requireNoExistingUnit(t *testing.T, unitPath string) {
	t.Helper()

	if serviceUnitExists(unitPath) {
		t.Skip("skipping: this machine already has a neru.service user unit")
	}
}

// TestRenderServiceUnit_IsAcceptedBySystemdAnalyze parses the unit Neru writes
// with systemd's own parser.
//
// Every other test here can pass while the unit is subtly wrong — a directive
// spelled the way the documentation spells the concept rather than the way
// systemd spells the key is accepted by every string comparison and ignored by
// systemd. `systemd-analyze verify` is the only reader whose opinion counts,
// and it reports an unparsed directive on stderr while still exiting 0, so the
// diagnostics are the assertion rather than the exit status.
func TestRenderServiceUnit_IsAcceptedBySystemdAnalyze(t *testing.T) {
	analyze, err := exec.LookPath("systemd-analyze")
	if err != nil {
		t.Skip("skipping: systemd-analyze is not installed")
	}

	// The verifier resolves ExecStart, so the unit points at a binary that
	// really exists — the test binary itself.
	binary, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	unitPath := filepath.Join(t.TempDir(), serviceUnitName)

	err = os.WriteFile(unitPath, []byte(renderServiceUnit(binary)), unitFilePerm)
	if err != nil {
		t.Fatalf("WriteFile(%s) error = %v", unitPath, err)
	}

	output, err := exec.CommandContext(t.Context(), analyze, "verify", unitPath).CombinedOutput()
	if err != nil {
		t.Fatalf("systemd-analyze verify error = %v, output:\n%s", err, output)
	}

	if diagnostics := strings.TrimSpace(string(output)); diagnostics != "" {
		t.Errorf("systemd-analyze verify reported diagnostics:\n%s", diagnostics)
	}
}

// TestStatusService_ReportsNotInstalledWhenTheUnitIsAbsent pins the acceptance
// criterion that matters most on a fresh machine: asking about a service nobody
// installed is answered, not refused.
func TestStatusService_ReportsNotInstalledWhenTheUnitIsAbsent(t *testing.T) {
	requireSystemdMachine(t)

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	unitPath, err := serviceUnitPath()
	if err != nil {
		t.Fatalf("serviceUnitPath() error = %v", err)
	}

	requireNoExistingUnit(t, unitPath)

	status := statusService()
	if !strings.Contains(status, "Service not installed") {
		t.Errorf("statusService() = %q, want it to report that nothing is installed", status)
	}

	if !strings.Contains(status, unitPath) {
		t.Errorf("statusService() = %q, want it to name %q", status, unitPath)
	}
}

// TestInstallService_RoundTripsThroughSystemd is the end-to-end claim: the unit
// Neru writes is one systemd accepts, enables and forgets again.
//
// It installs a real user unit and starts it, so it belongs to the tier that is
// allowed to take the machine over. The unit it installs points at the test
// binary, which exits immediately — the round trip is about systemd accepting
// and enabling the unit, not about the daemon staying up.
func TestInstallService_RoundTripsThroughSystemd(t *testing.T) {
	if os.Getenv("NERU_DESKTOP_TESTS") == "" {
		t.Skip("skipping machine-modifying test; run `just test-desktop` to include it")
	}

	requireSystemdMachine(t)

	// A machine systemd booted still need not have a *user* manager reachable
	// from this session — CI runners routinely do not — and every step below
	// talks to one.
	_, reachErr := systemctl("list-units", "--type=service")
	if reachErr != nil {
		t.Skip("skipping: no systemd user manager is reachable from this session")
	}

	unitPath, err := serviceUnitPath()
	if err != nil {
		t.Fatalf("serviceUnitPath() error = %v", err)
	}

	requireNoExistingUnit(t, unitPath)

	t.Cleanup(func() {
		uninstallErr := uninstallService()
		if uninstallErr != nil {
			t.Errorf("uninstallService() error = %v", uninstallErr)
		}

		_, statErr := os.Stat(unitPath)
		if !os.IsNotExist(statErr) {
			t.Errorf(
				"unit file still present at %s after uninstall (stat error = %v)",
				unitPath,
				statErr,
			)
		}

		status := statusService()
		if !strings.Contains(status, "Service not installed") {
			t.Errorf(
				"statusService() after uninstall = %q, want it to report nothing installed",
				status,
			)
		}
	})

	err = installService()
	if err != nil {
		t.Fatalf("installService() error = %v", err)
	}

	_, statErr := os.Stat(unitPath)
	if statErr != nil {
		t.Fatalf("unit file missing at %s after install: %v", unitPath, statErr)
	}

	enabled := systemctlWord("is-enabled")
	if enabled != "enabled" {
		t.Errorf("is-enabled = %q, want %q", enabled, "enabled")
	}

	status := statusService()
	if !strings.Contains(status, "Service installed") {
		t.Errorf("statusService() = %q, want it to report an installed unit", status)
	}

	reinstallErr := installService()
	if reinstallErr == nil {
		t.Error("installService() on top of an existing unit succeeded, want a refusal")
	}

	// The three verbs that only forward to systemctl. They are checked against
	// the installed unit rather than for a resulting run state: the unit points
	// at the test binary, which exits as soon as it is started, so what is being
	// claimed here is that systemd accepts each job.
	for _, verb := range []struct {
		name string
		call func() error
	}{
		{name: "stop", call: stopService},
		{name: "start", call: startService},
		{name: "restart", call: restartService},
	} {
		verbErr := verb.call()
		if verbErr != nil {
			t.Errorf("%sService() error = %v", verb.name, verbErr)
		}
	}
}
