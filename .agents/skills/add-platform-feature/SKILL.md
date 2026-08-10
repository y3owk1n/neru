---
name: add-platform-feature
description: "Implement or stub Neru functionality for a specific OS/backend: port contract, build-tagged file slots, factory wiring, CodeNotSupported stubs, capability matrix, contract tests, and the docs ownership table. Use for any change under internal/adapter/platform or any *_darwin/_linux/_windows file. Not for pure-Go domain logic."
---

# Platform work in Neru

Platform code carries Neru's strictest guardrails, and this is where agents
most often guess. A change that lands cleanly sits in an existing file slot,
crosses the boundary through a port, returns `CodeNotSupported` wherever it is
unimplemented, and reports itself honestly in the capability matrix.

## Before writing code

1. Read `internal/adapter/platform/AGENTS.md` — the single home for the rules
   this skill obeys: the One Rule, file slots, the factory and Linux's runtime
   compositor axis, loud stubs, capability honesty, coordinates, generated
   Wayland bindings. Add the `darwin/` or `linux/` guide for the native
   boundary you touch.
2. Read `internal/adapter/platform/profile.go`. It is the source of truth for
   each subsystem's backend family, primary-modifier expectations, and whether
   a backend needs CGO. CGO is a **per-backend** decision, not a per-OS one.
3. Read the port you're implementing in `internal/ports/`. If the capability
   doesn't have a port yet, define the interface there first and add a mock in
   `internal/ports/mocks`.

## Tests

- Unit tests with port mocks stay platform-neutral.
- Real-OS behavior goes in `*_integration_<os>_test.go` tagged
  `//go:build integration && <os>`.
- Contract tests pin `CodeNotSupported` per subsystem, not per stub. Update the
  subsystem's existing one if it has one, and write a new one when a caller
  could read the stub's `nil` as success, so a later "implementation" that
  silently no-ops fails a test. `internal/adapter/platform/AGENTS.md` names the
  ones that exist.

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
