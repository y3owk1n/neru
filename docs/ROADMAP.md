# Roadmap

This roadmap keeps the next improvements visible for contributors and helps
separate "stable on macOS today" from "portable by design, still incomplete".

## Near Term

- Strengthen macOS reliability around startup, config reloads, and mode transitions.
- Keep reducing global state to the minimum required by native bridge callbacks.
- Expand contract tests around ports, adapters, and reload behavior.
- Make unsupported platform capabilities fail loudly instead of silently no-oping.

## Cross-Platform Foundations

- Linux:
  - AT-SPI accessibility integration
  - screen and cursor management
  - global hotkeys and keyboard event capture
  - native overlay rendering
  - notifications and app watching
- Windows:
  - UI Automation integration
  - screen and cursor management
  - global hotkeys and keyboard event capture
  - native overlay rendering
  - notifications and app watching

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
