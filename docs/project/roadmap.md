# Roadmap

What Neru intends to work on next, and in what order. What works today is in the
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
   expect bugs and report them.
2. **Reliability over features.** Startup, config reload and mode transitions
   fail loudly and recover cleanly. Regressions here are fixed before anything
   else ships.
3. **More automated checks.** Anything that could break without an error gets
   a test.

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

## Helping out

Where contributions help most is in
[CONTRIBUTING.md](../../CONTRIBUTING.md#where-help-is-most-useful).
