# Contributing to Neru

Thanks for your interest in contributing. Neru is a small project, and
contributions of all kinds are welcome: code, docs, bug reports, config
examples and ideas.

This document covers the contribution process: how to propose a change, how to
commit it, and how to get it merged. Building and testing live in the
[development guide](docs/contributing/development.md), the system shape in
[architecture](docs/contributing/architecture.md), platform work in the
[porting guide](docs/contributing/porting.md), and code conventions in
[AGENTS.md](AGENTS.md).

## Code of conduct

This project follows our [Code of Conduct](CODE_OF_CONDUCT.md). By participating
you agree to uphold it. Report unacceptable behavior privately to
[@y3owk1n](https://github.com/y3owk1n), not in a public issue, so reports stay
confidential.

## Getting started

1. **Search existing issues** to see whether someone is already working on the
   same thing, or whether there is a related discussion.
2. **Open an issue first** for non-trivial changes, so we can agree on the
   approach before you write code.
3. **Prefer small, focused PRs** over large, sweeping ones.

Set up your environment with the
[development guide](docs/contributing/development.md#development-setup). On
Linux, install the [build dependencies](docs/contributing/development.md#build-dependencies)
first. oku provides the toolchain, not the system libraries a CGO build links
against.

## Making changes

1. **Fork** the repository and clone your fork.
2. **Create a branch** from `main`:

    ```bash
    git checkout -b feat/my-feature
    ```

3. **Make your changes**, following the conventions in [AGENTS.md](AGENTS.md).
   [Where things go](docs/contributing/development.md#where-things-go) maps new
   code to a directory. Platform code follows the
   [porting guide](docs/contributing/porting.md), starting with
   [the One Rule](docs/contributing/architecture.md#the-one-rule).
4. **Add or update tests.** New code needs coverage. The test tiers are in
   [Testing](docs/contributing/development.md#testing).
5. **Run the pre-commit checks:**

    ```bash
    just fmt      # format Go and Objective-C
    just lint     # golangci-lint, plus clang-tidy on macOS
    just test     # unit + integration, desktop-safe
    just build    # verify the build
    ```

    `just test` leaves your desktop alone. Quit any running `neru` daemon
    first, because a live daemon holding the socket makes the IPC integration
    tests skip. The tests that drive the real cursor, keyboard and overlays
    run only under `just test-desktop`, described in
    [Running integration tests](docs/contributing/development.md#running-integration-tests).

    Before pushing, run **`just ci`**. It is the pre-push gate: the same
    recipes CI gates your PR on, run on your host only. What it runs, and what
    it cannot check from one host, is in
    [What just ci covers](docs/contributing/development.md#what-just-ci-covers-and-what-it-does-not).

6. **Update the docs** in the same PR. Each fact has one home, and the
   [documentation checklist](docs/contributing/porting.md#documentation-checklist)
   says which file owns what. Update the owner rather than restating the fact
   somewhere else.

    **On linters:** the linter set is strict on purpose, and `//nolint` is the
    exception, not the default. Use one only when the finding is a false
    positive or the compliant form would be worse. Always name the specific
    linter and add a trailing `// reason`. If you suppress the same linter
    repeatedly, propose disabling it in `.golangci.yml`, with the reason
    recorded there, instead of scattering suppressions.

7. **Commit** using [conventional commits](#commit-messages), then push and open
   a pull request.

## Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/) to drive
automated releases through
[Release Please](https://github.com/googleapis/release-please).

**The changelog entry is the squash title, not your commit subjects.** Pull
requests here squash-merge, the only merge method the repository enables, so
the whole branch lands as one commit whose subject is the PR title. That title
is what Release Please reads. Write it for users.

Branch commits stay conventional all the same. A reviewer reads the branch
commit by commit, and the title you type is usually one of those subjects. Write
the branch subjects well and one of them is the right title.

**Format:**

```text
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

```text
feat(grid): add recursive subdivision mode
fix(hints): correct overlay positioning on multi-monitor setups
docs: update configuration reference for scroll mode
```

A longer message uses its body to explain why:

```text
feat: add grid-based navigation mode

Implement grid-based navigation as an alternative to hints. Grid mode divides
the screen into cells and allows precise cursor positioning without relying on
the accessibility tree.

Closes #123
```

## Pull requests

- **Title** follows the conventional commit format (for example
  `feat(hints): add multi-monitor support`).
- **Description** explains what changed and why. Include screenshots or
  recordings for UI changes.
- **Keep PRs focused** on one logical change.
- **Link related issues** (for example `Closes #123`).
- All CI checks must pass before merge.
- A maintainer will review. Be open to feedback and iterate.

## AI-assisted contributions

AI-assisted PRs are welcome, and the same review bar applies. The repo ships
shared context, skills and review profiles so your agent starts from the
project's actual rules. [AGENTS.md](AGENTS.md) is the entry point, and its
Agent Resources section lists the rest.

`CLAUDE.md` and `.claude/skills` are git symlinks. On Windows, clone with
symlink support enabled (`git config core.symlinks true`, which requires
Developer Mode), or read `AGENTS.md` directly. Claude Code asks for one-time
workspace trust before it runs the project's format-on-edit hook. That prompt
is expected.

Whatever tool you use, you own the result. Run the pre-commit checks, read the
diff yourself, and do not submit changes you cannot explain.

## Good first contributions

Not sure where to start? Any of these are welcome:

- Bug fixes from the [open issues](https://github.com/y3owk1n/neru/issues)
- Documentation improvements or typo fixes
- Config examples for common setups, added to the
  [showcases](docs/project/showcases.md)
- Demo videos or GIFs
- Performance improvements
- Additional test coverage

Well-scoped platform tasks:

- Improve capability detail text for an existing platform slice
- Add a contract test for a currently stubbed feature
- Reproduce and fix a bug labelled `platform: linux` or `platform: windows`
- Document missing backend assumptions in the package you are touching

Some platform changes are worth an issue before any code.
[Contributing safely](docs/contributing/porting.md#contributing-safely) lists
them, with the bar a platform PR has to clear. Longer-term direction is in the
[roadmap](docs/project/roadmap.md).

## Reporting bugs

Use the [issue forms](https://github.com/y3owk1n/neru/issues/new/choose) and
include:

1. **Your platform and Neru version**: macOS, Linux or Windows and its
   version, and the output of `neru --version`. On Linux, also give your
   desktop and session type.
2. **Steps to reproduce**, minimal and specific.
3. **Expected and actual behavior**.
4. **Logs**. File logging is off by default.
   [Log file locations](docs/guide/troubleshooting.md#log-file-locations) says
   how to turn it on, raise the level, and where the file is written.
5. **Screenshots or recordings** if the issue is visual.

`neru doctor` output helps too. It reports which capabilities your platform
supports.

## Feature requests

Open a [GitHub Issue](https://github.com/y3owk1n/neru/issues/new/choose) or start
a [Discussion](https://github.com/y3owk1n/neru/discussions) describing:

- **What** you would like to see.
- **Why** it would be useful (your use case).
- **How** you picture it working (optional but helpful).
