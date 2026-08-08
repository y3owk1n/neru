package architecture_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestClaudeGuideIsSymlinkToAgentsGuide pins the cross-agent contract: each
// AGENTS.md — the root guide and every nested area guide — is the single
// source of truth, and a sibling CLAUDE.md symlink must accompany it so every
// agent runtime reads the same file (AGENTS.md "Agent Resources").
func TestClaudeGuideIsSymlinkToAgentsGuide(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows checkouts may materialize symlinks as plain files")
	}

	repoRoot := findRepoRoot(t)
	found := 0

	walkRepoFiles(t, repoRoot, func(file repoFile) {
		if file.name != "AGENTS.md" {
			return
		}

		found++

		assertSymlinkTarget(t, filepath.Join(filepath.Dir(file.abs), "CLAUDE.md"), "AGENTS.md")
	})

	// Two, not one: the root guide sits at the top of the checkout and would
	// still be reached by a walk that had lost everything below it, which is
	// exactly the pruning mistake worth catching. The nested area guides are
	// what prove the walk descended.
	assertWalkedAtLeast(t, "AGENTS.md agent guides", found, 2)
}

// TestAgentSkillsStayCanonical pins the skill layout: .agents/skills is the
// canonical home read by every runtime, .claude/skills must remain a single
// directory symlink to it, and every skill ships a SKILL.md. A skill body
// added under .claude/skills directly would silently fork the two views,
// which is exactly the drift this guards against.
func TestAgentSkillsStayCanonical(t *testing.T) {
	repoRoot := findRepoRoot(t)

	entries, err := os.ReadDir(filepath.Join(repoRoot, ".agents", "skills"))
	if err != nil {
		t.Fatalf("reading .agents/skills: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal(".agents/skills is empty; the project skills are gone")
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			t.Errorf(
				".agents/skills/%s is not a directory; skill bodies are canonical here",
				entry.Name(),
			)

			continue
		}

		skillFile := filepath.Join(repoRoot, ".agents", "skills", entry.Name(), "SKILL.md")

		_, err := os.Stat(skillFile)
		if err != nil {
			t.Errorf(".agents/skills/%s is missing SKILL.md: %v", entry.Name(), err)
		}
	}

	if runtime.GOOS == "windows" {
		return
	}

	assertSymlinkTarget(
		t,
		filepath.Join(repoRoot, ".claude", "skills"),
		filepath.Join("..", ".agents", "skills"),
	)
}

// TestAgentWorktreesStayIgnored pins the other half of the agent-worktree
// contract: agent tooling parks whole checkouts of this repository under
// .claude/worktrees, and git must ignore that path. Untracked, each one shows
// in git status — so the tree is never clean while agent work is in progress —
// and `git add -A` sweeps a second checkout into the index.
//
// The rule has two sides, and the ignore entry has to satisfy both: it names
// worktrees beneath .claude rather than .claude itself, because settings.json,
// the agent definitions and the skills symlink stay tracked.
//
// Pruning the path from the walks in this package (isSkippedWalkDir) is a
// separate concern: a directory git ignores is still a directory a guardrail
// walks.
func TestAgentWorktreesStayIgnored(t *testing.T) {
	repoRoot := findRepoRoot(t)

	content, err := os.ReadFile(filepath.Join(repoRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}

	// The path the entry has to name, taken from the walks' own constant so the
	// two cannot drift, and the directory above it — the shortcut that would
	// ignore the tracked agent files along with the worktrees, taking
	// TestAgentSkillsStayCanonical's subject with it.
	worktreesPattern := filepath.ToSlash(agentWorktreeDir)
	claudePattern := filepath.ToSlash(filepath.Dir(agentWorktreeDir))

	found := false

	for line := range strings.SplitSeq(string(content), "\n") {
		pattern := strings.TrimSpace(line)

		if pattern == "" || strings.HasPrefix(pattern, "#") {
			continue
		}

		// Surrounding slashes are decoration for a pattern already anchored to
		// the directory .gitignore sits in, so trimming them collapses
		// ".claude/worktrees", "/.claude/worktrees/" and the two mixtures onto
		// one spelling. A negation ("!") survives the trim and matches neither,
		// which is what we want.
		switch strings.Trim(pattern, "/") {
		case worktreesPattern:
			found = true
		case claudePattern:
			t.Errorf(
				".gitignore ignores %q, which untracks the agent guide layout; "+
					"name worktrees beneath it instead",
				pattern,
			)
		}
	}

	if !found {
		t.Error(
			".gitignore does not ignore .claude/worktrees; agent checkouts parked " +
				"there show as untracked and `git add -A` stages them",
		)
	}
}

// assertSymlinkTarget fails the test unless path is a symlink pointing at
// exactly target.
func assertSymlinkTarget(t *testing.T, path, target string) {
	t.Helper()

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("%s missing: %v", path, err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s must be a symlink to %s, not a real file or directory", path, target)
	}

	got, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("reading %s symlink: %v", path, err)
	}

	if got != target {
		t.Fatalf("%s points at %q, want %q", path, got, target)
	}
}
