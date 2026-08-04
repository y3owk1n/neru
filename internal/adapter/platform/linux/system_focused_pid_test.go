//go:build linux

package linux

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

const (
	testAppFirefox   = "firefox"
	testAppKonsoleID = "org.kde.konsole"
	testAppKonsole   = "konsole"
)

func TestAppIDMatchesProcess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		appID       string
		comm        string
		exeBase     string
		cmdlineBase string
		want        bool
	}{
		{
			name:  "plain app_id matches comm",
			appID: testAppFirefox,
			comm:  testAppFirefox,
			want:  true,
		},
		{
			name:    "reverse-dns app_id matches exe basename",
			appID:   testAppKonsoleID,
			exeBase: testAppKonsole,
			want:    true,
		},
		{
			name:  "reverse-dns app_id matches comm on last segment",
			appID: "org.mozilla.firefox",
			comm:  testAppFirefox,
			want:  true,
		},
		{
			name:        "case-insensitive match against cmdline",
			appID:       "Alacritty",
			cmdlineBase: "alacritty",
			want:        true,
		},
		{
			name:  "full dotted app_id matches full process name",
			appID: "com.example.app",
			comm:  "com.example.app",
			want:  true,
		},
		{
			name:  "no substring false positive",
			appID: "code",
			comm:  "codeium-helper",
			want:  false,
		},
		{
			name:  "unrelated process does not match",
			appID: testAppKonsoleID,
			comm:  testAppFirefox,
			want:  false,
		},
		{
			name:  "empty app_id never matches",
			appID: "",
			comm:  testAppFirefox,
			want:  false,
		},
		{
			name:  "app_id with no candidates does not match",
			appID: testAppFirefox,
			want:  false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := appIDMatchesProcess(
				testCase.appID,
				testCase.comm,
				testCase.exeBase,
				testCase.cmdlineBase,
			)
			if got != testCase.want {
				t.Fatalf(
					"appIDMatchesProcess(%q, comm=%q, exe=%q, cmd=%q) = %v, want %v",
					testCase.appID,
					testCase.comm,
					testCase.exeBase,
					testCase.cmdlineBase,
					got,
					testCase.want,
				)
			}
		})
	}
}

func TestLastDottedSegment(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"org.kde.konsole": "konsole",
		"firefox":         "",
		"trailing.dot.":   "",
		".leading":        "leading",
		"a.b":             "b",
	}

	for in, want := range tests {
		if got := lastDottedSegment(in); got != want {
			t.Errorf("lastDottedSegment(%q) = %q, want %q", in, got, want)
		}
	}
}

// writeFakeProc builds a minimal /proc-like tree under a temp dir so the
// scanner can be exercised without touching the real /proc.
func writeFakeProc(t *testing.T, procs map[int]string) string {
	t.Helper()

	root := t.TempDir()
	for pid, comm := range procs {
		dir := filepath.Join(root, strconv.Itoa(pid))

		err := os.MkdirAll(dir, 0o755)
		if err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}

		err = os.WriteFile(filepath.Join(dir, "comm"), []byte(comm+"\n"), 0o644)
		if err != nil {
			t.Fatalf("write comm: %v", err)
		}
	}

	// A non-numeric entry that must be ignored by the scanner.
	err := os.MkdirAll(filepath.Join(root, "self"), 0o755)
	if err != nil {
		t.Fatalf("mkdir self: %v", err)
	}

	return root
}

func TestResolvePIDByAppID(t *testing.T) {
	t.Parallel()

	root := writeFakeProc(t, map[int]string{
		101: testAppFirefox,
		202: testAppKonsole,
		303: "some-daemon",
	})

	t.Run("matches reverse-dns app_id", func(t *testing.T) {
		t.Parallel()

		pid, ok := resolvePIDByAppID(testAppKonsoleID, root)
		if !ok || pid != 202 {
			t.Fatalf("resolvePIDByAppID = (%d, %v), want (202, true)", pid, ok)
		}
	})

	t.Run("matches plain app_id", func(t *testing.T) {
		t.Parallel()

		pid, ok := resolvePIDByAppID(testAppFirefox, root)
		if !ok || pid != 101 {
			t.Fatalf("resolvePIDByAppID = (%d, %v), want (101, true)", pid, ok)
		}
	})

	t.Run("no match returns false", func(t *testing.T) {
		t.Parallel()

		pid, ok := resolvePIDByAppID("org.gnome.Nautilus", root)
		if ok || pid != 0 {
			t.Fatalf("resolvePIDByAppID = (%d, %v), want (0, false)", pid, ok)
		}
	})

	t.Run("missing proc root returns false", func(t *testing.T) {
		t.Parallel()

		pid, ok := resolvePIDByAppID(testAppFirefox, filepath.Join(root, "does-not-exist"))
		if ok || pid != 0 {
			t.Fatalf("resolvePIDByAppID = (%d, %v), want (0, false)", pid, ok)
		}
	})
}
