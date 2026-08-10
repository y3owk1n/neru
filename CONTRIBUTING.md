# Contributing to Neru

Thanks for your interest in contributing! Neru is a small project with an
approachable codebase, and we welcome contributions of all kinds — code, docs,
bug reports, config examples, or ideas.

This document owns the **contribution process**: how to propose a change, how to
commit it, and how to get it merged. The technical guides own the rest —
[DEVELOPMENT.md](docs/DEVELOPMENT.md) for environment setup, building, and
testing; [ARCHITECTURE.md](docs/ARCHITECTURE.md) for how the codebase is
structured; [CROSS_PLATFORM.md](docs/CROSS_PLATFORM.md) for platform work; and
[AGENTS.md](AGENTS.md) for conventions and contracts.

---

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Making Changes](#making-changes)
- [Commit Messages](#commit-messages)
- [Pull Requests](#pull-requests)
- [Platform Work](#platform-work)
- [AI-Assisted Contributions](#ai-assisted-contributions)
- [Good First Contributions](#good-first-contributions)
- [Reporting Bugs](#reporting-bugs)
- [Feature Requests](#feature-requests)

---

## Code of Conduct

This project follows our [Code of Conduct](CODE_OF_CONDUCT.md). By participating
you agree to uphold it. Please report unacceptable behavior privately by
contacting [@y3owk1n](https://github.com/y3owk1n) directly — not via public
issues, so reports stay confidential.

---

## Getting Started

1. **Search existing issues** — check whether someone is already working on the
   same thing, or whether there's a related discussion.
2. **Open an issue first** for non-trivial changes. This avoids wasted effort and
   lets us align on approach before you write code.
3. **Small, focused PRs** are preferred over large, sweeping ones.

Set up your environment by following
[DEVELOPMENT.md](docs/DEVELOPMENT.md#development-setup) — Devbox is the
recommended path and provides the toolchain pre-configured. On Linux, read the
prerequisites there first: Devbox does not cover the system packages a CGO
build links against.

---

## Making Changes

1. **Fork** the repository and clone your fork.
2. **Create a branch** from `main`:

    ```bash
    git checkout -b feat/my-feature
    ```

3. **Make your changes**, following the conventions in
   [AGENTS.md](AGENTS.md). Where new code belongs is
   mapped out in [DEVELOPMENT.md](docs/DEVELOPMENT.md#adding-code).
4. **Add or update tests.** All new code needs coverage — see
   [DEVELOPMENT.md](docs/DEVELOPMENT.md#testing) for the test tiers and
   [AGENTS.md](AGENTS.md) (Conventions) for naming, mocks, and build tags.
5. **Run the pre-commit checks:**

    ```bash
    just fmt      # format Go and Objective-C
    just lint     # golangci-lint
    just test     # unit + integration — see the warning below
    just build    # verify the build
    ```

    > [!IMPORTANT]
    > On macOS, `just test` includes integration tests that **drive your real
    > cursor, keyboard, and overlays**, and they need Accessibility permission
    > granted to your terminal (System Settings → Privacy & Security →
    > Accessibility). Run `just test-unit` if you only want the safe subset,
    > and quit any running `neru` daemon first — a live daemon holding the
    > socket makes the IPC integration tests silently skip. Details in
    > [DEVELOPMENT.md](docs/DEVELOPMENT.md#testing).

    Before pushing, run **`just ci`** — the same recipes CI gates your PR on,
    run on your host only, where CI runs them on macOS, Linux and Windows. It
    is a superset of the checks above (adds `go vet`, the
    cross-platform foundation slice, a CGO-off type-check of the Linux and
    Windows builds, a `-race` pass over the unit suite, the CI profile of the
    integration suite, and a vulnerability scan). For the deepest verification
    on a real desktop session, `just test-all` runs full integration under
    `-race` too. Doing Linux or Windows work? Start with
    `just test-foundation` and `just build-linux` / `just build-windows`.

    That type-check is `just check-cross`, and it is the only part of the run
    that looks at the other two legs — worth knowing about, because everything
    else compiles for your host. What it covers, and the cgo-only Linux paths
    it cannot, are in
    [DEVELOPMENT.md](docs/DEVELOPMENT.md#what-just-ci-covers-and-what-it-does-not).

6. **Update the docs** in the same PR. Each fact has one home — the
   [documentation checklist](docs/CROSS_PLATFORM.md#documentation-checklist)
   says which file owns what, so please update the owner rather than restating
   it in a second place.

    **On linters:** the linter set is strict on purpose, and `//nolint` is the
    escape hatch, not the default. Use one only when the finding is a genuine
    false positive or the compliant form would be clearly worse — always
    with the specific linter named and a trailing `// reason`. If you find
    yourself suppressing the same linter repeatedly, that linter may be wrong
    for this codebase: propose disabling it in `.golangci.yml` (with the
    reason recorded there) instead of scattering suppressions.
7. **Commit** using [conventional commits](#commit-messages), then push and open
   a pull request.

---

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/) to power
automated releases via
[Release Please](https://github.com/googleapis/release-please).

**The artifact that reaches the changelog is the squash title, not your commit
subjects.** Pull requests here squash-merge — it is the only merge method the
repository enables — so the whole branch lands as one commit whose subject is
the PR title, and that title is what Release Please reads. Write *it* for
users.

Branch commits stay conventional all the same, for two reasons that do not
depend on the changelog: a reviewer reads the branch commit by commit, and a
subject that says what changed is the cheapest way to make that possible; and
the title you type is almost always one of them, so a branch of well-written
subjects hands you the right title for free.

**Format:**

```
<type>(<optional scope>): <subject>

<optional body>

<optional footer>
```

**Types:**

| Type       | When to use                            |
| ---------- | -------------------------------------- |
| `feat`     | New feature                            |
| `fix`      | Bug fix                                |
| `docs`     | Documentation only                     |
| `style`    | Formatting, no logic change            |
| `refactor` | Code restructuring, no behavior change |
| `perf`     | Performance improvement                |
| `test`     | Adding or updating tests               |
| `chore`    | Build, CI, dependencies, tooling       |

**Examples:**

```
feat(grid): add recursive subdivision mode
fix(hints): correct overlay positioning on multi-monitor setups
docs: update configuration reference for scroll mode
```

A fuller message earns its body by explaining *why*:

```
feat: add grid-based navigation mode

Implement grid-based navigation as an alternative to hints. Grid mode divides
the screen into cells and allows precise cursor positioning without relying on
the accessibility tree.

Closes #123
```

---

## Pull Requests

- **Title** follows the same conventional commit format (e.g.
  `feat(hints): add multi-monitor support`).
- **Description** explains _what_ changed and _why_. Include screenshots or
  recordings for UI changes.
- **Keep PRs focused** — one logical change per PR.
- **Link related issues** (e.g. `Closes #123`).
- All CI checks (lint, test, build) must pass before merge.
- A maintainer will review. Be open to feedback and iterate.

---

## Platform Work

Neru puts a strong emphasis on architectural separation, and platform changes
are where that matters most. Before writing Linux or Windows code:

- Read [The "One Rule"](docs/ARCHITECTURE.md#the-one-rule) — non-darwin code must
  never import the darwin platform package. It is enforced by both `depguard` and
  an architecture test.
- Check the current
  [platform status](docs/CROSS_PLATFORM.md#platform-status) and
  [capability matrix](docs/CROSS_PLATFORM.md#capability-matrix).
- Work through the
  [Cross-Platform Contributor Guide](docs/CROSS_PLATFORM.md#contributor-guide) —
  it covers file slots, the Linux backend model, CGO guidance, and the bar a
  platform PR has to clear.

Implement in the existing platform slot rather than inventing new file layout,
and keep macOS-specific assumptions out of shared code.

---

## AI-Assisted Contributions

AI-assisted PRs are welcome — the same review bar applies either way. The repo
ships shared context so your agent starts from the project's actual rules
instead of guessing:

- **[AGENTS.md](AGENTS.md)** is the cross-agent contract (architecture,
  commands, conventions). `CLAUDE.md` is a symlink to it, and
  `.cursor/rules/` + `.github/copilot-instructions.md` point at it, so Claude
  Code, Codex, Cursor, and Copilot all read the same guide. Personal overrides
  go in gitignored `AGENTS.local.md` / `CLAUDE.local.md`.
- **`.agents/skills/`** holds step-by-step workflows for the changes that are
  easiest to half-finish — adding a config option, adding a CLI command,
  platform work — plus contribution mechanics: `create-pr` encodes the commit
  and PR-template conventions below, and `file-issue` encodes the issue forms.
  `.claude/skills` is a symlink to it, so Claude Code, Codex, and OpenCode all
  discover the same skills.
- **`.claude/agents/`** holds focused review profiles
  (`platform-boundary-reviewer`, `deadlock-reviewer`) you can run on your diff
  before opening a PR.
- **`.claude/settings.json`** wires a format-on-edit hook so agent edits land
  already formatted. Claude Code asks for one-time workspace trust before
  running project hooks — that prompt is expected.

Two mechanical notes: `CLAUDE.md` and `.claude/skills` are git symlinks (the
same layout Apache Airflow and T3 Code use), so on Windows clone with symlink
support enabled (`git config core.symlinks true`, requires Developer Mode) or
just read `AGENTS.md` directly. The layout is pinned by
`internal/architecture/agent_contract_test.go`.

Whatever tool you use, you own the result: run the pre-commit gate
(`just fmt && just lint && just test && just build`), read the diff yourself,
and don't submit changes you can't explain.

---

## Good First Contributions

Not sure where to start? Any of these are welcome:

- 🐛 Bug fixes — check the [open issues](https://github.com/y3owk1n/neru/issues)
- 📝 Documentation improvements or typo fixes
- 📦 Config examples for common setups
- 🎥 Demo videos or GIFs
- ⚡ Performance improvements
- 🧪 Additional test coverage

For platform work specifically,
[Contributing safely](docs/CROSS_PLATFORM.md#contributing-safely) lists
well-scoped starter tasks — and the changes worth opening an issue about first.
Longer-term direction is in [ROADMAP.md](docs/ROADMAP.md).

---

## Reporting Bugs

Open a [GitHub Issue](https://github.com/y3owk1n/neru/issues/new) with:

1. **Your platform** (macOS/Linux/Windows and version; on Linux, your desktop
   and session type) and **Neru version** (`neru version`).
2. **Steps to reproduce** — minimal and specific.
3. **Expected vs actual behavior**.
4. **Logs** — set `log_level = "debug"` and attach the relevant lines. Log paths
   are listed in
   [TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md#log-file-locations).
5. **Screenshots or recordings** if the issue is visual.

`neru doctor` output is useful too — it reports which capabilities your platform
actually supports.

See also: [Troubleshooting Guide](docs/TROUBLESHOOTING.md).

---

## Feature Requests

Open a [GitHub Issue](https://github.com/y3owk1n/neru/issues/new) or start a
[Discussion](https://github.com/y3owk1n/neru/discussions) describing:

- **What** you'd like to see.
- **Why** it would be useful (your use case).
- **How** you envision it working (optional but helpful).

---

Thank you for helping make Neru better! 🙏
