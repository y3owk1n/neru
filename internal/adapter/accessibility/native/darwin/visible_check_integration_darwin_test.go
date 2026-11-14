//go:build integration && darwin

package darwin

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/accessibility/ax"
)

// visibleCheckSettings answers the visible check with one value for every app.
type visibleCheckSettings struct{ visible bool }

func (s *visibleCheckSettings) HintsMaxDepth() int { return 0 }

func (s *visibleCheckSettings) ShouldIgnoreClickableCheckForApp(string) bool { return false }

func (s *visibleCheckSettings) ShouldEnableVisibleCheckForApp(string) bool { return s.visible }

// TestResolveVisibleCheckEnabled_FollowsANewConfiguration pins that the
// per-process cache clears when the settings source returns a new value, so a
// reload that flips a per-app visible check reaches the next walk instead of
// the answer cached under the old configuration.
func TestResolveVisibleCheckEnabled_FollowsANewConfiguration(t *testing.T) {
	out, err := exec.CommandContext(t.Context(), "pgrep", "-x", "Dock").Output()
	if err != nil {
		t.Skip("no Dock process to read a bundle identifier from")
	}

	pid, err := strconv.Atoi(strings.Fields(string(out))[0])
	if err != nil {
		t.Fatalf("pgrep printed %q, want a pid", out)
	}

	current := ax.Settings(&visibleCheckSettings{visible: true})
	source := func() ax.Settings { return current }

	if !resolveVisibleCheckEnabled(pid, source) {
		t.Fatal("resolveVisibleCheckEnabled() = false under a configuration that enables it")
	}

	current = &visibleCheckSettings{visible: false}

	if resolveVisibleCheckEnabled(pid, source) {
		t.Error("resolveVisibleCheckEnabled() = true after a configuration that disables it, " +
			"so the cache kept the old answer")
	}
}
