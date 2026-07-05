---
name: greptile-review
description: Grill a diff or PR for real bugs the way Greptile does, before committing or pushing. Repo-agnostic self-review: catches fail-open logic, lifecycle races, partial-failure state, stale caches, resource/lock leaks, conditional-compilation gaps, and doc/config drift, verifies findings by building/running, re-checks that a fix did not introduce a new issue, and reports a 0-5 confidence score with P0/P1/P2 findings. Canonical copy lives in workspace-configs; this repo keeps the Neru house-rules block.
license: MIT
compatibility: opencode
metadata:
  audience: developers
  workflow: review
---

## What I do

I review a set of changes (working tree, a branch vs its base, or a PR) the way
Greptile reviews a pull request: not line-by-line on the diff alone, but with
whole-codebase context, hunting for the small number of changes that actually
break behavior. The goal is to catch the issues a review bot would flag *before*
the PR is opened, so the review loop is short instead of a back-and-forth.

I am repo-agnostic. Step 0 loads the current repo's own standards and build/test
commands, so I enforce this codebase's conventions, whatever language it is in.

## When to use me

- Before committing, or before pushing a branch / opening a PR.
- After addressing review feedback, to confirm the fix did not create a new bug
  (the most common cause of a second review round).
- On changes with conditional compilation, platform variants, or runtime routing,
  where it is easy to break a target you cannot see from the diff.

## How Greptile thinks (imitate this)

1. **Context beyond the diff.** It builds a graph of the repo and, for every
   changed function, looks at its callers, its callees, sibling implementations,
   and similar code elsewhere. Most real bugs live in the *relationship* between
   the change and the rest of the system, not in the changed line.
2. **Many angles.** It reviews the same change through several independent lenses
   (logic, concurrency, error paths, platform/variant, security) instead of one
   pass.
3. **It runs the code.** Greptile confirms a suspected bug with a focused
   reproduction before reporting it. Do the same: build, test, or trace the exact
   path. Report verified issues, not guesses.
4. **High signal.** It ranks by severity and suppresses noise. A wrong nit costs
   trust and wastes a round-trip. Prefer five real bugs over fifty nits.
5. **It knows the team's standards.** Ground every judgment in *this repo's* own
   documented rules, not generic style opinions (see Step 0).

## Procedure

### 0. Learn the house rules and toolchain

Before judging anything, learn how this repo expects code to look and how to
build/test it. Read what exists; skip what does not:

- Standards: `AGENTS.md`, `CLAUDE.md`, `.cursor/rules/`, `CONTRIBUTING.md`,
  `docs/` architecture / conventions / cross-platform guides, `ADR/`.
- Enforcement config: linters (`.golangci.yml`, `.eslintrc`, `ruff.toml`,
  `clippy`), formatters, `depguard`/import rules, CI workflows.
- Toolchain: detect how this repo builds, tests, and lints - `justfile`,
  `Makefile`, `package.json` scripts, `cargo`, `go`, `pyproject.toml`, CI steps.
  Use those commands in Step 4; do not assume `just`/`npm`/`make`.

Treat these as the review rules. A change that violates the repo's own documented
contract is a finding even if the code "works". This repo's rules are summarized
under House rules below.

### 1. Scope the change

Get the exact diff and its base. Do not review from memory.

```bash
git diff --stat <base>...HEAD      # branch vs its base (often main)
git diff <base>...HEAD             # full diff
git diff                           # uncommitted work
gh pr diff <N>                     # a specific PR
```

List every changed file and, for each, the changed symbols (functions, types,
and any build tags / conditional-compilation guards at the top of the file).

### 2. Build context beyond the diff

For each changed or newly-referenced symbol, before judging it, find:

- **Callers** - who calls it and what do they assume about its return, its error,
  its side effects? Use grep / LSP references, not intuition.
- **Callees** - what does it call, and what happens on each failure path?
- **Sibling implementations** - the same function under other build tags and
  platforms. In Neru this is the axis that breaks silently: `*_darwin.go`,
  `*_linux_*.go`, `*_windows.go`, `*_other.go`, and the `_cgo.go` / `_nocgo.go`
  split. A symbol used in shared or cgo code must be defined for *every* tag
  combination that compiles.
- **Similar patterns** - how does the rest of the codebase already do this
  (error wrapping, resource release, cleanup, capability reporting)? Flag the
  change if it is the odd one out.

### 3. Hunt bugs with the lenses

Run every lens below over the changed code plus the context you gathered. These
are the classes that produce real, high-severity findings.

### 4. Verify every finding

For each suspected issue, prove it before reporting:

- compile the affected variant / build-tag combination (see Commands),
- run or write a focused test that exercises the path, or
- trace the exact call sequence and cite the file:line that makes it fire.

Drop anything you cannot ground. An unverified finding is a false positive, and
false positives are the thing this skill exists to prevent.

### 5. Re-review your own fix

After proposing or applying a fix, re-run steps 2-4 on the changed lines *and* on
the callers and variant-siblings the fix touched. Fixes routinely introduce the
next bug (a stub added for one variant but not another, a release added on one
path but not a parallel path, a guard that now fails closed). This step is the
whole point: it stops the fix-one-find-two cycle.

## Bug-hunt lenses

- **Fail-open / fail-closed defaults.** A lookup fails and the code falls back to
  an empty/zero/nil value that then *passes* a guard it should fail. Example: an
  empty id makes an "is excluded" check return false, so an excluded item still
  gets the behavior. Ask: what does every guard do when its input is the zero
  value?
- **Partial failure leaves inconsistent state.** A multi-step side effect where a
  later step can fail after an earlier step succeeded: acquire without release
  (lock/grab/handle stuck held), press without release, a batch loop that returns
  on the first failed item and drops the rest, no cleanup on the error branch.
  Ask: if step N fails, is the world left half-changed? Is there cleanup or retry?
- **Concurrency and lifecycle races.** A goroutine/thread touches a
  channel/field/mutex before another assigns or releases it (nil-channel send,
  use-before-init, holding a lock across a blocking call, deadlock); a quit/stop
  path that is a no-op on one variant so a loop never exits; double-close; writing
  to a channel no one reads. Ask: what if this runs before/after, or concurrently
  with, the thing it depends on?
- **Interrupted syscalls / retryable errors swallowed.** A blocking read/poll/wait
  that treats `EINTR` or a transient error as fatal, killing a worker and leaking
  whatever it owned. Ask: which errors are retryable, and does the loop retry them?
- **Stale or shared cache reused without validation.** A cached value (geometry,
  origin, session, handle, token) is applied to a new context without checking it
  still belongs to that context, so two same-shaped things collide. Ask: is the
  cache keyed by identity, or only by a property two different objects can share?
- **Conditional-compilation / variant gaps.** A symbol referenced in shared code
  but not defined in a sibling variant (`_nocgo`, other `GOOS`, other `#ifdef`),
  so that variant fails to compile. Compile the variants your host can build.
- **Error handling.** Swallowed errors, unsupported paths that silently no-op
  instead of returning the repo's documented "not supported" error, errors
  returned raw to the user (leaking internals), resources not freed on error.
- **Boundaries and nil.** Off-by-one, empty slice/map, unchecked type assertion,
  nil deref, integer overflow mid-loop, unchecked index.
- **Doc / config drift.** Code changed but a parallel list did not: a dependency
  added to one package manager's list but not another's, a new build dep missing
  from the dev environment or CI, a version pinned in docs that disagrees with CI,
  capability/help output not updated, docs describing the old behavior. Real
  findings when they mislead a user or break a build.
- **Security.** Injection, command building from untrusted input, secrets in code,
  missing validation on an external boundary.

## House rules (this repo: Neru)

Grounded in `docs/CROSS_PLATFORM.md` and `docs/ARCHITECTURE.md`.

- **The One Rule.** Non-darwin-tagged code must never import
  `internal/core/infra/platform/darwin`. Enforced by `depguard`; confirm with
  `just lint`.
- **Variant matrix is complete.** Every symbol used under a build tag has a
  definition under that tag. Check the cgo/nocgo split and every `GOOS`.
- **Unsupported paths are honest.** New stubs return `CodeNotSupported` with a
  message naming the operation and platform, not a silent no-op.
- **Right file slot.** New platform code lives in the intended slot
  (`*_linux_wayland_<compositor>.go`, etc.); runtime routing goes through the
  `LinuxBackend` seams, not `XDG_CURRENT_DESKTOP` sniffing spread across the app.
- **Capability reporting and docs updated in the same change** when the support
  surface, backend plan, or build story changed.

## Commands

Verification, not decoration. Run the ones relevant to the change.

```bash
# Compile the variant that silently breaks: pure-Go / nocgo path (any host).
CGO_ENABLED=0 GOOS=linux   go build ./...
CGO_ENABLED=0 GOOS=windows go build ./...
CGO_ENABLED=0 GOOS=linux   go vet -tags linux ./...

# Focused test of a specific path (mirrors how Greptile's T-Rex reproduces).
go test -tags linux -run <TestName> -count=1 -v ./internal/<pkg>

# Repo recipes.
just build            # current host
just build-linux      # Linux cgo build (needs a Linux host/CI for native backends)
just build-windows    # Windows nocgo build
just lint             # golangci-lint, incl. depguard (The One Rule)
just test             # unit + integration on current OS
just test-foundation  # fast cross-platform-safe slice
```

Note: cgo cross-builds for a non-host OS need that platform's toolchain, so run
them on the target host or in CI. The `CGO_ENABLED=0` compile checks run anywhere
and catch the missing-variant class directly.

## Confidence score rubric

Compute the 0-5 score from what you actually found and verified, not vibes. The
score is the *lowest* band any finding forces. Start at 5 and drop:

| Score   | Meaning               | Trigger                                                                                 |
| ------- | --------------------- | --------------------------------------------------------------------------------------- |
| **5**   | Production ready      | No finding above P2; changed behavior is covered/verified. Merge.                       |
| **4**   | Minor polish          | Only P2s, or a single low-confidence P1. Merge after small fixes.                       |
| **3**   | Implementation issues | One or more verified P1s, no P0. Address before merge.                                  |
| **2**   | Significant bugs      | Multiple verified P1s, or a P1 in a core / irreversible path. Rework.                   |
| **0-1** | Critical              | Any verified P0 (crash, data loss, security, corruption), or the change does not build. |

Modifiers - apply and state them:

- **Blast radius.** A P1 in an input-injection / daemon-lifecycle / irreversible
  path drops the score one band versus the same bug in a test or internal script.
- **Verification ceiling.** If you could not build or exercise the changed path
  (e.g. a cgo Linux backend on a macOS host, cross-compile only), cap the score at
  4 and say the ceiling is verification, not a clean bill of health.
- **Unknowns.** If the diff touches areas you could not fully trace, say so and do
  not award 5.

Always state the one or two things that cap the score, so the number is auditable.

## Output format

Mirror a Greptile review. Lead with the score, then findings, high signal first.

- **Confidence score, 0-5** (per the rubric) with the one or two things that cap it.
- **Findings**, each as:
  - **Severity**: P0 (critical: crash, data loss, security), P1 (bug / wrong
    behavior / edge case), P2 (quality / maintainability).
  - **Type**: logic, syntax, or style.
  - **Location**: `path:line`.
  - **What breaks and when it triggers** - one tight paragraph. Name the exact
    condition that makes it fire.
  - **Suggested fix** - concise, concrete.
  - **Verified by** - the build, test, or trace that confirms it. If you could not
    verify, say so and mark it low-confidence.
- Note any suspected issue that turned out to be a deliberate/handled design, so
  it is not re-raised next round.
- Suppress nitpicks unless asked for them. If you found nothing real, say the
  change looks clean and give the score; do not manufacture findings.
