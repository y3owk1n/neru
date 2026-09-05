# Roadmap

What we intend to work on next, and where help is most valuable.

This document holds **intent and priority only**. It does not restate what
works today. That lives in one place, the
[Capability Matrix](CROSS_PLATFORM.md#capability-matrix), with anything still
missing enumerated under [Known Gaps](CROSS_PLATFORM.md#known-gaps).

**Related:** [Cross-Platform Guide](CROSS_PLATFORM.md) ·
[Contributing](../CONTRIBUTING.md) · [Architecture](ARCHITECTURE.md)

---

## Where things stand

- **macOS** is Stable and the reference implementation.
- **Linux** and **Windows** are both Beta with parity complete: every option,
  mode flag, action and command means what it means on macOS, and
  [Known Gaps](CROSS_PLATFORM.md#known-gaps) carries no entry for either.
  Nothing is left to build for the label. What moves a platform to Stable is
  the six-clean-releases rule under
  [What the labels mean](CROSS_PLATFORM.md#what-the-labels-mean), which is
  earned by use rather than by a feature.

So the roadmap has no platform feature list. The work now is proving both
platforms under real workloads and keeping the core reliable.

## Near term

1. **Prove Linux and Windows in use.** A bug filed against either platform
   outranks any new capability. The Windows push landed seventeen features in
   one release and has been exercised only by CI, so expect and report rough
   edges. The open one today is
   [#1483](https://github.com/y3owk1n/neru/issues/1483), an injected scroll on
   Windows carrying modifiers the user is physically holding.
2. **Reliability over features.** Startup, config reload and mode transitions
   fail loudly and recover cleanly. Regressions here are fixed before anything
   else ships.
3. **Guardrails keep growing.** Contracts that fail silently earn a test in
   `internal/architecture` (ADR 0011). Reload behavior and port contracts are
   the areas still thinnest.

## Open direction

Ideas with maintainer interest and no schedule. Each is an issue rather than a
promise, and a Discussion is where a new one starts
([Contributing](../CONTRIBUTING.md#feature-requests)).

- **React to a real mouse click inside a mode**
  ([#1417](https://github.com/y3owk1n/neru/issues/1417)).
- **Subgrid preview** in recursive grid, the boundary counterpart of
  `sub_key_preview` ([#1116](https://github.com/y3owk1n/neru/issues/1116)).
- **Auto-refresh hints when the accessibility tree changes**
  ([#1002](https://github.com/y3owk1n/neru/issues/1002)).
- **COSMIC** ([#898](https://github.com/y3owk1n/neru/issues/898)). COSMIC
  advertises layer-shell and the virtual pointer, so most of the wlroots path
  should apply as-is. A contributor's branch got it building; the remaining
  work is focused-window geometry for the overlay and hint search. Until it
  lands the daemon refuses to start there as `wayland-other`.
- **GNOME Wayland** stays unsupported and is not scheduled. Reviving it needs
  libei plus a GNOME Shell extension for window geometry, see
  [LINUX_DESKTOPS.md](LINUX_DESKTOPS.md#gnome-not-supported).

## Contributor priorities

In priority order:

1. **Platform bugs on Linux and Windows.** Issues labelled
   `needs: linux contributor` or `needs: windows contributor` are the ones the
   maintainer cannot reproduce on their own hardware.
2. **A new desktop** (COSMIC first). Add the backend by mechanism rather than
   by desktop, per
   [organize by mechanism](CROSS_PLATFORM.md#organize-by-mechanism-not-by-desktop).
3. **Config reload regression coverage** through the simulation harness in
   `internal/app/simulation_harness_test.go`.
4. **Retiring remaining globals** behind explicit interfaces, where the native
   bridge callbacks allow it.

New to the codebase?
[Contributing safely](CROSS_PLATFORM.md#contributing-safely) lists starter
tasks, the changes worth opening an issue for first, and the five-point bar a
platform change has to clear before it lands.
