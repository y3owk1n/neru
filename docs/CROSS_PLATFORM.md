# Cross-Platform Guide

Neru runs on macOS, Linux, and Windows from one shared Go core. This document
covers both sides of that:

- **[Part 1: Feature Parity Reference](#feature-parity-reference)**: what
  works on each platform, and how it is implemented.
- **[Part 2: Contributor Guide](#contributor-guide)**: where platform code
  lives, and how to add to it.

Every claim in Part 1 is derived from code under `internal/adapter/` and
`internal/app/`. **If this document and the code disagree, the code wins**,
and the disagreement is a bug worth fixing here.

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
is a claim about reliability. Fourteen Linux capabilities landed in a fortnight
and seventeen Windows tickets in one push, on platforms the maintainer does not
daily-drive, each proven by a CI job rather than by use.

**A Beta platform moves to Stable** after six consecutive releases in which no
bug specific to it is filed, meaning one a macOS user would not also hit. Count
bugs *filed* in that window, not ones still open at the end of it. The rule is
the same for Linux and Windows; only the label changes:

```bash
since=$(gh release view <tag-six-back> --json publishedAt --jq .publishedAt)
gh issue list --state all --label "platform: linux" --label bug \
  --search "created:>=${since%%T*}"
```

The query returns candidates. A person still has to discount cross-platform
bugs wearing a platform label, and it sees only what triage labelled.

### Per-platform

| Aspect               | macOS (Darwin)              | Linux                                    | Windows                        |
| -------------------- | --------------------------- | ---------------------------------------- | ------------------------------ |
| **Status**           | **Stable**                  | **Beta**                                 | **Beta**                       |
| **Build tag**        | `darwin`                    | `linux`                                  | `windows`                      |
| **CGO**              | Required (Objective-C)      | Per-backend; most Linux backends need it | Not used (pure Go Win32 / COM) |
| **Primary modifier** | `Cmd`                       | `Ctrl`                                   | `Ctrl`                         |
| **Display stack**    | Cocoa / Quartz              | X11, or Wayland (wlroots / KWin)         | Win32 / DWM                    |
| **Accessibility**    | AXUIElement                 | AT-SPI over D-Bus                        | UI Automation over COM         |
| **Native product**   | Yes (`Neru.app`, codesigned)| Binary + install script                  | Binary                         |

### Linux backends

Linux is not one target. The live backend is detected once at startup from
`XDG_CURRENT_DESKTOP`, `WAYLAND_DISPLAY`, and `DISPLAY`
([backend_linux.go](../internal/adapter/platform/backend_linux.go)). This is
the only place the compositor *family* is decided. The `display_server` field
in `neru info` and `neru doctor` is derived from the matched row rather than
read from the environment a second time.

| Backend                | Detected when                                                          | Status            |
| ---------------------- | ---------------------------------------------------------------------- | ----------------- |
| `x11`                  | `DISPLAY` set, no `WAYLAND_DISPLAY`                                    | Supported         |
| `wayland-wlroots`      | Sway, Hyprland, niri, River, Wayfire, or unset `XDG_CURRENT_DESKTOP`   | Supported         |
| `wayland-kde`          | `XDG_CURRENT_DESKTOP` contains `KDE`                                   | Supported         |
| `wayland-gnome`        | `XDG_CURRENT_DESKTOP` contains `GNOME`                                 | **Not supported** |
| `wayland-other`        | Any other Wayland compositor                                           | **Not supported** |
| `unknown`              | Neither `WAYLAND_DISPLAY` nor `DISPLAY`                                | **Not supported** |

> **GNOME Wayland does not run at all.** `platform.NewSystemPort` returns
> `CodeNotSupported` for `wayland-gnome`, `wayland-other`, and `unknown`, and
> that is the first step of daemon startup, so the daemon exits instead of
> starting degraded. Mutter implements neither `wlr-layer-shell` (overlays) nor
> `wlr-foreign-toplevel-management` (focused app), and exposes no
> input-injection path Neru can use. **Use an X11 session under GNOME.** The
> tables below have no GNOME column: nothing runs there.

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
| **Mouse buttons / drag**      | ✅ `CGEventPost`         | ✅ XTest ⁷             | ✅ `zwlr_virtual_pointer`    | ✅ libei                | ✅ `SendInput`               |
| **Scroll injection**          | ✅ both axes             | ✅ both axes ⁷         | ✅ both axes (uinput, virtual-pointer fallback) | ✅ both axes (uinput, libei fallback) | ✅ both axes                 |
| **Modified scroll (`--modifier`)** | ✅ `CGEventSetFlags` on every chunk | ✅ XTest key hold ⁷ | ✅ virtual keyboard + virtual pointer (uinput on Hyprland ⁹) | ✅ libei | ✅ `SendInput` key hold |
| **Smooth cursor animation**   | ✅ (incl. relative, opt-in) | ✅ incl. relative, opt-in | ✅ incl. relative, opt-in | ✅ incl. relative, opt-in | ✅ incl. relative, opt-in |
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

¹ **Font resolution.** Every platform resolves font *families* through the OS.
A family somebody named resolves to **that name**, not to what the platform
would render in its place. The exception is a family the platform can see is
missing: Linux (fontconfig) and Windows (GDI) send it to the sans baseline,
DejaVu Sans and Segoe UI, rather than to the platform's own substitute, so
`font_family = "Arial"` without Arial installed reports DejaVu Sans on Linux and
not the Liberation Sans that `fc-match Arial` names. A missing serif or mono
family lands on the sans baseline too. macOS, the non-CGO Linux build, and a
build whose fontconfig or GDI cannot be consulted check nothing and hand the
name to NSFont / Cairo / DirectWrite, which substitute at draw time.

The generic names are the same on all three: `sans`, `sans serif`, `serif`,
`mono`, `monospace` and the empty string, matched ignoring case, whitespace and
the separator between words (`internal/adapter/platform/fontgeneric`, ADR 0007).
What each resolves to is the platform's own: Helvetica Neue / Times New Roman /
Menlo on macOS, DejaVu Sans / Serif / Sans Mono on Linux, Segoe UI / Cambria /
Consolas on Windows. Resolved answers are cached under the family name
exactly as written (`internal/adapter/platform/fontcache`); the non-CGO Linux
build re-derives each time.

² **Service management** is the one row whose limit is not the display server:
it needs **systemd**, on every Linux backend. runit, OpenRC and s6 get
`CodeNotSupported` from every `neru services` subcommand, a stated boundary
rather than a gap. See "Service management on Linux" below.

³ **Smooth scroll granularity.** `smooth_scroll` animates everywhere, but only
some primitives can send a step shorter than a wheel notch. `zwlr_virtual_pointer_v1.axis`
carries a fractional value and wlroots forwards it as a continuous
`wl_pointer.axis`; libei's `ei_device_scroll_delta` is pixel-precise and KWin
forwards it the same way. X11 core scrolling is buttons 4 to 7, one notch per
event, and the XTEST pointer has no scroll valuator for the smooth XI2 path.
Windows sits with Wayland: `MOUSEEVENTF_WHEEL` counts 120ths of `WHEEL_DELTA`,
so the animator steps in 120ths of a notch.

**So X11 animates in notches, and a scroll worth one notch is not animated at
all.** The default `scroll.scroll_step` of 50 pixels is exactly one notch
there, so a plain `scroll_down` on X11 arrives as the single wheel click it
always did. From two notches up the same eased curve applies as everywhere
else.

Neru sends the same distance on every backend. On Wayland the animated path
spends that distance as a continuous delta where the unanimated one spends it
as notches, and an application may scale the two differently, so switching the
animation on can change how far a scroll reaches. Wayland steps declare axis
source `continuous` rather than `wheel`, because a wheel source invites a
toolkit to round the fraction back to a detent. Measured on wlroots (sway) by
`TestScrollAtCursor_DeliversSubNotchStepsWithSmoothScroll`, which maps a real
`xdg-shell` window and reads what the compositor delivers. **The X11 and KDE
conclusions are read from the sources named above and are not measured on
hardware**, and neither is the uinput `REL_WHEEL_HI_RES` route: the
headless-sway job reads no input devices at all.

⁴ **Native hint-search field.** Only macOS has a platform text control that
owns keyboard focus and brings the system input method with it. Everywhere else
the query is read from the event tap's key stream, so dead keys and IME
composition do not work there and a hint search takes plain characters. The
search *badge* on screen is a different thing and every platform draws one:
`hints.search_input_ui.*` means what it says on all three, and the badge never
captures a key.

⁵ **Screen capture** is taken per backend rather than through the desktop
portal everywhere, because a consent picker in front of a hint refresh is a
latency and consent-fatigue regression the blessed stack has no need to pay.
X11 reads the root window back with `XGetImage`; wlroots compositors implement
`wlr-screencopy-unstable-v1`, which needs no consent. Windows reads the desktop
DC with `BitBlt` into a 32-bit DIB, no consent gate and no cgo; the process is
per-monitor-v2 DPI aware, so the frame is the region's size in physical pixels.

**KWin implements neither**, so KDE Plasma pays the portal: pixels come from an
`org.freedesktop.portal.ScreenCast` session over PipeWire (`libpipewire-0.3`, a
required [build dependency](./LINUX_SETUP.md#build-dependencies) on every Linux
install). It is a **permission** rather than a missing capability, which is why
`CheckScreenCapturePermission` and `RequestScreenCapturePermission` report the
portal's real consent state there and "no gate" on X11, wlroots and Windows. The
prompt is paid once: the grant is persisted with a restore token in
`$XDG_STATE_HOME/neru/screen-cast.token`, and only the mode handler's
permission preflight can raise the dialog, never a capture. Sources are
requested as monitors with the cursor left out; windows are not asked for,
because a window stream carries no position.

Capture is a **region** operation on every backend, and what comes back covers
exactly the region asked for. A rectangle that leaves the screen, is
degenerate, or spans two monitors on Wayland (`wlr-screencopy` and a ScreenCast
stream are one output each) **fails** instead of coming back clipped, because a
clipped frame carries nothing that says where its own top-left is. On KDE a
region on a monitor the user chose not to share fails the same way. On a scaled
Wayland output the frame is in physical pixels, larger than the logical region
by the scale factor, as a Retina capture is on macOS.

⁶ **Vision on Linux and Windows is text-only**, and permanently so. macOS runs
three Vision requests (text, rectangles, saliency); an OCR engine answers the
first. `hints.vision.detect_rectangles` and the four `rectangle_*` options are
therefore declared macOS-only. The `contour` strategy is a separate,
dependency-free detector on every platform, not an implementation of
`detect_rectangles`. The other fourteen `hints.vision.*` options are read on
Linux as on macOS; Windows reads eleven, because its engine reports no per-word
confidence and the three `*_confidence` floors are declared inert there.

On Linux the engine is **tesseract**, linked through `#cgo pkg-config` like
every other native dependency, and required: a missing `libtesseract.so` stops
the daemon before any Neru code runs. Its **language data is a separate
package**, resolved at use (`TESSDATA_PREFIX` first, then the distribution
paths); a machine with no `eng.traineddata` gets `CodeNotSupported` naming that
file from `VisionPort.Health` and from `DetectElements`. Recognition runs at
word level for `--split-word` and at line level otherwise, LSTM engine in
sparse-text segmentation, scoped to the focused window (full-display OCR takes
seconds where one window takes tens of milliseconds). Recognized text is screen
content: never logged, never written to disk, cleared out of the engine before
each recognition returns.

On Windows the engine is **`Windows.Media.Ocr`**, the WinRT engine every
Windows 10 and 11 desktop ships, driven through raw vtables with no CGO
(`platform/windows/ocr.go`). It needs the **OCR language pack** for one of the
account's profile languages, which a language's *Basic typing* feature
installs; without one `VisionPort.Health` and `DetectElements` report
`CodeNotSupported` naming that remedy. The engine caps both image dimensions at
2600 pixels, so a wider frame (any 4K monitor) is box-averaged down by the
smallest whole factor that fits and the word boxes scaled back. Every word
scores one.

⁷ **X11 modifiers on injected input.** An X11 pointer event carries the
modifiers the server records as **held**, so an injected click or scroll used to
pick up whatever the user's hand was on: `Ctrl+J` bound to a plain
`scroll_down` sent ctrl+scroll, which most applications read as zoom. Neru
reads the live key state with `XQueryKeymap`, releases the modifiers the
injection would otherwise falsify, presses the ones asked for, and undoes both
when done, held across every chunk of an animated scroll. A modifier both held
and asked for is left alone. A drag holds that state for as long as the button
is down: press and release are separate calls, and the release undoes what the
press set up. Letting go of a modifier inside that window is not observed, and
the modifier reads as held until pressed and released once more. Restoring is
the deliberate bias, since the opposite drops a modifier the user is still
holding.

⁸ **Tray and notifications.** The tray icon carries the paused state on every
platform, since it is the only place a user can see that Neru is paused without
pressing a key. macOS swaps two template glyphs. Hosts that render icon bytes
literally (SNI hosts on Linux, the Win32 notification area) get the brand tile
desaturated toward grey, derived from the running tile
([icon/paused.go](../internal/adapter/systray/icon/paused.go)) so the two never
drift. Hover text is the tray icon's own ("Neru - Running" / "Neru - Paused")
on all three. Per-item menu tooltips exist nowhere: `com.canonical.dbusmenu`
defines no per-item tooltip property, so `MenuItem.SetTooltip` in
[systray/linux/systray.go](../internal/adapter/systray/linux/systray.go) is
empty by protocol, as is its Win32 twin.

**Notifications on Windows are balloon tips on that tray icon** (`Shell_NotifyIcon`
with `NIF_INFO`, rendered as toasts on Windows 10 and 11), because WinRT toasts
need an AppUserModelID an unpackaged exe does not have. With
`systray.enabled = false` there is nothing to attach a tip to, so
`ShowNotification` reports `CodeNotSupported` naming that reason. Alerts are
`MessageBoxW` and do not depend on the tray.

⁹ **Hyprland modified scroll.** With a virtual-keyboard modifier held, a
`zwlr_virtual_pointer` scroll produces no event on Hyprland
([#1474](https://github.com/y3owk1n/neru/pull/1474)), so there the modifier
goes out on the virtual keyboard and the scroll on the uinput wheel, leaving
the compositor to merge seat state across two devices. A `wl_display.sync`
confirms the modifier landed before the first notch is written; the release
waits a fixed period, since nothing reports how far a compositor has read a
kernel evdev device. `smooth_scroll` still applies, in whole notches, because
`REL_WHEEL` has no sub-notch value. The compositor is named from
`XDG_CURRENT_DESKTOP` beside the backend detection, not from
`HYPRLAND_INSTANCE_SIGNATURE`, which says which compositor is reachable rather
than which one is running.

¹⁰ **windows/arm64 overlay.** The Direct2D binding is pure Go and passes floats
through Go's stdcall shim, which mirrors integer arguments into the XMM
registers on amd64 only, so windows/arm64 builds the GDI surface alone
(`platform/windows/overlay_dcomp_other.go`).

### Notes on the ⚠️ entries

**Focused app on Wayland.** wlroots and KWin resolve the focused window through
`wlr-foreign-toplevel-management`, which exposes the window's **app_id** (the
identity per-app config keys on) but not its PID, because a Wayland client
cannot read another client's process credentials.
`SystemPort.FocusedApplicationPID` best-effort matches the app_id against
`/proc`; with no match it returns `CodeNotSupported` carrying the app_id rather
than a fabricated number. A session where *nothing* is focused is a different
answer and says so.

**An unfocused desktop is not a failure on X11 either.** `_NET_ACTIVE_WINDOW`
has four ways of not giving a window, reported as two kinds. Nothing focused is
`CodeNotSupported`, so callers degrade as they do on Wayland. A display no
*live* EWMH window manager owns (the `_NET_SUPPORTING_WM_CHECK` handshake, not
the leftover `_NET_SUPPORTED`), a failed read and a malformed property are
`CodeActionFailed`, each naming which. `_NET_WM_PID` splits the same way: a live
window that publishes no pid is `CodeNotSupported`, a window that closed under
the query is `CodeActionFailed`. `FocusedWindowBounds` on X11 tells the same two
apart: nothing focused is `found=false` with no error, the rest is an error, so
a caller widening to the active screen knows whether it is obeying an answer or
guessing.

**App watcher.** macOS gets focus changes from an NSWorkspace observer. Linux
subscribes to a backend focus-change fd (`linux.SubscribeFocusedApp`: X11 event
fd, or the wlroots toplevel manager) in `appwatcher/platform_linux.go` and
re-samples on each wake, with a 3s safety re-sample against coalesced events;
with no fd it polls `FocusedAppID` every 400ms. The identity is `WM_CLASS` (X11)
or `app_id` (Wayland). A sibling goroutine watches a display-configuration fd
and dispatches screen-parameter changes, so monitor hotplug regenerates
overlays. Windows installs an `EVENT_SYSTEM_FOREGROUND` hook through
`SetWinEventHook` on its own message-loop thread
(`appwatcher/platform_windows.go`) and resolves each foreground HWND to the
**executable path**, the identity `GetForegroundWindow` already gives. Display
changes come from a hidden top-level window receiving `WM_DISPLAYCHANGE` and
`WM_DPICHANGED` (`platform/windows/display_watcher.go`), coalesced into one
screen-parameters event. On both, only activate, deactivate and screen-params
are emitted; launch, terminate and Mission Control stay macOS-only.

**Global hotkeys on Wayland.** No Wayland protocol lets an ordinary client
register a global hotkey, so Neru matches chords itself on the evdev keyboard
proxy, the one reader of `/dev/input/event*` that the in-mode capture also runs
on ([global_hotkey_cgo.go](../internal/adapter/eventtap/linux/global_hotkey_cgo.go),
[evdev_proxy_cgo.go](../internal/adapter/eventtap/linux/evdev_proxy_cgo.go),
[ADR 0014](adr/0014-the-wayland-keyboard-is-a-proxy.md)). With `/dev/uinput`
writable the proxy holds every keyboard from daemon launch, whether or not
`[hotkeys]` is set, and re-emits it through a uinput device of its own, so a
matched chord is withheld from the focused app. It never keeps a keyboard that
has a key down: a daemon or a remapper started from a compositor binding is
held once the binding's modifier comes up, so no modifier is left stuck in the
compositor's picture of the device. Without it the
proxy reads passively, the chord matches all the same, and the app receives it
too. The process needs read access to `/dev/input` (the `input` group) and a
CGO build; a `CGO_ENABLED=0` build gets a stub whose `Start` reports
`CodeNotSupported` ([global_hotkey_nocgo.go](../internal/adapter/eventtap/linux/global_hotkey_nocgo.go)).
Either way Neru warns once with the remedy that fits and names the fallback,
binding `neru <mode>` in the compositor. While a mode is active the proxy hands
every press to the mode session, which is what **A global chord while a mode is
active** below is about.

**Native alerts on Linux.** Notifications and alerts both go to the session's
freedesktop notification daemon over D-Bus, in pure Go, so a `CGO_ENABLED=0`
build shows them too. An alert differs from a notification only in insistence:
critical urgency and no expiry. It is *not* modal: `NSAlert` stops the world and
returns which button was pressed, and no Wayland or X11 client can do that, so
a Linux alert informs rather than asks and callers take the safe default. A
missing config file therefore starts Neru on built-in defaults and says so,
instead of offering create / defaults / quit. Delivery needs a notification
daemon (mako, dunst, or the desktop's own) running or D-Bus activatable, which
`neru doctor` counts as present. With none, `ShowNotification` and `ShowAlert`
report `CodeNotSupported`, `neru doctor` downgrades the notifications row with
what to install, and the two startup alerts fall back to stderr.

**Service management on Linux.** The mechanism is a **systemd user unit**
anchored on `graphical-session.target`: `After=` and `WantedBy=` it because
every backend needs a display server, `PartOf=` it so a logout/login cycle
restarts the daemon instead of leaving an orphan. Coverage is systemd and no
other init system. What answers the question is systemd's runtime marker,
`/run/systemd/system`, not `systemctl` on `PATH`, which ships in packages
installed on machines running something else. Where the unit is written and
how a user drives it: [LINUX_SETUP.md](./LINUX_SETUP.md#systemd-user-service).

**Smooth cursor animation on Linux.** Off by default; opt in with
`smooth_cursor.move_mouse_enabled`. When enabled, `SystemAdapter.MoveCursorToPoint`
routes through `smoothCursorAnimator`
([mouse_animator.go](../internal/adapter/platform/linux/mouse_animator.go)):
one worker goroutine samples the current position, then steps the per-backend
warp (XTest / `zwlr_virtual_pointer` / libei) toward the target by linear
interpolation, coalescing so the latest target wins, and `WaitForCursorIdle`
blocks until it settles. It covers the flows macOS animates (grid cursor-follow,
`move_mouse`, selection moves); clicks stay instant. On Wayland the start point
comes from the client-side cursor cache, so a stale read skews the glide path,
never the landing point. Relative (hjkl) moves animate over
`smooth_cursor.relative_movement_duration`: X11 and KDE extend the absolute
animator's endpoint, wlroots drains the delta in integer chunks through native
relative motion ([relative_animator.go](../internal/adapter/platform/linux/relative_animator.go))
so it never reads the position cache. Position-dependent actions settle the
in-flight animation before acting.

---

## Input Injection

Every action type in [action.go](../internal/domain/action/action.go), click,
per-button down/up/toggle, absolute and relative moves, drag-while-held and
scroll, is dispatched through the shared `InfraAXClient.PerformAction`. The
dispatch, the action set and the mode logic are platform-neutral Go; only the
final injection primitive differs:

| Platform              | Primitive                                                                    |
| --------------------- | ---------------------------------------------------------------------------- |
| macOS                 | `CGEventPost` (`kCGEventMouseMoved` / `*MouseDragged` for moves)              |
| Linux X11             | XTest (`XTestFakeMotionEvent`, buttons 1/2/3, scroll 4/5 vert. + 6/7 horiz.)  |
| Linux Wayland wlroots | `zwlr_virtual_pointer` (+ `/dev/uinput` for scroll)                           |
| Linux Wayland KDE     | libei via `org.freedesktop.portal.RemoteDesktop` (+ `/dev/uinput` for scroll) |
| Windows               | `SendInput` / `SetCursorPos`                                                  |

Scrolling behaves the same on all three platforms, both axes included. Windows
posts `MOUSEEVENTF_HWHEEL` for the horizontal component with the sign flipped,
because Win32 reads a positive horizontal notch as right where the others read
it as left.

**Modifiers on a scroll** reach the primitive by two routes, because only one
primitive has a field for them. macOS stamps `CGEventSetFlags` on the scroll
event, always (the empty set included, since a NULL-source event inherits the
ambient session modifiers otherwise) and on every chunk of an animation. The
other three press the real key, scroll, and release it. On X11 that key event
feeds back into Neru's own `XGrabKeyboard`, so each one is announced to the
event tap before it goes out and consumed on the way back in; otherwise
`sticky_modifiers` latched a modifier nobody pressed. On Wayland the modifier
can only go out on the virtual keyboard (libei on KDE), so a modified scroll
skips the uinput batch and goes out on the wlroots/libei seat, everywhere but
Hyprland (footnote ⁹). A path with no backend to press through answers
`CodeNotSupported`; none scrolls unmodified and reports success.

**Held mouse buttons.** Press and release are separate actions, so every
backend keeps a [`mousestate.Tracker`](../internal/adapter/platform/mousestate/tracker.go)
recording which buttons are down, where, and with which modifiers. It drives
three behaviors identically everywhere: toggle actions resolve against it,
`EnsureMouseUp` releases every held button when Neru returns to idle, and on
macOS it selects the drag event type for cursor moves. Quartz requires
`kCGEventLeftMouseDragged` and friends with a matching button number instead of
`kCGEventMouseMoved`; X11, Wayland and Windows warp the pointer and let the
compositor infer the drag.

---

## Keyboard Capture And Hotkeys

| Aspect                | macOS                  | Linux X11               | Linux Wayland                            | Windows                 |
| --------------------- | ---------------------- | ----------------------- | ---------------------------------------- | ----------------------- |
| **In-mode capture**   | `CGEventTapCreate`     | `XGrabKeyboard`         | evdev proxy (lifetime `EVIOCGRAB` + uinput re-emit), wl-keyboard fallback | `WH_KEYBOARD_LL`        |
| **Global hotkeys**    | Per-key CGEventTap     | `XGrabKey`              | Chord matcher on the evdev proxy         | `RegisterHotKey`        |
| **CGO needed**        | Yes                    | Yes                     | Yes                                      | No                      |
| **Press/release**     | ✅ separate callbacks  | ✅ KeyPress/KeyRelease  | ⚠️ press-only in some configs            | ✅ `WM_HOTKEY` flags    |
| **Modifier passthrough** | ✅                  | ❌ grab is all-or-nothing | ✅ evdev only                          | ✅ hook forwards per event |
| **`PostModifierEvent`** | ✅                   | ✅                      | ✅ (`zwp_virtual_keyboard_v1`)           | ✅ `SendInput`          |
| **Sticky modifiers**  | ✅                     | ✅                      | ✅                                       | ✅                      |
| **Capture files**     | `eventtap/darwin/`     | `eventtap/linux/x11_cgo.go` | `eventtap/linux/evdev_proxy_cgo.go`, `evdev_session_cgo.go`, `wayland_cgo.go` | `eventtap/windows/` |
| **Hotkey files**      | `hotkeys/darwin/`      | `hotkeys/linux/x11_cgo.go`  | `hotkeys/linux/manager.go` + `eventtap/linux/global_hotkey_cgo.go` | `hotkeys/windows/` |

There is no separate Wayland hotkey file. The Wayland path lives in the common
`hotkeys/linux/manager.go`, which delegates to the evdev listener in the
eventtap package.

**A global chord while a mode is active.** A `[hotkeys]` binding keeps working
from inside a mode on macOS, Windows and Linux Wayland, and each gets there its
own way, because whichever mechanism can see the chord has to be the only one
that runs it. macOS hands it back: the in-mode tap looks the chord up in the
hotkey table the app pushed into it and returns the event untouched, so the
per-hotkey tap fires ([eventtap_darwin.m](../internal/adapter/platform/darwin/eventtap_darwin.m)).
Windows does the same for a Ctrl/Alt/Cmd chord one layer up: the low-level hook
passes it on without dispatching and leaves it to `RegisterHotKey`. Both are
told which chords the backend *took* rather than which ones the
configuration asked for (`Deps.PublishRegisteredHotkeys`,
[hotkey.go](../internal/app/keybinding/hotkey.go)), because a chord another
process owns is refused and handing that one back would drop it. Linux cannot
hand it to anybody: X11's in-mode capture is an exclusive `XGrabKeyboard`, and
the Wayland proxy hands every press to the mode session while one is open. So
there the chord reaches the mode handler, which resolves the global table
itself after the active mode's own table
([keymap.go](../internal/app/modes/keymap.go), `settledKeymaps`). That fallback
is shared code, unreachable on the two platforms whose taps hand the
chord back. Only chords carrying Ctrl/Alt/Cmd fall back: a bare key inside a
mode is a hint label or a grid cell key.

**X11 assembles chords from the keysym.** The X11 in-mode tap names the key
from the **keysym** `XLookupString` returns rather than from the string,
because with Ctrl held the two disagree (`Ctrl+C` gives `\x03` as a string and
`XK_c` as a keysym), and prepends the modifiers it tracks (`x11ChordFromLookup`,
[x11_cgo.go](../internal/adapter/eventtap/linux/x11_cgo.go)). The keysym is
state-resolved, so Shift has chosen the level the same way
`xkb_state_key_get_one_sym` does for the evdev reader, which is what makes both
backends call `Shift+;` the same thing. A keysym outside Latin-1 falls back to
the character the server produced, unprefixed. One consequence of the exclusive
capture is shared by both Linux backends: while a mode is open, a chord bound
in the *compositor* rather than in `[hotkeys]` cannot fire.

**One key, one name.** Both Linux readers of `/dev/input`, the in-mode tap and
the passive hotkey listener, resolve a scan code through the compositor's XKB
keymap (`keyName`/`modifierName`,
[evdev_xkb_cgo.go](../internal/adapter/eventtap/linux/evdev_xkb_cgo.go)). They
have to agree, because only one of them sees any given press. Following the
keymap is what makes a binding mean the key that *types* that character, and
what lets XKB options like `ctrl:swapcaps` reach Neru's own bindings. **On a
non-QWERTY layout this decides which physical key a `[hotkeys]` chord answers**:
the one bearing that character on the active layout. The keysym is named by
the character it types when it types one, and by keysym name only otherwise
(`neru_xkb_keysym_name`,
[wayland_keymap.c](../internal/adapter/platform/linux/wayland_keymap.c)), the
rule the X11 tap applies too. Shift has already chosen the level by the time a
keysym exists, and the shifted level's *name* is not its character (`Shift+[`
is `braceleft`), so a hand-written name table could never be complete. The one
named key XKB renames under Shift, `ISO_Left_Tab`, folds back to `Tab` on both
backends, which is what lets the default `Shift+Tab` hint binding fire. Fold
rows are keyed by the name libxkbcommon answers, the first one
`xkbcommon-keysyms.h` lists: the page keys are `Prior` and `Next`, and a row
keyed `Page_Up` was dead.

**Modifier passthrough (Wayland evdev, and Windows).** While a mode is active
Neru captures the keyboard exclusively, so shortcuts it does not bind
(`Ctrl+C`, `Ctrl+Tab`) are swallowed. With `general.passthrough_unbounded_keys`,
unbound Ctrl/Alt/Cmd chords reach the focused app instead. On the Wayland evdev
backend the proxy keyboard is the only keyboard the compositor sees, so a chord
the mode lets through is re-emitted on it together with the modifiers the user
is physically holding, and each stays the compositor's until released
(`evdevSession.handlePress` and `evdevProxy.forwardWithheld`). It is **not**
available on X11 (an `XGrabKeyboard` routes Neru's own XTest events back to
itself, and `XSendEvent` is ignored by most apps) nor on the rare wl-keyboard
fallback, which has no injection path. Windows needs no re-injection: a
`WH_KEYBOARD_LL` hook forwards or blocks each event on its own
(`eventtap/windows/tap.go`, `handleKey`). Classification (blacklist,
mode-intercepted keys, the mode's own hotkeys, and the global chords it falls
back to) and the post-passthrough hint refresh are shared in
[passthrough.go](../internal/app/modes/passthrough.go); only the final
re-injection is backend-specific. `general.should_exit_after_passthrough` exits
the mode after a passthrough. Both lists are re-derived whenever a mode opens,
the configuration is replaced, hints refresh after a passthrough, or the
focused application changes under an open mode, which keeps per-app overrides
meaningful after passing `Cmd+Tab` through.

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

macOS builds the richest tree by a wide margin: multiple window and system
sources, per-app strategy overrides, the Vision fallback, and deduplication.
Linux walks a single tree with tesseract beside it. Windows fetches the
window's control-view tree in one cached UI Automation query with
`Windows.Media.Ocr` beside it; the ⚠️ is the control view, since an element a
provider exposes only in the raw view is not a hint, and the answer is a role
or filter default, never a per-app branch.

**Linux is ⚠️, not a stub.** `atspi.Client` enables assistive-tech mode, finds
the active frame, and walks it (`ClickableNodes`) emitting native AT-SPI role
names. Configured roles are resolved into that vocabulary at config load
(`element.ResolveRoles`), so both sides of the filter speak AT-SPI. The
`BuildTree` stub in `native/linux/tree.go` is the macOS-style tree API and is
**not** on the Linux hints path. The ⚠️ is coverage: it depends on each app
exposing AT-SPI.

**Chromium and Electron apps on Linux** do not expose their web-content tree
over AT-SPI by default, and unlike macOS there is no per-app attribute Neru can
toggle to force it. The result is a frame with a single empty child. Launch the
app with `--force-renderer-accessibility`. Native GTK/Qt apps and Firefox need
no flag. This is Chromium behavior, not a Neru limitation.

**Picking the active frame on Wayland.** The AT-SPI `ACTIVE` state is
unreliable on wlroots compositors (the focused window can report `ACTIVE=false`
while background frames report `ACTIVE=true`), so Neru matches the AT-SPI frame
against the compositor's focused **app_id** from
`wlr-foreign-toplevel-management`, falling back to the `ACTIVE`/`SHOWING`
heuristic on X11 or when no app_id is available (`findActiveFrame` in
`atspi/scan.go`).

**Window-origin offset on Wayland.** A Wayland client cannot know its own
on-screen position, so AT-SPI reports element coordinates relative to the
window. Neru offsets them by the focused window's screen origin, supplied by a
compositor-specific `windowOriginSource`
([window_origin.go](../internal/adapter/accessibility/atspi/window_origin.go))
chosen from the detected backend:

| Compositor | Source                                                       | Limits                                                                                                                    |
| ---------- | ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------- |
| KDE / KWin | KWin script pushing focused-window geometry over D-Bus ([platform/kwin](../internal/adapter/platform/kwin)) | Reports on activation, on the focused window's geometry changing, and on it going away, so the cache follows a drag, resize, tile or maximize and empties when the desktop is focused. Neru watches `org.kde.KWin` on the session bus and reinstalls the script when KWin restarts. A drag reports its final rectangle, so a mid-drag query reads the start position. |
| niri       | `niri msg -j focused-window` / `focused-output`              | Floating and fullscreen windows only. **Tiled** windows, including a maximized column, expose no on-screen position ([niri#2381](https://github.com/niri-wm/niri/issues/2381)), so hints are misaligned there. |
| Sway       | `swaymsg -t get_tree`, focused node `rect` + `window_rect`   | none                                                                                                                       |
| Hyprland   | `hyprctl -j activewindow` `at` / `size`                      | none                                                                                                                       |
| Anything else (X11, River, Wayfire) | none                                                | X11 needs none, AT-SPI already reports screen coordinates there. The rest report no origin, so hints stay window-relative. |

Each source verifies the reported window size matches the AT-SPI frame (a
focus change can race the query) and is best-effort: an unavailable origin
degrades to unoffset coordinates rather than misplacing hints. The KWin source
checks identity as well as size, because its rectangle is a cache: the script
reports `resourceClass`, `resourceName` and caption, the AT-SPI frame carries
the app_id and title it was selected with, and a disagreement means the cached
rectangle belongs to a different window. Both comparisons are written to be
sure before they refuse (reverse-DNS app_id spelling tolerated, caption prefix
accepted in either direction, an identity neither side reported is not a
mismatch), because a false reject unoffsets hints that were placed correctly.

**A compositor that did not answer is not a compositor with no origin.** The
three CLI sources go through
[platform/compositorcli](../internal/adapter/platform/compositorcli), which
reports a CLI that could not be run, exited non-zero, timed out or printed
something undecodable as a failure naming the command, at `warn`. A compositor
that *did* answer and has no position to give (nothing focused, a tiled niri
window) stays a plain not-found, so ordinary layout never warns.

**The same sources answer `FocusedWindowBounds`,** which scopes vision and
contour detection and `neru action move_mouse --window` to the focused window.
The KWin arm of
[system_focused_window.go](../internal/adapter/platform/linux/system_focused_window.go)
reads the cache the AT-SPI path offsets by, and the wlroots arms use the same
`compositorcli` query. A Wayland compositor with no source (River, Wayfire)
reports `CodeNotSupported` there rather than "no focused window": both send the
caller to the active screen, but only one says so.

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
| **HiDPI**             | dynamic `contentsScale` + backing-change callback | `Xft.dpi`, one global factor  | `wl_output` scale + `wp_fractional_scale_v1` / `wp_viewporter` | per-monitor-v2 DPI aware |
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
| **Grid**          | What an open subgrid shows     | ✅ the subgrid alone       | ✅ the subgrid alone       | ⚠️ the parent cells return under it on the next repaint |
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
> hidden, is separate from the two grid indicators above and is macOS-only:
> `virtualpointer.Overlay` is a no-op on every non-darwin build, paired with
> `CGDisplayHideCursor`, which has no equivalent elsewhere.

> **`hints.ui.placement` means the same thing on all three platforms.** `top`
> puts the badge above the element's centre with an arrow pointing down at it,
> `center` over it with no arrow, `bottom` below it with an arrow pointing up
> (the default). Linux and Windows take the offsets and the arrow from one
> implementation (`adapter/overlay/render/badge.PlaceHint`), so a placement
> lands on the same pixel on both. macOS computes its own in Objective-C (the
> deliberate exception ADR 0007 records) with a shorter, wider arrow, so a badge
> sits a few pixels closer to its element there. On Windows the Win32 surface
> has no path primitive, so the arrow is a triangle over a slightly larger one
> in the border colour, which leaves the badge's own edge running across the
> arrow's base ([#1303](https://github.com/y3owk1n/neru/issues/1303)).

> **`recursive_grid.ui.sub_key_preview` is one drawing on all three platforms**
> ([#1297](https://github.com/y3owk1n/neru/issues/1297)). Each backend divides
> the cell by the *next* level's grid dimensions and draws the key that selects
> each sub-cell in its own place; the centre sub-cell of an odd-by-odd division
> is left blank for the cell's own label, and nothing is previewed at the
> deepest level. `sub_key_preview_autohide_multiplier` measures a **sub-cell**,
> which must reach `sub_key_preview_font_size × multiplier` in both width and
> height, from one implementation (`recursivegrid.Style.ShowSubKeyPreviewIn`,
> with the macOS copy held to it by
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

Two entries left this table in ADR 0013 and neither is coming back. **Smooth
scroll animation** now animates on every backend, with X11 limited to whole
notches (footnote ³), and a limit on one backend is that backend's documented
limit rather than an exclusive. The **Vision (OCR) hint strategy** was met on
Linux by tesseract and on Windows by `Windows.Media.Ocr`; its
rectangle-detection half has no OCR answer, so `detect_rectangles` and the four
`rectangle_*` options stay macOS-only and are declared as such.

Linux and Windows have no exclusive *user-facing* features. Their unique
elements (evdev, `zwlr_virtual_pointer`, libei, the Wayland sync-cursor
surface, `WH_KEYBOARD_LL`, `RegisterHotKey`, SDF rendering) are mechanisms
serving cross-platform features, listed in the
[Capability Matrix](#capability-matrix).

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
carries it as install-time item 2.

**Not Linux gaps**, and deliberately so: secure input detection and system
cursor hide are [Platform Exclusives](#platform-exclusives); GNOME Wayland and
COSMIC are supported-desktop decisions, not capabilities; X11 modifier
passthrough is impossible for the display server (`XGrabKeyboard` is
all-or-nothing and `XSendEvent` is ignored by most applications); and
`neru services` on a non-systemd init is a stated boundary.

**The `CGO_ENABLED=0` Linux build is outside the boundary too**, and says so
itself. It is a distribution convenience: cursor, clicks, scroll, hotkeys,
keyboard capture, overlay, screen enumeration, display hotplug, focused app,
`neru key` and the `vision` strategy are all `CodeNotSupported` there, so it
announces what kind of build it is once at startup, naming what will not work
and how to leave it ([ADR 0012](./adr/0012-the-first-hour-must-not-lie.md)). A
CGO build never prints it. The tray, notifications and alerts are pure Go and
keep working, which is why the build exists.

**Windows**

None. Parity is complete; the `feed` action and `neru key` inject through
`SendInput`, the call the pointer and modifier passthrough already use.

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
- [ports/font.go](../internal/ports/font.go): the FontResolver port
- [architecture/platform_slots_test.go](../internal/architecture/platform_slots_test.go): the file-layout rules, as executable checks
- [ARCHITECTURE.md](./ARCHITECTURE.md) and the root [AGENTS.md](../AGENTS.md) conventions

Contributing Linux support? Nothing is reserved and waiting for you. Read the
Linux files the package already has before writing anything. A
single-platform directory such as `internal/adapter/platform/linux/` drops the
OS token and splits by backend (`system_x11_cgo.go`,
`system_wayland_wlroots_cgo.go`), while a mixed package carries it
(`internal/adapter/platform/factory_linux.go`,
`internal/adapter/overlay/backend_linux.go`).

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

Optional extensions (Tier 3): `RelativeCursorMover` and `CursorSynchronizer`
on `SystemPort`, `HotkeyReleaseRegistrar` and `HotkeyHealthReporter` on
`HotkeyPort`, `OverlayKeyboardPassthroughReporter` on `EventTapPort`,
`OverlayCapabilityReporter` on `OverlayPort`, and `SyntheticModifierSink` on
the `tap.Tap` backend contract (Linux only, declared in a `_linux.go` file
beside `Tap`, because only X11 cannot tell its own injected key events apart
from the user's).

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
Examples: `accessibility/priming_*.go`, `accessibility/supplementary_*.go`,
`appwatcher/platform_*.go`, `ipc/transport_unix.go`.

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

Two rules:

- **Declare it in `ports`.** An interface defined in the consuming package is
  undiscoverable to a contributor on another platform.
- **The caller must have a working fallback.** An optional extension is an
  optimization or a platform-native shortcut, never the only path.

Adapters opting in should assert it: `var _ ports.RelativeCursorMover =
(*SystemAdapter)(nil)`, so a signature drift fails to compile instead of
silently downgrading the platform to the generic path.

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

The third rule has three deliberate escapes, all narrow:

- **Shared vocabulary**: `adapter/ipc` (the CLI/daemon wire protocol),
  `adapter/logger`, and `adapter/platform` (the SystemPort factory plus the
  `Profile` that `neru doctor` prints). These are data and plumbing, not OS
  behavior.
- **Composition root**: `wiring.go`, `startup_phases.go`, `cmd/neru/main.go`.
  Wiring adapters to ports is their job.
- **Build-tagged dispatch**: any `*_darwin.go` / `*_linux*.go` /
  `*_windows.go` / `*_other.go` file in the app layer is Tier 2.

Anything else is a violation. `knownLayeringExceptions` exists for edges that
cannot be fixed in the same change; it is **empty**, and a second test fails
if an entry stops being a real violation, so the list can only shrink.
`internal/adapter/overlay` carried an entry until #1213 and retired it by
moving what did not belong above the line (the per-mode `Context` types, which
are mode state and live in `internal/app/components/`) rather than relocating
the render models that do belong below it.

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
dropped, because the directory carries it. `overlay/linux/wayland_cgo.go` and
`platform/linux/system_x11_cgo.go` keep only the axes that still vary; a
`system_linux_x11_cgo.go` inside `platform/linux/` would say linux twice. That
is why the four Linux backend rows above hold no files today: every Linux
backend split lives inside a single-platform directory. The rows are the
spelling to use if a mixed package ever needs one.

The `*_integration_cgo.go` row exists for one situation and should stay rare.
Go rejects `import "C"` in a `_test.go` file, so an integration test that needs
C (`accessibility/native/linux/scroll_probe_integration_cgo.go`, mapping a
Wayland window to measure what a compositor delivers) puts it in a non-test
file. The `integration` term keeps that file out of every product build, and
the C stays inline in the cgo preamble, because a `.c` file beside it would
compile into the package unconditionally.

What the guardrail test checks:

- A file constrained to exactly one GOOS must carry that OS as a name token. A
  `tree.go` that is secretly `//go:build darwin` is invisible to anyone
  scanning the directory.
- A file whose constraint is a pure negation is a fallback and must be named
  `*_other.go`. `_stub.go`, `_default.go`, `_fallback.go`, `_noop.go` and
  friends are rejected: one slot, one spelling.
- A file gated on cgo must say so: `*_cgo.go` or `*_nocgo.go`.
- Every file in a **single-platform package** declares its OS tag. The set of
  exempt directories is derived from the tree: a directory earns the exemption
  when every file in it targets the same one OS.
- Every relative `#include` resolves
  ([cgo_includes_test.go](../internal/architecture/cgo_includes_test.go)). A
  broken include is invisible to `go vet` and to `just check-cross`, since
  `CGO_ENABLED=0` skips the file.

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
adapter and a small build-tagged factory, usually ten lines, which is the only
place that knows which implementation exists.

### When a backend does not earn a package

The test is whether every platform has something substantial to say. If one
does and the others answer in eighty-line stubs, build-tagged files in a single
package are clearer. That is the case for
`overlay/render/{grid,hints,recursivegrid,modeindicator,stickyindicator}`: each
is one real renderer plus small stubs, and `overlay_other.go` is the obvious
file to open.

### Giving a capability its own packages

A package that reads as "shared code plus platform files" is usually one
generic shell specialised by build-tagged concrete types. Creating a backend
package for it is three moves in order:

1. **Find the seam.** List the methods the shell calls on the platform type.
2. **Extract the contract into a leaf package** (`accessibility/ax`,
   `eventtap/tap`). It has to be a leaf: the backends import it and the factory
   imports the backends, so anything else is an import cycle.
3. **Move each platform into a package behind a build-tagged factory.**

When the shell talks to *package-level* symbols rather than to methods on a
value, alias instead of abstracting. `accessibility/native` works this way:
each platform's files live in their own package and a build-tagged file aliases
the four types and binds the functions, leaving the shell platform-agnostic
without an interface.

Two traps: **named function types do not interchange** (a method taking
`darwin.Callback` does not satisfy an interface wanting `func(string)`, so put
callback types in the contract package), and **typed nil** (a factory returning
a concrete `*T` as an interface hands back a non-nil interface holding a nil
pointer; `staticcheck` reports this as SA4023).

### Where the render models live

`hints.Hint`, `grid.Style` and the other render models sit under
`adapter/overlay/render/` rather than in the domain, because `hints.Hint`,
`hints.StyleMode` and `hints.Overlay` are one concept every backend needs all
three of, and splitting them by layer produces two packages named `hints`.
Nothing above the overlay names them any more (#1213). The per-mode `Context`
types, which are mode state rather than render models, live in
`internal/app/components/{hints,grid,recursivegrid}`.

`grid.Style`, `recursivegrid.Style` and `hints.StyleMode` are each declared
once for every platform. Their fields hold the values the configuration writes,
and the packed-ARGB and float forms Cairo and GDI want are accessors that
convert at the point of use. When a type looks platform-specific, check whether
it differs in meaning or only in representation; the second kind belongs in an
accessor.

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
[DEVELOPMENT.md](./DEVELOPMENT.md#common-tasks). Two apply specifically to
platform work:

- `just build && just test-foundation`: the cross-platform-safe baseline to run
  before touching anything.
- `just release-ci-linux <arch> <version>` / `just release-ci-windows <arch>
  <version>`: the tagged release binaries CI produces.

Only the target OS can run `just test` meaningfully, since integration tests
are tagged per-OS.

### `just build-linux` needs a Linux-targeting C compiler

`just build-windows` cross-compiles from any host, because Windows is a CGO-off
build. `just build-linux` does not: Linux needs CGO for the X11 and Wayland
backends, and a macOS clang compiles Go's cgo runtime against the macOS SDK and
fails. The recipe checks the compiler's target triple up front and refuses with
the alternatives. From a macOS host, use:

- `just lint-cross`: compiles and lints the linux/amd64 build with CGO on, in
  Docker
- `just check-cross`: a fast CGO-off type-check of the Linux and Windows
  builds, no Docker needed
- `CGO_ENABLED=0 GOOS=linux GOARCH=<arch> go build ./cmd/neru`: a pure-Go
  Linux binary. The CGO-only backends compile out, so it is not the shipped
  product

The guard fails open: it only refuses when the compiler positively reports a
non-Linux target. The tagged Linux release binaries are built by CI on a native
Linux runner.

### `just lint` only sees your own platform

golangci-lint honours build tags, so a `//go:build linux` file is invisible to
`just lint` on macOS. A *build* break in one of those files is caught by
`just check-cross`, which `just ci` runs. Lint findings still need reproducing:

```bash
CGO_ENABLED=0 GOOS=linux golangci-lint run ./internal/...
```

Without cgo, the `*_cgo.go` files are excluded, so anything they alone use is
reported as `unused` and any helper they alone call is reported by `unparam`.
Those are artifacts of the no-cgo build; CI lints Linux with cgo enabled.
Findings in plain-`linux` files are real. The cgo-only paths need a Linux
toolchain: `just lint-cross` runs them in the Linux CI image, and without
Docker, CI is the check.

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

Desktop environments share mechanisms, so the axis that varies is
usually the mechanism:

- **Input**: KDE and GNOME both use libei (RemoteDesktop portal); wlroots and
  COSMIC use `zwlr_virtual_pointer`. One libei backend serves several DEs.
- **Overlay**: layer-shell works on KDE, wlroots, and COSMIC; only
  GNOME/Mutter lacks it.
- **Genuinely DE-specific**: active-window geometry (KWin D-Bus vs Mutter
  D-Bus) and hotkey registration. These belong in DE-named files such as
  `internal/adapter/accessibility/atspi/kwin_origin.go`, or in a DE-named
  package when more than one subsystem needs the same fact:
  `internal/adapter/platform/kwin` holds the KWin geometry bridge because the
  AT-SPI window origin and `FocusedWindowBounds` are two readings of it. What
  is shared across compositors goes in a package named for the mechanism:
  `internal/adapter/platform/compositorcli` is how both callers ask niri, Sway
  and Hyprland their question.

Use a `*_linux_wayland_<compositor>.go` sub-slot only when a compositor family
needs a path no other family shares, spelled without the OS token inside
`internal/adapter/platform/linux/`: `system_wayland_wlroots_*.go`
(virtual-pointer input) and `system_wayland_kde_*.go` (libei input), with
`system_wayland_input.go` as the shared routing seam.

**To add a compositor** (COSMIC, say): add a `LinuxBackend` value and detection
in `backend_linux.go`, route it in the factory and the relevant dispatch seams,
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

**Smooth cursor animation on Windows** is the Linux animator's shape with
`SetCursorPos` as the sink
([mouse_animator.go](../internal/adapter/platform/windows/mouse_animator.go)):
off by default, opt in with `smooth_cursor.move_mouse_enabled`, one worker
goroutine sampling `GetCursorPos` once per request and stepping toward the
target with the latest target winning. Relative moves extend the pending
endpoint over `smooth_cursor.relative_movement_duration`, clamped to the active
screen, and position-dependent actions settle the animation before acting.

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
