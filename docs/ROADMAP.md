# Roadmap

This roadmap keeps the next improvements visible for contributors and helps
separate "stable on macOS today" from "portable by design, still incomplete".

**Related:** [Cross-Platform Guide](CROSS_PLATFORM.md#feature-parity-reference) ·
[Development Guide](DEVELOPMENT.md) · [Architecture](ARCHITECTURE.md)

---

## Table of Contents

- [Near Term](#near-term)
- [Cross-Platform Foundations](#cross-platform-foundations)
- [Contributor Priorities](#contributor-priorities)
- [Definition Of Done For New Platform Work](#definition-of-done-for-new-platform-work)

---

## Near Term

- Strengthen macOS reliability around startup, config reloads, and mode transitions.
- Keep reducing global state to the minimum required by native bridge callbacks.
- Expand contract tests around ports, adapters, and reload behavior.
- Make unsupported platform capabilities fail loudly instead of silently no-oping.

## Cross-Platform Foundations

- Linux (X11):
    - [x] native overlay rendering
    - [x] screen and cursor management
    - [x] keyboard event capture & global hotkeys (`XGrabKeyboard` / `XGrabKey`)
    - [x] AT-SPI accessibility integration (shared)
    - [x] active app detection & app watcher (event-driven)
    - [ ] freedesktop notifications and alerts
- Linux (Wayland wlroots):
    - [x] native layer-shell overlay rendering
    - [x] virtual pointer injection and cursor discovery
    - [x] keyboard event capture
    - [x] global hotkeys (passive evdev read of Neru's own config)
    - [x] AT-SPI accessibility integration (shared)
    - [x] active app detection & app watcher (`wlr-foreign-toplevel` app_id)
    - [ ] freedesktop notifications and alerts
- Linux (Wayland KDE Plasma):
    - [x] input injection (libei via RemoteDesktop portal)
    - [x] native overlay rendering (wlr-layer-shell + Cairo)
    - [x] global hotkeys & event capture (passive evdev read)
    - [x] app watcher / focus tracking
    - [ ] persist the RemoteDesktop portal grant across daemon restarts
    - [ ] freedesktop notifications and alerts
- Linux (Wayland GNOME) — not supported; the daemon refuses to start:
    - [ ] input injection (libei or GNOME shell extension)
    - [ ] native overlay rendering
    - [ ] global hotkeys & event capture
    - [ ] focused-app source (Mutter exposes no `wlr-foreign-toplevel` equivalent)
- Windows:
    - [x] UI Automation integration — initial coverage (shallow tree)
    - [x] screen and cursor management
    - [x] global hotkeys and keyboard event capture
    - [x] native overlay rendering — layered Win32 window + GDI
    - [ ] app watcher (foreground-window notifications) and display hotplug events
    - [ ] toast notifications
    - [ ] `monitor_select` mode, grid transition animation, horizontal scroll

## Contributor Priorities

If you want to help, the highest-leverage areas are:

1. Platform adapter implementations in `internal/core/infra/platform`.
2. Overlay implementations and capability reporting.
3. Config reload regression coverage.
4. Reducing compatibility globals behind explicit interfaces.

## Definition Of Done For New Platform Work

- Return real values instead of `CodeNotSupported`.
- Add unit tests next to the implementation.
- Add integration tests when the feature needs a real OS session.
- Update `ARCHITECTURE.md` and user-facing docs when support changes.
