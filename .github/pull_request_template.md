## Description

<!-- What does this PR do? Why is it needed? -->

## Related Issues

<!-- Link related issues: Closes #123, Fixes #456 -->

## Target Platform

<!-- Check all that apply: -->

- [ ] Platform-agnostic (shared logic, no OS-specific code)
- [ ] macOS
- [ ] Linux
- [ ] Windows

## Type of Change

<!-- Check the one that applies: -->

- [ ] `feat` — New feature
- [ ] `fix` — Bug fix
- [ ] `refactor` — Code restructuring (no behavior change)
- [ ] `perf` — Performance improvement
- [ ] `docs` — Documentation only
- [ ] `test` — Adding or updating tests
- [ ] `chore` — Build, CI, dependencies, tooling

## Cross-Platform Checklist

<!-- If your PR touches OS-specific code, verify the following. Check N/A if not applicable. -->

- [ ] OS-specific files use correct build tags (e.g., `//go:build darwin`)
- [ ] No darwin imports from untagged (shared) code — [The One Rule](https://github.com/y3owk1n/neru/blob/main/docs/ARCHITECTURE.md#the-one-rule)
- [ ] Stub implementations added for other platforms (returning `CodeNotSupported`)
- [ ] N/A — This PR does not touch platform-specific code

## General Checklist

- [ ] Code formatted (`just fmt`)
- [ ] `just ci` passes — the same checks CI runs, on your host only (format
      check, lint, vet, tests including a unit `-race` pass, vulnerability
      scan, build); CI runs them on macOS, Linux and Windows
- [ ] Tests added/updated for new or changed functionality
- [ ] Documentation updated (if applicable)
- [ ] PR title is a [conventional commit](https://www.conventionalcommits.org/)
      subject written for users — this PR squash-merges, so the title is what
      Release Please ships in the changelog, not the commits on the branch

## Screenshots / Recordings

<!-- For UI changes, add before/after screenshots or a short recording. Delete this section if not applicable. -->

## Additional Context

<!-- Anything else reviewers should know? Design decisions, trade-offs, alternative approaches considered? Delete this section if not applicable. -->
