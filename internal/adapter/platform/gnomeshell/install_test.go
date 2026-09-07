//go:build linux

package gnomeshell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtensionDir_FollowsXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/data")

	if got, want := extensionDir(), filepath.Join(
		"/tmp/data",
		"gnome-shell",
		"extensions",
		UUID,
	); got != want {
		t.Fatalf("extensionDir() = %q, want %q", got, want)
	}

	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "/home/someone")

	if got, want := extensionDir(), filepath.Join(
		"/home/someone/.local/share/gnome-shell/extensions",
		UUID,
	); got != want {
		t.Fatalf("extensionDir() = %q, want %q", got, want)
	}
}

func TestInstall_WritesOnceAndLeavesAnUpToDateCopyAlone(t *testing.T) {
	dir := filepath.Join(t.TempDir(), UUID)

	if Installed(dir) {
		t.Fatal("Installed() = true before anything was written")
	}

	changed, err := Install(dir)
	if err != nil || !changed {
		t.Fatalf("first Install() = (%v, %v), want a write", changed, err)
	}

	for _, name := range extensionFileNames {
		_, err = os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s not written: %v", name, err)
		}
	}

	changed, err = Install(dir)
	if err != nil || changed {
		t.Fatalf("second Install() = (%v, %v), want nothing written", changed, err)
	}

	if !Installed(dir) {
		t.Fatal("Installed() = false after a write")
	}

	err = os.WriteFile(filepath.Join(dir, "extension.js"), []byte("stale"), extensionFileMode)
	if err != nil {
		t.Fatal(err)
	}

	changed, err = Install(dir)
	if err != nil || !changed {
		t.Fatalf("Install() over a stale copy = (%v, %v), want a rewrite", changed, err)
	}
}

const otherUUID = "a@b.c"

func TestParseStringArray_ReadsGSettingsOutput(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "empty typed array", text: "@as []\n", want: nil},
		{name: "empty array", text: "[]", want: nil},
		{name: "two uuids", text: "['a@b.c', 'd@e.f']\n", want: []string{otherUUID, "d@e.f"}},
		{name: "double quotes", text: `["a@b.c"]`, want: []string{otherUUID}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := parseStringArray(testCase.text)
			if strings.Join(got, ",") != strings.Join(testCase.want, ",") {
				t.Fatalf("parseStringArray(%q) = %q, want %q", testCase.text, got, testCase.want)
			}
		})
	}

	if got, want := formatStringArray(
		[]string{otherUUID, UUID},
	), "['a@b.c', '"+UUID+"']"; got != want {
		t.Fatalf("formatStringArray() = %q, want %q", got, want)
	}
}
