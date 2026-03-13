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
- [ ] No darwin imports from untagged (shared) code — [The One Rule](docs/ARCHITECTURE.md#the-one-rule)
- [ ] Stub implementations added for other platforms (returning `CodeNotSupported`)
- [ ] N/A — This PR does not touch platform-specific code

## General Checklist

- [ ] Code formatted (`just fmt`)
- [ ] Linters pass (`just lint`)
- [ ] Tests pass (`just test`)
- [ ] Build succeeds (`just build`)
- [ ] Tests added/updated for new or changed functionality
- [ ] Documentation updated (if applicable)
- [ ] Commit messages follow [conventional commits](https://www.conventionalcommits.org/)

## Screenshots / Recordings

<!-- For UI changes, add before/after screenshots or a short recording. Delete this section if not applicable. -->

## Additional Context

<!-- Anything else reviewers should know? Design decisions, trade-offs, alternative approaches considered? Delete this section if not applicable. -->
