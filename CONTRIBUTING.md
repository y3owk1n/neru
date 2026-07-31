# Contributing to Neru

Thanks for your interest in contributing! Neru is a small project with an
approachable codebase, and we welcome contributions of all kinds — code, docs,
bug reports, config examples, or ideas.

This document owns the **contribution process**: how to propose a change, how to
commit it, and how to get it merged. The technical guides own the rest —
[DEVELOPMENT.md](docs/DEVELOPMENT.md) for environment setup, building, and
testing; [ARCHITECTURE.md](docs/ARCHITECTURE.md) for how the codebase is
structured; [CROSS_PLATFORM.md](docs/CROSS_PLATFORM.md) for platform work; and
[CODING_STANDARDS.md](docs/CODING_STANDARDS.md) for style.

---

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Making Changes](#making-changes)
- [Commit Messages](#commit-messages)
- [Pull Requests](#pull-requests)
- [Platform Work](#platform-work)
- [Good First Contributions](#good-first-contributions)
- [Reporting Bugs](#reporting-bugs)
- [Feature Requests](#feature-requests)

---

## Code of Conduct

This project follows our [Code of Conduct](CODE_OF_CONDUCT.md). By participating
you agree to uphold it. Please report unacceptable behavior via
[GitHub Issues](https://github.com/y3owk1n/neru/issues) or by contacting
[@y3owk1n](https://github.com/y3owk1n) directly.

---

## Getting Started

1. **Search existing issues** — check whether someone is already working on the
   same thing, or whether there's a related discussion.
2. **Open an issue first** for non-trivial changes. This avoids wasted effort and
   lets us align on approach before you write code.
3. **Small, focused PRs** are preferred over large, sweeping ones.

Set up your environment by following
[DEVELOPMENT.md](docs/DEVELOPMENT.md#development-setup) — Devbox is the
recommended path and provides every tool pre-configured.

---

## Making Changes

1. **Fork** the repository and clone your fork.
2. **Create a branch** from `main`:

    ```bash
    git checkout -b feat/my-feature
    ```

3. **Make your changes**, following
   [CODING_STANDARDS.md](docs/CODING_STANDARDS.md). Where new code belongs is
   mapped out in [DEVELOPMENT.md](docs/DEVELOPMENT.md#adding-code).
4. **Add or update tests.** All new code needs coverage — see
   [DEVELOPMENT.md](docs/DEVELOPMENT.md#testing) for the test tiers and
   [TESTING_PATTERNS.md](docs/testing/TESTING_PATTERNS.md) for the patterns.
5. **Run the pre-commit checks:**

    ```bash
    just fmt      # format Go and Objective-C
    just lint     # golangci-lint
    just test     # unit + integration
    just build    # verify the build
    ```

    Doing Linux or Windows work? Start with `just test-foundation` and
    `just build-linux` / `just build-windows`.

6. **Update the docs** in the same PR. Each fact has one home — the
   [documentation checklist](docs/CROSS_PLATFORM.md#documentation-checklist)
   says which file owns what, so please update the owner rather than restating
   it in a second place.
7. **Commit** using [conventional commits](#commit-messages), then push and open
   a pull request.

---

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/) to power
automated releases via
[Release Please](https://github.com/googleapis/release-please). **The commit
subject is what ships in the changelog**, so write it for users.

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

## Good First Contributions

Not sure where to start?

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
