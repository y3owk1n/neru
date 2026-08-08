package architecture_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// skippedWalkDirNames are the directory names every repository walk in this
// package skips: version control, build outputs and vendored third-party code,
// none of which contain first-party source subject to these guardrails.
var skippedWalkDirNames = map[string]bool{
	".git": true, "bin": true, "build": true,
	"node_modules": true, "vendor": true,
}

// agentWorktreeDir is where agent tooling parks its checkouts, relative to the
// repository root. Everything below it belongs to another copy of this
// repository, whatever state that copy is in.
var agentWorktreeDir = filepath.Join(".claude", "worktrees")

// isSkippedWalkDir reports whether the directory at path should be pruned from
// a walk over the checkout rooted at repoRoot.
//
// Beyond those names it prunes two things, both of which hold first-party
// source belonging to a *different* checkout: the directory agent tooling parks
// its worktrees in, and any directory below the root carrying its own .git
// entry — a nested worktree or clone parked anywhere else. A guardrail reading
// either reports a path the diff under test does not contain and cannot fix.
//
// Neither can be expressed by directory name, which is why this takes a path:
// the worktrees are named per run, a first-party directory may legitimately be
// called worktrees, and pruning .claude wholesale would blind the tests that
// pin the agent guide layout inside it.
func isSkippedWalkDir(repoRoot, path string) bool {
	// The root is never pruned. It carries the same .git marker as a nested
	// checkout, and it may sit in a directory called anything at all — a
	// checkout in ~/build pruned at the first callback would leave every walk
	// here visiting nothing and every guardrail passing green over it.
	if filepath.Clean(path) == filepath.Clean(repoRoot) {
		return false
	}

	if skippedWalkDirNames[filepath.Base(path)] {
		return true
	}

	if filepath.Clean(path) == filepath.Join(repoRoot, agentWorktreeDir) {
		return true
	}

	_, statErr := os.Lstat(filepath.Join(path, ".git"))

	return statErr == nil
}

// TestWalkPruning_SkipsOtherCheckoutsOnly pins what every walk in this package
// prunes, against a tree built to look like a checkout with agent worktrees
// parked inside it.
//
// Another checkout inside this one is the case worth a test: its files are
// first-party source of a different copy of this repository, so a guardrail
// that reads them reports a violation the current diff cannot contain and
// cannot fix. Neither obvious shortcut works — a directory named "worktrees"
// is walked here, and .claude survives whole, because
// TestClaudeGuideIsSymlinkToAgentsGuide and TestAgentSkillsStayCanonical live
// inside it.
func TestWalkPruning_SkipsOtherCheckoutsOnly(t *testing.T) {
	root := t.TempDir()

	// The tree under test carries its own .git; being a checkout is what makes
	// it the root, not something to prune.
	writeWalkFile(t, root, ".git/config")

	writeWalkFile(t, root, "internal/domain/keep.go")
	writeWalkFile(t, root, ".claude/skills/create-pr/SKILL.md")

	// A first-party directory that happens to be called worktrees.
	writeWalkFile(t, root, "worktrees/keep.go")

	// An agent worktree: git leaves a .git file pointing at the real git dir.
	writeWalkFile(t, root, ".claude/worktrees/agent-a/.git")
	writeWalkFile(t, root, ".claude/worktrees/agent-a/internal/domain/keep.go")

	// A nested clone, where .git is a directory instead.
	writeWalkFile(t, root, ".claude/worktrees/agent-b/.git/config")
	writeWalkFile(t, root, ".claude/worktrees/agent-b/cmd/neru/main.go")

	// A leftover with no .git at all — an interrupted removal, or tooling that
	// copies rather than checks out. Still another checkout's source.
	writeWalkFile(t, root, ".claude/worktrees/agent-c/internal/domain/keep.go")

	// A nested checkout parked outside .claude, which the name has no way to
	// reach.
	writeWalkFile(t, root, "tmp/scratch-checkout/.git")
	writeWalkFile(t, root, "tmp/scratch-checkout/internal/domain/keep.go")
	writeWalkFile(t, root, "tmp/keep.go")

	// The names pruned before nested checkouts were a concern, still pruned.
	writeWalkFile(t, root, "vendor/dep/keep.go")
	writeWalkFile(t, root, "node_modules/dep/index.js")

	want := []string{
		".claude/skills/create-pr/SKILL.md",
		"internal/domain/keep.go",
		"tmp/keep.go",
		"worktrees/keep.go",
	}

	got := walkUnskippedFiles(t, root)

	if !slices.Equal(got, want) {
		t.Errorf("walk visited\n\t%s\nwant\n\t%s",
			strings.Join(got, "\n\t"), strings.Join(want, "\n\t"))
	}
}

// TestWalkPruning_WalksARootNamedLikeASkippedDir keeps the pruning from ever
// swallowing the tree it is supposed to walk. The root is a checkout and may
// sit in a directory called anything — clone this repo into ~/build and a rule
// that judged the root by its own name or its .git would prune at the first
// callback, leaving every guardrail in this package passing over no files at
// all. Silent vacuity is the one failure a guardrail suite cannot report.
func TestWalkPruning_WalksARootNamedLikeASkippedDir(t *testing.T) {
	root := filepath.Join(t.TempDir(), "build")

	writeWalkFile(t, root, ".git/config")
	writeWalkFile(t, root, "internal/domain/keep.go")

	want := []string{"internal/domain/keep.go"}

	got := walkUnskippedFiles(t, root)

	if !slices.Equal(got, want) {
		t.Errorf("walk visited %v, want %v", got, want)
	}
}

// walkUnskippedFiles walks root the way every guardrail in this package does
// and returns the slash-relative paths of the files it reached, sorted.
func walkUnskippedFiles(t *testing.T, root string) []string {
	t.Helper()

	var found []string

	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if isSkippedWalkDir(root, path) {
				return filepath.SkipDir
			}

			return nil
		}

		relPath, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}

		found = append(found, filepath.ToSlash(relPath))

		return nil
	})
	if walkErr != nil {
		t.Fatalf("WalkDir(%s) error = %v", root, walkErr)
	}

	slices.Sort(found)

	return found
}

// writeWalkFile creates an empty file at relPath under root, parents included.
func writeWalkFile(t *testing.T, root, relPath string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(relPath))

	mkdirErr := os.MkdirAll(filepath.Dir(path), 0o750)
	if mkdirErr != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), mkdirErr)
	}

	writeErr := os.WriteFile(path, nil, 0o600)
	if writeErr != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, writeErr)
	}
}
