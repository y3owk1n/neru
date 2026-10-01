# Platform porting guide

Where platform code goes and what a platform change must leave behind. Layers
are in [architecture](architecture.md), and per-platform status in
[platform support](../reference/platform-support.md).

## First stops

Read these before changing platform code:

- [The Three Tiers](#the-three-tiers): decides where your code goes
- [platform/profile.go](../../internal/adapter/platform/profile.go): per-subsystem backend family and CGO expectations
- [ports/system.go](../../internal/ports/system.go): the main OS contract, plus the optional-extension pattern
- [ports/capabilities.go](../../internal/ports/capabilities.go) and [capability_presets.go](../../internal/ports/capability_presets.go): the capability registry `neru doctor` reports
- the package's existing files for your platform (there are no placeholders)

## The Three Tiers

A platform-varying capability takes one of three forms, by **who needs it**:

| Tier                            | Use when                                                              | Mechanism                                                                  |
| ------------------------------- | --------------------------------------------------------------------- | -------------------------------------------------------------------------- |
| **1: Port**                     | app, domain, or more than one adapter package needs it                  | interface in `internal/ports`, adapter in `internal/adapter`        |
| **2: In-package dispatch**      | exactly one adapter package needs it                                    | build-tagged `platform_<os>.go` files, **unexported** functions             |
| **3: Optional port extension**  | only some platforms can offer it, and the caller has a real fallback  | interface **declared in `ports`**, reached by type assertion                |

### Tier 1: Port

The app and domain layers never import an adapter package to reach an OS
capability. A port is done only with all four:

1. An interface in `internal/ports`, documenting what each platform does and
   what a caller does when it cannot
2. An adapter in `internal/adapter/<subsystem>/` with `var _ ports.XPort = (*Adapter)(nil)`
3. A mock in `internal/ports/mocks/`, not a hand-rolled fake in a `_test.go`
4. An entry in `ports.PlatformCapabilities` so `neru doctor` reports it

[`keyfeed`](../../internal/adapter/keyfeed/) is the reference example: shared
normalization untagged in `keyfeed.go`, one unexported `postKey` per platform,
`Adapter` implementing the port, capability entry, mock, contract tests.

### Tier 2: In-package dispatch

A capability only one adapter package uses is not a port. Put an
**unexported** function such as `platformActiveScreenBounds` in
`platform_darwin.go`, `platform_other.go` and so on, because an exported one
becomes another package's dependency and so a badly specified port. Examples:
`accessibility/priming_*.go`, `appwatcher/platform_*.go`,
`ipc/transport_unix.go`.

### Tier 3: Optional port extension

When only some platforms can do a job better than shared code, declare a small
interface **in `ports`, next to the port it extends**, such as
`RelativeCursorMover` in `ports/system.go`. The caller finds it with a type
assertion and keeps a working fallback. Adapters that opt in assert it
(`var _ ports.RelativeCursorMover = (*SystemAdapter)(nil)`) so a signature
drift fails to compile instead of silently downgrading the platform. The one
extension off `ports` is `SyntheticModifierSink` on the Linux `tap.Tap`
backend contract, because only X11 cannot tell its own injected key events
from the user's.

### Not ports

Do not lift these behind interfaces: `platform/{darwin,linux,windows}`
internals, `wlr_protocol`, overlay drawing in `internal/adapter/overlay`,
`logger`, and IPC transport. A port that already exists reaches them.

### Dependency direction

[layering_test.go](../../internal/architecture/layering_test.go) enforces three
rules:

| Rule                                                | Why |
| --------------------------------------------------- | --- |
| `internal/domain` imports no adapter, app, or UI | a domain package that needs an OS cannot be unit-tested |
| `internal/{domain,ports,derrors,adapter}` never import `internal/app` | adapters implement ports, and no dependency points upward |
| app code reaches adapters only through ports        | only the composition root knows which adapter exists |

The third rule exempts shared vocabulary (`adapter/ipc`, `adapter/logger`, and
`adapter/platform` for the factory and `Profile`), the composition root
(`wiring.go`, `startup_phases.go`, `cmd/neru/main.go`), and Tier 2 dispatch
files in the app layer. `knownLayeringExceptions` is **empty**, and a second
test fails if an entry stops being a real violation.

## File Layout Rules

The filename declares the slot.
[platform_slots_test.go](../../internal/architecture/platform_slots_test.go)
enforces these, so a violation fails `just test`:

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

Inside a one-platform package (`adapter/*/darwin`, `adapter/*/linux`,
`adapter/platform/windows`) drop the OS token, because the directory carries
it: `overlay/linux/wayland_cgo.go`, not
`overlay/linux/overlay_linux_wayland_cgo.go`. The four Linux backend rows are
the spelling for a mixed package, and none uses them today. The
`*_integration_cgo.go` row exists because Go rejects `import "C"` in a
`_test.go` file.

The guardrail rejects `_stub.go`, `_default.go`, `_fallback.go`, `_noop.go`
and similar names, and requires an OS tag on every file in a single-platform
package. [cgo_includes_test.go](../../internal/architecture/cgo_includes_test.go)
checks that every relative `#include` resolves. Never add empty platform files.

## Backend packages

Every OS capability is a contract plus one directory per GOOS:

```
adapter/accessibility/{ax, atspi, native/{darwin,linux,windows}}
adapter/eventtap/{tap, darwin, linux, windows}
adapter/hotkeys/{darwin, linux, windows}
adapter/systray/{darwin, linux, windows, icon}
adapter/overlay/{manager, darwin, linux, windows}
```

The parent package keeps the port adapter and a small build-tagged factory,
the only place that knows which implementation exists. A backend earns a
package only when every platform has a substantial implementation. One real
renderer plus small stubs stays as build-tagged files, as in
`overlay/render/{grid,hints,recursivegrid,modeindicator,stickyindicator}`.

To split a capability, extract the methods the shell calls into a leaf
contract package (`accessibility/ax`, `eventtap/tap`) and move each platform
behind a build-tagged factory. Alias package-level symbols instead, as
`accessibility/native` does. Callback types go in the contract package, since
named function types do not interchange. A factory returning a concrete `*T`
as an interface returns a typed nil (`staticcheck` SA4023).

Render models (`hints.Hint`, `grid.Style`) live in `adapter/overlay/render/`,
declared once for all platforms with the Cairo and GDI forms as accessors.
Per-mode `Context` types live in `internal/app/components/`.

## Where to implement what

| Capability                                                   | Primary location                                             |
| ------------------------------------------------------------ | ------------------------------------------------------------ |
| screen bounds, cursor, dark mode, notifications, permissions | `internal/adapter/platform/<os>/`                         |
| global hotkeys                                               | `internal/adapter/hotkeys/`                               |
| keyboard event capture                                       | `internal/adapter/eventtap/`                              |
| accessibility integration                                    | `internal/adapter/accessibility/` (`ax/`, `atspi/`, `native/`) |
| overlay window orchestration **and all Linux/Windows drawing** | `internal/adapter/overlay/`                             |
| overlay rendering by mode (macOS only, stubs elsewhere)  | `internal/adapter/overlay/render/*/overlay_*.go`          |
| app watcher and other isolated platform hooks                | dispatch-style `platform_*.go` in the relevant package       |

Worked examples: [x11_cgo.go](../../internal/adapter/hotkeys/linux/x11_cgo.go),
[wayland_cgo.go](../../internal/adapter/eventtap/linux/wayland_cgo.go),
[system_common.go](../../internal/adapter/platform/linux/system_common.go).

## Build and test commands

Run `just build && just test-foundation` first. Host and target pairs are in
[Cross-platform checks](development.md#cross-platform-checks), and Linux
dependencies in [Building on Linux](development.md#building-on-linux).

## Linux backend model

Linux has two separate axes:

- **Compile time (OS + CGO)**: build tags and file suffixes. KDE and GNOME are
  both `linux` + Wayland at compile time, so a suffix never encodes one desktop.
  The `common` slot holds shared types, fallbacks and routing, `x11` the X11
  code, and `wayland` compositor capture, overlay, layer-shell and outputs.
- **Run time (which compositor is live)**: the `LinuxBackend` family in
  [backend_linux.go](../../internal/adapter/platform/backend_linux.go),
  detected from environment variables and routed by `factory.go` and dispatch
  seams such as `system_wayland_input.go`.

Most Linux accessibility stays shared around AT-SPI even where other subsystems split.

### Organize by mechanism, not by desktop

Desktops share mechanisms, so split code by mechanism:

- **Input**: KDE, COSMIC and GNOME use libei (RemoteDesktop portal), and
  wlroots uses `zwlr_virtual_pointer`. A runtime probe routes between them.
- **Screen capture**: KDE and COSMIC read the portal's ScreenCast stream, and
  wlroots uses `zwlr_screencopy`.
- **Overlay**: layer-shell works on KDE, wlroots and COSMIC. GNOME lacks it,
  so its overlay is the X11 backend on Xwayland.
- **DE-specific**: active-window geometry and hotkey registration go in
  DE-named files, or a DE-named package (`platform/kwin`, `platform/gnomeshell`)
  when several subsystems need the same fact. A mechanism shared across
  compositors gets a package named for it, such as `platform/compositorcli` for
  niri, Sway and Hyprland.

**To add a compositor**: add a `LinuxBackend` value and detection in
`backend_linux.go`, then route it in the factory and the dispatch seams. Add a
sub-slot (`system_wayland_wlroots_*.go`, `system_wayland_kde_*.go`) *only* for
a path no existing mechanism file fits. Per-desktop setup is in
[Linux desktops](../guide/linux-desktops.md).

## Windows model

Windows is one backend family. Use `*_windows.go` and pure Go Win32 and COM
bindings (`x/sys/windows` or syscall), and add no further Windows backend
naming without a real need. User Interface Privilege Isolation keeps elevated
windows out of reach of `SendInput` and the `WH_KEYBOARD_LL` hook
([Known Gaps](../reference/platform-support.md#known-gaps)). Smooth cursor
animation ([mouse_animator.go](../../internal/adapter/platform/windows/mouse_animator.go))
reuses the Linux animator's shape with `SetCursorPos` as the sink, and stays
off unless `smooth_cursor.move_mouse_enabled` is set.

## CGO guidance

CGO is a per-backend decision, and
[profile.go](../../internal/adapter/platform/profile.go) is the source of truth:

- **macOS**: CGO throughout (Objective-C bridge).
- **Linux**: per backend. Wayland and compositor integrations often need CGO,
  and AT-SPI and notifications prefer pure Go over D-Bus. `*_nocgo.go`
  variants must still compile and report what they cannot do.
- **Windows**: pure Go bindings for hotkeys, hooks, monitor APIs and UIA.

A backend that changes how Neru builds updates profile.go, the
[justfile](../../justfile), this guide, the PR description and package comments.

## Hotkeys and modifiers

Shared code uses `Primary` for the main accelerator modifier (`Cmd` on macOS,
`Ctrl` elsewhere), keeps key translation inside `adapter/platform`, and never
names X11, Wayland, Carbon or Win32 concepts
([modifiers.go](../../internal/domain/action/modifiers.go),
[binder.go](../../internal/app/keybinding/binder.go),
[config.go](../../internal/config/config.go)). macOS layout changes are in the
[Codebase navigation guide](architecture.md#codebase-navigation-guide).

## Adding a new capability

**Tier 1, extending an existing port**:

1. Add the method to the port, documenting what each platform should do
2. Implement it in the darwin adapter
3. Add a Linux shared fallback in `system_common.go`, and push
   backend-specific behavior into `system_x11_cgo.go` or `system_wayland.go`
4. Add a Windows implementation or an explicit `CodeNotSupported` stub
5. Add the method to the mock in `internal/ports/mocks/`
6. Update capability reporting if platform support changed

**Tier 1, a new port**: all of the above, plus `internal/ports/<name>.go`, an
adapter package, a [doctor entry](#adding-a-capability-to-neru-doctor), and
wiring in `startup_phases.go`. Copy [`keyfeed`](../../internal/adapter/keyfeed/).

### Adding a capability to `neru doctor`

Add the `PlatformCapabilities` field and a `CapabilityKey` constant, register
the pair in `Entries()`, which every renderer iterates, and fill all three
presets in [capability_presets.go](../../internal/ports/capability_presets.go).
[capabilities_test.go](../../internal/ports/capabilities_test.go) fails on an
unregistered field.

## Errors and capability reporting

Unimplemented behavior returns `CodeNotSupported`
([policy](architecture.md#the-codenotsupported-policy)). Keep `capabilities.go`,
`capability_presets.go` and [info.go](../../internal/app/ipcctrl/info.go) true.

Options, flags and actions declare their platforms in `PlatformSupport()`
beside their vocabulary (`internal/config`, `internal/domain/modecmd`,
`internal/domain/action`, each in `platform_support.go`), then run
`just gensupportref`. `internal/architecture/platform_support_test.go` fails
while a word has no column.

## Testing checklist

[Testing](development.md#testing) lists the tiers. A platform change's tests
show that the adapter returns `CodeNotSupported` where unsupported, the
capability matrix reflects the new state, backend selection routes to the
intended Linux slot, and shared logic stays platform-neutral.

## Documentation checklist

Land docs in the same PR. Each fact has one home. Update it there, and link to
it from everywhere else rather than restating it. A list, path, default, value
or procedure is never copied. Two things may be restated in passing: a term's
short gloss where it is first used (such as "`Primary` is `Cmd` on macOS"), and
the README's pitch, install command and showcase examples, since it is the
landing page. The homes:

| What changed | Owner |
| --- | --- |
| A capability's status, label or user-visible limit, a platform exclusive | [platform support](../reference/platform-support.md) |
| The API, protocol or mechanism behind a capability | [platform internals](platform-internals.md) |
| A gap closed or discovered | [Known Gaps](../reference/platform-support.md#known-gaps) |
| Which platforms an option, mode flag or action does anything on | the `PlatformSupport()` declaration beside that vocabulary, then `just gensupportref` |
| A config option, its default or its platform column | [configuration reference](../reference/configuration.md) |
| A command or flag | [CLI reference](../reference/cli.md), with mode flags through `just genflagref` |
| Install methods, prebuilt binaries, from source, uninstall | [installation](../guide/installation.md) |
| First run, per-OS permissions, first hotkey | [getting started](../guide/getting-started.md) |
| How each mode is used, from the user's side | [using Neru](../guide/using-neru.md) |
| Config location, layering, reload, `config set` | [configuring Neru](../guide/configuring.md) |
| What a binding step can be, which binding wins, sequence failure | [how bindings work](../concepts/bindings.md) |
| A user-facing term's definition | [glossary](../concepts/glossary.md), worded to match `CONTEXT.md` |
| A workflow recipe | [recipes](../guide/recipes.md) |
| Scripting examples / the IPC wire format | [scripting](../guide/scripting.md) / [IPC protocol](../reference/ipc.md) |
| Linux runtime libraries, input group, uinput, portals, systemd | [Linux guide](../guide/linux.md), kept desktop-agnostic |
| Desktop-specific setup, protocol support, or a desktop workaround | [Linux desktops](../guide/linux-desktops.md) |
| A symptom and its fix, log locations | [troubleshooting](../guide/troubleshooting.md) |
| Layer boundaries, port contracts, data flow, `CodeNotSupported` policy, overlay split, coordinates | [architecture](architecture.md), shape only, never status |
| File layout, tiers, backend packages, porting checklists | this guide |
| Build recipes, Linux build dependencies, test tiers, `just ci` contents, docs site, release | [development guide](development.md) |
| Contribution process, commits, PRs, starter tasks | [CONTRIBUTING.md](../../CONTRIBUTING.md) |
| Go style, logging, naming | [AGENTS.md](../../AGENTS.md) (Conventions) |
| What comes next | [roadmap](../project/roadmap.md), intent and priority only |

## Contributing safely

Starter tasks are in [Good first contributions](../../CONTRIBUTING.md#good-first-contributions).

**Open or link an issue first** before changing shared input semantics,
introducing CGO to a pure-Go backend, moving shared logic into platform
packages, or mixing backend detection into app or service code.

A platform PR, even a small one, puts the code in the intended file slot,
returns an explicit error on unsupported paths, updates capability reporting,
tests the new behavior or contract, and updates the docs that own what changed.
