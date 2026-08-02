# Restructure Plan

Where the codebase layout is heading, why, and in what order.

This document owns the **layout migration**: the structural problems that remain,
the target tree, and the sequence for getting there. It is temporary — once the
last step lands, this file is deleted and its conclusions live in
[ARCHITECTURE.md](ARCHITECTURE.md) (shape) and
[CROSS_PLATFORM.md](CROSS_PLATFORM.md) (platform work).

**Related:** [Architecture](ARCHITECTURE.md) ·
[Cross-Platform Guide](CROSS_PLATFORM.md) ·
[Development Guide](DEVELOPMENT.md) · [Coding Standards](CODING_STANDARDS.md)

---

## Table of Contents

- [Goal](#goal)
- [What Is Not Changing](#what-is-not-changing)
- [The Four Problems](#the-four-problems)
- [Target Layout](#target-layout)
- [Sequence](#sequence)
- [Rules for Every Step](#rules-for-every-step)

---

## Goal

Two outcomes, in priority order:

1. **Platform work should be obvious.** Adding a compositor, a backend, or a
   whole OS should be "make a package, add a line to the factory" — not "learn
   the filename suffix grammar, then edit six files in a directory where 45 of
   the 54 are about a different operating system."
2. **A new contributor should be able to navigate by `ls`.** The directory tree
   should describe the work, not the architecture diagram.

Everything below serves those two sentences. A change that does not is churn.

---

## What Is Not Changing

Stated up front, because a restructure that touches everything is a restructure
nobody can review.

- **The hexagonal model.** Ports and adapters stay. Only their *names and
  depth* change.
- **The architecture guardrail tests** (`internal/architecture/`). These are the
  reason a restructure this size is safe at all. They get path updates, never
  relaxations.
- **The native bridges** — `platform/darwin`, `platform/linux`,
  `platform/windows`. These are cgo compilation units; headers and symbols want
  to share a package. Splitting them would duplicate cgo flags for no gain.
- **`internal/` + `cmd/`.** We are not adopting `golang-standards/project-layout`
  (`pkg/`, `api/`, `configs/`). It is not an official standard, it is aimed at
  libraries, and Neru ships a binary.
- **Behavior.** Every step is a refactor. If a step changes what the program
  does, the step was done wrong.

---

## The Four Problems

### 1. `internal/core/` is a load-bearing nothing

`internal/core/` holds ~55k of ~76k non-test lines. A path segment that contains
almost everything discriminates nothing. Paired with `infra/`, it produces:

```
internal/core/infra/overlay/render/overlayutil/native/   # 7 segments
```

`core` / `infra` / `domain` is DDD vocabulary. Our own docs say "ports and
adapters", but the tree says `ports` and `infra`. Make them agree.

### 2. `internal/app` is a 72-file, 19k-line package

One directory holds the composition root, the IPC controller and ~15
`ipc_*.go` handler files, hotkeys, lifecycle, theme observers, the sequence
engine, sleep handlers, and layout-change hooks. `*App` carries **119 methods**.

The good news: it is flat, not tangled. `IPCController` already takes an
`IPCControllerDeps` struct rather than `*App`, and `interfaces.go` already
defines `ModeService`. The pieces are extractable today — they are cohabiting,
not fused.

### 3. Backends are filename suffixes instead of packages

`internal/core/infra/accessibility/` is **54 files in one package**: macOS AX,
Linux AT-SPI, X11, Wayland, plus Hyprland / KWin / niri / Sway geometry, plus
Windows UIA. `overlay/` is 20 files and 13.5k lines across the same span;
`eventtap/` is 19.

There are **231 build-tagged files**, using filename suffixes Go itself does not
understand — `_linux_wayland_wlroots_cgo.go`, `_linux_wayland_evdev_cgo.go` —
held together by explicit `//go:build` lines and a 516-line
`platform_slots_test.go` policing the naming convention.

That test does heroic work to preserve a convention that a package boundary
would enforce for free.

### 4. Render models live under an adapter

`layering_test.go` records it: every overlay backend imported
`internal/app/components` for its render models, and the fix was to move those
packages *down* into `internal/core/infra/overlay/render/`. So `hints.Hint` and
`grid.Style` now live under an adapter directory, imported by both the app and
adapter layers, which required carving out `sharedInfraPackages` to permit.

**This is not a layout problem.** Moving the packages was the first instinct and
it does not work, because the types are not all platform-neutral:

| Symbol | Where declared | Neutral? |
| ------ | -------------- | -------- |
| `hints.Hint`, `hints.SearchInput*` | `hints/types.go` | yes |
| `hints.Context`, `grid.Context` | `*/context.go` | yes |
| `hints.StyleMode`, `grid.Style` | `overlay_darwin.go` **and** `overlay_linux_common.go` **and** `overlay_windows.go` | **no** — three build-tagged declarations |
| `hints.BuildStyle`, `grid.BuildStyle` | same three files | **no** |

The app layer names `grid.Style` and `hints.StyleMode` by value, and those are
per-platform types. A domain package cannot hold three mutually-exclusive
build-tagged definitions of one type without dragging platform tags into the
domain, which is precisely what the domain layer exists to avoid.

So `sharedInfraPackages` is load-bearing, not laziness. Removing it needs the
per-platform `Style`/`StyleMode` types unified behind one neutral type, with the
platform-specific fields converted adapter-side — a redesign with real behavior
risk, not a move. The neutral half (`Hint`, `Context`, `SearchInput*`) *could*
move on its own, but that splits each package in two, forces an import alias at
every site that touches both halves, and still leaves the exception in place.

**Verdict: deferred.** Do it as a type-design change when the styling code is
being touched anyway, not as part of the layout migration.

---

## Target Layout

```
cmd/neru/                      entry point
internal/
  domain/                      pure logic — was core/domain
    action/ element/ grid/ hint/ recursivegrid/ state/
    render/                    render models — was adapter/overlay/render
      hints/ grid/ recursivegrid/ modeindicator/ stickyindicator/ virtualpointer/
  ports/                       the cross-platform contract — was core/ports
  errors/                      derrors — was core/errors
  adapter/                     was core/infra
    accessibility/
      accessibility.go         shared, platform-neutral adapter
      factory_darwin.go        small
      factory_linux.go         small — runtime compositor detection
      factory_windows.go       small
      macos/                   no build tags inside; the directory IS the tag
      atspi/
      x11/
      wayland/wlroots/ wayland/gnome/ wayland/kde/
      uia/
    overlay/  eventtap/  hotkeys/  systray/  vision/  ...   same shape
    ipc/  logger/  apptrace/
  platform/                    native cgo bridges — unchanged
    darwin/  linux/  windows/  mousestate/
  daemon/                      composition root + lifecycle — was app/
  ipc/                         controller + handlers — was app/ipc_*.go
  hotkey/                      was app/hotkeys*.go
  sequence/                    was app/sequence*.go
  mode/                        was app/modes
  cli/  config/  ui/  buildinfo/  architecture/
```

Path depth drops, `ls` becomes informative, and build tags collapse from 231
files to a handful of factory files.

---

## Sequence

Each step is independently shippable with the full suite green. Steps 2 and 3
are mechanical and low-risk; step 6 is the one that actually delivers the goal.

| # | Step | Status | Notes |
| - | ---- | ------ | ----- |
| 1 | `govulncheck` + coverage in CI | **done** | `-race` and `-trimpath` were already covered |
| 2 | Flatten `core/`, rename `infra/` → `adapter/` | **done** | Pure move + import rewrite |
| 3 | Render models → `internal/domain/render/` | **deferred** | Not a move — needs the per-platform `Style` types unified first ([why](#4-render-models-live-under-an-adapter)) |
| 4 | Split `internal/app` → `ipcctrl`/`sequence` | **done** | 72 files → 45 |
| 5 | Shrink `*App` to consumer-defined interfaces | **done for hotkeys** | 119 → 106 methods; ongoing beyond that |
| 6 | Backends as packages: accessibility → overlay → eventtap | | The cross-platform payoff |

Step 2 also retired the word "infra" from the docs, dropped the empty
`internal/core` doc-only package, and renamed `core/errors` to `internal/derrors`
so the directory and the package name (`derrors`) finally agree.

**On step 4:** `internal/app` is down from 72 files to 48. Two packages came out:

- `internal/app/ipcctrl` — the command handlers. It came out cheaply because
  `IPCController` already took a `Deps` struct instead of reaching for the App,
  so the only edges to cut were four identifiers. Types dropped their
  `IPCController` prefix (`ipcctrl.Controller`, `ipcctrl.Deps`,
  `ipcctrl.NewModesHandler`), which the old flat package could not have.
- `internal/app/sequence` — the parts of an action sequence that do not need
  the App: nesting depth, failure policy, outcome. The executor itself stays in
  `internal/app` because running a step means dispatching it through the App.

**On step 5.** `hotkeys.go` could not come out with the rest of step 4: it hangs
off `*App` for dispatch. What blocked it was a cycle — the sequence executor's
`executeStep` called `executeHotkeyAction`, and hotkeys called back into
`runActionSequence`. They were one unit pretending to be two.

Breaking it took two moves:

1. `sequence.Executor` absorbed the executor half. Its five dependencies are all
   functions or one-method interfaces (`CommandHandler` is just
   `HandleCommand`), so the executor drives modes and IPC without importing
   either.
2. `internal/app/hotkey` then took the binding half, with the repeat
   bookkeeping — two mutexes, a cancel table and a cached bundle identifier —
   that had been sitting on `*App` where nothing else touched it.

The interfaces `hotkey` needs are declared at the consumer and are two or three
methods each: `ModeBindings` (2), `EnabledState` (3), `FocusedApp` (1). That is
the whole point — the binder names what it calls, not the components it calls.

`*App` is down from 119 methods to 106. The remainder is mostly the systray's
`AppInterface`, which is legitimately wide because the menu really does toggle
all of those things; narrowing it further is a menu redesign, not a refactor.

`internal/ipc` in the target tree above is therefore `internal/app/ipcctrl`
today: a package name of `ipc` would collide with `internal/adapter/ipc`, which
every one of these files imports for `ipc.Command` and `ipc.Response`. A
per-file import alias is a worse tax than a slightly longer package name.

**On step 6:** do `accessibility` alone first and live with it for a release
before touching `overlay` and `eventtap`. All three at once is a merge-conflict
nightmare for anyone with an open branch, and accessibility is the one where the
per-OS divergence is worst. Once it lands, most of `platform_slots_test.go` can
be deleted — package boundaries enforce what the filename convention currently
only checks.

---

## Rules for Every Step

1. **No behavior changes inside a move.** If a rename reveals a bug, fix it in a
   separate commit before or after, never inside the move.
2. **Guardrail tests get path updates, never exemptions.** If a step needs a new
   entry in `knownLayeringExceptions`, the step is wrong.
3. **Update the docs in the same change.** The
   [documentation checklist](CROSS_PLATFORM.md#documentation-checklist) says
   which file owns which fact.
4. **Check the non-Go references.** Moves break more than imports: `justfile`
   hardcodes `internal/core/infra` in `fmt`, `fmt-check`, `test-foundation`, and
   `WLR_PROTOCOL_DIR`; `.golangci.yml` `depguard` names package paths; the
   architecture tests carry path prefixes as string constants.
5. **`just fmt && just lint && just test && just build` before every commit**,
   plus `just check-cross` for anything touching platform slots.
