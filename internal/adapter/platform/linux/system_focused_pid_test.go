//go:build linux

package linux

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
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

// TestWaylandNoFocusedAppError is the wlroots half of what #1495 fixed for X11.
// The blessed Wayland stack answers an unfocused session the same way an
// unfocused X11 display does, so `neru doctor` has to explain it the same way:
// "focused-app inspection is unavailable" on a session where nothing has taken
// focus yet sends the user looking for something to install.
func TestWaylandNoFocusedAppError(t *testing.T) {
	t.Parallel()

	err := waylandNoFocusedAppError()

	if !derrors.IsNotSupported(err) {
		t.Fatalf("waylandNoFocusedAppError() = %v, want %q so callers degrade through it",
			err, derrors.CodeNotSupported)
	}

	// The sentinel's premise is that a native backend answered, which a build
	// without the foreign-toplevel client cannot have done.
	wrapsSentinel := errors.Is(err, errNoFocusedWindow)
	if wrapsSentinel != nativeBackendsCompiledIn {
		t.Fatalf("waylandNoFocusedAppError() wraps errNoFocusedWindow = %v, want %v",
			wrapsSentinel, nativeBackendsCompiledIn)
	}

	if !nativeBackendsCompiledIn {
		return
	}

	feature := focusedAppFeature
	detail := NewSystemAdapter(backendWaylandWlroots).unavailableDetail(feature, err)

	if strings.Contains(detail, "unavailable") {
		t.Errorf("an unfocused wlroots session is described as %q; it must not claim "+
			"the capability is unavailable", detail)
	}

	// "takes focus" rather than "focus": the sentence has to promise the answer
	// arrives when a window is focused, not merely mention the word.
	for _, word := range []string{feature, backendWaylandWlroots, "takes focus"} {
		if !strings.Contains(detail, word) {
			t.Errorf("the unfocused-session detail %q does not mention %q", detail, word)
		}
	}
}
