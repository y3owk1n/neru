---
name: create-pr
description: "Commit working changes and open a Neru pull request the maintainer's way: conventional commit subjects written for the changelog, the just ci gate, and the repo PR template filled honestly. Use when asked to commit, create a PR, open a pull request, or ship finished work in this repo."
---

# Committing and opening a PR in Neru

Release Please turns commit subjects directly into the public changelog, and
the PR template checkboxes are review contract, not decoration. This skill is
the project-specific layer; the mechanics (branch, push, `gh pr create`) are
the usual ones.

## Before committing

1. Work happens on a branch, never on `main`. Branch names are short and
   kebab-case; a type prefix like `fix/hint-overlay-flicker` is welcome.
2. Run the full gate — it is exactly what CI runs, so surprises surface here:

   ```bash
   just ci
   ```

   For a docs-only change, `just fmt-check && just lint` is an acceptable
   fast path, but say so in the PR body.

## Commit messages

Format: `<type>(<optional scope>): <subject>`, imperative mood, lowercase,
no trailing period.

- **The subject ships in the changelog. Write it for a Neru user, not for the
  diff.** `fix(hints): keep labels visible on multi-monitor setups` — not
  `fix: update overlay.go`.
- Types that appear in the changelog: `feat`, `fix`, `perf`, `revert`,
  `improve`, `experiment`, `docs`. Hidden from it: `refactor`, `test`,
  `chore`, `ci`, `build`, `style`. (`release-please-config.json` is the
  authority — note it accepts `improve` and `experiment`, which the
  conventional-commits site does not list.)
- Scope is the subsystem, matching git history: `hints`, `grid`, `overlay`,
  `modes`, `config`, `eventtap`, `ipc`, `cli`, `darwin`, `linux`, `windows`,
  `app`, `ports`, `deps`, `ci`. Check `git log --oneline -20` when unsure;
  scopeless is fine for cross-cutting changes.
- The body earns its place by explaining *why*, and carries `Closes #123`
  when it fixes an issue.
- Breaking changes use `!` after the type/scope and a `BREAKING CHANGE:`
  footer — this drives a major version bump, so never add it casually.
- **No AI attribution trailers.** Do not add `Co-Authored-By: Claude`,
  "Generated with", or similar to commits or PR bodies.
- One logical change per commit, one logical change per PR. If the diff wants
  two types, it wants two PRs.

## The pull request

- **Title** is a conventional commit subject too (squash merges make it the
  changelog entry).
- **Body** follows `.github/pull_request_template.md` — all sections, in
  order: Description, Related Issues, Target Platform, Type of Change,
  Cross-Platform Checklist, General Checklist, Screenshots / Recordings,
  Additional Context. Delete the two optional trailing sections only if
  truly not applicable.
- **Check a checkbox only after verifying it.** "`just ci` passes" means you
  ran it in this worktree and it exited 0. An unchecked box with a one-line
  reason is honest; a false checkmark is not. The Cross-Platform Checklist
  has an explicit N/A option for shared-code-only changes.
- UI-visible changes (overlays, hints, grid) get a screenshot or short
  recording — build with `just build`, run `./bin/neru launch`.
- Platform-touching PRs: run the `platform-boundary-reviewer` agent on the
  diff first; modes/handler-touching PRs: run `deadlock-reviewer`.

## After opening

Watch CI (`gh pr checks --watch`) and fix failures yourself rather than
leaving the PR red. Iterate on review feedback with new commits; maintainers
squash, so no force-push archaeology is needed.
