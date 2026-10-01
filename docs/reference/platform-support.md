# Cross-Platform Guide

Neru runs on macOS, Linux, and Windows from one shared Go core. This document
covers both sides of that:

- **[Part 1: Feature Parity Reference](#feature-parity-reference)**: what
  works on each platform, and how it is implemented.
- **[Part 2: Contributor Guide](../contributing/porting.md)**: where platform code
  lives, and how to add to it.

Every claim in Part 1 is derived from code under `internal/adapter/` and
`internal/app/`. **If this document and the code disagree, the code wins**,
and the disagreement is a bug worth fixing here. Where a row or footnote names
a file, test or ADR, that is where the full mechanism is written down; this
document states the claim and points there.

**Related:** [Architecture](../contributing/architecture.md) · [Linux setup](../guide/linux.md) ·
[Linux desktops](../guide/linux-desktops.md) · [Development Guide](../contributing/development.md)

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

**Part 2: [Contributor Guide](../contributing/porting.md)**

- [First Stops](../contributing/porting.md#first-stops)
- [The Three Tiers](../contributing/porting.md#the-three-tiers)
- [File Layout Rules](../contributing/porting.md#file-layout-rules)
- [Backend Packages](../contributing/porting.md#backend-packages)
- [Where To Implement What](../contributing/porting.md#where-to-implement-what)
- [Build And Test Commands](../contributing/porting.md#build-and-test-commands)
- [Linux Backend Model](../contributing/porting.md#linux-backend-model)
- [Windows Model](../contributing/porting.md#windows-model)
- [CGO Guidance](../contributing/porting.md#cgo-guidance)
- [Hotkeys And Modifiers](../contributing/porting.md#hotkeys-and-modifiers)
- [Adding A New Capability](../contributing/porting.md#adding-a-new-capability)
- [Errors And Capability Reporting](../contributing/porting.md#errors-and-capability-reporting)
- [Testing Checklist](../contributing/porting.md#testing-checklist)
- [Documentation Checklist](../contributing/porting.md#documentation-checklist)
- [Contributing Safely](../contributing/porting.md#contributing-safely)

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
[ADR 0013](../adr/0013-parity-is-measured-in-words-not-subsystems.md), and it is
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
([backend_linux.go](../../internal/adapter/platform/backend_linux.go)). This is
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
> [LINUX_DESKTOPS.md](../guide/linux-desktops.md#gnome-wayland) carries the measured
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
| **Cursor move**               | ✅ `CGEventPost` ([`postMouseMoveLocked`](../../internal/adapter/platform/darwin/accessibility_mouse_darwin.m)) | ✅ XTest (`XTestFakeMotionEvent`) | ✅ `zwlr_virtual_pointer` | ✅ libei                | ✅ `SetCursorPos`, glided while a button is held |
| **Mouse buttons / drag**      | ✅ `CGEventPost`         | ✅ XTest ⁷             | ✅ `zwlr_virtual_pointer` ⁷  | ✅ libei ⁷              | ✅ `SendInput` ⁷             |
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
| **Text measurement**          | ✅ CoreText              | ✅ Cairo text extents ¹ | ✅ Cairo text extents ¹      | ✅ Cairo text extents ¹ | ✅ GDI `GetTextExtentPoint32W` ¹ |
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

**Text measurement** (`ports.TextMeasurer`) is the same text layer asked how
much room a string takes, off the draw path and cached per string and font in
the same package. It never hops to a UI thread. On Windows GDI measures for
both renderers. The non-CGO Linux build has no Cairo and reports
`CodeNotSupported`, which leaves callers on their estimate.

The Linux and Windows backends size every box drawn around text from it: hint
badges, the hint search badge, the mode and sticky-modifier indicators,
recursive-grid label plates and the monitor picker's badge (`badge.TextWidth`).
Widths are kept per character and per unit of font size, in a table per family
and weight, so one table serves every size and display scale. Both backends
fill the tables when a configuration is applied (`manager.WarmBadgeTextWidths`),
so a draw measures nothing. They used to give every character 0.7 of the font
size, which a W outgrows and an I never fills, so a box clipped wide labels on
the GDI renderer, which clips text to its rectangle, and sat loose around narrow
ones. macOS has always measured.

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
[required build dependency](../guide/linux.md#build-dependencies). It is a
**permission** rather than a missing capability: the grant is persisted with a
restore token under `$XDG_STATE_HOME/neru/`, only the mode handler's permission
preflight can raise the dialog, and `CheckScreenCapturePermission` reports the
real consent state there and "no gate" elsewhere. Capture is a **region**
operation on every backend, and a region that leaves the screen, spans two
Wayland outputs, or lies on a monitor the user declined to share **fails**
rather than coming back clipped, because a clipped frame cannot say where its
own top-left is. Scaled outputs return physical pixels, as Retina does on
macOS. How KDE streams are placed on outputs is in
[portal_screencast.go](../../internal/adapter/platform/linux/portal_screencast.go).

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

⁷ **Modifiers on injected input (X11, Wayland and Windows).** An X11 pointer
event, a Wayland pointer event and a `SendInput` mouse event all carry whatever
modifiers the keyboard currently holds, so a hotkey chord still held while a
hint was chosen used to make the click a ctrl+click, and a global `Alt+Space`
bound to `action left_click` an alt+click. Neru reads the live key state
(`XQueryKeymap`, `GetAsyncKeyState`), releases the modifiers the injection
would falsify, presses the ones asked for, and undoes both when done, held
across every chunk of an animated scroll and across a drag until its release.
Restoring is the deliberate bias, since the opposite drops a modifier the user
is still holding. On Wayland the keyboard the compositor reads is the evdev
proxy's, so the proxy does the releasing and re-pressing
(`LiftHeldModifiers` / `RestoreLiftedModifiers`), around a button event and
around a scroll, animated or not.
Without a forwarding proxy (no `/dev/uinput`, or the wl-keyboard fallback)
there is nothing to lift, since the compositor reads the physical keyboards
itself, and the click stays modified.

⁸ **Tray and notifications.** The tray icon carries the paused state on every
platform: macOS swaps template glyphs, SNI hosts and the Win32 notification
area get a desaturated tile derived from the running one
([icon/paused.go](../../internal/adapter/systray/icon/paused.go)). Per-item menu
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
([global_hotkey_cgo.go](../../internal/adapter/eventtap/linux/global_hotkey_cgo.go),
[ADR 0014](../adr/0014-the-wayland-keyboard-is-a-proxy.md)). With `/dev/uinput`
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
[LINUX_SETUP.md](../guide/linux.md#systemd-user-service).

**Smooth cursor animation on Linux.** Off by default; opt in with
`smooth_cursor.move_mouse_enabled`. One worker goroutine steps the per-backend
warp toward the target by linear interpolation, latest target wins, and
`WaitForCursorIdle` blocks until it settles
([mouse_animator.go](../../internal/adapter/platform/linux/mouse_animator.go)).
Relative moves on wlroots drain the delta through native relative motion
([relative_animator.go](../../internal/adapter/platform/linux/relative_animator.go))
so they never read the position cache. Clicks stay instant, and
position-dependent actions settle the animation before acting.

---

## Input Injection

Every action type in [action.go](../../internal/domain/action/action.go), click,
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
backend keeps a [`mousestate.Tracker`](../../internal/adapter/platform/mousestate/tracker.go)
recording which buttons are down, where, and with which modifiers. Toggle
actions resolve against it, and `EnsureMouseUp` releases every held button when
Neru returns to idle. On macOS it selects the drag event type for cursor moves,
which Quartz requires. On Windows an application reads a drag only out of
intermediate motion, and a WinUI text control (Windows Terminal's is one) only
out of *relative* motion. A press, one jump and a release select nothing, and
absolute motion, however finely interpolated, selects the single character
under the press. Neru therefore spreads a warp made while a button is held
over a short glide of relative `MOUSEEVENTF_MOVE` deltas. Each step goes out as
two of them, and the second travels at most 4 pixels on either axis, because
Windows doubles a relative move that passes its first mouse threshold and the
move that ends a step may also be the move that ends the drag. Two events then
follow each delta. A `SetCursorPos` puts the pointer on the pixel the move
aimed at, since the pointer speed and threshold settings scale relative motion
and the error would otherwise accumulate across the glide. An absolute
`MOUSEEVENTF_MOVE` at that same pixel redraws the pointer, because Windows
moves it for `SetCursorPos` during a held drag without redrawing it. The
smooth-cursor animator lands its own steps the same way, so enabling
`smooth_cursor.move_mouse_enabled` does not change what an application reads.
The release of a held button waits a few milliseconds after its last motion so
the application processes the move before the button-up. The Linux backends warp
the pointer and let the compositor infer the drag.

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
(`Deps.PublishRegisteredHotkeys`, [hotkey.go](../../internal/app/keybinding/hotkey.go)).
Linux cannot hand it to anybody, since X11's capture is an exclusive
`XGrabKeyboard` and the Wayland proxy owns every press while a mode is open, so
there the mode handler resolves the global table itself after the mode's own
([keymap.go](../../internal/app/modes/keymap.go), `settledKeymaps`). Only chords
carrying Ctrl/Alt/Cmd fall back: a bare key inside a mode is a hint label or a
grid cell key. One consequence shared by both Linux backends: while a mode is
open, a chord bound in the *compositor* rather than in `[hotkeys]` cannot fire.

**One key, one name.** Both Linux readers of `/dev/input` resolve a scan code
through the compositor's XKB keymap
([evdev_xkb_cgo.go](../../internal/adapter/eventtap/linux/evdev_xkb_cgo.go)), and
the X11 tap names the key from the state-resolved **keysym** rather than the
string `XLookupString` returns, so all backends call `Shift+;` the same thing
and XKB options like `ctrl:swapcaps` reach Neru's own bindings. **On a
non-QWERTY layout this decides which physical key a `[hotkeys]` chord answers**:
the one bearing that character on the **reference layout**. A layout is
ASCII-capable when its letter row types ASCII. On X11 the reference is the
active layout when it is ASCII-capable, else the first ASCII-capable layout of
the keymap, else the active layout. The X11 tap reads the keymap when a mode
starts, so a layout added mid-mode applies from the next one. On Wayland the
reference is the first ASCII-capable layout of the compositor's keymap, else
the active layout. Wayland cannot prefer the active layout because the
compositor tells only the focused client which one it is, and Neru never holds
keyboard focus. With two Latin layouts, such as `us` and Dvorak, Wayland
bindings therefore stay on the first. Either way a `us,ru` or `ru,us` keymap
keeps every binding, hint label and grid key on its physical key while Russian
is active. Shift, Caps Lock, NumLock and AltGr still choose the level, and Neru
identifies modifiers in the active layout, which is where remaps like
`ctrl:swapcaps` are defined. `general.kb_layout_to_use` forces the reference by
the layout's XKB name on both backends, and the X11 `[hotkeys]` grab follows
it too. A keysym is named by the
character it types when it types one, and by keysym name otherwise
([wayland_keymap.c](../../internal/adapter/platform/linux/wayland_keymap.c)); the
one key XKB renames under Shift, `ISO_Left_Tab`, folds back to `Tab`, which is
what lets the default `Shift+Tab` hint binding fire.

**Windows names punctuation in the reference layout too.** Neru names letters
and digits by virtual-key code, and every layout keeps `VK_A` to `VK_Z` and
`VK_0` to `VK_9` on the keys of those names, Russian included. Punctuation sits
on the `VK_OEM_*` codes, whose characters come from the layout, so both naming
a key and parsing a `[hotkeys]` chord use the reference layout chosen by the
X11 rule. The active layout is the foreground window's, and the fallback list
is the user's installed layouts
([layout.go](../../internal/adapter/platform/windows/layout.go)). So `` ` `` and
`;` keep answering while Russian is active. `general.kb_layout_to_use` forces
the reference by keyboard layout identifier. `RegisterHotKey` keeps the key a
hotkey resolved to, so Neru checks the reference layout once a second and
registers the hotkeys again when it changes.

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
[passthrough.go](../../internal/app/modes/passthrough.go); only the re-injection
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
([window_origin.go](../../internal/adapter/accessibility/atspi/window_origin.go)):

| Compositor | Source                                                       | Limits                                                                                                                    |
| ---------- | ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------- |
| KDE / KWin | KWin script pushing focused-window geometry over D-Bus ([platform/kwin](../../internal/adapter/platform/kwin)) | Reports on activation and on the focused window moving, resizing or going away; Neru reinstalls the script when KWin restarts. A drag reports its final rectangle. |
| niri       | `niri msg -j focused-window` / `focused-output`              | Floating and fullscreen windows only. **Tiled** windows expose no on-screen position ([niri#2381](https://github.com/niri-wm/niri/issues/2381)), so hints are misaligned there. |
| Sway       | `swaymsg -t get_tree`, focused node `rect` + `window_rect`   | none                                                                                                                       |
| Hyprland   | `hyprctl -j activewindow` `at` / `size`                      | none                                                                                                                       |
| GNOME      | GNOME Shell extension reporting the focused window's frame over D-Bus ([platform/gnomeshell](../../internal/adapter/platform/gnomeshell)) | The daemon installs and enables the extension; the shell loads it at the next login. |
| Anything else (X11, River, Wayfire) | none                                                | X11 needs none, AT-SPI already reports screen coordinates there. The rest report no origin, so hints stay window-relative. |

Each source verifies the reported window matches the AT-SPI frame (size on
all of them, identity as well on KWin and GNOME, because their rectangle is a
cache) and is best-effort: an unavailable origin degrades to unoffset
coordinates rather than misplacing hints. The CLI sources go through
[platform/compositorcli](../../internal/adapter/platform/compositorcli), which
warns when a compositor could not be run or answered garbage and stays quiet
when it answered with no position (nothing focused, a tiled niri window).

**The same sources answer `FocusedWindowBounds`,** which scopes vision and
contour detection, `neru action move_mouse --window` and a grid, recursive
grid or bisect session started with `capture_scope = "window"` to the focused
window
([system_focused_window.go](../../internal/adapter/platform/linux/system_focused_window.go)).
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
| **Window identity**   | panel title `neru-overlay`               | `WM_CLASS` and name `neru-overlay`     | layer-shell namespace `neru-overlay`             | window class `neru-overlay`        |
| **HiDPI**             | dynamic `contentsScale` + backing-change callback | `Xft.dpi`, one global factor  | `wl_output` scale + `wp_fractional_scale_v1` / `wp_viewporter` | per-monitor-v2 DPI aware, `GetDpiForMonitor` per window multiplies fonts, padding, lines and radii |
| **Multi-monitor**     | per-display clamping, screen-change tracking | all monitors enumerated, per-monitor render, live RandR hotplug | one `wl_surface` per output (max 16), live hotplug | cursor-screen tracking, live `WM_DISPLAYCHANGE` hotplug, one panel window per display for monitor_select |
| **Buffers**           | layer-backed, OS-managed                 | single Cairo surface                   | triple-buffered SHM pool                         | canvas bitmap + 2-buffer flip swapchain; one persistent DIB on the fallback |
| **Rounded rects / borders** | NSBezierPath                       | Cairo arc path + stroke                | Cairo arc path + stroke                          | Direct2D rounded rects; software SDF on the fallback |
| **Text**              | NSFontManager                            | Cairo `select_font_face` / `show_text` | Cairo `select_font_face` / `show_text`           | DirectWrite text formats, cached per family and size; cached GDI fonts + `DrawTextW` on the fallback |
| **Coordinate origin** | bottom-left (Y-flipped in the adapter)   | top-left                               | top-left                                         | top-left                           |
| **Thread model**      | main-thread dispatch                     | `renderMu` mutex                       | `renderMu` mutex (also guards `wl_display`)      | dedicated UI thread (`LockOSThread`); draws queue and return, the thread presents |

Grid cell sizes are planned in apparent units on every platform. macOS and
Wayland report logical screen bounds already. X11 and Windows report physical
pixels, so the planner multiplies its cell-size range by the same factor the
overlay scales fonts with (`SystemPort.ScreenScale`), and
`recursive_grid.min_size_width` / `min_size_height` are read the same way.

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
| **Bisect**        | Transition animation           | ✅                         | ✅                         | ✅                          |
| **Bisect**        | Virtual pointer indicator      | ✅                         | ✅                         | ✅                          |
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
> implementation (`recursivegrid.Style.SubKeyPreviewFontSizeIn`).

> **Region-grid labels are fitted to their cells by one implementation on all
> three platforms** ([#1691](https://github.com/y3owk1n/neru/issues/1691)):
> `recursivegrid.Style.FitDraw` answers once per draw, from text measured when
> the Style was built (`ports.TextMeasurer`), and `FitTransition` answers what
> a depth transition holds from its first frame to its last. macOS is handed
> both answers before its animation starts, because its frames never return to
> Go. Its Objective-C copy of the autohide rule and the two tests pinning that
> copy were deleted. The fit takes the display scale, so a dense X11 or Windows
> display no longer keeps a label that its drawn font has outgrown.
> Grid mode fits its labels the same way (`grid.Style.LabelFontSizeFor`), at
> the alphabet's average character width and never hiding one. The monitor
> picker fits its key and the monitor's name to the badge, per monitor
> (`manager.MonitorSelectStyle.FittedTo`). All three backends capped the badge
> at 80% of the monitor and none of them fitted the text drawn in it.

---

## Platform Support Per Word

Every option, mode flag and action carries a platform column, declared once
beside the vocabulary that owns it:
[`internal/config/platform_support.go`](../../internal/config/platform_support.go),
[`internal/domain/modecmd/platform_support.go`](../../internal/domain/modecmd/platform_support.go)
and
[`internal/domain/action/platform_support.go`](../../internal/domain/action/platform_support.go).
The table below is a projection of those declarations, as are the warning the
daemon prints once at load and the `platform_support` row in `neru doctor`
([ADR 0013](../adr/0013-parity-is-measured-in-words-not-subsystems.md)).

It lists only the words whose column is narrower than every platform. The
several hundred that work everywhere are declared too, and
`internal/architecture/platform_support_test.go` fails the build when a word is
neither. Writing one of these where it is inert is never a config error: the
file loads, the daemon runs, and one warning says which lines mean nothing
here, so one configuration can be carried between platforms
([ADR 0008](../adr/0008-a-vocabulary-has-one-home.md)).

This table answers a different question from the
[Capability Matrix](#capability-matrix). The matrix says whether a subsystem
works; this says whether a word a person wrote does anything.

<!-- BEGIN GENERATED PLATFORM SUPPORT: edit the platform_support.go declarations, then run `just gensupportref` -->

| Word | Kind | macOS | Linux | Windows | Why |
| ---- | ---- | --- | --- | --- | --- |
| `general.hide_overlay_in_screen_share` | option | ✅ | ❌ | ❌ | hiding the overlay from a screen share is an NSWindow sharing level, a Quartz concept with no X11, Wayland or Win32 counterpart |
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
| `hints.ignore_clickable_check` | option | ✅ | ❌ | ❌ | the clickable check is AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult it |
| `hints.app_configs.ignore_clickable_check` | option | ✅ | ❌ | ❌ | the clickable check is AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult it |
| `grid.app_configs.ignore_clickable_check` | option | ✅ | ❌ | ❌ | the clickable check is AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult it |
| `recursive_grid.app_configs.ignore_clickable_check` | option | ✅ | ❌ | ❌ | the clickable check is AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult it |
| `bisect.app_configs.ignore_clickable_check` | option | ✅ | ❌ | ❌ | the clickable check is AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult it |
| `scroll.app_configs.ignore_clickable_check` | option | ✅ | ❌ | ❌ | the clickable check is AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult it |
| `app_configs.ignore_clickable_check` | option | ✅ | ❌ | ❌ | the clickable check is AX-specific; the AT-SPI and UIA walks decide what is clickable their own way and never consult it |
| `hints.visible_check_enabled` | option | ✅ | ❌ | ✅ | the visibility hit-test is an AX and UIA question; the AT-SPI walk never consults it |
| `hints.app_configs.visible_check_enabled` | option | ✅ | ❌ | ✅ | the visibility hit-test is an AX and UIA question; the AT-SPI walk never consults it |
| `grid.app_configs.visible_check_enabled` | option | ✅ | ❌ | ✅ | the visibility hit-test is an AX and UIA question; the AT-SPI walk never consults it |
| `recursive_grid.app_configs.visible_check_enabled` | option | ✅ | ❌ | ✅ | the visibility hit-test is an AX and UIA question; the AT-SPI walk never consults it |
| `bisect.app_configs.visible_check_enabled` | option | ✅ | ❌ | ✅ | the visibility hit-test is an AX and UIA question; the AT-SPI walk never consults it |
| `scroll.app_configs.visible_check_enabled` | option | ✅ | ❌ | ✅ | the visibility hit-test is an AX and UIA question; the AT-SPI walk never consults it |
| `app_configs.visible_check_enabled` | option | ✅ | ❌ | ✅ | the visibility hit-test is an AX and UIA question; the AT-SPI walk never consults it |
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
([ADR 0013](../adr/0013-parity-is-measured-in-words-not-subsystems.md)).

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
([ADR 0013](../adr/0013-parity-is-measured-in-words-not-subsystems.md)).

**Linux**

None. Parity is complete on the blessed stack;
[What the labels mean](#what-the-labels-mean) says why the label is still Beta.

The `input`-group membership Wayland global hotkeys need is a host setup step
rather than a gap: Neru warns with the remedy when the listener cannot start,
and [LINUX_SETUP.md](../guide/linux.md#install-time-environment-adjustments)
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
startup ([ADR 0012](../adr/0012-the-first-hour-must-not-lie.md)). The tray,
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
   ([ADR 0008](../adr/0008-a-vocabulary-has-one-home.md)), and the absence is
   pinned by `internal/architecture/named_key_tables_test.go`.

Otherwise none; macOS is the reference implementation.
