# Roadmap

What we intend to work on next, and where help is most valuable.

This document holds **intent and priority only**. It deliberately does not
restate what currently works — that lives in one place, the
[Capability Matrix](CROSS_PLATFORM.md#capability-matrix), with the outstanding
per-platform items enumerated under
[Known Gaps](CROSS_PLATFORM.md#known-gaps).

**Related:** [Cross-Platform Guide](CROSS_PLATFORM.md#feature-parity-reference) ·
[Contributing](../CONTRIBUTING.md) · [Architecture](ARCHITECTURE.md)

---

## Near term

Work that is not tied to any one platform:

- Strengthen macOS reliability around startup, config reloads, and mode
  transitions.
- Keep reducing global state to the minimum required by native bridge callbacks.
- Expand contract tests around ports, adapters, and reload behavior.
- Make unsupported platform capabilities fail loudly instead of silently
  no-oping.

## Cross-platform foundations

Linux is [beta and Windows alpha](CROSS_PLATFORM.md#what-the-labels-mean) —
meaning Linux is good for daily driving, while Windows is worth trying but not
yet worth switching to. Every remaining item is tracked as a numbered entry in
[Known Gaps](CROSS_PLATFORM.md#known-gaps) — pick one from there rather than
from a duplicate list here, so the status you read is the status the code
reports.

What Linux owes macOS, and what it does not, is settled in
[ADR 0013](adr/0013-parity-is-measured-in-words-not-subsystems.md): parity is a
behavioral promise about every option, mode flag, action and command, made on
wlroots Wayland with a CGO build, with a closed set of macOS-only exemptions.

The two largest open areas:

- **Linux** — Wayland global hotkeys, which need `input`-group membership and a
  CGO build rather than missing code, and whose remaining work is failing
  loudly with the remedy.
- **Windows** — foreground-window and display-hotplug events, which currently
  block per-app config re-application and monitor tracking.

GNOME Wayland remains unsupported; the daemon refuses to start there. Reviving
it needs libei plus a GNOME Shell extension — see
[LINUX_DESKTOPS.md](LINUX_DESKTOPS.md#gnome-not-supported).

## Contributor priorities

The highest-leverage areas, roughly in order:

1. Platform adapter implementations in `internal/adapter/platform`.
2. Overlay implementations and capability reporting.
3. Config reload regression coverage.
4. Reducing compatibility globals behind explicit interfaces.

New to the codebase?
[Contributing safely](CROSS_PLATFORM.md#contributing-safely) lists good starter
tasks, the changes worth opening an issue for first, and the five-point bar a
platform change has to clear before it lands.
