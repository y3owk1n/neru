//go:build linux

package clipboard_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/clipboard"
	"github.com/y3owk1n/neru/internal/derrors"
)

// fakeCopier installs an executable named name in dir that writes its standard
// input to out, standing in for a real clipboard tool.
func fakeCopier(t *testing.T, dir, name, out string) {
	t.Helper()

	script := "#!/bin/sh\n/bin/cat > " + out + "\n"

	err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755) //nolint:gosec
	if err != nil {
		t.Fatalf("write fake %s: %v", name, err)
	}
}

func TestAdapter_WriteText_UsesTheCopierForTheSession(t *testing.T) {
	tests := []struct {
		name     string
		wayland  string
		display  string
		install  []string
		wantTool string
	}{
		{"wayland prefers wl-copy", "wayland-0", ":0", []string{"wl-copy", "xclip"}, "wl-copy"},
		{
			"wayland without wl-copy falls back to Xwayland",
			"wayland-0",
			":0",
			[]string{"xclip"},
			"xclip",
		},
		{"x11 uses xclip", "", ":0", []string{"wl-copy", "xclip", "xsel"}, "xclip"},
		{"x11 without xclip uses xsel", "", ":0", []string{"xsel"}, "xsel"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			bin := t.TempDir()
			outs := map[string]string{}

			for _, tool := range testCase.install {
				outs[tool] = filepath.Join(t.TempDir(), tool+".out")
				fakeCopier(t, bin, tool, outs[tool])
			}

			t.Setenv("PATH", bin)
			t.Setenv("WAYLAND_DISPLAY", testCase.wayland)
			t.Setenv("DISPLAY", testCase.display)

			err := clipboard.NewAdapter(nil).WriteText(context.Background(), "héllo\x00 wörld")
			if err != nil {
				t.Fatalf("WriteText() = %v, want nil", err)
			}

			for tool, out := range outs {
				got, readErr := os.ReadFile(out)
				if tool != testCase.wantTool {
					if readErr == nil {
						t.Errorf("%s ran, want only %s", tool, testCase.wantTool)
					}

					continue
				}

				if readErr != nil {
					t.Fatalf("%s did not run: %v", tool, readErr)
				}

				if string(got) != "héllo wörld" {
					t.Errorf("%s received %q, want the text with NUL bytes dropped", tool, got)
				}
			}
		})
	}
}

func TestAdapter_WriteText_WithoutACopierIsNotSupported(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", ":0")

	err := clipboard.NewAdapter(nil).WriteText(context.Background(), "text")
	if !derrors.IsNotSupported(err) {
		t.Fatalf("WriteText() = %v, want CodeNotSupported naming what to install", err)
	}
}
