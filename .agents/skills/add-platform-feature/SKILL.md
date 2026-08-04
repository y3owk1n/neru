---
name: add-platform-feature
description: "Implement or stub Neru functionality for a specific OS/backend: port contract, build-tagged file slots, factory wiring, CodeNotSupported stubs, capability matrix, contract tests, and the docs ownership table. Use for any change under internal/adapter/platform or any *_darwin/_linux/_windows file. Not for pure-Go domain logic."
---

# Platform work in Neru

Platform code is where Neru's guardrails are strictest, because this is where
agents most often guess. The failure modes are always the same: importing the
darwin package from shared code, inventing a new file-layout convention,
silently no-opping an unsupported call, or reporting a stub as `supported`.

## Before writing code

1. Read `internal/adapter/platform/profile.go` first. It is the source of
   truth for each subsystem's backend family, primary-modifier expectations,
   and whether a backend needs CGO. CGO is a **per-backend** decision, not a
   per-OS one.
2. Read the port you're implementing in `internal/ports/`. If the capability
   doesn't have a port yet, define the interface there first and add a mock in
   `internal/ports/mocks`.

## The rules

- **The One Rule**: non-darwin-tagged code never imports
  `internal/adapter/platform/darwin`. Enforced by `depguard` and
  `internal/architecture/dependency_boundary_test.go`. Cross the boundary via
  `ports.SystemPort` or a build-tagged dispatch pair.
- **File slots** — use the existing slot, never invent layout:
  `*_darwin.go`, `*_windows.go`, `*_other.go` (non-target fallback),
  `*_linux_common.go`, `*_linux_x11.go`, `*_linux_wayland.go`,
  `*_linux_wayland_<compositor>.go`.
- **Factory**: `internal/adapter/platform/factory.go` and its build-tagged
  siblings are the only place that picks a `ports.SystemPort` implementation.
  Linux adds a runtime axis: `backend_linux.go` detects the live compositor
  (wlroots / KDE / GNOME / other) and routes. Do not probe the compositor
  anywhere else.
- **Stubs are loud**: unimplemented behavior returns
  `derrors.New(derrors.CodeNotSupported, ...)` — never a silent no-op, never
  `nil`. Callers degrade via `derrors.IsNotSupported(err)`.
- **Capability honesty**: update `internal/ports/capabilities.go` /
  `capability_presets.go` in the same change. `neru doctor` reports this
  matrix; a stub must report `stub`, not `supported`.
- **Coordinates**: shared code is global top-left origin, Y down, unscaled
  pixels. Flipping from Cocoa's bottom-left happens inside the darwin adapter
  only; never leak flipped coordinates into shared Go
  (`internal/domain/geometry` owns conversions).
- Wayland protocol bindings under
  `internal/adapter/platform/linux/wlr_protocol/` are generated — regenerate
  with `just generate-all-protocols`, never hand-edit.

## Tests

- Unit tests with port mocks stay platform-neutral.
- Real-OS behavior goes in `*_integration_<os>_test.go` tagged
  `//go:build integration && <os>`.
- Every new stub gets (or updates) a contract test pinning the
  `CodeNotSupported` behavior, so a later "implementation" that silently
  no-ops fails a test.

## Docs (same change, not a follow-up)

`docs/CROSS_PLATFORM.md` has the ownership table ("Documentation Checklist") —
each fact has exactly one home. Capability status goes in
`docs/CROSS_PLATFORM.md`, never in `docs/ARCHITECTURE.md` (shape, not status).
Linux setup specifics go in `docs/LINUX_SETUP.md` / `docs/LINUX_DESKTOPS.md`.

## Verify

```bash
just lint                      # depguard enforces the One Rule
go test ./internal/architecture/
just test-foundation
```

Cross-compile the platforms you touched (`just build-linux`,
`just build-windows`) even when you can't run them — build tags hide breakage
from a darwin-only `just build`. Then the standard pre-commit gate, and ask a
maintainer to run integration tests on the target OS if you can't.
