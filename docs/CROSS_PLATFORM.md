# Cross-Platform Contributor Guide

This guide is the practical entry point for contributors working on Linux,
Windows, or platform-neutral infrastructure in Neru.

It explains:

- where platform code lives
- how to choose the right file before writing code
- how Linux backend splits work
- when to use CGO
- how to add a new platform capability safely
- what tests and docs to update when you are done

For the higher-level design, see [ARCHITECTURE.md](./ARCHITECTURE.md).

---

## Goals

Neru aims to make cross-platform work predictable and low-friction.

The guiding principles are:

- shared business logic stays in pure Go
- platform-specific code is easy to locate
- Linux backend differences are explicit
- contributors implement in existing slots instead of inventing new file layout
- unsupported features fail clearly with `CodeNotSupported`

---

## First Stops

Before changing code, read these files first:

- [internal/core/infra/platform/profile.go](file:///Users/kylewong/Dev/neru/internal/core/infra/platform/profile.go)
- [internal/core/ports/system.go](file:///Users/kylewong/Dev/neru/internal/core/ports/system.go)
- [internal/core/ports/capabilities.go](file:///Users/kylewong/Dev/neru/internal/core/ports/capabilities.go)
- [internal/core/ports/capability_presets.go](file:///Users/kylewong/Dev/neru/internal/core/ports/capability_presets.go)
- [docs/ARCHITECTURE.md](./ARCHITECTURE.md)
- [docs/go/CONVENTIONS.md](./go/CONVENTIONS.md)

If you are contributing Linux support, also inspect the reserved backend files in
the package you plan to touch, such as:

- `*_linux_common.go`
- `*_linux_x11.go`
- `*_linux_wayland.go`

---

## First 15 Minutes

If you are new to the codebase, this is the recommended startup path.

### Any platform

1. Read [profile.go](file:///Users/kylewong/Dev/neru/internal/core/infra/platform/profile.go)
2. Read the relevant port in `internal/core/ports/`
3. Find the implementation slot you expect to touch
4. Run:

```bash
just build
just test-foundation
```

### Linux contributors

1. Identify whether your work belongs in `common`, `x11`, or `wayland`
2. Open the target file in that slot before writing code
3. Build a Linux foundations binary from any host if needed:

```bash
just build-linux
```

4. If you are on Linux, also run:

```bash
just test
```

### Windows contributors

1. Start in the `*_windows.go` slot for the package you are changing
2. Build a Windows foundations binary from any host if needed:

```bash
just build-windows
```

3. If you are on Windows, run:

```bash
just test
```

### macOS contributors

If your work touches the real native product path, run:

```bash
just build-darwin
just test
```

---

## File Layout Rules

Platform-specific files should make the intended implementation slot obvious.

Use these suffixes:

- `*_darwin.go`: macOS implementation
- `*_windows.go`: Windows implementation
- `*_linux_common.go`: shared Linux wrapper or current fallback behavior
- `*_linux_x11.go`: Linux X11 implementation slot
- `*_linux_wayland.go`: Linux Wayland implementation slot
- `*_other.go`: non-target fallback for dispatch-style packages

Do not create new ad hoc platform filenames if an existing slot already exists.

Do not create fake empty `darwin` / `linux` / `windows` files just for symmetry.
Only add a new file when there is a real new implementation slot.

---

## Build And Test Commands

These are the main contributor commands:

| Goal | Command |
| --- | --- |
| build for current host | `just build` |
| build macOS binary on macOS | `just build-darwin` |
| build Linux foundations binary | `just build-linux` |
| build Windows foundations binary | `just build-windows` |
| run focused cross-platform-safe tests | `just test-foundation` |
| run full unit + integration suite on current OS | `just test` |
| run lint checks | `just lint` |

Notes:

- `just build-linux` and `just build-windows` are currently best viewed as
  foundations smoke tests while those platforms are still mostly scaffolding.
- `just release-ci <version>` still builds the current cross-platform release
  artifact matrix.
- macOS remains the only fully native product path today.

---

## Where To Implement What

Use this table as the default routing guide.

| Capability | Primary location |
| --- | --- |
| screen bounds, cursor, dark mode, notifications, permissions | `internal/core/infra/platform/<os>/` |
| global hotkeys | `internal/core/infra/hotkeys/` |
| keyboard event capture | `internal/core/infra/eventtap/` |
| accessibility integration | `internal/core/infra/accessibility/` |
| overlay window orchestration | `internal/ui/overlay/` |
| overlay rendering by mode | `internal/app/components/*/overlay_*.go` |
| app watcher / isolated platform hooks | dispatch-style `platform_*.go` files in the relevant package |

Examples:

- X11 hotkeys belong in [manager_linux_x11.go](/Users/kylewong/Dev/neru/internal/core/infra/hotkeys/manager_linux_x11.go)
- Wayland keyboard capture belongs in [eventtap_linux_wayland.go](/Users/kylewong/Dev/neru/internal/core/infra/eventtap/eventtap_linux_wayland.go)
- shared Linux system fallbacks belong in [system_linux_common.go](/Users/kylewong/Dev/neru/internal/core/infra/platform/linux/system_linux_common.go)

---

## Linux Backend Model

Linux is treated as a backend family, not a single target.

The expected split is:

- `common`: Linux-shared wrapper behavior, current fallback behavior, or backend selection
- `x11`: X11-specific implementation
- `wayland`: Wayland or compositor-specific implementation

This does not mean every Linux package must fully implement both backends
immediately. It means contributors should write code in the right slot from the
start.

Use `common` for:

- shared Linux types
- shared fallback behavior
- backend detection or routing
- helpers reused by both X11 and Wayland

Use `x11` for:

- X11 display enumeration
- X11 event capture
- X11 overlays
- X11 cursor movement or pointer queries

Use `wayland` for:

- compositor-specific capture or overlay behavior
- layer-shell integrations
- Wayland-specific output enumeration or pointer behavior

Accessibility is the main exception: much of Linux accessibility can stay
shared around AT-SPI, even when other capabilities split by backend.

---

## Windows Model

Windows is currently treated as one backend family.

For now, prefer:

- `*_windows.go` for the implementation slot
- pure Go Win32 / COM bindings where practical

Do not introduce additional Windows backend naming until there is a real reason.

---

## CGO Guidance

Do not decide CGO usage by OS alone.

Use [profile.go](/Users/kylewong/Dev/neru/internal/core/infra/platform/profile.go)
as the source of truth for the current backend plan.

Current intent:

- macOS: CGO required
- Linux: backend-dependent
- Windows: prefer pure Go first

Good default instincts:

- AT-SPI and freedesktop notifications should prefer pure Go / D-Bus paths
- X11 may be possible in pure Go depending on library choice
- some Wayland/compositor integrations may require CGO or native helpers
- Win32 hotkeys, hooks, monitor APIs, and UIA should prefer pure Go bindings first

If you introduce a backend that changes the build story, update:

- [profile.go](/Users/kylewong/Dev/neru/internal/core/infra/platform/profile.go)
- [justfile](/Users/kylewong/Dev/neru/justfile)
- this document

When in doubt, make the build assumption explicit in your PR description and in
the relevant backend comments.

---

## Hotkeys And Modifiers

Shared code should avoid hard-coding macOS conventions.

Use these rules:

- Use `Primary` when you mean "main accelerator modifier"
- `Primary` maps to `Cmd` on macOS and `Ctrl` on Linux/Windows
- keep backend-specific key translation inside infra/platform code
- do not spread X11, Wayland, Carbon, or Win32 naming into shared app logic

Relevant files:

- [internal/config/config.go](file:///Users/kylewong/Dev/neru/internal/config/config.go)
- [internal/core/domain/action/modifiers.go](file:///Users/kylewong/Dev/neru/internal/core/domain/action/modifiers.go)
- [internal/app/hotkeys.go](file:///Users/kylewong/Dev/neru/internal/app/hotkeys.go)

---

## Adding A New Capability

Use this flow.

### Option A: Broad system capability

If multiple services or app layers will need the capability:

1. Add it to [internal/core/ports/system.go](file:///Users/kylewong/Dev/neru/internal/core/ports/system.go)
2. Implement it in the darwin adapter
3. Add Linux common fallback behavior in `system_linux_common.go`
4. Add Windows fallback behavior in `system.go` under the Windows platform package
5. If Linux needs backend-specific behavior, push that implementation into `system_linux_x11.go` or `system_linux_wayland.go`
6. Update capability reporting if the support surface changed

### Option B: Isolated package-only platform behavior

If only one infra package needs the capability:

1. Keep the shared package code platform-agnostic
2. Use `platform_darwin.go` and `platform_other.go` dispatch files when that pattern fits
3. If Linux needs backend-specific behavior, add Linux backend files in that package instead of pushing the logic up into shared app code

---

## Error Handling Rules

For unimplemented platform behavior, return `CodeNotSupported`.

Example:

```go
return derrors.New(derrors.CodeNotSupported, "ScreenBounds not yet implemented on linux")
```

Use clear messages that name the missing operation and platform.

Do not silently no-op unless the behavior is explicitly documented as best-effort.

When a feature becomes real:

- replace the `CodeNotSupported` return
- update capability details
- remove stale TODO wording if it no longer applies

---

## Capability Reporting

Capability reporting is part of the contributor contract, not just a user nicety.

When you implement or partially implement a feature, review:

- [internal/core/ports/capabilities.go](file:///Users/kylewong/Dev/neru/internal/core/ports/capabilities.go)
- [internal/core/ports/capability_presets.go](file:///Users/kylewong/Dev/neru/internal/core/ports/capability_presets.go)
- [internal/app/ipc_info.go](file:///Users/kylewong/Dev/neru/internal/app/ipc_info.go)

`neru doctor` should help contributors and users understand what is actually
available on the current platform.

If a feature remains incomplete, keep the capability honest.

---

## Testing Checklist

Every platform contribution should update tests at the right level.

Use this checklist:

- unit tests for shared parsing, normalization, routing, or config logic
- contract tests for unsupported behavior and capability semantics
- integration tests for real platform behavior on the target OS

Typical test placement:

- `*_test.go`: shared or mocked logic
- `*_integration_linux_test.go`: real Linux behavior
- `*_integration_darwin_test.go`: real macOS behavior
- `*_integration_windows_test.go`: real Windows behavior when added

Good questions to answer in tests:

- does the adapter return the right error when unsupported?
- does the capability matrix reflect the new state?
- does backend selection route to the right Linux slot?
- does shared logic stay platform-neutral?

---

## Documentation Checklist

When you land platform work, update docs in the same PR.

Usually that means checking these files:

- [README.md](/Users/kylewong/Dev/neru/README.md)
- [docs/ARCHITECTURE.md](./ARCHITECTURE.md)
- [docs/DEVELOPMENT.md](./DEVELOPMENT.md)
- [docs/go/CONVENTIONS.md](./go/CONVENTIONS.md)

Update them when:

- the capability matrix changed
- the backend plan changed
- the build or CGO story changed
- a contributor-facing file naming pattern changed

---

## Suggested First Contributions

Good cross-platform starter tasks:

- improve capability detail text for an existing platform slice
- replace a Linux `CodeNotSupported` return with real X11 or AT-SPI behavior
- add a contract test for a currently stubbed feature
- add Linux or Windows integration test scaffolding
- document missing backend assumptions in the package you are touching

Higher-risk tasks:

- changing shared input semantics
- introducing CGO to a backend that was previously pure Go
- moving shared logic into platform packages
- mixing backend detection into app/service code

If your change falls into the higher-risk group, open or link an issue first.

---

## What "Done" Looks Like

A good platform PR usually leaves the repo better in five ways:

- the implementation lives in the intended file slot
- unsupported paths remain explicit and honest
- capability reporting is updated
- tests cover the new behavior or contract
- docs tell the next contributor what changed

That is the bar to aim for, even for small slices.
