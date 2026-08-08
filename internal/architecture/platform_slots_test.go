package architecture_test

import (
	"go/build/constraint"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// These tests enforce the file-layout rules documented in
// docs/CROSS_PLATFORM.md ("File Layout Rules"). The rules exist so a
// contributor can tell what a file is for from its name alone; without a
// guardrail they drift back into ad hoc naming one PR at a time.
//
// knownOS lists the GOOS values the repo targets. "unix" is deliberately absent
// — it is a build-tag family, not a GOOS, and is handled as a named slot below.
const (
	osDarwin  = "darwin"
	osLinux   = "linux"
	osWindows = "windows"
)

var knownOS = []string{osDarwin, osLinux, osWindows}

// wholePlatformDirs are packages that are entirely one platform's code. There
// the directory carries the meaning, so individual files need no OS suffix —
// but they must still declare an explicit build tag, which
// TestPlatformPackagesTagEveryFile checks.
//
// The set is derived from the tree rather than listed by hand. A hand-written
// list would have to be edited every time a backend becomes its own package,
// and the edit would look identical whether it was correct or was papering over
// a package that is not actually single-platform. Deriving it means the
// exemption is only ever granted to a directory that has earned it.
func wholePlatformDirs(t *testing.T) map[string]string {
	t.Helper()

	// candidates maps a directory to the single OS every constrained file in it
	// targets, or to "" once a file contradicts that.
	candidates := map[string]string{}

	for _, file := range goFiles(t) {
		if file.base == docGoFile {
			continue
		}

		constraints := parseConstraint(t, file.absPath)

		targetOS := ""
		if len(constraints.positiveOS) == 1 {
			targetOS = constraints.positiveOS[0]
		}

		seen, known := candidates[file.dir]
		switch {
		case !known:
			candidates[file.dir] = targetOS
		case seen != targetOS:
			// Mixed platforms, or a file with no single positive OS term.
			candidates[file.dir] = ""
		}
	}

	whole := map[string]string{}

	for dir, targetOS := range candidates {
		if targetOS != "" {
			whole[dir] = targetOS
		}
	}

	return whole
}

// fallbackSuffixes are the only legal names for a file whose build constraint
// is a pure negation (no positive GOOS term).
//
//   - "_other.go"  the documented non-target fallback slot
//   - "_unix.go"   the conventional Go name for the !windows side of a split;
//     "unix" is an established build-tag family, so renaming these to _other
//     would be less clear, not more
var fallbackSuffixes = []string{"_other.go", "_unix.go"}

// bannedFallbackSuffixes are ad hoc fallback names that have shown up in the
// past. They all mean "_other.go"; having four spellings for one slot is
// exactly the drift these tests exist to stop.
var bannedFallbackSuffixes = []string{
	"_stub.go",
	"_stubs.go",
	"_default.go",
	"_fallback.go",
	"_noop.go",
	"_generic.go",
	"_unsupported.go",
}

// fileConstraint holds what a file's build tag says about where it compiles.
type fileConstraint struct {
	// positiveOS are GOOS terms the file requires (e.g. "linux" for
	// `linux && cgo`). A file requiring more than one is a multi-platform
	// file and is exempt from the suffix rule.
	positiveOS []string
	// negatedOS are GOOS terms the file excludes (e.g. "darwin" for `!darwin`).
	negatedOS []string
	// requiresCgo / excludesCgo record the cgo polarity, if any.
	requiresCgo bool
	excludesCgo bool
	// hasConstraint is false for files with no //go:build line at all.
	hasConstraint bool
}

func TestPlatformFilesUseTheDocumentedSlots(t *testing.T) {
	whole := wholePlatformDirs(t)

	for _, file := range goFiles(t) {
		constraints := parseConstraint(t, file.absPath)
		if !constraints.hasConstraint {
			continue
		}

		checkBannedFallbackName(t, file)
		checkOSSuffixMatchesConstraint(t, file, constraints, whole)
		checkFallbackName(t, file, constraints)
		checkCgoSuffix(t, file, constraints)
	}
}

// checkBannedFallbackName rejects the ad hoc spellings of the fallback slot.
func checkBannedFallbackName(t *testing.T, file goFile) {
	t.Helper()

	for _, banned := range bannedFallbackSuffixes {
		if strings.HasSuffix(file.base, banned) {
			t.Errorf(
				"%s: platform files must not use the %q suffix; the fallback "+
					"slot is %q (docs/CROSS_PLATFORM.md, File Layout Rules)",
				file.relPath,
				banned,
				"_other.go",
			)
		}
	}
}

// checkOSSuffixMatchesConstraint catches a file that compiles for exactly one
// OS but whose name does not say so — the reverse of the mistake Go's implicit
// suffix rule already prevents. A file named tree.go that is secretly
// darwin-only is invisible to anyone scanning the directory.
func checkOSSuffixMatchesConstraint(
	t *testing.T,
	file goFile,
	constraints fileConstraint,
	whole map[string]string,
) {
	t.Helper()

	if len(constraints.positiveOS) != 1 {
		// No positive OS term (pure negation, handled by checkFallbackName), or
		// several (a genuine multi-platform file like `darwin || linux`).
		return
	}

	if _, single := whole[file.dir]; single {
		return
	}

	targetOS := constraints.positiveOS[0]
	if hasNameToken(file.base, targetOS) {
		return
	}

	t.Errorf(
		"%s: build constraint restricts this file to %s, but the filename does "+
			"not contain a %q slot token; rename it to *_%s.go (or *_%s_<backend>.go)",
		file.relPath,
		targetOS,
		targetOS,
		targetOS,
		targetOS,
	)
}

// checkFallbackName requires that a file excluded from one or more platforms,
// without targeting any specific one, uses a recognized fallback slot name.
func checkFallbackName(t *testing.T, file goFile, constraints fileConstraint) {
	t.Helper()

	if len(constraints.positiveOS) > 0 || len(constraints.negatedOS) == 0 {
		return
	}

	for _, suffix := range fallbackSuffixes {
		if strings.HasSuffix(file.base, suffix) {
			return
		}
	}

	t.Errorf(
		"%s: build constraint excludes %s without targeting a platform, so this "+
			"is a fallback slot; name it *_other.go",
		file.relPath,
		strings.Join(constraints.negatedOS, ", "),
	)
}

// checkCgoSuffix requires a file gated on cgo to say so in its name.
//
// This one is worth the churn: before it existed, a plain name usually meant
// the cgo variant (system_linux_x11.go beside system_x11_nocgo.go) but
// in internal/ui/overlay it meant the opposite — manager_linux_wayland.go was
// the *nocgo* file sitting beside manager_linux_wayland_cgo.go. Reading the
// build tag was the only way to tell which convention a package followed.
func checkCgoSuffix(t *testing.T, file goFile, constraints fileConstraint) {
	t.Helper()

	switch {
	case constraints.requiresCgo && !hasNameToken(file.base, "cgo"):
		t.Errorf(
			"%s: build constraint requires cgo, but the filename does not say "+
				"so; rename it to *_cgo.go",
			file.relPath,
		)
	case constraints.excludesCgo && !strings.HasSuffix(file.base, "_nocgo.go"):
		t.Errorf(
			"%s: build constraint excludes cgo, but the filename does not say "+
				"so; rename it to *_nocgo.go",
			file.relPath,
		)
	}
}

// TestPlatformPackagesTagEveryFile pins the other half of the whole-platform
// package exemption: files there may skip the OS suffix precisely because the
// package is single-platform, which only holds if every file declares the tag.
// An untagged file would compile into every build and break cross-compilation.
func TestPlatformPackagesTagEveryFile(t *testing.T) {
	whole := wholePlatformDirs(t)

	for _, file := range goFiles(t) {
		wantOS, single := whole[file.dir]
		if !single {
			continue
		}

		// doc.go is allowed to stay untagged so `go vet ./...` can resolve the
		// package on other hosts; it holds only the package comment.
		if file.base == docGoFile {
			continue
		}

		constraints := parseConstraint(t, file.absPath)

		if !constraints.hasConstraint {
			t.Errorf(
				"%s: every file in %s must declare //go:build %s — the package "+
					"is single-platform and nothing else keeps it out of other builds",
				file.relPath,
				file.dir,
				wantOS,
			)

			continue
		}

		if len(constraints.positiveOS) != 1 || constraints.positiveOS[0] != wantOS {
			t.Errorf(
				"%s: file in %s must be constrained to %s, got %v",
				file.relPath,
				file.dir,
				wantOS,
				constraints.positiveOS,
			)
		}
	}
}

type goFile struct {
	absPath string
	relPath string
	dir     string
	base    string
}

// goFiles returns every non-test Go file in the repo. Test files are excluded:
// they follow the *_integration_<os>_test.go convention, which the suffix rules
// here would misread.
func goFiles(t *testing.T) []goFile {
	t.Helper()

	repoRoot := findRepoRoot(t)

	var files []goFile

	walkRepoFiles(t, repoRoot, func(file repoFile) {
		if filepath.Ext(file.name) != goExt || strings.HasSuffix(file.name, "_test.go") {
			return
		}

		files = append(files, goFile{
			absPath: file.abs,
			relPath: file.rel,
			dir:     file.dir,
			base:    file.name,
		})
	})

	assertWalkedAtLeast(t, "non-test Go files", len(files), bulkWalkFloor)

	return files
}

// parseConstraint reads a file's //go:build line and summarizes which GOOS and
// cgo terms it requires or excludes.
func parseConstraint(t *testing.T, path string) fileConstraint {
	t.Helper()

	fileSet := token.NewFileSet()

	parsed, err := parser.ParseFile(
		fileSet,
		path,
		nil,
		parser.PackageClauseOnly|parser.ParseComments,
	)
	if err != nil {
		t.Fatalf("ParseFile(%s) error = %v", path, err)
	}

	var summary fileConstraint

	for _, group := range parsed.Comments {
		for _, comment := range group.List {
			if !constraint.IsGoBuild(comment.Text) {
				continue
			}

			expr, parseErr := constraint.Parse(comment.Text)
			if parseErr != nil {
				t.Fatalf("constraint.Parse(%s) error = %v", path, parseErr)
			}

			summary.hasConstraint = true
			collectTerms(expr, false, &summary)
		}
	}

	return summary
}

// collectTerms walks a build expression, recording each tag with the polarity
// it appears under. Negation flips as it descends through NotExpr.
func collectTerms(expr constraint.Expr, negated bool, summary *fileConstraint) {
	switch node := expr.(type) {
	case *constraint.TagExpr:
		recordTag(node.Tag, negated, summary)
	case *constraint.NotExpr:
		collectTerms(node.X, !negated, summary)
	case *constraint.AndExpr:
		collectTerms(node.X, negated, summary)
		collectTerms(node.Y, negated, summary)
	case *constraint.OrExpr:
		collectTerms(node.X, negated, summary)
		collectTerms(node.Y, negated, summary)
	}
}

func recordTag(tag string, negated bool, summary *fileConstraint) {
	if tag == "cgo" {
		if negated {
			summary.excludesCgo = true
		} else {
			summary.requiresCgo = true
		}

		return
	}

	for _, osName := range knownOS {
		if tag != osName {
			continue
		}

		if negated {
			summary.negatedOS = appendUnique(summary.negatedOS, tag)
		} else {
			summary.positiveOS = appendUnique(summary.positiveOS, tag)
		}

		return
	}
}

func appendUnique(values []string, value string) []string {
	if slices.Contains(values, value) {
		return values
	}

	return append(values, value)
}

// hasNameToken reports whether the filename contains token as a whole
// underscore-separated segment, so "system_x11_cgo.go" matches both
// "linux" and "cgo" but "linuxish.go" matches neither.
func hasNameToken(base, token string) bool {
	name := strings.TrimSuffix(base, ".go")

	return slices.Contains(strings.Split(name, "_"), token)
}

// TestPackageCommentsAreReachableOnEveryTarget catches a package comment that
// only exists on some platforms.
//
// Go takes the package comment from whichever file carries it, so putting it in
// a build-tagged file silently removes it from every other target — `go doc`
// shows nothing and revive's package-comments check passes only on the tagged
// platform. This was real in four packages: platform/linux had its comment
// under `linux && cgo`, so CGO_ENABLED=0 builds had none, and modeindicator,
// stickyindicator and overlayutil/native each hid theirs in a per-OS file.
//
// The rule: if a package has any untagged file, the package comment must be in
// an untagged one. Packages that are entirely build-tagged (platform/darwin,
// platform/linux, wlr_protocol) are exempt — there is nowhere else to put it.
func TestPackageCommentsAreReachableOnEveryTarget(t *testing.T) {
	type pkgInfo struct {
		hasUntaggedFile    bool
		commentFiles       []string
		untaggedCommentFor bool
	}

	packages := make(map[string]*pkgInfo)

	for _, file := range goFiles(t) {
		info, ok := packages[file.dir]
		if !ok {
			info = &pkgInfo{}
			packages[file.dir] = info
		}

		tagged := parseConstraint(t, file.absPath).hasConstraint
		if !tagged {
			info.hasUntaggedFile = true
		}

		if !hasPackageComment(t, file.absPath) {
			continue
		}

		info.commentFiles = append(info.commentFiles, file.base)

		if !tagged {
			info.untaggedCommentFor = true
		}
	}

	for dir, info := range packages {
		if len(info.commentFiles) == 0 || !info.hasUntaggedFile || info.untaggedCommentFor {
			continue
		}

		t.Errorf(
			"%s: the package comment is only in build-tagged file(s) %v, so targets "+
				"excluded by those tags have no package documentation; move it to an "+
				"untagged doc.go",
			dir,
			info.commentFiles,
		)
	}
}

// TestPackagesHaveExactlyOnePackageComment catches a package comment duplicated
// across files.
//
// golangci-lint's godoclint check finds these, but only for the platform it is
// running on: a duplicate in a windows-tagged file is invisible to a lint run on
// macOS. This test parses every file regardless of build tags, so one run on any
// host covers all targets — which is how the duplicates in the Windows overlay
// render packages were found.
func TestPackagesHaveExactlyOnePackageComment(t *testing.T) {
	commentFiles := make(map[string][]string)

	for _, file := range goFiles(t) {
		if hasPackageComment(t, file.absPath) {
			commentFiles[file.dir] = append(commentFiles[file.dir], file.base)
		}
	}

	for dir, files := range commentFiles {
		if len(files) > 1 {
			slices.Sort(files)
			t.Errorf(
				"%s: %d files carry a package comment (%v); exactly one should — "+
					"keep it in doc.go and demote the others to file comments by "+
					"leaving a blank line before the package clause",
				dir,
				len(files),
				files,
			)
		}
	}
}

// hasPackageComment reports whether a file carries real package documentation —
// a comment group immediately preceding the package clause, ignoring groups
// that contain only tool directives.
func hasPackageComment(t *testing.T, path string) bool {
	t.Helper()

	fileSet := token.NewFileSet()

	parsed, err := parser.ParseFile(
		fileSet,
		path,
		nil,
		parser.PackageClauseOnly|parser.ParseComments,
	)
	if err != nil {
		t.Fatalf("ParseFile(%s) error = %v", path, err)
	}

	if parsed.Doc == nil {
		return false
	}

	// A group of only tool directives documents nothing. They attach to the
	// package clause because that is how they scope file-wide, so counting them
	// would flag every file carrying a file-wide lint exemption.
	for _, comment := range parsed.Doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
		if !strings.HasPrefix(text, "nolint:") && !strings.HasPrefix(text, "go:") {
			return true
		}
	}

	return false
}
