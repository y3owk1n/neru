package architecture_test

import (
	"go/ast"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// bulkWalkFloor is the floor a guardrail asserts when its subject is a whole
// class of file — every Go file, every test file, every contributor doc. The
// checkout holds hundreds of each, so ten catches a walk that has lost its
// root or pruned its subject and never fires on ordinary growth.
const bulkWalkFloor = 10

// keptGoFile is the first-party source file the fixture trees below expect a
// walk to reach, wherever they park it.
const keptGoFile = "internal/domain/keep.go"

// skippedWalkDirNames are the directory names every repository walk in this
// package skips: version control, build outputs and vendored third-party code,
// none of which contain first-party source subject to these guardrails.
//
// This is the only such list in the package. doc_links_test.go used to carry a
// second one naming .devbox, .opencode and demos as well; those three are not
// here because adding them would prune trees the other walks read today, and
// pruning more is a change to what a guardrail detects rather than a
// consolidation of where the rule lives. That list never did any work of its
// own — the root-and-docs rule beside it had already excluded all three.
var skippedWalkDirNames = map[string]bool{
	".git": true, "bin": true, "build": true,
	"node_modules": true, "vendor": true,
}

// agentWorktreeDir is where agent tooling parks its checkouts, relative to the
// repository root. Everything below it belongs to another copy of this
// repository, whatever state that copy is in.
var agentWorktreeDir = filepath.Join(".claude", "worktrees")

// isSkippedWalkDir reports whether the directory at dirPath should be pruned
// from a walk over the checkout rooted at repoRoot.
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
func isSkippedWalkDir(repoRoot, dirPath string) bool {
	// The root is never pruned. It carries the same .git marker as a nested
	// checkout, and it may sit in a directory called anything at all — a
	// checkout in ~/build pruned at the first callback would leave every walk
	// here visiting nothing and every guardrail passing green over it.
	if filepath.Clean(dirPath) == filepath.Clean(repoRoot) {
		return false
	}

	if skippedWalkDirNames[filepath.Base(dirPath)] {
		return true
	}

	if filepath.Clean(dirPath) == filepath.Join(repoRoot, agentWorktreeDir) {
		return true
	}

	_, statErr := os.Lstat(filepath.Join(dirPath, ".git"))

	return statErr == nil
}

// findRepoRoot locates the checkout this test binary was built from, rather
// than the working directory it happens to run in, so a guardrail reads the
// tree it is guarding.
func findRepoRoot(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}

	return filepath.Dir(filepath.Dir(filepath.Dir(currentFile)))
}

// repoFile is one file a repository walk reached.
type repoFile struct {
	// abs is the absolute path, for opening and parsing the file.
	abs string
	// rel is the path relative to the repository root, slash-separated: the
	// spelling every guardrail here uses when it names a file in a failure.
	rel string
	// dir is rel's directory, slash-separated, which is how a guardrail asks
	// which package a file belongs to.
	dir string
	// name is the base name, which is what most of the filtering is done on.
	name string
}

// walkRepoFiles hands visit every file of the checkout rooted at repoRoot, with
// the directories that hold no first-party source of *this* checkout pruned
// (isSkippedWalkDir).
//
// It is the one walk in this package. Seven guardrails used to hand-roll the
// same preamble — check IsDir, consult the skip rule, return SkipDir, filter
// the rest by extension — so a change to what a walk is entitled to read meant
// finding every copy, and one copy with its own answer meant two rules for one
// question.
//
// It counts nothing on its callers' behalf, because no two of them judge the
// same files: each filters the stream to its own subject and then asserts a
// floor on what that filter kept. A guardrail reaching none of its subjects
// reports success without having checked anything, which is what
// assertWalkedAtLeast is for and what TestRepoWalk_EveryWalkAssertsAFloor
// keeps from being optional.
func walkRepoFiles(t *testing.T, repoRoot string, visit func(file repoFile)) {
	t.Helper()

	walkErr := filepath.WalkDir(
		repoRoot,
		func(entryPath string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if entry.IsDir() {
				if isSkippedWalkDir(repoRoot, entryPath) {
					return filepath.SkipDir
				}

				return nil
			}

			relPath, relErr := filepath.Rel(repoRoot, entryPath)
			if relErr != nil {
				return relErr
			}

			slashed := filepath.ToSlash(relPath)

			visit(repoFile{
				abs:  entryPath,
				rel:  slashed,
				dir:  path.Dir(slashed),
				name: entry.Name(),
			})

			return nil
		},
	)
	if walkErr != nil {
		t.Fatalf("walking the checkout at %s: %v", repoRoot, walkErr)
	}
}

// assertWalkedAtLeast fails the test when a guardrail reached fewer than floor
// of the files it judges.
//
// Silent vacuity is the one failure a guardrail suite cannot report: a bad
// root, an over-eager prune or a rename of the thing being matched leaves the
// check passing over nothing, and it keeps passing forever. The floors are
// tripwires against that, not measurements of the tree — each sits far below
// what a healthy checkout holds, so it fires on a broken walk and never on
// ordinary growth.
func assertWalkedAtLeast(t *testing.T, subject string, found, floor int) {
	t.Helper()

	if found >= floor {
		return
	}

	t.Errorf(
		"the walk reached %d %s, want at least %d; the walk is broken and this "+
			"check would pass vacuously",
		found, subject, floor,
	)
}

// TestRepoWalk_SkipsOtherCheckoutsOnly pins what walkRepoFiles prunes, against
// a tree built to look like a checkout with agent worktrees parked inside it.
//
// Another checkout inside this one is the case worth a test: its files are
// first-party source of a different copy of this repository, so a guardrail
// that reads them reports a violation the current diff cannot contain and
// cannot fix. Neither obvious shortcut works — a directory named "worktrees"
// is walked here, and .claude survives whole, because
// TestClaudeGuideIsSymlinkToAgentsGuide and TestAgentSkillsStayCanonical live
// inside it.
func TestRepoWalk_SkipsOtherCheckoutsOnly(t *testing.T) {
	root := t.TempDir()

	// The tree under test carries its own .git; being a checkout is what makes
	// it the root, not something to prune.
	writeWalkFile(t, root, ".git/config")

	writeWalkFile(t, root, keptGoFile)
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
		keptGoFile,
		"tmp/keep.go",
		"worktrees/keep.go",
	}

	got := walkedFiles(t, root)

	if !slices.Equal(got, want) {
		t.Errorf("walk visited\n\t%s\nwant\n\t%s",
			strings.Join(got, "\n\t"), strings.Join(want, "\n\t"))
	}
}

// TestRepoWalk_WalksARootNamedLikeASkippedDir keeps the pruning from ever
// swallowing the tree it is supposed to walk. The root is a checkout and may
// sit in a directory called anything — clone this repo into ~/build and a rule
// that judged the root by its own name or its .git would prune at the first
// callback, leaving every guardrail in this package passing over no files at
// all. Silent vacuity is the one failure a guardrail suite cannot report.
func TestRepoWalk_WalksARootNamedLikeASkippedDir(t *testing.T) {
	root := filepath.Join(t.TempDir(), "build")

	writeWalkFile(t, root, ".git/config")
	writeWalkFile(t, root, keptGoFile)

	want := []string{keptGoFile}

	got := walkedFiles(t, root)

	if !slices.Equal(got, want) {
		t.Errorf("walk visited %v, want %v", got, want)
	}
}

// TestRepoWalk_DescribesEachFileItHandsOver pins the fields a guardrail reads
// off a visited file: the absolute path it opens, the slash-relative path it
// names in a failure message, the directory it judges the package by, and the
// base name it filters on.
func TestRepoWalk_DescribesEachFileItHandsOver(t *testing.T) {
	root := t.TempDir()

	writeWalkFile(t, root, keptGoFile)

	var seen []repoFile

	walkRepoFiles(t, root, func(file repoFile) {
		seen = append(seen, file)
	})

	assertWalkedAtLeast(t, "files in the fixture tree", len(seen), 1)

	if len(seen) != 1 {
		t.Fatalf("walk visited %d files, want 1", len(seen))
	}

	want := repoFile{
		abs:  filepath.Join(root, "internal", "domain", "keep.go"),
		rel:  keptGoFile,
		dir:  "internal/domain",
		name: "keep.go",
	}

	if seen[0] != want {
		t.Errorf("walk reported %+v, want %+v", seen[0], want)
	}
}

// TestRepoWalk_EveryWalkAssertsAFloor keeps the vacuity floors from being
// optional. One walker means one place a pruning mistake, a bad root or a
// rename can silence every guardrail here at once — and a guardrail that
// inspects nothing reports success, forever, with no output to notice.
//
// The floor is the only evidence a check has that it looked at something, so
// every function that calls the walker owes one — the fixtures here included,
// because a fixture tree that failed to be written is the same silent pass in
// miniature. There is no exemption, deliberately: one would be the hole the
// next walk slips through.
func TestRepoWalk_EveryWalkAssertsAFloor(t *testing.T) {
	walkers := 0

	forEachTestFile(t, func(source repoFile, _ *token.FileSet, file *ast.File) {
		if source.dir != architecturePackageDir {
			return
		}

		for _, decl := range file.Decls {
			funcDecl, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || funcDecl.Body == nil {
				continue
			}

			if !callsAnyMethod(funcDecl.Body, []string{"walkRepoFiles"}) {
				continue
			}

			walkers++

			if callsAnyMethod(funcDecl.Body, []string{"assertWalkedAtLeast"}) {
				continue
			}

			t.Errorf(
				"%s: %s walks the checkout without asserting a floor on what it "+
					"found; call assertWalkedAtLeast, or the guardrail behind it "+
					"passes green over an empty walk",
				source.rel, funcDecl.Name.Name,
			)
		}
	})

	assertWalkedAtLeast(t, "functions calling the walker", walkers, walkingFunctionFloor)
}

// walkingFunctionFloor is the fewest functions expected to call the walker.
// Collapsing seven hand-rolled walks into one left ten callers; below half of
// that, this check has stopped recognizing them rather than the package having
// stopped walking.
const walkingFunctionFloor = 5

// architecturePackageDir is where this package's own files live, as the walker
// spells a directory.
const architecturePackageDir = "internal/architecture"

// walkedFiles walks root through the shared walker and returns the
// slash-relative paths it reached, sorted.
func walkedFiles(t *testing.T, root string) []string {
	t.Helper()

	var found []string

	walkRepoFiles(t, root, func(file repoFile) {
		found = append(found, file.rel)
	})

	// Every fixture here leaves something to reach; a walk that reaches nothing
	// means the tree was never written, not that the pruning is right.
	assertWalkedAtLeast(t, "files in the fixture tree", len(found), 1)

	slices.Sort(found)

	return found
}

// writeWalkFile creates an empty file at relPath under root, parents included.
func writeWalkFile(t *testing.T, root, relPath string) {
	t.Helper()

	filePath := filepath.Join(root, filepath.FromSlash(relPath))

	mkdirErr := os.MkdirAll(filepath.Dir(filePath), 0o750)
	if mkdirErr != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(filePath), mkdirErr)
	}

	writeErr := os.WriteFile(filePath, nil, 0o600)
	if writeErr != nil {
		t.Fatalf("WriteFile(%s) error = %v", filePath, writeErr)
	}
}
