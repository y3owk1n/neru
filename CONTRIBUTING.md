# Contributing to Neru

Code, docs, bug reports, config examples and ideas are all welcome. Building
and testing are in the [development guide](docs/contributing/development.md),
and code conventions in [AGENTS.md](AGENTS.md).

## Code of conduct

This project follows the [Code of Conduct](CODE_OF_CONDUCT.md). Report
unacceptable behavior privately to [@y3owk1n](https://github.com/y3owk1n), not
in a public issue.

## Getting started

Search existing issues first, open an issue to agree on the approach for a
non-trivial change, and keep PRs small and focused.

Set up with the [development guide](docs/contributing/development.md#development-setup).
On Linux, install the [build dependencies](docs/contributing/development.md#build-dependencies)
first, because oku provides the toolchain but not the system libraries.

## Making changes

1. **Fork** the repository, clone your fork, and branch from `main`
   (`git checkout -b feat/my-feature`).
2. **Make your changes** following [AGENTS.md](AGENTS.md).
   [Where things go](docs/contributing/development.md#where-things-go) maps code
   to directories. Platform code follows the
   [porting guide](docs/contributing/porting.md) and
   [the One Rule](docs/contributing/architecture.md#the-one-rule).
3. **Add or update tests** for new code
   ([Testing](docs/contributing/development.md#testing)).
4. **Run the pre-commit checks:**

    ```bash
    just fmt && just lint && just test && just build
    ```

    Use `//nolint` only for a false positive or where the compliant form is
    worse, naming the linter with a trailing `// reason`. To suppress one
    linter repeatedly, propose disabling it in `.golangci.yml` instead.

    Quit any running `neru` daemon first, because it holds the socket and the
    IPC integration tests skip. Tests that drive the real cursor and keyboard
    run only under `just test-desktop`
    ([Running integration tests](docs/contributing/development.md#running-integration-tests)).

    Before pushing, run **`just ci`**, the same recipes CI gates your PR on,
    run on your host only
    ([what it covers](docs/contributing/development.md#what-just-ci-covers-and-what-it-does-not)).

5. **Update the docs** in the same PR. The
   [documentation checklist](docs/contributing/porting.md#documentation-checklist)
   names the one file that owns each fact. Update that file, not a copy.

6. **Commit** with [conventional commits](#commit-messages), push, and open a
   pull request.

## Commit messages

[Conventional Commits](https://www.conventionalcommits.org/) drive releases
through [Release Please](https://github.com/googleapis/release-please).

**The changelog entry is the PR title.** PRs squash-merge, the only merge
method enabled, so the branch lands as one commit whose subject is the PR
title. Write it for users. Keep branch commits conventional too, since
reviewers read them one by one and one of them usually becomes the title.

```text
<type>(<optional scope>): <subject>

<optional body explaining why>

<optional footer, such as Closes #123>
```

Types: `feat` (new feature), `fix` (bug fix), `docs` (documentation only),
`style` (formatting), `refactor` (no behavior change), `perf` (performance),
`test` (tests), and `chore` (build, CI, dependencies, tooling).

Examples: `feat(grid): add recursive subdivision mode`,
`fix(hints): correct overlay positioning on multi-monitor setups`.

## Pull requests

The **title** follows the commit format. The **description** says what changed
and why, links issues (`Closes #123`), and shows screenshots or recordings for
UI changes. All CI checks must pass, and a maintainer reviews before merge.

## AI-assisted contributions

AI-assisted PRs get the same review bar. [AGENTS.md](AGENTS.md) is the entry
point to the shared context, skills and review profiles. `CLAUDE.md` and
`.claude/skills` are git symlinks, so on Windows clone with
`git config core.symlinks true` (needs Developer Mode) or read `AGENTS.md`
directly. Claude Code asks once for workspace trust before running the
format-on-edit hook. You own the result: run the checks, read the diff, and do
not submit changes you cannot explain.

## Where help is most useful

1. **Platform bugs on Linux and Windows.** Issues labelled
   `needs: linux contributor` or `needs: windows contributor` are the ones the
   maintainer cannot reproduce on their own hardware.
2. **A new desktop**, added by mechanism rather than by desktop
   ([organize by mechanism](docs/contributing/porting.md#organize-by-mechanism-not-by-desktop)).
3. **Config reload regression coverage** through the simulation harness in
   `internal/app/simulation_harness_test.go`.
4. **Retiring remaining globals** behind explicit interfaces, where the native
   bridge callbacks allow it.

## Good first contributions

- Bug fixes from the [open issues](https://github.com/y3owk1n/neru/issues)
- Documentation fixes, demo videos or GIFs, test coverage, performance
- Config examples in the [community configurations](docs/project/showcases.md)
- Platform: capability detail text, a contract test for a stubbed feature, a
  `platform: linux` or `platform: windows` bug, or backend assumptions
  documented in the package you touch

[Contributing safely](docs/contributing/porting.md#contributing-safely) lists
platform changes that need an issue first. Direction is in the
[roadmap](docs/project/roadmap.md).

## Reporting bugs

Use the [issue forms](https://github.com/y3owk1n/neru/issues/new/choose) and
include:

1. **Platform and version**: OS and its version, `neru --version`, and on
   Linux the desktop and session type.
2. **Minimal steps to reproduce**.
3. **Expected and actual behavior**.
4. **Logs**. File logging is off by default, so enable it first
   ([Log file locations](docs/guide/troubleshooting.md#log-file-locations)).
5. **Screenshots or recordings** for visual issues, and `neru doctor` output.

## Feature requests

Open an [issue](https://github.com/y3owk1n/neru/issues/new/choose) or a
[Discussion](https://github.com/y3owk1n/neru/discussions) saying what you want,
your use case, and optionally how you picture it working.
