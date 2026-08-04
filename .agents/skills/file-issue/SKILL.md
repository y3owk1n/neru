---
name: file-issue
description: "File a Neru bug report or feature request that matches the repo's issue forms: duplicate check first, every required field filled with real diagnostics, correct labels. Use when asked to open, file, or draft a GitHub issue for Neru. Not for pull requests."
---

# Filing an issue in Neru

Blank issues are disabled (`.github/ISSUE_TEMPLATE/config.yml`) — everything
goes through a form, and an agent-filed issue must contain the same fields the
form enforces. `gh issue create` bypasses form validation, so you are the
validator.

## Always, before filing

1. **Search for duplicates** — the forms make humans attest to this, so do it
   for real: `gh issue list --search "<keywords>" --state all`, plus
   discussions for feature ideas. Found one? Comment there instead of filing.
2. **Route non-issues away.** Questions and config help belong in
   Discussions; Linux porting talk belongs in discussion #559; "is this
   supported yet" is answered by the Platform Support matrix in the README
   and `docs/CROSS_PLATFORM.md`. Filing an issue for these is wrong even if
   the user asked for an issue — say so and offer the right venue.

## Bug report (`--label bug`)

Mirror `bug_report.yml`'s fields as markdown sections, all of them:

- **Neru version** — real output of `neru version`, never guessed.
- **Operating system** and **OS version** (`sw_vers` on macOS; distro +
  compositor on Linux, since behavior differs across X11/wlroots/KDE/GNOME).
- **Navigation mode** — hints / grid / recursive grid / scroll / n.a.
- **What happened / What did you expect** — observed vs expected, concrete.
- **Steps to reproduce** — numbered, from `neru launch`, minimal.
- **Config (relevant sections)** — only the TOML sections involved, fenced as
  `toml`. Strip anything personal (macro contents, exec commands).
- **Screenshots / recordings** and **Additional context** when they help;
  `neru doctor` output is gold for capability/permission-shaped bugs.

Linux/Windows caveat from the form: many features are intentionally
unimplemented there. If `neru doctor` reports the capability as `stub`, it is
a roadmap item, not a bug — check the matrix before filing.

## Feature request (`--label enhancement`)

Mirror `feature_request.yml`:

- **Category** — one of: Navigation, Mouse actions, Configuration, CLI / IPC,
  UI / Overlays, Performance, Platform support (Linux / Windows), Other.
- **Target platform** — All platforms / macOS / Linux / Windows.
- **Problem or use case** — the need, not the mechanism. This is the section
  maintainers judge; write it first and best.
- **Proposed solution**, **Alternatives considered**, optional
  **Screenshots / mockups**.
- **Contribution** — state plainly whether a PR will follow.

## Filing

Draft the body, show it to the user before creating (an issue posts publicly
under their account), then:

```bash
gh issue create --title "<concise, user-facing summary>" --label bug --body-file <draft>
```

Title style matches the tracker: plain sentence, no conventional-commit
prefix, no trailing period.
