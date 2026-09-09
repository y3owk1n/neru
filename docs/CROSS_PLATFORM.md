# Cross-Platform Guide

Neru runs on macOS, Linux, and Windows from one shared Go core. This document
covers both sides of that:

- **[Part 1: Feature Parity Reference](#feature-parity-reference)**: what
  works on each platform, and how it is implemented.
- **[Part 2: Contributor Guide](#contributor-guide)**: where platform code
  lives, and how to add to it.

Every claim in Part 1 is derived from code under `internal/adapter/` and
`internal/app/`. **If this document and the code disagree, the code wins**,
and the disagreement is a bug worth fixing here. Where a row or footnote names
a file, test or ADR, that is where the full mechanism is written down; this
document states the claim and points there.

**Related:** [Architecture](./ARCHITECTURE.md) · [Linux setup](./LINUX_SETUP.md) ·
[Linux desktops](./LINUX_DESKTOPS.md) · [Development Guide](./DEVELOPMENT.md)

---

## Table of Contents

**Part 1: [Feature Parity Reference](#feature-parity-reference)**

- [Platform Status](#platform-status)
- [Capability Matrix](#capability-matrix)
- [Input Injection](#input-injection)
- [Keyboard Capture And Hotkeys](#keyboard-capture-and-hotkeys)
- [Accessibility And Hints](#accessibility-and-hints)
- [Overlay Rendering](#overlay-rendering)
- [Mode Coverage](#mode-coverage)
- [Platform Support Per Word](#platform-support-per-word)
- [Platform Exclusives](#platform-exclusives)
- [Known Gaps](#known-gaps)

**Part 2: [Contributor Guide](#contributor-guide)**

- [First Stops](#first-stops)
- [The Three Tiers](#the-three-tiers)
- [File Layout Rules](#file-layout-rules)
- [Backend Packages](#backend-packages)
- [Where To Implement What](#where-to-implement-what)
- [Build And Test Commands](#build-and-test-commands)
- [Linux Backend Model](#linux-backend-model)
- [Windows Model](#windows-model)
- [CGO Guidance](#cgo-guidance)
- [Hotkeys And Modifiers](#hotkeys-and-modifiers)
- [Adding A New Capability](#adding-a-new-capability)
- [Errors And Capability Reporting](#errors-and-capability-reporting)
- [Testing Checklist](#testing-checklist)
- [Documentation Checklist](#documentation-checklist)
- [Contributing Safely](#contributing-safely)

---

# Feature Parity Reference

## Platform Status

### What the labels mean

**Stable**: fully featured *and* proven in use. A gap on this platform is a
bug.

**Beta**: good for daily driving. Every navigation mode works and behaves as it
does on a stable platform. A platform is Beta either because something is still
missing, or because what is there has not yet been proven outside CI. Which one
applies is stated per platform below.

**Alpha**: worth trying, not yet worth switching to. Core navigation works, but
hint coverage is incomplete and per-app config does not re-apply on focus
change.

Every claim behind these labels is enumerated in the
[Capability Matrix](#capability-matrix) and [Known Gaps](#known-gaps). If a
label and the matrix disagree, the matrix is right.

**Linux and Windows parity is complete.** On both, every option, mode flag,
action and command means what it means on macOS, and
[Known Gaps](#known-gaps) carries no entry for either. That is the promise of
[ADR 0013](./adr/0013-parity-is-measured-in-words-not-subsystems.md), and it is
kept. The headless-sway and native `windows-latest` CI jobs gate merges.

**Both stay Beta anyway**, because parity is a claim about coverage and Stable
is a claim about reliability: the Linux and Windows work landed fast, on
platforms the maintainer does not daily-drive, each piece proven by a CI job
rather than by use.

**A Beta platform moves to Stable** after six consecutive releases in which no
bug specific to it is filed, meaning one a macOS user would not also hit. Count
bugs *filed* in that window, not ones still open at the end of it:

```bash
since=$(gh release view <tag-six-back> --json publishedAt --jq .publishedAt)
gh issue list --state all --label "platform: linux" --label bug \
  --search "created:>=${since%%T*}"
```

The query returns candidates. A person still has to discount cross-platform
bugs wearing a platform label.

### Per-platform

| Aspect               | macOS (Darwin)              | Linux                                    | Windows                        |
| -------------------- | --------------------------- | ---------------------------------------- | ------------------------------ |
| **Status**           | **Stable**                  | **Beta**                                 | **Beta**                       |
| **Build tag**        | `darwin`                    | `linux`                                  | `windows`                      |
| **CGO**              | Required (Objective-C)      | Per-backend; most Linux backends need it | Not used (pure Go Win32 / COM) |
| **Primary modifier** | `Cmd`                       | `Ctrl`                                   | `Ctrl`                         |
| **Display stack**    | Cocoa / Quartz              | X11, or Wayland (wlroots / KWin / COSMIC / Mutter) | Win32 / DWM           |
| **Accessibility**    | AXUIElement                 | AT-SPI over D-Bus                        | UI Automation over COM         |
| **Native product**   | Yes (`Neru.app`, codesigned)| Binary + install script                  | Binary                         |

### Linux backends

Linux is not one target. The live backend is detected once at startup from
`XDG_CURRENT_DESKTOP`, `WAYLAND_DISPLAY`, and `DISPLAY`
([backend_linux.go](../internal/adapter/platform/backend_linux.go)). This is
the only place the compositor *family* is decided; the `display_server` field
in `neru doctor` is derived from the matched row.

| Backend                | Detected when                                                          | Status            |
| ---------------------- | ---------------------------------------------------------------------- | ----------------- |
| `x11`                  | `DISPLAY` set, no `WAYLAND_DISPLAY`                                    | Supported         |
| `wayland-wlroots`      | Sway, Hyprland, niri, River, Wayfire, labwc, a `:wlroots` tag, or unset `XDG_CURRENT_DESKTOP` | Supported         |
| `wayland-kde`          | `XDG_CURRENT_DESKTOP` contains `KDE`                                   | Supported         |
| `wayland-cosmic`       | `XDG_CURRENT_DESKTOP` contains `COSMIC`                                | Supported         |
| `wayland-gnome`        | `XDG_CURRENT_DESKTOP` contains `GNOME`                                 | Supported with Xwayland |
| `wayland-other`        | Any other Wayland compositor                                           | **Not supported** |
| `unknown`              | Neither `WAYLAND_DISPLAY` nor `DISPLAY`                                | **Not supported** |

> **The daemon refuses to start on `wayland-other` and `unknown`.**
> `platform.NewSystemPort` returns `CodeNotSupported` for both, and that is
> the first step of daemon startup, so the daemon exits instead of starting
> degraded. `wayland-gnome` is refused the same way when `DISPLAY` is unset,
> because Mutter implements no `wlr-layer-shell` and the overlay there is an
> override-redirect window on Xwayland.
>
> **GNOME reads as the KDE column below with four differences**: the overlay is
> X11 + Cairo on Xwayland; focused-app identity, the app watcher and the window
> origin come from the Neru GNOME Shell extension (a stub until one re-login
> after the daemon installs it); the cursor position is re-learned through an
> Xwayland discovery window; and keyboard capture is the evdev proxy alone.
> [LINUX_DESKTOPS.md](LINUX_DESKTOPS.md#gnome-wayland) carries the measured
> detail.

---

## Capability Matrix

Status of each `ports.SystemPort`-level capability, with the mechanism that
implements it. The KDE and wlroots columns differ only where noted; both are
Wayland with `wlr-layer-shell` overlays.

**Legend:** ✅ supported · ⚠️ works with known limits · 🟡 stub (`CodeNotSupported`
or no-op) · ❌ no code path · ➖ macOS-only capability, exempt from parity
(see [Platform Exclusives](#platform-exclusives))

This table answers whether a *subsystem* works. Whether every option, mode
flag, action and command means the same thing on each platform is what
[Known Gaps](#known-gaps) and [Platform Support Per Word](#platform-support-per-word)
answer.

| Capability                    | macOS                    | Linux X11              | Linux Wayland (wlroots)      | Linux Wayland (KDE)     | Windows                      |
| ----------------------------- | ------------------------ | ---------------------- | ---------------------------- | ----------------------- | ---------------------------- |
| **Screen bounds / enumeration** | ✅ Cocoa               | ✅ XRandR              | ✅ xdg-output                | ✅ xdg-output           | ✅ `EnumDisplayMonitors`     |
| **Display hotplug events**    | ✅ screen-params notif.  | ✅ RandR event fd      | ✅ `wl_output` events        | ✅ `wl_output` events   | ✅ `WM_DISPLAYCHANGE`        |
| **Focused app identity**      | ✅ NSWorkspace + AX      | ✅ `_NET_ACTIVE_WINDOW` / `WM_CLASS` | ⚠️ app_id only (see below) | ⚠️ app_id only     | ✅ `GetForegroundWindow`     |
| **App watcher (focus change)**| ✅ NSWorkspace observer  | ✅ event-driven        | ✅ event-driven              | ✅ event-driven         | ✅ `SetWinEventHook`         |
| **Keymap learns the focused app** | ✅ published by the watcher | ✅ published by the watcher | ✅ published by the watcher | ✅ published by the watcher | ✅ published by the watcher |
| **Cursor position**           | ✅ `CGEventGetLocation`  | ✅ `XQueryPointer`     | ✅ `hyprctl` on Hyprland, else sync-surface cache | ✅ sync-surface cache | ✅ `GetCursorPos` |
| **Cursor move**               | ✅ `CGEventPost` ([`postMouseMoveLocked`](../internal/adapter/platform/darwin/accessibility_mouse_darwin.m)) | ✅ XTest (`XTestFakeMotionEvent`) | ✅ `zwlr_virtual_pointer` | ✅ libei                | ✅ `SetCursorPos`            |
| **Mouse buttons / drag**      | ✅ `CGEventPost`         | ✅ XTest ⁷             | ✅ `zwlr_virtual_pointer`    | ✅ libei                | ✅ `SendInput` ⁷             |
| **Scroll injection**          | ✅ both axes             | ✅ both axes ⁷         | ✅ both axes (uinput, virtual-pointer fallback) | ✅ both axes (uinput, libei fallback) | ✅ both axes ⁷               |
| **Modified scroll (`--modifier`)** | ✅ `CGEventSetFlags` on every chunk | ✅ XTest key hold ⁷ | ✅ virtual keyboard + virtual pointer (uinput on Hyprland ⁹) | ✅ libei | ✅ `SendInput` key hold ⁷ |
| **Smooth cursor animation**   | ✅ (incl. relative, opt-in) | ✅ incl. relative, opt-in | ✅ incl. relative, opt-in | ✅ incl. relative, opt-in | ✅ incl. relative, opt-in |
| **Held-key glide (`held_repeat`)** | ✅ `CGEventPost` per tick | ✅ XTest per tick | ✅ virtual-pointer relative motion per tick | ✅ libei per tick | ✅ `SetCursorPos` per tick |
| **Smooth scroll animation**   | ✅                       | ⚠️ whole notches only ³ | ✅ continuous axis ³ (whole notches when modified on Hyprland ⁹) | ⚠️ libei scroll delta, unverified ³ | ✅ 120ths of a notch ³ |
| **Element discovery (hints)** | ✅ AXUIElement           | ⚠️ AT-SPI walk         | ⚠️ AT-SPI walk               | ⚠️ AT-SPI walk          | ⚠️ UIA, control view only    |
| **Overlay**                   | ✅ NSPanel + CoreAnimation | ✅ X11 + Cairo       | ✅ layer-shell + Cairo       | ✅ layer-shell + Cairo  | ✅ DirectComposition + Direct2D (GDI fallback; windows/arm64 is GDI only ¹⁰) |
| **Global hotkeys**            | ✅ per-key CGEventTap    | ✅ `XGrabKey`          | ✅ evdev proxy (`input` group) | ✅ evdev proxy (`input` group) | ✅ `RegisterHotKey`          |
| **Keyboard capture**          | ✅ CGEventTap            | ✅ `XGrabKeyboard`     | ✅ evdev proxy (uinput; wl-keyboard fallback) | ✅ evdev proxy (uinput) | ✅ `WH_KEYBOARD_LL`          |
| **Modifier passthrough**      | ✅                       | ❌                     | ✅ evdev backend only        | ✅ evdev backend only   | ✅ `WH_KEYBOARD_LL` forwards or blocks per event |
| **Dark mode detection**       | ✅ Cocoa appearance      | ✅ xdg appearance portal | ✅ xdg appearance portal   | ✅ kdeglobals + portal  | ✅ registry                  |
| **Font resolution**           | ✅ NSFont                | ✅ fontconfig          | ✅ fontconfig                | ✅ fontconfig           | ✅ GDI `EnumFontFamiliesExW` ¹ |
| **System tray**               | ✅ NSStatusItem ⁸        | ✅ D-Bus StatusNotifierItem ⁸ | ✅ StatusNotifierItem ⁸      | ✅ StatusNotifierItem ⁸ | ✅ Win32 notification area ⁸ |
| **Native alerts**             | ✅ NSAlert               | ⚠️ D-Bus, not modal    | ⚠️ D-Bus, not modal          | ⚠️ D-Bus, not modal     | ✅ `MessageBoxW`             |
| **Native notifications**      | ✅ UNNotification        | ✅ `org.freedesktop.Notifications` | ✅ `org.freedesktop.Notifications` | ✅ `org.freedesktop.Notifications` | ✅ Tray balloon tips ⁸ |
| **Secure input detection**    | ✅                       | ➖ always false        | ➖ always false              | ➖ always false         | ➖ always false              |
| **System cursor hide**        | ✅ `CGDisplayHideCursor` | ➖                     | ➖                           | ➖                      | ➖                           |
| **`monitor_select` mode**     | ✅ native panels         | ✅ Cairo panels        | ✅ Cairo panels              | ✅ Cairo panels         | ✅ layered panels            |
| **Native hint-search field**  | ✅ NSTextField overlay   | 🟡 key-stream input ⁴  | 🟡 key-stream input ⁴        | 🟡 key-stream input ⁴   | 🟡 key-stream input ⁴        |
| **Screen capture**            | ✅ ScreenCaptureKit      | ✅ `XGetImage`         | ✅ `wlr-screencopy`          | ⚠️ portal ScreenCast, consent ⁵ | ✅ `BitBlt` ⁵        |
| **Vision / OCR detection**    | ✅ Vision framework      | ⚠️ tesseract, text only ⁶ | ⚠️ tesseract, text only ⁶ | ⚠️ tesseract, text only ⁶ | ⚠️ `Windows.Media.Ocr`, text only ⁶ |
| **Key feed (`neru key`)**     | ✅ `CGEventPost`         | ✅ uinput               | ✅ uinput / virtual-keyboard | ✅ uinput               | ✅ `SendInput`               |
| **Service management (`neru services`)** | ✅ launchd user agent | ⚠️ systemd user unit only ² | ⚠️ systemd user unit only ² | ⚠️ systemd user unit only ² | ✅ Task Scheduler logon task |

¹ **Font resolution.** Every platform resolves font *families* through the OS,
and a named family resolves to **that name**. A family the platform can see is
missing goes to the sans baseline (DejaVu Sans on Linux, Segoe UI on Windows)
rather than to the platform's own substitute; macOS and the non-CGO Linux build
check nothing and let NSFont / Cairo substitute at draw time. The generic names
`sans`, `serif`, `mono` (and their spelling variants, and the empty string)
resolve to each platform's own faces
(`internal/adapter/platform/fontgeneric`, ADR 0007); answers are cached per
family name (`internal/adapter/platform/fontcache`).

² **Service management** is the one row whose limit is not the display server:
it needs **systemd**, on every Linux backend. runit, OpenRC and s6 get
`CodeNotSupported` from every `neru services` subcommand, a stated boundary
rather than a gap. See "Service management on Linux" below.

³ **Smooth scroll granularity.** `smooth_scroll` animates everywhere, but only
some primitives can send a step shorter than a wheel notch: the wlroots virtual
pointer and libei carry fractional deltas, Windows counts 120ths of a notch,
and X11 core scrolling is one notch per event. Neru sends the same distance on
every backend; Linux paths that send notches convert at 30 px per notch and
round to the nearest, never fewer than one, so on X11 a `scroll_step` under 45
px is a single unanimated click and everything from two notches up gets the
same eased curve as elsewhere. Wayland steps declare axis source `continuous`
so toolkits do not round the fraction back to a detent. The wlroots behaviour is
measured by `TestScrollAtCursor_DeliversSubNotchStepsWithSmoothScroll`; the
X11 and KDE conclusions are read from the protocol sources and not measured on
hardware.

⁴ **Native hint-search field.** Only macOS has a platform text control that
owns keyboard focus and brings the system input method with it. Everywhere else
the query is read from the event tap's key stream, so dead keys and IME
composition do not work there. The search *badge* is a different thing: every
platform draws one, and `hints.search_input_ui.*` means the same on all three.

⁵ **Screen capture** is taken per backend rather than through the desktop
portal everywhere, because a consent picker in front of a hint refresh is a
latency regression the blessed stack has no need to pay. X11 uses `XGetImage`,
wlroots `wlr-screencopy`, Windows `BitBlt`, none of which asks consent. **KWin
implements neither**, so KDE pays the portal: an
`org.freedesktop.portal.ScreenCast` session over PipeWire, a
[required build dependency](./LINUX_SETUP.md#build-dependencies). It is a
**permission** rather than a missing capability: the grant is persisted with a
restore token under `$XDG_STATE_HOME/neru/`, only the mode handler's permission
preflight can raise the dialog, and `CheckScreenCapturePermission` reports the
real consent state there and "no gate" elsewhere. Capture is a **region**
operation on every backend, and a region that leaves the screen, spans two
Wayland outputs, or lies on a monitor the user declined to share **fails**
rather than coming back clipped, because a clipped frame cannot say where its
own top-left is. Scaled outputs return physical pixels, as Retina does on
macOS. How KDE streams are placed on outputs is in
[portal_screencast.go](../internal/adapter/platform/linux/portal_screencast.go).

⁶ **Vision on Linux and Windows is text-only**, and permanently so. macOS runs
three Vision requests (text, rectangles, saliency); an OCR engine answers only
the first, so `hints.vision.detect_rectangles` and the four `rectangle_*`
options are declared macOS-only. The `contour` strategy is a separate,
dependency-free detector on every platform. On Linux the engine is
**tesseract**, linked through cgo and required at build time, with its language
data resolved at use (`TESSDATA_PREFIX`, then the distribution paths); a missing
`eng.traineddata` gets `CodeNotSupported` naming that file. On Windows it is
**`Windows.Media.Ocr`** through raw vtables with no CGO
(`platform/windows/ocr.go`); it needs the OCR language pack for one of the
account's languages, reports no per-word confidence (so the three
`*_confidence` floors are declared inert there), and caps images at 2600 px,
which Neru downsamples to and scales back from. Recognized text is screen
content: never logged, never written to disk.

⁷ **Modifiers on injected input (X11 and Windows).** An X11 pointer event and a
`SendInput` mouse event both carry whatever modifiers the keyboard currently
holds, so a hotkey chord still held while a hint was chosen used to make the
click a ctrl+click. Neru reads the live key state (`XQueryKeymap`,
`GetAsyncKeyState`), releases the modifiers the injection would falsify,
presses the ones asked for, and undoes both when done, held across every chunk
of an animated scroll and across a drag until its release. Restoring is the
deliberate bias, since the opposite drops a modifier the user is still holding.

⁸ **Tray and notifications.** The tray icon carries the paused state on every
platform: macOS swaps template glyphs, SNI hosts and the Win32 notification
area get a desaturated tile derived from the running one
([icon/paused.go](../internal/adapter/systray/icon/paused.go)). Per-item menu
tooltips exist nowhere, because `com.canonical.dbusmenu` defines none.
**Notifications on Windows are balloon tips on that tray icon**, because WinRT
toasts need an AppUserModelID an unpackaged exe does not have; with
`systray.enabled = false` there is nothing to attach a tip to, so
`ShowNotification` reports `CodeNotSupported` naming that reason. Alerts are
`MessageBoxW` and do not depend on the tray.

⁹ **Hyprland modified scroll.** With a virtual-keyboard modifier held, a
`zwlr_virtual_pointer` scroll produces no event on Hyprland
([#1474](https://github.com/y3owk1n/neru/pull/1474)), so there the modifier
goes out on the virtual keyboard and the scroll on the uinput wheel, in whole
notches. The compositor is named from `XDG_CURRENT_DESKTOP` beside the backend
detection.

¹⁰ **windows/arm64 overlay.** The Direct2D binding passes floats through Go's
stdcall shim, which handles them on amd64 only, so windows/arm64 builds the GDI
surface alone (`platform/windows/overlay_dcomp_other.go`).

### Notes on the ⚠️ entries

**Focused app on Wayland.** wlroots and KWin resolve the focused window through
`wlr-foreign-toplevel-management`, which exposes the window's **app_id** (what
per-app config keys on) but not its PID. `SystemPort.FocusedApplicationPID`
best-effort matches the app_id against `/proc` and otherwise returns
`CodeNotSupported` carrying the app_id rather than a fabricated number.

**An unfocused desktop is not a failure on X11 either.** Nothing focused is
`CodeNotSupported`, so callers degrade as they do on Wayland; a display with no
live EWMH window manager, a failed read or a malformed property is
`CodeActionFailed` naming which. `FocusedWindowBounds` tells the same two apart
(`found=false` with no error versus an error), so a caller widening to the
active screen knows whether it is obeying an answer or guessing.

**App watcher.** macOS observes NSWorkspace. Linux subscribes to a backend
focus-change fd (X11 event fd, or the wlroots toplevel manager) in
`appwatcher/platform_linux.go`, with a 3s safety re-sample and a 400ms poll
when no fd exists; the identity is `WM_CLASS` or `app_id`. Windows installs an
`EVENT_SYSTEM_FOREGROUND` hook on its own message-loop thread
(`appwatcher/platform_windows.go`) and reports the **executable path**; display
changes arrive through a hidden window receiving `WM_DISPLAYCHANGE`
(`platform/windows/display_watcher.go`). On both, only activate, deactivate and
screen-params are emitted; launch, terminate and Mission Control stay
macOS-only.

**Global hotkeys on Wayland.** No Wayland protocol lets an ordinary client
register a global hotkey, so Neru matches chords itself on the evdev keyboard
proxy that the in-mode capture also runs on
([global_hotkey_cgo.go](../internal/adapter/eventtap/linux/global_hotkey_cgo.go),
[ADR 0014](adr/0014-the-wayland-keyboard-is-a-proxy.md)). With `/dev/uinput`
writable the proxy holds every keyboard from daemon launch and re-emits it
through a uinput device, so a matched chord is withheld from the focused app; it
never grabs a keyboard that has a key down, so no modifier is left stuck.
Without uinput it reads passively and the app receives the chord too. It needs
the `input` group and a CGO build; a `CGO_ENABLED=0` build gets a stub whose
`Start` reports `CodeNotSupported`. Either way Neru warns once with the remedy
and names the fallback, binding `neru <mode>` in the compositor.

**Native alerts on Linux.** Notifications and alerts both go to the session's
freedesktop notification daemon over D-Bus, in pure Go. An alert is a
notification with critical urgency and no expiry, and it is *not* modal: no
Wayland or X11 client can stop the world and return a button, so a Linux alert
informs and callers take the safe default (a missing config file starts Neru on
defaults and says so). With no notification daemon, both report
`CodeNotSupported`, `neru doctor` says what to install, and the two startup
alerts fall back to stderr.

**Service management on Linux.** A **systemd user unit** anchored on
`graphical-session.target` (`After=`, `WantedBy=`, `PartOf=`), so a logout and
login restarts the daemon rather than orphaning it. Coverage is systemd and no
other init: the check is systemd's runtime marker `/run/systemd/system`, not
`systemctl` on `PATH`. Where the unit lives and how to drive it:
[LINUX_SETUP.md](./LINUX_SETUP.md#systemd-user-service).

**Smooth cursor animation on Linux.** Off by default; opt in with
`smooth_cursor.move_mouse_enabled`. One worker goroutine steps the per-backend
warp toward the target by linear interpolation, latest target wins, and
`WaitForCursorIdle` blocks until it settles
([mouse_animator.go](../internal/adapter/platform/linux/mouse_animator.go)).
Relative moves on wlroots drain the delta through native relative motion
([relative_animator.go](../internal/adapter/platform/linux/relative_animator.go))
so they never read the position cache. Clicks stay instant, and
position-dependent actions settle the animation before acting.

---

## Input Injection

Every action type in [action.go](../internal/domain/action/action.go), click,
per-button down/up/toggle, absolute and relative moves, drag-while-held and
scroll, is dispatched through the shared `InfraAXClient.PerformAction`. Only
the final injection primitive differs:

| Platform              | Primitive                                                                    |
| --------------------- | ---------------------------------------------------------------------------- |
| macOS                 | `CGEventPost` (`kCGEventMouseMoved` / `*MouseDragged` for moves)              |
| Linux X11             | XTest (`XTestFakeMotionEvent`, buttons 1/2/3, scroll 4/5 vert. + 6/7 horiz.)  |
| Linux Wayland wlroots | `zwlr_virtual_pointer` (+ `/dev/uinput` for scroll)                           |
| Linux Wayland KDE     | libei via `org.freedesktop.portal.RemoteDesktop` (+ `/dev/uinput` for scroll) |
| Windows               | `SendInput` / `SetCursorPos`                                                  |

Scrolling behaves the same on all three platforms, both axes included. Windows
posts `MOUSEEVENTF_HWHEEL` with the sign flipped, because Win32 reads a positive
horizontal notch as right where the others read it as left.

**Modifiers on a scroll** reach the primitive by two routes. macOS stamps
`CGEventSetFlags` on every scroll event and every chunk of an animation. The
other three press the real key, scroll, and release it; on X11 that key event is
announced to Neru's own event tap first so `sticky_modifiers` does not latch it,
and on Wayland it goes out on the virtual keyboard (libei on KDE), everywhere
but Hyprland (footnote ⁹). A path with no backend to press through answers
`CodeNotSupported`; none scrolls unmodified and reports success.

**Held mouse buttons.** Press and release are separate actions, so every
backend keeps a [`mousestate.Tracker`](../internal/adapter/platform/mousestate/tracker.go)
recording which buttons are down, where, and with which modifiers. Toggle
actions resolve against it, `EnsureMouseUp` releases every held button when
Neru returns to idle, and on macOS it selects the drag event type for cursor
moves, which Quartz requires; the other platforms warp the pointer and let the
compositor infer the drag.

---

## Keyboard Capture And Hotkeys

| Aspect                | macOS                  | Linux X11               | Linux Wayland                            | Windows                 |
| --------------------- | ---------------------- | ----------------------- | ---------------------------------------- | ----------------------- |
| **In-mode capture**   | `CGEventTapCreate`     | `XGrabKeyboard`         | evdev proxy (lifetime `EVIOCGRAB` + uinput re-emit), wl-keyboard fallback | `WH_KEYBOARD_LL`        |
| **Global hotkeys**    | Per-key CGEventTap     | `XGrabKey`              | Chord matcher on the evdev proxy         | `RegisterHotKey`        |
| **CGO needed**        | Yes                    | Yes                     | Yes                                      | No                      |
| **Press/release**     | ✅ separate callbacks  | ✅ KeyPress/KeyRelease  | ✅ evdev press/release                   | ✅ press, then `GetAsyncKeyState` poll |
| **Modifier passthrough** | ✅                  | ❌ grab is all-or-nothing | ✅ evdev only                          | ✅ hook forwards per event |
| **`PostModifierEvent`** | ✅                   | ✅                      | ✅ (`zwp_virtual_keyboard_v1`)           | ✅ `SendInput`          |
| **Sticky modifiers**  | ✅                     | ✅                      | ✅                                       | ✅                      |
| **Capture files**     | `eventtap/darwin/`     | `eventtap/linux/x11_cgo.go` | `eventtap/linux/evdev_proxy_cgo.go`, `evdev_session_cgo.go`, `wayland_cgo.go` | `eventtap/windows/` |
| **Hotkey files**      | `hotkeys/darwin/`      | `hotkeys/linux/x11_cgo.go`  | `hotkeys/linux/manager.go` + `eventtap/linux/global_hotkey_cgo.go` | `hotkeys/windows/` |

There is no separate Wayland hotkey file: `hotkeys/linux/manager.go` delegates
to the evdev listener in the eventtap package.

**A held global hotkey.** Every manager implements `HotkeyReleaseRegistrar` and
reports a hold as one press and one release, which is what `[held_repeat]`
repeats between. macOS folds autorepeat in the per-hotkey tap; the evdev proxy
fires the release when the chord's key comes up; X11 asks the server for
detectable autorepeat (`XkbSetDetectableAutoRepeat`) on both connections so a
held key is repeated presses and one release; `RegisterHotKey` reports the
press only, so Windows registers with `MOD_NOREPEAT` and polls
`GetAsyncKeyState` every 10 ms while a hotkey is held.

**A global chord while a mode is active.** A `[hotkeys]` binding keeps working
from inside a mode on every platform, each its own way, because whichever
mechanism can see the chord has to be the only one that runs it. macOS and
Windows hand the chord back: the in-mode tap returns it untouched so the
per-hotkey tap or `RegisterHotKey` fires, and both are told which chords the
backend *took* rather than which the configuration asked for
(`Deps.PublishRegisteredHotkeys`, [hotkey.go](../internal/app/keybinding/hotkey.go)).
Linux cannot hand it to anybody, since X11's capture is an exclusive
`XGrabKeyboard` and the Wayland proxy owns every press while a mode is open, so
there the mode handler resolves the global table itself after the mode's own
([keymap.go](../internal/app/modes/keymap.go), `settledKeymaps`). Only chords
carrying Ctrl/Alt/Cmd fall back: a bare key inside a mode is a hint label or a
grid cell key. One consequence shared by both Linux backends: while a mode is
open, a chord bound in the *compositor* rather than in `[hotkeys]` cannot fire.

**One key, one name.** Both Linux readers of `/dev/input` resolve a scan code
through the compositor's XKB keymap
([evdev_xkb_cgo.go](../internal/adapter/eventtap/linux/evdev_xkb_cgo.go)), and
the X11 tap names the key from the state-resolved **keysym** rather than the
string `XLookupString` returns, so all backends call `Shift+;` the same thing
and XKB options like `ctrl:swapcaps` reach Neru's own bindings. **On a
non-QWERTY layout this decides which physical key a `[hotkeys]` chord answers**:
the one bearing that character on the active layout. A keysym is named by the
character it types when it types one, and by keysym name otherwise
([wayland_keymap.c](../internal/adapter/platform/linux/wayland_keymap.c)); the
one key XKB renames under Shift, `ISO_Left_Tab`, folds back to `Tab`, which is
what lets the default `Shift+Tab` hint binding fire.

**Modifier passthrough (Wayland evdev, and Windows).** While a mode is active
Neru captures the keyboard exclusively, so shortcuts it does not bind are
swallowed. With `general.passthrough_unbounded_keys`, unbound Ctrl/Alt/Cmd
chords reach the focused app instead: the Wayland proxy re-emits them on its
uinput keyboard together with the modifiers the user is physically holding, and
the Windows `WH_KEYBOARD_LL` hook forwards or blocks each event on its own. It
is **not** available on X11 (an `XGrabKeyboard` routes Neru's own XTest events
back to itself, and `XSendEvent` is ignored by most apps) nor on the wl-keyboard
fallback; since the platform column cannot say "Linux, except X11", turning it
on under X11 warns once at load and `neru doctor` lists the option in its
`platform_support` row (`config.X11InertWords`). Classification and the
post-passthrough hint refresh are shared in
[passthrough.go](../internal/app/modes/passthrough.go); only the re-injection
is backend-specific. `general.should_exit_after_passthrough` exits the mode
after a passthrough.

---

## Accessibility And Hints

| Aspect                  | macOS                                       | Linux                                              | Windows                              |
| ----------------------- | ------------------------------------------- | -------------------------------------------------- | ------------------------------------ |
| **Backend**             | AXUIElement (CGO ObjC bridge)               | AT-SPI over D-Bus (pure Go)                        | UI Automation over COM (pure Go)     |
| **Client**              | `InfraAXClient` → ObjC bridge               | `atspi.Client` → `org.a11y.atspi`                  | `UIAClient` → raw COM vtables        |
| **Files**               | `native/darwin/element.go`, `tree.go`       | `accessibility/atspi/`, `native/linux/element.go`, `factory_linux.go` | `native/windows/automation.go`, `element.go`, `tree.go` |
| **Traversal**           | Full recursive walk of the AXUIElement hierarchy | Recursive walk of the active frame's subtree, depth/node capped | Control-view tree in one cached `FindAll`, any depth |
| **Sources collected**   | Frontmost + all windows, popovers, menubar, dock, notification center, Stage Manager, PIP | Active frame's subtree only          | Foreground window's control view     |
| **Filtering**           | Role matching, size/position heuristics, excluded apps, dedup | Native AT-SPI roles, `SHOWING` state, on-screen extents | `IsControlElement` + `IsContentElement`, non-zero bounds |
| **Strategies**          | `axtree` (default), `vision` and `contour`, incl. per-app overrides | `axtree`, `vision` (text only) and `contour` | `axtree`, `vision` (text only) and `contour` |
| **Popovers / menus**    | ✅ dedicated detection                      | ⚠️ only if inside the active frame's subtree       | 🟡                                   |

macOS builds the richest tree by a wide margin. Linux walks a single AT-SPI
tree with tesseract beside it. Windows fetches the window's control-view tree in
one cached UI Automation query with `Windows.Media.Ocr` beside it; the ⚠️ is
the control view, since an element a provider exposes only in the raw view is
not a hint, and the answer is a role or filter default, never a per-app branch.

**Linux is ⚠️, not a stub.** `atspi.Client` walks the active frame emitting
native AT-SPI role names, and configured roles are resolved into that vocabulary
at config load (`element.ResolveRoles`). The ⚠️ is coverage: it depends on each
app exposing AT-SPI.

**Chromium and Electron apps on Linux** do not expose their web-content tree
over AT-SPI by default, and there is no per-app attribute Neru can toggle to
force it. Launch the app with `--force-renderer-accessibility`. Native GTK/Qt
apps and Firefox need no flag. This is Chromium behavior, not a Neru limitation.

**Picking the active frame.** The AT-SPI `ACTIVE` state cannot be trusted
across applications (wlroots can report the focused window `ACTIVE=false`;
Chromium on X11 reports `ACTIVE=true` regardless), so Neru matches the AT-SPI
frame against the focused window's identity instead: app_id and title on
Wayland, WM_CLASS and `_NET_WM_NAME` on X11, read as one snapshot
(`linux.FocusedAppIdentity`). The `ACTIVE`/`SHOWING` heuristic remains the
fallback when no identity is available (`findActiveFrame` in `atspi/scan.go`).

**Window-origin offset on Wayland.** A Wayland client cannot know its own
on-screen position, so AT-SPI reports element coordinates relative to the
window. Neru offsets them by the focused window's screen origin, supplied by a
compositor-specific `windowOriginSource`
([window_origin.go](../internal/adapter/accessibility/atspi/window_origin.go)):

| Compositor | Source                                                       | Limits                                                                                                                    |
| ---------- | ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------- |
| KDE / KWin | KWin script pushing focused-window geometry over D-Bus ([platform/kwin](../internal/adapter/platform/kwin)) | Reports on activation and on the focused window moving, resizing or going away; Neru reinstalls the script when KWin restarts. A drag reports its final rectangle. |
| niri       | `niri msg -j focused-window` / `focused-output`              | Floating and fullscreen windows only. **Tiled** windows expose no on-screen position ([niri#2381](https://github.com/niri-wm/niri/issues/2381)), so hints are misaligned there. |
| Sway       | `swaymsg -t get_tree`, focused node `rect` + `window_rect`   | none                                                                                                                       |
| Hyprland   | `hyprctl -j activewindow` `at` / `size`                      | none                                                                                                                       |
| GNOME      | GNOME Shell extension reporting the focused window's frame over D-Bus ([platform/gnomeshell](../internal/adapter/platform/gnomeshell)) | The daemon installs and enables the extension; the shell loads it at the next login. |
| Anything else (X11, River, Wayfire) | none                                                | X11 needs none, AT-SPI already reports screen coordinates there. The rest report no origin, so hints stay window-relative. |

Each source verifies the reported window matches the AT-SPI frame (size on
all of them, identity as well on KWin and GNOME, because their rectangle is a
cache) and is best-effort: an unavailable origin degrades to unoffset
coordinates rather than misplacing hints. The CLI sources go through
[platform/compositorcli](../internal/adapter/platform/compositorcli), which
warns when a compositor could not be run or answered garbage and stays quiet
when it answered with no position (nothing focused, a tiled niri window).

**The same sources answer `FocusedWindowBounds`,** which scopes vision and
contour detection and `neru action move_mouse --window` to the focused window
([system_focused_window.go](../internal/adapter/platform/linux/system_focused_window.go)).
A Wayland compositor with no source (River, Wayfire) reports `CodeNotSupported`
there rather than "no focused window", so a caller widening to the active
screen knows why.

---

## Overlay Rendering

### Architecture

The platforms split responsibility differently, which is the single most
important thing to know before touching overlay code:

- **macOS**: each render component owns its own NSPanel. Files such as
  `adapter/overlay/render/hints/overlay_darwin.go` call the Objective-C bridge
  directly, and rendering is GPU-backed via CoreAnimation.
- **Linux and Windows**: the render components hold the shared `Style` and a
  thin wrapper; all real rendering happens in the overlay **manager**
  (`overlay/linux/x11_cgo.go`, `overlay/linux/wayland_cgo.go`,
  `overlay/windows/manager.go`), drawing every element into one shared
  surface.

### Implementation

| Aspect                | macOS                                    | Linux X11                              | Linux Wayland                                   | Windows                            |
| --------------------- | ---------------------------------------- | -------------------------------------- | ------------------------------------------------ | ---------------------------------- |
| **Window type**       | NSPanel, borderless non-activating       | override-redirect X11 window           | `wlr_layer_shell_v1` overlay surface             | `WS_POPUP` HWND, `WS_EX_NOREDIRECTIONBITMAP` (layered on the GDI fallback) |
| **Rendering**         | CoreAnimation (CALayer, GPU)             | Cairo on an Xlib surface (CPU)         | Cairo into SHM buffers (CPU)                     | Direct2D on a DirectComposition swapchain (GPU); GDI + software SDF on the fallback (CPU) |
| **Per-pixel alpha**   | clear color + non-opaque layer           | `CAIRO_OPERATOR_CLEAR`                 | `CAIRO_OPERATOR_CLEAR`                           | premultiplied swapchain; `AC_SRC_ALPHA` via `UpdateLayeredWindowIndirect` on the fallback |
| **Click-through**     | `setIgnoresMouseEvents:YES`              | XFixes empty input region              | empty `wl_surface` input region                  | `WS_EX_TRANSPARENT` + `HTTRANSPARENT`  |
| **Always on top**     | `NSScreenSaverWindowLevel`               | `_NET_WM_STATE_ABOVE` + `MapRaised`    | overlay layer                                    | `HWND_TOPMOST`                     |
| **Focus prevention**  | non-activating panel                     | `override_redirect=YES`                | controlled keyboard interactivity                | `WS_EX_NOACTIVATE`                 |
| **HiDPI**             | dynamic `contentsScale` + backing-change callback | `Xft.dpi`, one global factor  | `wl_output` scale + `wp_fractional_scale_v1` / `wp_viewporter` | per-monitor-v2 DPI aware, `GetDpiForMonitor` per window multiplies fonts, padding, lines and radii |
| **Multi-monitor**     | per-display clamping, screen-change tracking | all monitors enumerated, per-monitor render, live RandR hotplug | one `wl_surface` per output (max 16), live hotplug | cursor-screen tracking, live `WM_DISPLAYCHANGE` hotplug, one panel window per display for monitor_select |
| **Buffers**           | layer-backed, OS-managed                 | single Cairo surface                   | triple-buffered SHM pool                         | canvas bitmap + 2-buffer flip swapchain; one persistent DIB on the fallback |
| **Rounded rects / borders** | NSBezierPath                       | Cairo arc path + stroke                | Cairo arc path + stroke                          | Direct2D rounded rects; software SDF on the fallback |
| **Text**              | NSFontManager                            | Cairo `select_font_face` / `show_text` | Cairo `select_font_face` / `show_text`           | DirectWrite text formats, cached per family and size; cached GDI fonts + `DrawTextW` on the fallback |
| **Coordinate origin** | bottom-left (Y-flipped in the adapter)   | top-left                               | top-left                                         | top-left                           |
| **Thread model**      | main-thread dispatch                     | `renderMu` mutex                       | `renderMu` mutex (also guards `wl_display`)      | dedicated UI thread (`LockOSThread`); draws queue and return, the thread presents |

### Animation

| Animation                    | macOS                                | Linux X11 / Wayland                | Windows                            |
| ---------------------------- | ------------------------------------ | ---------------------------------- | ---------------------------------- |
| **Grid transition**          | NSTimer @120Hz, ease-in-out, full redraw | goroutine, ease-in-out @120fps  | goroutine, ease-in-out @120fps, presented on the UI thread |
| **Mouse action indicator**   | `CABasicAnimation` (scale + opacity) | goroutine, scale + opacity @120fps | goroutine, cubic easing @60fps     |
| **Smooth cursor**            | ✅ stepped linear interpolation      | ✅ stepped linear interpolation    | ✅ stepped linear interpolation    |
| **Smooth scroll**            | ✅ ease-out cubic                    | ✅ ease-out cubic                  | ✅ ease-out cubic, 120ths of a notch |

---

## Mode Coverage

Mode logic (labelling, alphabets, matching, search filtering, grid subdivision,
recursion depth, scroll amounts, cell navigation) is pure domain Go under
`internal/domain/` and behaves **identically on all three platforms**. Only the
rows below differ, and every difference traces to rendering or element
discovery rather than the mode itself.

| Mode              | Feature                        | macOS                      | Linux                      | Windows                     |
| ----------------- | ------------------------------ | -------------------------- | -------------------------- | --------------------------- |
| **Hints**         | Element discovery              | ✅ full AX tree            | ⚠️ AT-SPI, toolkit-dependent | ⚠️ UIA, control view only |
| **Hints**         | `vision` strategy + per-app overrides | ✅                  | ⚠️ tesseract; text only, no rectangles | ⚠️ `Windows.Media.Ocr`; text only, no rectangles, no confidence |
| **Hints**         | Menubar / dock elements        | ✅                         | 🟡                         | 🟡                          |
| **Hints**         | Search input badge             | ✅                         | ✅ Cairo badge             | ✅                          |
| **Hints**         | Label arrow / tail             | ✅ NSBezierPath            | ✅ Cairo triangle          | ✅ sampled triangle, see below |
| **Hints**         | Label placement                | ✅ top / center / bottom   | ✅ top / center / bottom   | ✅ top / center / bottom   |
| **Grid**          | Virtual pointer indicator      | ✅                         | ✅                         | ✅                          |
| **Grid**          | What an open subgrid shows     | ✅ the subgrid alone       | ✅ the subgrid alone       | ✅ the subgrid alone       |
| **Recursive grid**| Transition animation           | ✅                         | ✅                         | ✅                          |
| **Recursive grid**| Virtual pointer indicator      | ✅                         | ✅                         | ✅                          |
| **Recursive grid**| Sub-key preview                | ✅ mini-grid of next keys  | ✅ mini-grid of next keys  | ✅ mini-grid of next keys   |
| **Scroll**        | Smooth scroll animation        | ✅                         | ✅ (X11: whole notches)    | ✅ (120ths of a notch)     |
| **Monitor select**| Whole mode                     | ✅ native panels           | ✅ Cairo panels            | ✅ one layered window per display |

Everything else is shared: multi-letter labels, label direction,
hide-unmatched, split-word, interactive search *behavior*, boundary highlight,
mode indicator, sticky-modifier indicator, all pending actions on grid cells,
backtracking, and every scroll granularity.

> The **cursor-replacement virtual pointer**, drawn when the real cursor is
> hidden, is separate from the two grid indicators above and is macOS-only,
> paired with `CGDisplayHideCursor`, which has no equivalent elsewhere.

> **`hints.ui.placement` means the same thing on all three platforms.** Linux
> and Windows take the offsets and the arrow from one implementation
> (`adapter/overlay/render/badge.PlaceHint`); macOS computes its own in
> Objective-C (the exception ADR 0007 records) with a shorter, wider arrow. On
> Windows the Win32 surface has no path primitive, so the arrow is a triangle
> over a slightly larger one in the border colour
> ([#1303](https://github.com/y3owk1n/neru/issues/1303)).

> **`recursive_grid.ui.sub_key_preview` is one drawing on all three platforms**
> ([#1297](https://github.com/y3owk1n/neru/issues/1297)), and
> `sub_key_preview_autohide_multiplier` measures a **sub-cell** from one
> implementation (`recursivegrid.Style.ShowSubKeyPreviewIn`, with the macOS
> copy held to it by
> `internal/architecture/sub_key_preview_autohide_rule_test.go`).

---

## Platform Support Per Word

Every option, mode flag and action carries a platform column, declared once
beside the vocabulary that owns it:
[`internal/config/platform_support.go`](../internal/config/platform_support.go),
[`internal/domain/modecmd/platform_support.go`](../internal/domain/modecmd/platform_support.go)
and
[`internal/domain/action/platform_support.go`](../internal/domain/action/platform_support.go).
The table below is a projection of those declarations, as are the warning the
daemon prints once at load and the `platform_support` row in `neru doctor`
([ADR 0013](./adr/0013-parity-is-measured-in-words-not-subsystems.md)).

It lists only the words whose column is narrower than every platform. The
several hundred that work everywhere are declared too, and
`internal/architecture/platform_support_test.go` fails the build when a word is
neither. Writing one of these where it is inert is never a config error: the
file loads, the daemon runs, and one warning says which lines mean nothing
here, so one configuration can be carried between platforms
([ADR 0008](./adr/0008-a-vocabulary-has-one-home.md)).

This table answers a different question from the
[Capability Matrix](#capability-matrix). The matrix says whether a subsystem
works; this says whether a word a person wrote does anything.

<!-- BEGIN GENERATED PLATFORM SUPPORT: edit the platform_support.go declarations, then run `just gensupportref` -->

| Word | Kind | macOS | Linux | Windows | Why |
| ---- | ---- | --- | --- | --- | --- |
| `general.hide_overlay_in_screen_share` | option | ✅ | ❌ | ❌ | hiding the overlay from a screen share is an NSWindow sharing level, a Quartz concept with no X11, Wayland or Win32 counterpart |
| `general.kb_layout_to_use` | option | ✅ | ❌ | ❌ | the keyboard layout is detected rather than chosen outside macOS |
| `hints.include_menubar_hints` | option | ✅ | ❌ | ❌ | the menu bar, the Dock, Notification Center, Stage Manager, picture-in-picture and the screen-capture chrome are macOS surfaces with no counterpart |
| `hints.additional_menubar_hints_targets` | option | ✅ | ❌ | ❌ | the menu bar, the Dock, Notification Center, Stage Manager, picture-in-picture and the screen-capture chrome are macOS surfaces with no counterpart |
| `hints.include_dock_hints` | option | ✅ | ❌ | ❌ | the menu bar, the Dock, Notification Center, Stage Manager, picture-in-picture and the screen-capture chrome are macOS surfaces with no counterpart |
| `hints.include_nc_hints` | option | ✅ | ❌ | ❌ | the menu bar, the Dock, Notification Center, Stage Manager, picture-in-picture and the screen-capture chrome are macOS surfaces with no counterpart |
| `hints.include_stage_manager_hints` | option | ✅ | ❌ | ❌ | the menu bar, the Dock, Notification Center, Stage Manager, picture-in-picture and the screen-capture chrome are macOS surfaces with no counterpart |
| `hints.include_pip_hints` | option | ✅ | ❌ | ❌ | the menu bar, the Dock, Notification Center, Stage Manager, picture-in-picture and the screen-capture chrome are macOS surfaces with no counterpart |
| `hints.include_screen_capture_hints` | option | ✅ | ❌ | ❌ | the menu bar, the Dock, Notification Center, Stage Manager, picture-in-picture and the screen-capture chrome are macOS surfaces with no counterpart |
| `hints.detect_mission_control` | option | ✅ | ❌ | ❌ | Mission Control is a macOS concept, so the detection never fires and the hooks never run |
| `hints.on_mission_control_activated` | option | ✅ | ❌ | ❌ | Mission Control is a macOS concept, so the detection never fires and the hooks never run |
| `hints.on_mission_control_deactivated` | option | ✅ | ❌ | ❌ | Mission Control is a macOS concept, so the detection never fires and the hooks never run |
| `hints.max_depth` | option | ✅ | ❌ | ❌ | only the AX walk takes a depth limit; the AT-SPI walk uses a fixed one and the UIA walk records the option without reading it |
| `hints.ignore_clickable_check` | option | ✅ | ❌ | ❌ | the clickable and visibility checks are AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult these |
| `hints.visible_check_enabled` | option | ✅ | ❌ | ❌ | the clickable and visibility checks are AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult these |
| `hints.app_configs.ignore_clickable_check` | option | ✅ | ❌ | ❌ | the clickable and visibility checks are AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult these |
| `hints.app_configs.visible_check_enabled` | option | ✅ | ❌ | ❌ | the clickable and visibility checks are AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult these |
| `grid.app_configs.ignore_clickable_check` | option | ✅ | ❌ | ❌ | the clickable and visibility checks are AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult these |
| `grid.app_configs.visible_check_enabled` | option | ✅ | ❌ | ❌ | the clickable and visibility checks are AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult these |
| `recursive_grid.app_configs.ignore_clickable_check` | option | ✅ | ❌ | ❌ | the clickable and visibility checks are AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult these |
| `recursive_grid.app_configs.visible_check_enabled` | option | ✅ | ❌ | ❌ | the clickable and visibility checks are AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult these |
| `scroll.app_configs.ignore_clickable_check` | option | ✅ | ❌ | ❌ | the clickable and visibility checks are AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult these |
| `scroll.app_configs.visible_check_enabled` | option | ✅ | ❌ | ❌ | the clickable and visibility checks are AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult these |
| `app_configs.ignore_clickable_check` | option | ✅ | ❌ | ❌ | the clickable and visibility checks are AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult these |
| `app_configs.visible_check_enabled` | option | ✅ | ❌ | ❌ | the clickable and visibility checks are AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult these |
| `grid.prewarm_enabled` | option | ✅ | ❌ | ❌ | only the darwin grid overlay prewarms its layers; the other backends draw on demand |
| `hints.vision.minimum_confidence` | option | ✅ | ✅ | ❌ | Windows.Media.Ocr reports no per-word confidence, so every word scores one there and a floor keeps everything; the Vision framework and tesseract score each word |
| `hints.vision.button_min_confidence` | option | ✅ | ✅ | ❌ | Windows.Media.Ocr reports no per-word confidence, so every word scores one there and a floor keeps everything; the Vision framework and tesseract score each word |
| `hints.vision.generic_clickable_min_confidence` | option | ✅ | ✅ | ❌ | Windows.Media.Ocr reports no per-word confidence, so every word scores one there and a floor keeps everything; the Vision framework and tesseract score each word |
| `hints.vision.detect_rectangles` | option | ✅ | ❌ | ❌ | rectangle detection has no OCR answer, so it stays macOS-only even where the vision strategy lands; that half is text-only |
| `hints.vision.rectangle_max_candidates` | option | ✅ | ❌ | ❌ | rectangle detection has no OCR answer, so it stays macOS-only even where the vision strategy lands; that half is text-only |
| `hints.vision.rectangle_min_size` | option | ✅ | ❌ | ❌ | rectangle detection has no OCR answer, so it stays macOS-only even where the vision strategy lands; that half is text-only |
| `hints.vision.rectangle_min_aspect` | option | ✅ | ❌ | ❌ | rectangle detection has no OCR answer, so it stays macOS-only even where the vision strategy lands; that half is text-only |
| `hints.vision.rectangle_max_aspect` | option | ✅ | ❌ | ❌ | rectangle detection has no OCR answer, so it stays macOS-only even where the vision strategy lands; that half is text-only |
| `hide_cursor` | action | ✅ | ❌ | ❌ | a Wayland client may not hide another client's cursor, and the blessed Linux stack is Wayland; Windows has no equivalent either |
| `show_cursor` | action | ✅ | ❌ | ❌ | a Wayland client may not hide another client's cursor, and the blessed Linux stack is Wayland; Windows has no equivalent either |

<!-- END GENERATED PLATFORM SUPPORT -->

---

## Platform Exclusives

Features available on exactly one platform, with why they do not port. This is
a **closed set**: anything not listed here is a gap rather than an exclusive,
whatever the [Capability Matrix](#capability-matrix) currently reports
([ADR 0013](./adr/0013-parity-is-measured-in-words-not-subsystems.md)).

| Feature                                   | Platform | Location                                                | Why it is exclusive                                          |
| ----------------------------------------- | -------- | ------------------------------------------------------- | ------------------------------------------------------------ |
| System cursor hide + virtual-pointer replacement | macOS | `app/modes/cursor_darwin.go`, `adapter/overlay/render/virtualpointer/overlay_darwin.go` | A Wayland client may not hide another client's cursor. X11 could (`xfixes` is already linked in `platform/linux/cgo.go`), but the blessed stack is Wayland, so shipping it on one backend would not be parity |
| Screen-sharing hide                       | macOS    | `platform/darwin/overlay_darwin.m`                      | NSWindow sharing level is a Quartz concept                    |
| Secure input detection                    | macOS    | `platform/darwin/secureinput.go`                        | `CGSessionCopyCurrentDictionary`, a private API; neither X11 nor Wayland has the concept |

Two entries left this table in ADR 0013 and neither is coming back: **smooth
scroll animation** now animates on every backend (X11 in whole notches,
footnote ³), and the **Vision (OCR) hint strategy** was met by tesseract and
`Windows.Media.Ocr`, with only its rectangle-detection half staying macOS-only.

Linux and Windows have no exclusive *user-facing* features. Their unique
elements (evdev, `zwlr_virtual_pointer`, libei, `WH_KEYBOARD_LL`,
`RegisterHotKey`, SDF rendering) are mechanisms serving cross-platform
features, listed in the [Capability Matrix](#capability-matrix).

---

## Known Gaps

Work that is missing, as opposed to deliberately platform-specific.
A gap is anything a person can *write* (an option, a mode flag, an action, a
command) that means less here than it does on macOS, whether or not the
[Capability Matrix](#capability-matrix) reports its subsystem as supported
([ADR 0013](./adr/0013-parity-is-measured-in-words-not-subsystems.md)).

**Linux**

None. Parity is complete on the blessed stack;
[What the labels mean](#what-the-labels-mean) says why the label is still Beta.

The `input`-group membership Wayland global hotkeys need is a host setup step
rather than a gap: Neru warns with the remedy when the listener cannot start,
and [LINUX_SETUP.md](./LINUX_SETUP.md#install-time-environment-adjustments)
carries it.

**Not Linux gaps**, and deliberately so: secure input detection and system
cursor hide are [Platform Exclusives](#platform-exclusives); GNOME Wayland
needing a shell extension for its focused window is Mutter's decision; X11
modifier passthrough is impossible for the display server; and `neru services`
on a non-systemd init is a stated boundary.

**The `CGO_ENABLED=0` Linux build is outside the boundary too**, and says so
itself: cursor, clicks, scroll, hotkeys, keyboard capture, overlay, screen
enumeration, focused app, `neru key` and the `vision` strategy are all
`CodeNotSupported` there, so it announces what kind of build it is once at
startup ([ADR 0012](./adr/0012-the-first-hour-must-not-lie.md)). The tray,
notifications and alerts are pure Go and keep working, which is why the build
exists.

**Windows**

None. Parity is complete.

**Not a Windows gap**: the three `hints.vision.*_confidence` floors are inert
because `Windows.Media.Ocr` reports no per-word confidence, a boundary of the
engine in the same way X11 modifier passthrough is a boundary of the display
server. [What the labels mean](#what-the-labels-mean) says why the label is
Beta.

**macOS**

1. Named keys without a Carbon keycode: `Insert` and `F21` to `F24` validate but
   never fire, because Carbon declares no virtual key code for them. They stay
   in the shared key vocabulary so one config file works on every platform
   ([ADR 0008](./adr/0008-a-vocabulary-has-one-home.md)), and the absence is
   pinned by `internal/architecture/named_key_tables_test.go`.

Otherwise none; macOS is the reference implementation.

---

# Contributor Guide

Guiding principles:

- shared business logic stays in pure Go
- platform-specific code is easy to locate
- Linux backend differences are explicit
- contributors implement in existing slots instead of inventing new file layout
- unsupported features fail loudly with `CodeNotSupported`

## First Stops

Read these before changing platform code:

- [The Three Tiers](#the-three-tiers): **start here**, it decides where your code goes
- [platform/profile.go](../internal/adapter/platform/profile.go): per-subsystem backend family and CGO expectations
- [ports/system.go](../internal/ports/system.go): the main OS contract, plus the optional-extension pattern
- [ports/capabilities.go](../internal/ports/capabilities.go) and [capability_presets.go](../internal/ports/capability_presets.go): the capability registry `neru doctor` reports
- [architecture/platform_slots_test.go](../internal/architecture/platform_slots_test.go): the file-layout rules, as executable checks
- [ARCHITECTURE.md](./ARCHITECTURE.md) and the root [AGENTS.md](../AGENTS.md) conventions

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
`AppWatcherPort`, `SystrayPort`, `FontResolver`.

Optional extensions (Tier 3): `RelativeCursorMover`, `CursorSynchronizer`
and `InstantCursorMover` on `SystemPort`, `HotkeyReleaseRegistrar` and
`HotkeyHealthReporter` on `HotkeyPort`, `OverlayKeyboardPassthroughReporter`
on `EventTapPort`, `OverlayCapabilityReporter` on `OverlayPort`, and
`SyntheticModifierSink` on the `tap.Tap` backend contract (Linux only, because
only X11 cannot tell its own injected key events apart from the user's).

[`keyfeed`](../internal/adapter/keyfeed/) is the reference example: shared
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
enforced by [layering_test.go](../internal/architecture/layering_test.go):

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
[platform_slots_test.go](../internal/architecture/platform_slots_test.go), so
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
  ([cgo_includes_test.go](../internal/architecture/cgo_includes_test.go)),
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
[x11_cgo.go](../internal/adapter/hotkeys/linux/x11_cgo.go), Wayland keyboard
capture in [wayland_cgo.go](../internal/adapter/eventtap/linux/wayland_cgo.go),
shared Linux system fallbacks in
[system_common.go](../internal/adapter/platform/linux/system_common.go).

## Build And Test Commands

Every build and test recipe is catalogued in
[DEVELOPMENT.md](./DEVELOPMENT.md#common-tasks). What matters for platform
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
  family in [backend_linux.go](../internal/adapter/platform/backend_linux.go),
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
[LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md); host setup lives in
[LINUX_SETUP.md](./LINUX_SETUP.md).

## Windows Model

Windows is one backend family, Beta because every mode works and what is there
is proven by CI rather than by use;
[What the labels mean](#what-the-labels-mean) carries the rule that moves it to
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
([mouse_animator.go](../internal/adapter/platform/windows/mouse_animator.go)):
off by default, opt in with `smooth_cursor.move_mouse_enabled`, relative moves
extend the pending endpoint, and position-dependent actions settle the
animation before acting.

## CGO Guidance

**Do not decide CGO usage by OS alone.** CGO is a per-backend decision, and
[profile.go](../internal/adapter/platform/profile.go) is the source of truth.
Current intent:

- **macOS**: CGO required throughout (Objective-C bridge)
- **Linux**: backend-dependent; several backends require it, and `*_nocgo.go`
  variants must still compile and degrade honestly
- **Windows**: pure Go first

Good default instincts: AT-SPI and freedesktop notifications prefer pure Go /
D-Bus; Wayland and compositor integrations often need CGO or native helpers;
Win32 hotkeys, hooks, monitor APIs, and UIA prefer pure Go bindings.

If you introduce a backend that changes the build story, update
[profile.go](../internal/adapter/platform/profile.go), the
[justfile](../justfile), and this document, and state the build assumption in
your PR description and the backend's package comments.

## Hotkeys And Modifiers

Shared code must not hard-code macOS conventions:

- use `Primary` when you mean "the main accelerator modifier"; it maps to
  `Cmd` on macOS and `Ctrl` on Linux/Windows
- keep backend-specific key translation inside `adapter/platform` code
- never leak X11, Wayland, Carbon, or Win32 naming into shared app logic

Relevant files: [config.go](../internal/config/config.go),
[modifiers.go](../internal/domain/action/modifiers.go),
[binder.go](../internal/app/keybinding/binder.go).

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
[`keyfeed`](../internal/adapter/keyfeed/).

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
edit, and [capabilities_test.go](../internal/ports/capabilities_test.go) fails
if a field is added without registering it. Then fill the entry in all three
presets in [capability_presets.go](../internal/ports/capability_presets.go).

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
[capabilities.go](../internal/ports/capabilities.go),
[capability_presets.go](../internal/ports/capability_presets.go), and
[info.go](../internal/app/ipcctrl/info.go). A stub must report `stub`, not
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
| A gap closed or discovered                      | **this file**, [Known Gaps](#known-gaps)                      |
| Which platforms an option, mode flag or action does anything on | the `PlatformSupport()` declaration beside that vocabulary, then `just gensupportref` |
| Desktop-specific setup, protocol support, or a DE workaround | [LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md)         |
| Host dependencies, permissions, or deployment   | [LINUX_SETUP.md](./LINUX_SETUP.md), kept DE-agnostic          |
| A layer boundary, port contract, or data flow   | [ARCHITECTURE.md](./ARCHITECTURE.md)                          |
| A build recipe or test tier                     | [DEVELOPMENT.md](./DEVELOPMENT.md)                            |
| Go style, logging, or naming                    | [AGENTS.md](../AGENTS.md) (Conventions)                       |
| What the project claims to support, at a glance | [README.md](../README.md)                                     |
| What comes next                                 | [ROADMAP.md](./ROADMAP.md), intent and priority only          |

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
