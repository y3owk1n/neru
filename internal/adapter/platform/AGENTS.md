# Platform adapters — contracts

The One Rule (root `AGENTS.md`) is enforced here hardest: non-darwin-tagged code never imports `platform/darwin`; cross via `ports.SystemPort` or a build-tagged dispatch pair. Only `platform/darwin/**`, `*_darwin.go`, and `*_integration_darwin_test.go` files are exempt.

- **Start at `profile.go`** — source of truth for each subsystem's backend family and whether a backend needs CGO (a per-backend decision, not per-OS).
- **File slots** — use the existing slot, never invent layout: `*_darwin.go`, `*_windows.go`, `*_other.go` (non-target fallback), `*_linux_common.go`, `*_linux_x11.go`, `*_linux_wayland.go`, `*_linux_wayland_<compositor>.go`.
- **Factory** — `factory.go` + build-tagged siblings are the only place that picks a `ports.SystemPort`. Linux adds a runtime axis: `backend_linux.go` detects the live compositor (wlroots / KDE / GNOME / other); never probe the compositor elsewhere.
- **Stubs are loud** — return `derrors.CodeNotSupported`, never a silent no-op, and pin it with a contract test. Keep `ports/capabilities.go` / `capability_presets.go` honest: a stub reports `stub`, not `supported` (`neru doctor` reports this matrix).
- **Coordinates** — shared code is global top-left origin, Y down, unscaled pixels. Cocoa's bottom-left flip happens inside the darwin adapter (`accessibility_screen_darwin.m`) only; conversions live in `internal/domain/geometry`.
- **`linux/wlr_protocol/` is generated** (`just generate-all-protocols`); never hand-edit.

Cross-compile every platform you touch (`just build-linux`, `just build-windows`) — build tags hide breakage from a host-only build. The `add-platform-feature` skill walks the full checklist; run the `platform-boundary-reviewer` agent on the diff before opening a PR.
