package architecture_test

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

// docURLSources are files outside the docs that link to a doc by URL. `neru
// config init` copies the default config into users' config files, and the
// install scripts print their links in the user's terminal.
var docURLSources = []string{
	"configs/default-config.toml",
	"scripts/install.sh",
	"scripts/install.ps1",
}

// docURLSourceDirs are checked file by file too. They hold the issue forms
// and the agent skills.
var docURLSourceDirs = []string{".github/", ".agents/skills/"}

// mainDocURLPattern matches a URL to a file on main. A URL pinned to a tag
// points at a tree other than this checkout, and is not judged here.
var mainDocURLPattern = regexp.MustCompile(
	`https://(?:github\.com/y3owk1n/neru/(?:blob|tree)/main|` +
		`raw\.githubusercontent\.com/y3owk1n/neru/main)/([A-Za-z0-9_./-]+)(#[A-Za-z0-9_-]+)?`,
)

// markdownLinkPattern matches an inline markdown link and captures its target.
var markdownLinkPattern = regexp.MustCompile(`\]\(([^)\s]+)\)`)

// explicitAnchorPattern matches an HTML anchor a doc declares by hand.
var explicitAnchorPattern = regexp.MustCompile(`<a\s+(?:id|name)="([^"]+)"`)

// A link between docs that lands on a missing page or a missing heading reads
// as fine in review and fails only when a reader follows it. Renaming a doc or
// rewording a heading breaks every link to it at once, and nothing else
// notices (ADR 0011).
func TestDocLinks_ResolveToAPageAndHeading(t *testing.T) {
	repoRoot := findRepoRoot(t)
	anchors := newAnchorIndex(repoRoot)

	for _, doc := range markdownFiles(t, repoRoot) {
		content, readErr := os.ReadFile(doc.absPath)
		if readErr != nil {
			t.Fatalf("ReadFile(%s) error = %v", doc.relPath, readErr)
		}

		for _, match := range markdownLinkPattern.FindAllStringSubmatch(proseOnly(string(content)), -1) {
			target := match[1]
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}

			file, fragment, _ := strings.Cut(target, "#")

			resolved := doc.relPath
			if file != "" {
				resolved = path.Clean(path.Join(path.Dir(doc.relPath), file))
			}

			if !strings.HasSuffix(resolved, ".md") {
				continue
			}

			if problem := anchors.check(resolved, fragment); problem != "" {
				t.Errorf("%s links to %q: %s", doc.relPath, target, problem)
			}
		}
	}
}

// The default config, the install scripts, the issue forms and the skills
// link to docs on main. Users see these links in a config file or a terminal
// long after a doc moves, so they get the same check as links between docs.
func TestDocURLs_ShippedOutsideTheDocsResolve(t *testing.T) {
	repoRoot := findRepoRoot(t)
	anchors := newAnchorIndex(repoRoot)

	var sources []string

	walkRepoFiles(t, repoRoot, func(file repoFile) {
		for _, dir := range docURLSourceDirs {
			if strings.HasPrefix(file.rel, dir) {
				sources = append(sources, file.rel)
			}
		}
	})

	sources = append(sources, docURLSources...)
	assertWalkedAtLeast(t, "files carrying doc URLs", len(sources), bulkWalkFloor)

	for _, source := range sources {
		content, readErr := os.ReadFile(path.Join(repoRoot, source))
		if readErr != nil {
			t.Fatalf("ReadFile(%s) error = %v", source, readErr)
		}

		for _, match := range mainDocURLPattern.FindAllStringSubmatch(string(content), -1) {
			target := strings.TrimSuffix(match[1], "/")
			if problem := anchors.check(target, strings.TrimPrefix(match[2], "#")); problem != "" {
				t.Errorf("%s links to %q: %s", source, match[0], problem)
			}
		}
	}
}

func TestGithubHeadingSlug(t *testing.T) {
	tests := []struct {
		heading string
		want    string
	}{
		{"Installation & Setup", "installation--setup"},
		{"`[recursive_grid]`", "recursive_grid"},
		{"The \"One Rule\"", "the-one-rule"},
		{"Method 3: Nix Flake", "method-3-nix-flake"},
		{"[Linux Setup](LINUX_SETUP.md) notes", "linux-setup-notes"},
		{
			"Drags: press at the start, release at the destination",
			"drags-press-at-the-start-release-at-the-destination",
		},
	}

	for _, tt := range tests {
		if got := githubHeadingSlug(tt.heading); got != tt.want {
			t.Errorf("githubHeadingSlug(%q) = %q, want %q", tt.heading, got, tt.want)
		}
	}
}

// anchorIndex lazily reads the headings of the files links point at.
type anchorIndex struct {
	repoRoot string
	files    map[string]map[string]bool
}

func newAnchorIndex(repoRoot string) *anchorIndex {
	return &anchorIndex{repoRoot: repoRoot, files: map[string]map[string]bool{}}
}

// check returns why relPath#fragment does not resolve, or "" when it does. A
// directory or a non-markdown file resolves when it exists.
func (a *anchorIndex) check(relPath, fragment string) string {
	info, statErr := os.Stat(path.Join(a.repoRoot, relPath))
	if statErr != nil {
		return "the file does not exist"
	}

	if fragment == "" || info.IsDir() || !strings.HasSuffix(relPath, ".md") {
		return ""
	}

	known, cached := a.files[relPath]
	if !cached {
		content, readErr := os.ReadFile(path.Join(a.repoRoot, relPath))
		if readErr != nil {
			return readErr.Error()
		}

		known = headingAnchors(string(content))
		a.files[relPath] = known
	}

	if !known[strings.ToLower(fragment)] {
		return fmt.Sprintf("%s has no heading with anchor #%s", relPath, fragment)
	}

	return ""
}

// headingAnchors returns the anchors GitHub gives a document: one per ATX
// heading, with -1, -2 suffixes on repeats, plus any declared by hand.
func headingAnchors(content string) map[string]bool {
	anchors := map[string]bool{}
	seen := map[string]int{}

	for line := range strings.SplitSeq(proseOnly(content), "\n") {
		trimmed := strings.TrimLeft(line, "#")
		if len(trimmed) == len(line) || len(line)-len(trimmed) > 6 ||
			!strings.HasPrefix(trimmed, " ") {
			continue
		}

		slug := githubHeadingSlug(strings.TrimSpace(trimmed))
		if n := seen[slug]; n > 0 {
			anchors[fmt.Sprintf("%s-%d", slug, n)] = true
		} else {
			anchors[slug] = true
		}

		seen[slug]++
	}

	for _, match := range explicitAnchorPattern.FindAllStringSubmatch(content, -1) {
		anchors[strings.ToLower(match[1])] = true
	}

	return anchors
}

var (
	headingLinkPattern = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	headingHTMLPattern = regexp.MustCompile(`<[^>]+>`)
)

// githubHeadingSlug returns GitHub's anchor for a heading. GitHub lowercases
// the rendered text, keeps letters, digits, underscores, hyphens and spaces,
// and turns each space into a hyphen.
func githubHeadingSlug(heading string) string {
	text := headingLinkPattern.ReplaceAllString(heading, "$1")
	text = headingHTMLPattern.ReplaceAllString(text, "")

	var slug strings.Builder

	for _, r := range strings.ToLower(text) {
		switch {
		case r == ' ':
			slug.WriteRune('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r):
			slug.WriteRune(r)
		}
	}

	return slug.String()
}

// proseOnly blanks fenced code blocks, so a `# comment` in a shell example is
// not read as a heading and a link inside an example is not checked. It keeps
// the line count unchanged.
func proseOnly(content string) string {
	lines := strings.Split(content, "\n")
	fence := ""

	for index, line := range lines {
		trimmed := strings.TrimSpace(line)

		switch {
		case fence == "" && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")):
			fence = trimmed[:3]
			lines[index] = ""
		case fence != "":
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}

			lines[index] = ""
		}
	}

	return strings.Join(lines, "\n")
}
