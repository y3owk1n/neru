# Platform Porting Guide

Guiding principles:

- shared business logic stays in pure Go
- platform-specific code is easy to locate
- Linux backend differences are explicit
- contributors implement in existing slots instead of inventing new file layout
- unsupported features fail loudly with `CodeNotSupported`

## First Stops

Read these before changing platform code:

- [The Three Tiers](#the-three-tiers): **start here**, it decides where your code goes
- [platform/profile.go](../../internal/adapter/platform/profile.go): per-subsystem backend family and CGO expectations
- [ports/system.go](../../internal/ports/system.go): the main OS contract, plus the optional-extension pattern
- [ports/capabilities.go](../../internal/ports/capabilities.go) and [capability_presets.go](../../internal/ports/capability_presets.go): the capability registry `neru doctor` reports
- [architecture/platform_slots_test.go](../../internal/architecture/platform_slots_test.go): the file-layout rules, as executable checks
- [ARCHITECTURE.md](./architecture.md) and the root [AGENTS.md](../../AGENTS.md) conventions

Contributing Linux support? Nothing is reserved and waiting for you. Read the
Linux files the package already has before writing anything. A
single-platform directory such as `internal/adapter/platform/linux/` drops the
OS token and splits by backend (`system_x11_cgo.go`,
`system_wayland_wlroots_cgo.go`), while a mixed package carries it
(`internal/adapter/platform/factory_linux.go`).

## The Three Tiers

Before choosing a file, choose a **tier**. Every platform-varying capability in
Neru is expressed one of exactly three ways, and the deciding question is **who
needs the capability**:

| Tier                            | Use when                                                              | Mechanism                                                                  |
| ------------------------------- | --------------------------------------------------------------------- | -------------------------------------------------------------------------- |
| **1: Port**                     | app, domain, or more than one adapter package needs it                  | interface in `internal/ports`, adapter in `internal/adapter`        |
| **2: In-package dispatch**      | exactly one adapter package needs it                                    | build-tagged `platform_<os>.go` files, **unexported** functions             |
| **3: Optional port extension**  | only some platforms can offer it, and the caller has a real fallback  | interface **declared in `ports`**, reached by type assertion                |

### Tier 1: Port

The app and domain layers must never import an adapter package to reach an OS
capability. If they need it, it is a port. All four, or it is not done:

1. Interface in `internal/ports`, documented with what each platform is
   expected to do and what a caller must do when it cannot.
2. Adapter in `internal/adapter/<subsystem>/`, with
   `var _ ports.XPort = (*Adapter)(nil)`.
3. Mock in `internal/ports/mocks/`. Hand-rolled fakes in `_test.go` files rot
   silently when the contract changes; the shared mock does not.
4. An entry in `ports.PlatformCapabilities` so `neru doctor` reports it.

Current ports: `SystemPort`, `AccessibilityPort`, `OverlayPort`, `EventTapPort`,
`HotkeyPort`, `IPCPort`, `VisionPort`, `TextInputPort`, `KeyFeedPort`,
`AppWatcherPort`, `SystrayPort`, `FontResolver`, `TextMeasurer`.

Optional extensions (Tier 3): `RelativeCursorMover`, `CursorSynchronizer`
and `InstantCursorMover` on `SystemPort`, `HotkeyReleaseRegistrar` and
`HotkeyHealthReporter` on `HotkeyPort`, `OverlayKeyboardPassthroughReporter`
on `EventTapPort`, `OverlayCapabilityReporter` on `OverlayPort`, and
`SyntheticModifierSink` on the `tap.Tap` backend contract (Linux only, because
only X11 cannot tell its own injected key events apart from the user's).

[`keyfeed`](../../internal/adapter/keyfeed/) is the reference example: shared
normalization untagged in `keyfeed.go`, one unexported `postKey` per platform,
`Adapter` implementing the port, capability entry, mock, contract tests.

### Tier 2: In-package dispatch

A capability only one adapter package uses does **not** become a port. Wrapping
it in an interface buys no test seam and no substitutability. Use build-tagged
files inside that package with **unexported** functions:

```go
// platform_darwin.go
func platformActiveScreenBounds() image.Rectangle { /* Cocoa */ }

// platform_other.go
func platformActiveScreenBounds() image.Rectangle { return image.Rectangle{} }
```

Keeping them unexported is the whole point: an exported one becomes another
package's dependency, and the seam has quietly become a badly-specified port.
Examples: `accessibility/priming_*.go`, `appwatcher/platform_*.go`,
`ipc/transport_unix.go`.

### Tier 3: Optional port extension

Some platforms can do a job better than shared code can, but not all can do it
at all, so it cannot go on the base port without forcing every adapter to carry
a stub. Declare a small interface **in `ports`, next to the port it extends**,
and let the caller find it by type assertion:

```go
// ports/system.go
type RelativeCursorMover interface {
    MoveCursorBy(ctx context.Context, delta image.Point) (handled bool, err error)
}

// the caller always has a fallback
if mover, ok := s.system.(ports.RelativeCursorMover); ok { /* fast path */ }
```

Two rules: **declare it in `ports`**, since an interface defined in the
consuming package is undiscoverable to a contributor on another platform, and
**the caller must have a working fallback**, since an optional extension is an
optimization, never the only path. Adapters opting in assert it
(`var _ ports.RelativeCursorMover = (*SystemAdapter)(nil)`) so a signature
drift fails to compile instead of silently downgrading the platform.

### Not ports

Do not lift these behind interfaces: `platform/{darwin,linux,windows}`
internals, `wlr_protocol`, overlay drawing in `internal/adapter/overlay`,
`logger`, and IPC transport. They are implementation, reached through a port
that already exists.

### Dependency direction

The tiers only mean something if the arrows point one way. Three rules,
enforced by [layering_test.go](../../internal/architecture/layering_test.go):

| Rule                                                | Why |
| --------------------------------------------------- | --- |
| `internal/domain` imports no adapter, app, or UI | domain is pure Go; a domain package that needs an OS cannot be unit-tested |
| `internal/{domain,ports,derrors,adapter}` never import `internal/app` | adapters implement ports; the hexagon has no upward edges |
| app code reaches adapters only through ports        | only the composition root knows which adapter exists |

The third rule has three deliberate escapes, all narrow: **shared vocabulary**
(`adapter/ipc`, `adapter/logger`, and `adapter/platform` for the factory and
the `Profile` that `neru doctor` prints), the **composition root**
(`wiring.go`, `startup_phases.go`, `cmd/neru/main.go`), and **build-tagged
dispatch** files in the app layer, which are Tier 2. Anything else is a
violation. `knownLayeringExceptions` exists for edges that cannot be fixed in
the same change; it is **empty**, and a second test fails if an entry stops
being a real violation, so the list can only shrink.

## File Layout Rules

Once the tier is settled, the filename declares the slot. These rules are
enforced by
[platform_slots_test.go](../../internal/architecture/platform_slots_test.go), so
a violation fails `just test` rather than review:

| Suffix                            | Meaning                                                   |
| --------------------------------- | --------------------------------------------------------- |
| `*_darwin.go`                     | macOS                                                     |
| `*_windows.go`                    | Windows                                                   |
| `*_linux.go`                      | Linux, with no backend axis to split on                   |
| `*_other.go`                      | non-target fallback for dispatch-style packages           |
| `*_unix.go`                       | the `!windows` side of a split (established Go convention) |
| `*_<goarch>.go` + `*_other.go`    | an architecture split inside a one-platform directory: Go's own arch token on the file that needs it, `_other.go` for the rest (`platform/windows/overlay_dcomp_amd64.go`) |
| `*_linux_common.go`               | Linux-shared wrapper, fallback, or backend routing        |
| `*_linux_x11.go`                  | X11                                                       |
| `*_linux_wayland.go`              | Wayland                                                   |
| `*_linux_wayland_<compositor>.go` | one compositor family needing a distinct path             |
| `*_cgo.go` / `*_nocgo.go`         | CGO and pure-Go variants of the same slot                 |
| `*_integration_cgo.go`            | cgo scaffolding for an integration test, `//go:build … && integration` so it never ships |

Inside a package that is already one platform (`adapter/*/darwin`,
`adapter/*/linux`, `adapter/platform/windows`, and so on) the OS token is
dropped, because the directory carries it: `overlay/linux/wayland_cgo.go`, not
`overlay/linux/overlay_linux_wayland_cgo.go`. That is why the four Linux
backend rows hold no files today; they are the spelling to use if a mixed
package ever needs one. The `*_integration_cgo.go` row exists because Go
rejects `import "C"` in a `_test.go` file; the `integration` term keeps such a
file out of every product build, and the C stays inline in the cgo preamble.

What the guardrail test checks:

- A file constrained to exactly one GOOS must carry that OS as a name token.
- A file whose constraint is a pure negation is a fallback and must be named
  `*_other.go`. `_stub.go`, `_default.go`, `_fallback.go`, `_noop.go` and
  friends are rejected: one slot, one spelling.
- A file gated on cgo must say so: `*_cgo.go` or `*_nocgo.go`.
- Every file in a **single-platform package** declares its OS tag; the exempt
  set is derived from the tree, not listed.
- Every relative `#include` resolves
  ([cgo_includes_test.go](../../internal/architecture/cgo_includes_test.go)),
  which `go vet` and `just check-cross` cannot see under `CGO_ENABLED=0`.

Two rules that save review cycles: do not invent new ad hoc platform filenames
when a slot already exists, and do not create empty `darwin` / `linux` /
`windows` files for symmetry.

## Backend Packages

Every OS capability is a contract plus one directory per operating system,
each named for its GOOS:

```
adapter/accessibility/{ax, atspi, native/{darwin,linux,windows}}
adapter/eventtap/{tap, darwin, linux, windows}
adapter/hotkeys/{darwin, linux, windows}
adapter/systray/{darwin, linux, windows, icon}
adapter/overlay/{manager, darwin, linux, windows}
```

The directory names the platform, so the filenames inside do not have to, and
`ls` answers "what do I touch for Wayland?". The parent package keeps the port
adapter and a small build-tagged factory, the only place that knows which
implementation exists.

### When a backend does not earn a package

The test is whether every platform has something substantial to say. If one
does and the others answer in eighty-line stubs, build-tagged files in a single
package are clearer. That is the case for
`overlay/render/{grid,hints,recursivegrid,modeindicator,stickyindicator}`: each
is one real renderer plus small stubs, and `overlay_other.go` is the obvious
file to open.

### Giving a capability its own packages

Creating a backend package is three moves in order: find the seam (the methods
the shared shell calls on the platform type), extract that contract into a
**leaf** package (`accessibility/ax`, `eventtap/tap`; it must be a leaf, since
the backends import it and the factory imports the backends), then move each
platform into a package behind a build-tagged factory. When the shell talks to
package-level symbols rather than methods on a value, alias instead of
abstracting, as `accessibility/native` does. Two traps: named function types do
not interchange (put callback types in the contract package), and a factory
returning a concrete `*T` as an interface hands back a typed nil
(`staticcheck` SA4023).

### Where the render models live

`hints.Hint`, `grid.Style` and the other render models sit under
`adapter/overlay/render/` rather than in the domain, because each is one
concept every backend needs all of, and splitting them by layer produces two
packages named `hints`. Nothing above the overlay names them (#1213); the
per-mode `Context` types, which are mode state, live in
`internal/app/components/`. Each `Style` is declared once for every platform:
its fields hold what the configuration writes, and the packed-ARGB and float
forms Cairo and GDI want are accessors. When a type looks platform-specific,
check whether it differs in meaning or only in representation.

## Where To Implement What

| Capability                                                   | Primary location                                             |
| ------------------------------------------------------------ | ------------------------------------------------------------ |
| screen bounds, cursor, dark mode, notifications, permissions | `internal/adapter/platform/<os>/`                         |
| global hotkeys                                               | `internal/adapter/hotkeys/`                               |
| keyboard event capture                                       | `internal/adapter/eventtap/`                              |
| accessibility integration                                    | `internal/adapter/accessibility/` (`ax/`, `atspi/`, `native/`) |
| overlay window orchestration **and all Linux/Windows drawing** | `internal/adapter/overlay/`                             |
| overlay rendering by mode (**macOS only**; stubs elsewhere)  | `internal/adapter/overlay/render/*/overlay_*.go`          |
| app watcher and other isolated platform hooks                | dispatch-style `platform_*.go` in the relevant package       |

Worked examples: X11 hotkeys in
[x11_cgo.go](../../internal/adapter/hotkeys/linux/x11_cgo.go), Wayland keyboard
capture in [wayland_cgo.go](../../internal/adapter/eventtap/linux/wayland_cgo.go),
shared Linux system fallbacks in
[system_common.go](../../internal/adapter/platform/linux/system_common.go).

## Build And Test Commands

Every build and test recipe is catalogued in
[DEVELOPMENT.md](./development.md#common-tasks). What matters for platform
work:

- `just build && just test-foundation` is the cross-platform-safe baseline to
  run before touching anything. Only the target OS can run `just test`
  meaningfully, since integration tests are tagged per-OS.
- `just build-windows` cross-compiles from any host (CGO off). `just build-linux`
  does not: Linux needs CGO, and a macOS clang compiles the cgo runtime against
  the wrong SDK, so the recipe refuses when the compiler reports a non-Linux
  target. From macOS use `just check-cross` (fast CGO-off type-check of both
  other targets, no Docker) or `just lint-cross` (the linux/amd64 build with
  CGO on, in Docker). Tagged Linux release binaries are built by CI on a native
  runner.
- `just lint` only sees your own platform, because golangci-lint honours build
  tags. Reproduce Linux findings with
  `CGO_ENABLED=0 GOOS=linux golangci-lint run ./internal/...`, ignoring the
  `unused` and `unparam` reports that come only from the excluded `*_cgo.go`
  files; the cgo paths need `just lint-cross` or CI.

## Linux Backend Model

Linux is a backend *family*, not a single target. Keep two axes separate:

- **Compile-time axis (OS + CGO)**, expressed by build tags and file suffixes.
  Build tags cannot distinguish compositors: KDE and GNOME are both `linux` +
  Wayland at compile time, so a suffix never encodes a single desktop on its
  own.
- **Runtime axis (which compositor is live)**, expressed by the `LinuxBackend`
  family in [backend_linux.go](../../internal/adapter/platform/backend_linux.go),
  detected from environment variables and routed by `factory.go` plus dispatch
  seams such as `system_wayland_input.go`.

Within the compile-time axis, choose the slot by purpose:

| Slot      | Use for                                                                    |
| --------- | -------------------------------------------------------------------------- |
| `common`  | shared Linux types, shared fallbacks, backend detection/routing, helpers    |
| `x11`     | X11 display enumeration, event capture, overlays, pointer queries and warps |
| `wayland` | compositor capture/overlay behavior, layer-shell, output enumeration        |

Accessibility is the main exception: most Linux accessibility stays shared
around AT-SPI even where other subsystems split.

### Organize by mechanism, not by desktop

Desktop environments share mechanisms, so the axis that varies is usually the
mechanism:

- **Input**: KDE, COSMIC and GNOME all use libei (RemoteDesktop portal), and
  wlroots uses `zwlr_virtual_pointer`. One libei backend serves several DEs;
  the routing is a runtime probe, not a backend switch.
- **Screen capture**: KDE and COSMIC read the portal's ScreenCast stream, and
  wlroots uses `zwlr_screencopy`.
- **Overlay**: layer-shell works on KDE, wlroots, and COSMIC. GNOME/Mutter
  lacks it, and its overlay is the X11 backend unchanged, drawn on Xwayland.
- **Genuinely DE-specific**: active-window geometry and hotkey registration.
  These go in DE-named files, or in a DE-named package when more than one
  subsystem needs the same fact: `internal/adapter/platform/kwin` and
  `internal/adapter/platform/gnomeshell` each serve the AT-SPI window origin,
  `FocusedWindowBounds` and (for GNOME) the app watcher. What is shared across
  compositors goes in a package named for the mechanism:
  `internal/adapter/platform/compositorcli` is how both callers ask niri, Sway
  and Hyprland their question.

Use a `*_linux_wayland_<compositor>.go` sub-slot only when a compositor family
needs a path no other family shares, spelled without the OS token inside
`internal/adapter/platform/linux/`: `system_wayland_wlroots_*.go` and
`system_wayland_kde_*.go`, with `system_wayland_input.go` as the shared routing
seam.

**To add a compositor**: add a `LinuxBackend` value and detection in
`backend_linux.go`, route it in the factory and the relevant dispatch seams,
and add a new compositor sub-slot *only* if it cannot reuse an existing
mechanism file.

Per-DE decisions, measured protocol support, and known issues live in
[LINUX_DESKTOPS.md](../guide/linux-desktops.md); host setup lives in
[LINUX_SETUP.md](../guide/linux.md).

## Windows Model

Windows is one backend family, Beta because every mode works and what is there
is proven by CI rather than by use;
[What the labels mean](../reference/platform-support.md#what-the-labels-mean) carries the rule that moves it to
Stable. Prefer `*_windows.go` as the implementation slot and pure Go Win32 /
COM bindings (via `x/sys/windows` or syscall) over CGO. Do not introduce
additional Windows backend naming until there is a real reason.

**Elevated windows are out of reach.** User Interface Privilege Isolation stops
`SendInput` from reaching a window at a higher integrity level and withholds
its keystrokes from the `WH_KEYBOARD_LL` hook. While an elevated app has focus,
Neru still gets its global hotkeys and still moves the cursor, but the mode it
opens cannot see the keys typed into it and every click, scroll and injected
key is dropped. The remedy is to run Neru elevated too, which costs an
administrator token on a process that watches every keystroke, a hand-edited
service task, and a UAC prompt per manual launch. The secure desktop stays out
of reach either way.

**Smooth cursor animation on Windows** is the Linux animator's shape with
`SetCursorPos` as the sink
([mouse_animator.go](../../internal/adapter/platform/windows/mouse_animator.go)):
off by default, opt in with `smooth_cursor.move_mouse_enabled`, relative moves
extend the pending endpoint, and position-dependent actions settle the
animation before acting.

## CGO Guidance

**Do not decide CGO usage by OS alone.** CGO is a per-backend decision, and
[profile.go](../../internal/adapter/platform/profile.go) is the source of truth.
Current intent:

- **macOS**: CGO required throughout (Objective-C bridge)
- **Linux**: backend-dependent; several backends require it, and `*_nocgo.go`
  variants must still compile and degrade honestly
- **Windows**: pure Go first

Good default instincts: AT-SPI and freedesktop notifications prefer pure Go /
D-Bus; Wayland and compositor integrations often need CGO or native helpers;
Win32 hotkeys, hooks, monitor APIs, and UIA prefer pure Go bindings.

If you introduce a backend that changes the build story, update
[profile.go](../../internal/adapter/platform/profile.go), the
[justfile](../../justfile), and this document, and state the build assumption in
your PR description and the backend's package comments.

## Hotkeys And Modifiers

Shared code must not hard-code macOS conventions:

- use `Primary` when you mean "the main accelerator modifier"; it maps to
  `Cmd` on macOS and `Ctrl` on Linux/Windows
- keep backend-specific key translation inside `adapter/platform` code
- never leak X11, Wayland, Carbon, or Win32 naming into shared app logic

Relevant files: [config.go](../../internal/config/config.go),
[modifiers.go](../../internal/domain/action/modifiers.go),
[binder.go](../../internal/app/keybinding/binder.go).

On macOS, per-hotkey CGEventTaps are re-registered on keyboard-layout change
(via `NeruSetKeymapLayoutChangeCallback2`) because `NeruKeyNameToCode` maps key
names to layout-aware keycodes.

## Adding A New Capability

Start from [The Three Tiers](#the-three-tiers). The tier decides everything
below.

**Tier 1, extending an existing port** (a new OS operation the app needs, and a
port already covers that subsystem):

1. Add the method to the port, documenting what each platform should do
2. Implement it in the darwin adapter
3. Add a Linux shared fallback in `system_common.go`
4. Add a Windows implementation or explicit `CodeNotSupported` stub
5. Push backend-specific Linux behavior down into `system_x11_cgo.go` or
   `system_wayland.go`
6. Add the method to the mock in `internal/ports/mocks/`
7. Update capability reporting if the support surface changed

**Tier 1, a whole new port**: everything above, plus a new
`internal/ports/<name>.go`, an adapter package under `internal/adapter/`, a
`PlatformCapabilities` field **and** its `Entries()` registration, and wiring in
`startup_phases.go`. Copy the shape of
[`keyfeed`](../../internal/adapter/keyfeed/).

**Tier 2, one adapter package only**: keep the shared package code
platform-agnostic, use `platform_darwin.go` / `platform_other.go` dispatch
files with unexported functions, and add Linux backend files inside that
package rather than pushing detection up into shared app or service code.

**Tier 3, an optional extension**: declare the interface in `ports` beside the
port it extends, implement it on the adapters that can, assert compliance with
`var _ ports.X = (*SystemAdapter)(nil)`, and give the caller a fallback.

### Adding a capability to `neru doctor`

`PlatformCapabilities` is a registry, not just a struct. Add the field, add a
`CapabilityKey` constant, and register the pair in `Entries()`. Every renderer
(`neru doctor`, the IPC info map) iterates `Entries()`, so that is the only
edit, and [capabilities_test.go](../../internal/ports/capabilities_test.go) fails
if a field is added without registering it. Then fill the entry in all three
presets in [capability_presets.go](../../internal/ports/capability_presets.go).

## Errors And Capability Reporting

Unimplemented platform behavior returns `CodeNotSupported`, never a silent
no-op, unless the behavior is explicitly documented as best-effort:

```go
return derrors.New(derrors.CodeNotSupported, "ScreenBounds not yet implemented on linux")
```

Name the missing operation and the platform in the message. Callers degrade
via `derrors.IsNotSupported(err)`.

**A word is not the same question as a subsystem.** When the thing you shipped
or stubbed is an option, a mode flag or an action rather than a capability, the
answer goes in the `PlatformSupport()` declaration beside that vocabulary
(`internal/config/platform_support.go`,
`internal/domain/modecmd/platform_support.go`,
`internal/domain/action/platform_support.go`), and
`internal/architecture/platform_support_test.go` fails the build while a word
has no column. Regenerate the published table with `just gensupportref`.

Capability reporting is part of the contract, since it is what `neru doctor`
prints. When you implement or partially implement a feature, review
[capabilities.go](../../internal/ports/capabilities.go),
[capability_presets.go](../../internal/ports/capability_presets.go), and
[info.go](../../internal/app/ipcctrl/info.go). A stub must report `stub`, not
`supported`, and a shipped feature must stop reporting `stub`.

## Testing Checklist

- **unit tests** for shared parsing, normalization, routing, or config logic
  (`*_test.go`, using mocks from `internal/ports/mocks`)
- **contract tests** pinning `CodeNotSupported` behavior and capability
  semantics
- **integration tests** for real platform behavior, tagged per-OS
  (`*_integration_linux_test.go`, `*_integration_darwin_test.go`,
  `*_integration_windows_test.go`)

Questions your tests should answer: does the adapter return the right error
when the feature is unsupported? Does the capability matrix reflect the new
state? Does backend selection route to the intended Linux slot? Does shared
logic stay platform-neutral?

## Documentation Checklist

Land docs in the same PR as the platform work. Each fact has exactly one home.
Update the one that owns it rather than restating it elsewhere:

| What changed                                    | Update                                                        |
| ----------------------------------------------- | ------------------------------------------------------------- |
| A capability's status or mechanism              | **this file**, the parity tables in Part 1                    |
| A gap closed or discovered                      | **this file**, [Known Gaps](../reference/platform-support.md#known-gaps)                      |
| Which platforms an option, mode flag or action does anything on | the `PlatformSupport()` declaration beside that vocabulary, then `just gensupportref` |
| Desktop-specific setup, protocol support, or a DE workaround | [LINUX_DESKTOPS.md](../guide/linux-desktops.md)         |
| Host dependencies, permissions, or deployment   | [LINUX_SETUP.md](../guide/linux.md), kept DE-agnostic          |
| A layer boundary, port contract, or data flow   | [ARCHITECTURE.md](./architecture.md)                          |
| A build recipe or test tier                     | [DEVELOPMENT.md](./development.md)                            |
| Go style, logging, or naming                    | [AGENTS.md](../../AGENTS.md) (Conventions)                       |
| What the project claims to support, at a glance | [README.md](../../README.md)                                     |
| What comes next                                 | [ROADMAP.md](../project/roadmap.md), intent and priority only          |

ARCHITECTURE.md deliberately does **not** track per-platform support. It
describes shape, not status. Do not add a capability table there.

## Contributing Safely

**Good starter tasks:**

- improve capability detail text for an existing platform slice
- add a contract test for a currently stubbed feature
- reproduce and fix a bug labelled `platform: linux` or `platform: windows`
- document missing backend assumptions in the package you are touching

**Higher-risk, open or link an issue first:**

- changing shared input semantics
- introducing CGO to a backend that was previously pure Go
- moving shared logic into platform packages
- mixing backend detection into app or service code

**A good platform PR** leaves the repo better in five ways: the implementation
sits in the intended file slot, unsupported paths stay explicit and honest,
capability reporting is updated, tests cover the new behavior or contract, and
the docs tell the next contributor what changed. That is the bar even for
small slices.
