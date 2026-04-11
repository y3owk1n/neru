# Roadmap

This roadmap keeps the next improvements visible for contributors and helps
separate "stable on macOS today" from "portable by design, still incomplete".

## Near Term

- Strengthen macOS reliability around startup, config reloads, and mode transitions.
- Keep reducing global state to the minimum required by native bridge callbacks.
- Expand contract tests around ports, adapters, and reload behavior.
- Make unsupported platform capabilities fail loudly instead of silently no-oping.

## Cross-Platform Foundations

- Linux (X11):
  - [x] native overlay rendering
  - [x] screen and cursor management
  - [x] keyboard event capture & global hotkeys
  - [ ] AT-SPI accessibility integration (shared)
  - [ ] notifications and active app detection
- Linux (Wayland wlroots):
  - [x] native layer-shell overlay rendering
  - [x] virtual pointer injection and cursor discovery
  - [x] keyboard event capture
  - [ ] AT-SPI accessibility integration (shared)
  - [ ] notifications and active app detection
- Linux (Wayland GNOME):
  - [ ] input injection (libei or GNOME shell extension)
  - [ ] native overlay rendering
  - [ ] global hotkeys & event capture
- Linux (Wayland KDE Plasma):
  - [ ] input injection (libei or KWin protocols)
  - [ ] native overlay rendering
  - [ ] global hotkeys & event capture
- Windows:
  - [ ] UI Automation integration
  - [ ] screen and cursor management
  - [ ] global hotkeys and keyboard event capture
  - [ ] native overlay rendering
  - [ ] notifications and app watching

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
- Update `docs/ARCHITECTURE.md` and user-facing docs when support changes.
