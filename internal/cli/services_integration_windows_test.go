//go:build integration && windows

package cli

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
)

// testTaskPath is a task name no real install uses, so the round trip below
// never touches a \Neru task the machine already has.
func testTaskPath(t *testing.T) string {
	t.Helper()

	return fmt.Sprintf(`\NeruTest-%d`, os.Getpid())
}

// requireTaskScheduler skips when the scheduler cannot be reached at all,
// which a locked-down session reports as an error on the very first call.
func requireTaskScheduler(t *testing.T, path string) {
	t.Helper()

	status := statusServiceTask(path)
	if strings.HasPrefix(status, "Service status unavailable") {
		t.Skipf("skipping: %s", status)
	}
}

// TestServiceTask_RoundTrip registers, inspects, drives and deletes a task on
// the real scheduler, which is the only reader whose opinion of the XML counts.
//
// The action is this test binary with `launch` as its argument, which exits at
// once, so the task's state after Run may already be back to ready; the
// assertion is that Run was accepted, and that status reports an installed
// task, not that a daemon stays up.
func TestServiceTask_RoundTrip(t *testing.T) {
	path := testTaskPath(t)
	requireTaskScheduler(t, path)

	t.Cleanup(func() { _ = uninstallServiceTask(path) })

	if status := statusServiceTask(path); !strings.Contains(status, "not installed") {
		t.Fatalf("statusServiceTask() before install = %q, want not installed", status)
	}

	err := installServiceTask(path)
	if err != nil {
		t.Fatalf("installServiceTask() error = %v", err)
	}

	err = installServiceTask(path)
	if !derrors.IsCode(err, derrors.CodeInvalidInput) {
		t.Errorf("second installServiceTask() error = %v, want %v", err, derrors.CodeInvalidInput)
	}

	status := statusServiceTask(path)
	if !strings.HasPrefix(status, "Service installed: ") ||
		!strings.Contains(status, "enabled at login") {
		t.Errorf("statusServiceTask() after install = %q, want installed and enabled", status)
	}

	for _, step := range []struct {
		name string
		call func(string) error
	}{
		{name: "stop", call: func(p string) error { return driveServiceTask("stop", p, stopTask) }},
		{name: "start", call: func(p string) error { return driveServiceTask("start", p, runTask) }},
		{name: "restart", call: func(p string) error { return driveServiceTask("restart", p, restartTask) }},
	} {
		err = step.call(path)
		if err != nil {
			t.Errorf("%s error = %v", step.name, err)
		}
	}

	err = uninstallServiceTask(path)
	if err != nil {
		t.Fatalf("uninstallServiceTask() error = %v", err)
	}

	if status := statusServiceTask(path); !strings.Contains(status, "not installed") {
		t.Errorf("statusServiceTask() after uninstall = %q, want not installed", status)
	}

	err = uninstallServiceTask(path)
	if err != nil {
		t.Errorf("uninstallServiceTask() on nothing = %v, want nil", err)
	}
}

// TestDriveServiceTask_RefusesWhenNothingIsInstalled pins that start, stop and
// restart on a machine with no task say so rather than reporting success.
func TestDriveServiceTask_RefusesWhenNothingIsInstalled(t *testing.T) {
	path := testTaskPath(t)
	requireTaskScheduler(t, path)

	err := driveServiceTask("start", path, runTask)
	if !derrors.IsCode(err, derrors.CodeInvalidInput) {
		t.Fatalf("driveServiceTask() error = %v, want %v", err, derrors.CodeInvalidInput)
	}

	if !strings.Contains(err.Error(), "neru services install") {
		t.Errorf("driveServiceTask() message = %q, want it to name the next step", err.Error())
	}
}
