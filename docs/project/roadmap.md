# Roadmap

Intent and priority only. What works today is in the
[Capability Matrix](../reference/platform-support.md#capability-matrix), and
what is missing under [Known Gaps](../reference/platform-support.md#known-gaps).

## Where things stand

Each platform's label is in
[Platform status](../reference/platform-support.md#platform-status). Linux and
Windows have nothing left to build for theirs. A platform reaches Stable
through use, not features, per
[What the labels mean](../reference/platform-support.md#what-the-labels-mean),
so the roadmap has no platform feature list.

## Near term

1. **Prove Linux and Windows in use.** A bug filed against either platform
   outranks any new capability. Windows has had little real-world use, so
   expect bugs and report them, such as
   [#1483](https://github.com/y3owk1n/neru/issues/1483): an injected scroll on
   Windows carries modifiers the user is physically holding.
2. **Reliability over features.** Startup, config reload and mode transitions
   fail loudly and recover cleanly. Regressions here are fixed before anything
   else ships.
3. **More guardrails.** A contract that fails silently gets a test in
   `internal/architecture` (ADR 0011). Reload behavior and port contracts have
   the fewest guardrails so far.

## Open direction

Ideas with maintainer interest and no schedule. Each is an issue rather than a
promise, and a Discussion is where a new one starts
([Contributing](../../CONTRIBUTING.md#feature-requests)).

- **React to a real mouse click inside a mode**
  ([#1417](https://github.com/y3owk1n/neru/issues/1417)).
- **Subgrid preview** in recursive grid, the boundary counterpart of
  `sub_key_preview` ([#1116](https://github.com/y3owk1n/neru/issues/1116)).
- **Auto-refresh hints when the accessibility tree changes**
  ([#1002](https://github.com/y3owk1n/neru/issues/1002)).

## Contributor priorities

1. **Platform bugs on Linux and Windows.** Issues labelled
   `needs: linux contributor` or `needs: windows contributor` are the ones the
   maintainer cannot reproduce on their own hardware.
2. **A new desktop**, added by mechanism rather than by desktop
   ([organize by mechanism](../contributing/porting.md#organize-by-mechanism-not-by-desktop)).
3. **Config reload regression coverage** through the simulation harness in
   `internal/app/simulation_harness_test.go`.
4. **Retiring remaining globals** behind explicit interfaces, where the native
   bridge callbacks allow it.

Starter tasks are in
[Good first contributions](../../CONTRIBUTING.md#good-first-contributions), and
platform changes that need an issue first in
[Contributing safely](../contributing/porting.md#contributing-safely).
