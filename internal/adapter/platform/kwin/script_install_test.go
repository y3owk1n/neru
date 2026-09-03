//go:build linux

package kwin

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestScriptDir_RefusesAnythingButAPrivateRuntimeDirectory pins where the
// geometry script is allowed to live.
//
// The script is code KWin executes, so the directory holding it decides who can
// decide what runs inside the compositor. A fallback to os.TempDir() used to
// cover an unset XDG_RUNTIME_DIR, which put a fixed-named executable script in
// a directory every user on the machine can write to; refusing is the only
// answer that is not a worse one.
func TestScriptDir_RefusesAnythingButAPrivateRuntimeDirectory(t *testing.T) {
	tests := []struct {
		name string
		// runtimeDir builds the value XDG_RUNTIME_DIR is set to. An empty
		// string is the unset session, which is what the variable reads as
		// when logind never made one.
		runtimeDir func(t *testing.T) string
		wantErr    error
	}{
		{
			name:       "unset is refused rather than falling back to a shared directory",
			runtimeDir: func(*testing.T) string { return "" },
			wantErr:    errNoRuntimeDir,
		},
		{
			name: "a private 0700 directory is what a logind session gives",
			runtimeDir: func(t *testing.T) string {
				t.Helper()

				return privateDir(t)
			},
		},
		{
			name: "a group- or world-readable directory is refused",
			runtimeDir: func(t *testing.T) string {
				t.Helper()

				dir := privateDir(t)
				chmodForTest(t, dir, 0o755)

				return dir
			},
			wantErr: errRuntimeDirNotPrivate,
		},
		{
			name: "a directory only others can write is refused too",
			runtimeDir: func(t *testing.T) string {
				t.Helper()

				dir := privateDir(t)
				chmodForTest(t, dir, 0o702)

				return dir
			},
			wantErr: errRuntimeDirNotPrivate,
		},
		{
			name: "a path that is not a directory is refused",
			runtimeDir: func(t *testing.T) string {
				t.Helper()

				path := filepath.Join(privateDir(t), "not-a-directory")
				writeFileForTest(t, path, "", 0o600)

				return path
			},
			wantErr: errRuntimeDirNotPrivate,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := test.runtimeDir(t)
			t.Setenv("XDG_RUNTIME_DIR", value)

			dir, err := scriptDir()

			if test.wantErr == nil {
				if err != nil {
					t.Fatalf("scriptDir() = %v, want the runtime directory", err)
				}

				if dir != value {
					t.Fatalf("scriptDir() = %q, want %q", dir, value)
				}

				return
			}

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("scriptDir() = (%q, %v), want %v", dir, err, test.wantErr)
			}
		})
	}
}

// TestWriteScript_ReplacesWhateverWasThereWithAnOwnerOnlyFile pins the write
// itself. Renaming a fresh file over the target is what keeps a stale copy's
// permissions from being inherited, keeps a symlink from redirecting the write,
// and keeps loadScript from ever being handed half a script.
func TestWriteScript_ReplacesWhateverWasThereWithAnOwnerOnlyFile(t *testing.T) {
	tests := []struct {
		name string
		// existing prepares whatever is already at the script's path.
		existing func(t *testing.T, path string)
		// verify checks anything beyond "the script is there, owner-only".
		verify func(t *testing.T, dir string)
	}{
		{
			name:     "a path with nothing at it",
			existing: func(*testing.T, string) {},
		},
		{
			name: "a stale copy left group- and world-readable",
			existing: func(t *testing.T, path string) {
				t.Helper()
				writeFileForTest(t, path, "stale", 0o666)
			},
		},
		{
			name: "a symlink pointing at a file elsewhere",
			existing: func(t *testing.T, path string) {
				t.Helper()

				target := filepath.Join(filepath.Dir(path), "elsewhere")
				writeFileForTest(t, target, "not the script", 0o600)

				err := os.Symlink(target, path)
				if err != nil {
					t.Fatalf("planting a symlink at the script path: %v", err)
				}
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()

				contents, err := os.ReadFile(filepath.Join(dir, "elsewhere"))
				if err != nil {
					t.Fatalf("reading the symlink's target: %v", err)
				}

				if string(contents) != "not the script" {
					t.Errorf("the symlink's target was overwritten with the script; "+
						"the write must replace the link, not follow it (target = %q)",
						string(contents))
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := privateDir(t)
			path := filepath.Join(dir, scriptFileName)

			test.existing(t, path)

			err := writeScript(path)
			if err != nil {
				t.Fatalf("writeScript() = %v, want the script written", err)
			}

			info, statErr := os.Lstat(path)
			if statErr != nil {
				t.Fatalf("stat of the written script: %v", statErr)
			}

			if info.Mode()&os.ModeSymlink != 0 {
				t.Fatal("the script path is still a symlink; the write followed it " +
					"instead of replacing it")
			}

			if info.Mode().Perm() != scriptFileMode {
				t.Errorf("the written script is mode %04o, want %04o",
					info.Mode().Perm(), scriptFileMode)
			}

			contents, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("reading the written script: %v", readErr)
			}

			if string(contents) != geometryScript {
				t.Error("the written script is not the geometry script")
			}

			if test.verify != nil {
				test.verify(t, dir)
			}

			assertNoWriteLeftovers(t, dir)
		})
	}
}

// privateDir gives a directory with the mode logind gives XDG_RUNTIME_DIR.
// t.TempDir already creates one, and the chmod says so out loud rather than
// leaving the case resting on that.
func privateDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	chmodForTest(t, dir, 0o700)

	return dir
}

func chmodForTest(t *testing.T, path string, mode os.FileMode) {
	t.Helper()

	err := os.Chmod(path, mode)
	if err != nil {
		t.Fatalf("chmod %04o on %s: %v", mode, filepath.Base(path), err)
	}
}

func writeFileForTest(t *testing.T, path string, contents string, mode os.FileMode) {
	t.Helper()

	err := os.WriteFile(path, []byte(contents), mode)
	if err != nil {
		t.Fatalf("writing %s: %v", filepath.Base(path), err)
	}
}

// assertNoWriteLeftovers fails when the write left its temporary file behind.
// One leftover is harmless; one per install attempt, in a runtime directory
// that lives as long as the session, is not, and the defer that removes it is
// easy to lose in a later edit.
func assertNoWriteLeftovers(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("listing the script directory: %v", err)
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "."+scriptFileName) {
			t.Errorf("writeScript left %s behind", entry.Name())
		}
	}
}
