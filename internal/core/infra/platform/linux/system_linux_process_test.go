//go:build linux

//nolint:testpackage // Exercises unexported procfs helpers directly.
package linux

import (
	"os"
	"testing"
)

func TestLinuxApplicationNameByPID(t *testing.T) {
	t.Parallel()

	// The test process itself is a guaranteed-live PID with a readable
	// /proc/<pid>/comm on any Linux backend, CGO or not.
	name, err := linuxApplicationNameByPID(os.Getpid())
	if err != nil {
		t.Fatalf("linuxApplicationNameByPID(self) error: %v", err)
	}

	if name == "" {
		t.Fatal("linuxApplicationNameByPID(self) returned empty name")
	}
}

func TestLinuxApplicationBundleIDByPID(t *testing.T) {
	t.Parallel()

	id, err := linuxApplicationBundleIDByPID(os.Getpid())
	if err != nil {
		t.Fatalf("linuxApplicationBundleIDByPID(self) error: %v", err)
	}

	if id == "" {
		t.Fatal("linuxApplicationBundleIDByPID(self) returned empty identifier")
	}
}

func TestLinuxApplicationNameByPID_InvalidPID(t *testing.T) {
	t.Parallel()

	// PID -1 has no /proc entry, so the read must surface an error rather than
	// a bogus name.
	_, err := linuxApplicationNameByPID(-1)
	if err == nil {
		t.Fatal("linuxApplicationNameByPID(-1) expected an error, got nil")
	}
}
