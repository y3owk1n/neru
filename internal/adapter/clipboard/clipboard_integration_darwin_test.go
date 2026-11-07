//go:build integration && darwin

package clipboard_test

import (
	"context"
	"os/exec"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/clipboard"
)

// TestAdapter_WriteText_ReachesThePasteboard writes through the adapter and
// reads it back with pbpaste, text past ASCII included, since pbcopy mangles
// that when LANG is unset.
func TestAdapter_WriteText_ReachesThePasteboard(t *testing.T) {
	const text = "neru clipboard check: héllo wörld ✓"

	err := clipboard.NewAdapter(nil).WriteText(context.Background(), text)
	if err != nil {
		t.Fatalf("WriteText() = %v", err)
	}

	cmd := exec.CommandContext(context.Background(), "/usr/bin/pbpaste")
	cmd.Env = []string{"LANG=en_US.UTF-8"}

	got, err := cmd.Output()
	if err != nil {
		t.Fatalf("pbpaste: %v", err)
	}

	if string(got) != text {
		t.Fatalf("pasteboard holds %q, want %q", got, text)
	}
}
