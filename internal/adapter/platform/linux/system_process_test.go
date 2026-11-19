//go:build linux

package linux

import (
	"os"
	"path/filepath"
	"testing"
)

// taskCommLen is the kernel's TASK_COMM_LEN minus the terminating NUL, the
// most of the executable's name /proc/<pid>/comm keeps.
const taskCommLen = 15

func TestLinuxApplicationNameByPID(t *testing.T) {
	t.Parallel()

	// The test process itself is a guaranteed-live PID with a readable
	// /proc/<pid>/comm on any Linux backend, CGO or not.
	name, err := linuxApplicationNameByPID(os.Getpid())
	if err != nil {
		t.Fatalf("linuxApplicationNameByPID(self) error: %v", err)
	}

	want := filepath.Base(os.Args[0])
	if len(want) > taskCommLen {
		want = want[:taskCommLen]
	}

	if name != want {
		t.Fatalf("linuxApplicationNameByPID(self) = %q, want %q", name, want)
	}
}

func TestLinuxApplicationBundleIDByPID(t *testing.T) {
	t.Parallel()

	bundleID, err := linuxApplicationBundleIDByPID(os.Getpid())
	if err != nil {
		t.Fatalf("linuxApplicationBundleIDByPID(self) error: %v", err)
	}

	// Per-app config matches on the basename of argv[0], not its full path.
	if want := filepath.Base(os.Args[0]); bundleID != want {
		t.Fatalf("linuxApplicationBundleIDByPID(self) = %q, want %q", bundleID, want)
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
