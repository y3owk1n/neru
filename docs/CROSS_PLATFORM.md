# Cross-Platform Guide

Neru runs on macOS, Linux, and Windows from one shared Go core. This document
covers both sides of that:

- **[Part 1 — Feature Parity Reference](#feature-parity-reference)**: what
  actually works on each platform, and how it is implemented.
- **[Part 2 — Contributor Guide](#contributor-guide)**: where platform code
  lives, and how to add to it.

Every claim in Part 1 is derived from code under `internal/adapter/`,
`internal/app/`. **If this document and the code
disagree, the code wins** — and the disagreement is a bug worth fixing here.

**Related:** [Architecture](./ARCHITECTURE.md) · [Linux setup](./LINUX_SETUP.md) ·
[Linux desktops](./LINUX_DESKTOPS.md) · [Development Guide](./DEVELOPMENT.md)

---

## Table of Contents

**Part 1 — [Feature Parity Reference](#feature-parity-reference)**

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

**Part 2 — [Contributor Guide](#contributor-guide)**

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

Three words carry the whole promise, so they are worth pinning down:

**Stable** — fully featured. Everything Neru does works here. This is the
reference implementation, and a gap on this platform is a bug.

**Beta** — good for daily driving. Every navigation mode works and behaves the
same as it does on a stable platform; what is missing sits around the edges
(a few animations, a KDE-only capture gap) rather than in your way.

**Alpha** — worth trying, not yet worth switching to. Core navigation works, but
hint coverage is incomplete and per-app config does not re-apply on focus
change. You will notice the difference in ordinary use.

Every claim behind these labels is enumerated in the
[Capability Matrix](#capability-matrix) and [Known Gaps](#known-gaps). If a
label and the matrix disagree, the matrix is right.

**Linux moves from Beta to Stable** when its [Known Gaps](#known-gaps) entries
are empty for the blessed stack and the headless-sway CI job is green and
required for merge — not before, and not on a judgment call
([ADR 0013](./adr/0013-parity-is-measured-in-words-not-subsystems.md)).

### Per-platform

| Aspect               | macOS (Darwin)              | Linux                                    | Windows                        |
| -------------------- | --------------------------- | ---------------------------------------- | ------------------------------ |
| **Status**           | **Stable**                  | **Beta**                                 | **Alpha**                      |
| **Build tag**        | `darwin`                    | `linux`                                  | `windows`                      |
| **CGO**              | Required (Objective-C)      | Per-backend; most Linux backends need it | Not used (pure Go Win32 / COM) |
| **Primary modifier** | `Cmd`                       | `Ctrl`                                   | `Ctrl`                         |
| **Display stack**    | Cocoa / Quartz              | X11, or Wayland (wlroots / KWin)         | Win32 / DWM                    |
| **Accessibility**    | AXUIElement                 | AT-SPI over D-Bus                        | UI Automation over COM         |
| **Native product**   | Yes (`Neru.app`, codesigned)| Binary + install script                  | Binary                         |

### Linux backends

Linux is not one target. The live backend is detected once at startup from
`XDG_CURRENT_DESKTOP`, `WAYLAND_DISPLAY`, and `DISPLAY`
([backend_linux.go](../internal/adapter/platform/backend_linux.go)). This is the
only place the compositor *family* is decided — the `display_server` field in
`neru info` and `neru doctor` names the stack of whichever row below matched,
rather than reading the environment a second time:

| Backend                | Detected when                                       | Status                       |
| ---------------------- | --------------------------------------------------- | ---------------------------- |
| `x11`                  | `DISPLAY` set, no `WAYLAND_DISPLAY`                 | Supported                    |
| `wayland-wlroots`      | Sway, Hyprland, niri, River, Wayfire, or unset `XDG_CURRENT_DESKTOP` | Supported   |
| `wayland-kde`          | `XDG_CURRENT_DESKTOP` contains `KDE`                | Supported                    |
| `wayland-gnome`        | `XDG_CURRENT_DESKTOP` contains `GNOME`              | **Not supported**            |
| `wayland-other`        | Any other Wayland compositor                        | **Not supported**            |
| `unknown`              | Neither `WAYLAND_DISPLAY` nor `DISPLAY`             | **Not supported**            |

> **GNOME Wayland does not run at all.** `platform.NewSystemPort` returns
> `CodeNotSupported` for `wayland-gnome`, `wayland-other`, and `unknown`, and
> that is the first step of daemon startup — the daemon exits instead of
> starting in a degraded state. Mutter implements neither `wlr-layer-shell`
> (overlays) nor `wlr-foreign-toplevel-management` (focused app), and exposes no
> input-injection path Neru can use. **Use an X11 session under GNOME.** The
> tables below therefore have no GNOME column: nothing runs there.

---

## Capability Matrix

Status of each `ports.SystemPort`-level capability, with the mechanism that
implements it. The KDE and wlroots columns differ only where noted; both are
Wayland with `wlr-layer-shell` overlays.

**Legend:** ✅ supported · ⚠️ works with known limits · 🟡 stub (`CodeNotSupported`
or no-op) · ❌ no code path · ➖ macOS-only capability, exempt from parity
(see [Platform Exclusives](#platform-exclusives))

This table answers whether a *subsystem* works. It cannot answer whether every
option, mode flag, action and command means the same thing on each platform —
that is what [Known Gaps](#known-gaps) tracks, per
[ADR 0013](./adr/0013-parity-is-measured-in-words-not-subsystems.md).

| Capability                    | macOS                    | Linux X11              | Linux Wayland (wlroots)      | Linux Wayland (KDE)     | Windows                      |
| ----------------------------- | ------------------------ | ---------------------- | ---------------------------- | ----------------------- | ---------------------------- |
| **Screen bounds / enumeration** | ✅ Cocoa               | ✅ XRandR              | ✅ xdg-output                | ✅ xdg-output           | ✅ `EnumDisplayMonitors`     |
| **Display hotplug events**    | ✅ screen-params notif.  | ✅ RandR event fd      | ✅ `wl_output` events        | ✅ `wl_output` events   | 🟡                           |
| **Focused app identity**      | ✅ NSWorkspace + AX      | ✅ `_NET_ACTIVE_WINDOW` / `WM_CLASS` | ⚠️ app_id only (see below) | ⚠️ app_id only     | ✅ `GetForegroundWindow`     |
| **App watcher (focus change)**| ✅ NSWorkspace observer  | ✅ event-driven        | ✅ event-driven              | ✅ event-driven         | 🟡                           |
| **Keymap learns the focused app** | ✅ published by the watcher | ✅ published by the watcher | ✅ published by the watcher | ✅ published by the watcher | ⚠️ asked when the keymap settles ¹ |
| **Cursor position**           | ✅ `CGEventGetLocation`  | ✅ `XQueryPointer`     | ✅ compositor IPC (Hyprland) / sync-surface trick | ✅ sync-surface trick | ✅ `GetCursorPos` |
| **Cursor move**               | ✅ `CGEventPost` ([`postMouseMoveLocked`](../internal/adapter/platform/darwin/accessibility_mouse_darwin.m)) | ✅ XTest (`XTestFakeMotionEvent`) | ✅ `zwlr_virtual_pointer` | ✅ libei                | ✅ `SetCursorPos`            |
| **Mouse buttons / drag**      | ✅ `CGEventPost`         | ✅ XTest               | ✅ `zwlr_virtual_pointer`    | ✅ libei                | ✅ `SendInput`               |
| **Scroll injection**          | ✅ both axes             | ✅ both axes           | ✅ both axes (uinput + virtual pointer) | ✅ libei     | ⚠️ vertical only             |
| **Modified scroll (`--modifier`)** | ✅ `CGEventSetFlags` on every chunk | ✅ XTest key hold | ✅ virtual keyboard, uinput batch skipped | ✅ libei | ✅ `SendInput` key hold |
| **Smooth cursor animation**   | ✅ (incl. relative, opt-in) | ✅ incl. relative, opt-in | ✅ incl. relative, opt-in | ✅ incl. relative, opt-in | ❌                        |
| **Smooth scroll animation**   | ✅                       | ⚠️ whole notches only ⁴ | ✅ continuous virtual-pointer axis ⁴ | ⚠️ libei scroll delta, unverified ⁴ | ❌       |
| **Element discovery (hints)** | ✅ AXUIElement           | ⚠️ AT-SPI walk         | ⚠️ AT-SPI walk               | ⚠️ AT-SPI walk          | ⚠️ UIA, shallow tree         |
| **Overlay**                   | ✅ NSPanel + CoreAnimation | ✅ X11 + Cairo       | ✅ layer-shell + Cairo       | ✅ layer-shell + Cairo  | ✅ layered HWND + GDI        |
| **Global hotkeys**            | ✅ per-key CGEventTap    | ✅ `XGrabKey`          | ⚠️ passive evdev read        | ⚠️ passive evdev read   | ✅ `RegisterHotKey`          |
| **Keyboard capture**          | ✅ CGEventTap            | ✅ `XGrabKeyboard`     | ✅ evdev grab (wl-keyboard fallback) | ✅ evdev grab   | ✅ `WH_KEYBOARD_LL`          |
| **Modifier passthrough**      | ✅                       | ❌                     | ✅ evdev backend only        | ✅ evdev backend only   | ❌                           |
| **Dark mode detection**       | ✅ Cocoa appearance      | ✅ xdg appearance portal | ✅ xdg appearance portal   | ✅ kdeglobals + portal  | ✅ registry                  |
| **Font resolution**           | ✅ NSFont                | ✅ fontconfig          | ✅ fontconfig                | ✅ fontconfig           | ⚠️ generic-alias map only ²  |
| **System tray**               | ✅ NSStatusItem          | ✅ D-Bus StatusNotifierItem | ✅ StatusNotifierItem        | ✅ StatusNotifierItem   | ✅ Win32 notification area   |
| **Native alerts**             | ✅ NSAlert               | ⚠️ D-Bus, not modal    | ⚠️ D-Bus, not modal          | ⚠️ D-Bus, not modal     | ✅ `MessageBoxW`             |
| **Native notifications**      | ✅ UNNotification        | ✅ `org.freedesktop.Notifications` | ✅ `org.freedesktop.Notifications` | ✅ `org.freedesktop.Notifications` | 🟡          |
| **Secure input detection**    | ✅                       | ➖ always false        | ➖ always false              | ➖ always false         | ➖ always false              |
| **System cursor hide**        | ✅ `CGDisplayHideCursor` | ➖                     | ➖                           | ➖                      | ➖                           |
| **`monitor_select` mode**     | ✅ native panels         | ✅ Cairo panels        | ✅ Cairo panels              | ✅ Cairo panels         | 🟡 `CodeNotSupported`        |
| **Native hint-search field**  | ✅ NSTextField overlay   | 🟡 key-stream input ⁵  | 🟡 key-stream input ⁵        | 🟡 key-stream input ⁵   | 🟡 key-stream input ⁵        |
| **Screen capture**            | ✅ ScreenCaptureKit      | ✅ `XGetImage`         | ✅ `wlr-screencopy`          | ❌ ⁶                    | ❌                           |
| **Vision / OCR detection**    | ✅ Vision framework      | ⚠️ tesseract, text only ⁷ | ⚠️ tesseract, text only ⁷ | ❌ no capture (entry 2) | ❌                           |
| **Key feed (`neru key`)**     | ✅ `CGEventPost`         | ✅ uinput               | ✅ uinput / virtual-keyboard | ✅ uinput               | 🟡 `CodeNotSupported`        |
| **Service management (`neru services`)** | ✅ launchd user agent | ⚠️ systemd user unit only ³ | ⚠️ systemd user unit only ³ | ⚠️ systemd user unit only ³ | 🟡 `CodeNotSupported` |

¹ Per-app hotkey overrides need to know which application is focused. Where the
app watcher fires, it publishes that identity to the mode handler and the keymap
is re-settled from it, so switching applications mid-mode changes what the next
key does; the platform is asked at most until the watcher first fires. Windows
has no watcher, so the keymap asks each time it settles — which means overrides
there settle **when the mode opens** rather than the instant you switch apps.
The same applies to a Linux session whose compositor exposes no focused-app
source (GNOME/Mutter). No platform is asked on a keystroke; ADR 0005 has the
reasoning.

² macOS and Linux resolve font *families* through the OS (NSFont, fontconfig).
Windows only maps the generic aliases `sans` / `serif` / `mono` to Segoe UI /
Cambria / Consolas and passes every other name straight to GDI without checking
it; an unavailable family falls back to whatever GDI substitutes.

A family somebody named resolves to **that name**: the answer is the family the
config asked for, not the family the platform would render in its place. The one
exception is Linux with fontconfig, the only backend that can tell an installed
family from a missing one — a missing one falls back to DejaVu Sans rather than
to fontconfig's own substitution for the name, so `font_family = "Arial"`
without Arial installed reports DejaVu Sans and not the Liberation Sans
`fc-match Arial` names. It is the sans baseline whatever face the name asked
for: the serif and mono baselines are what the `serif` and `mono` aliases
resolve to, so a missing `Times New Roman` also lands on DejaVu Sans. Where the
baseline is itself missing, fontconfig chooses that machine's generic, so the
fallback is always a family it has. macOS, Windows, the non-CGO Linux build, and a CGO build whose
fontconfig cannot be consulted at all check nothing: they hand the written name
to NSFont / GDI / Cairo, which substitute when the text is drawn — as Cairo does
on Linux too, for whichever name it is given.

Which names count as generic is the same on all three: `sans`, `sans serif`,
`serif`, `mono`, `monospace` and the empty string, matched ignoring case,
surrounding whitespace and the separator between words — `sans-serif`,
`sans_serif` and `sansserif` are one name. Every other family name is passed to
the platform trimmed. One parser answers that for all three
(`internal/adapter/platform/fontgeneric`, ADR 0007); what each generic resolves
to is the platform's own — Helvetica Neue / Times New Roman / Menlo on macOS,
DejaVu Sans / DejaVu Serif / DejaVu Sans Mono on Linux, the Windows families
above.

Where an answer is remembered — macOS, Windows and the fontconfig-backed Linux
resolver — it is remembered under the family name exactly as written
(`internal/adapter/platform/fontcache`); the non-CGO Linux build re-derives it
each time. Either way what a name resolves to depends on that name alone and
never on what was resolved before it.

³ The Linux columns of this table are display servers, and service management is
the one row whose limit sits on a different axis: it needs **systemd**, on every
Linux backend alike. A machine booted by runit, OpenRC or s6 gets
`CodeNotSupported` from every `neru services` subcommand — a stated boundary
rather than a gap, see "Service management on Linux" below.

⁴ `smooth_scroll` animates on every Linux backend, but only Wayland can make a
step shorter than a wheel notch. `zwlr_virtual_pointer_v1.axis` carries a
fractional value with no discrete step count, and wlroots forwards exactly that
to the focused client as a continuous `wl_pointer.axis`; libei's
`ei_device_scroll_delta` is pixel-precise and KWin forwards it the same way.
X11 has no such value to send: core scrolling is buttons 4 to 7 and a button
event is one notch by definition, and the XTEST pointer the server creates for
`XTestFakeButtonEvent` is allocated with two axes, `Rel X` and `Rel Y`
(`CorePointerProc` in xorg-server's `dix/devices.c`), so it has no scroll
valuator for the smooth XI2 path real devices use.

**So X11 animates in notches, and a scroll worth one notch is not animated at
all.** The default `scroll.scroll_step` of 50 pixels is exactly one notch there,
so a plain `scroll_down` on X11 arrives as the single wheel click it always did
— deliberately, and immediately: delivering it late would be added latency and
nothing else. From two notches up (`scroll_step_half`, `scroll_step_full`, or a
`scroll_step` above 60) the same eased curve applies as everywhere else, and
those are the scrolls the animation is worth having for.

⁵ This row is about the native *field* — a platform text control that owns
keyboard focus and brings the system's input method with it. Only macOS has
one. Everywhere else the query is read from the event tap's key stream, which
is why dead keys and IME composition do not work there and a hint search takes
plain characters.

**What the box on screen is, is a different question, and every platform draws
one.** Linux paints the search badge onto the shared overlay surface with the
same Cairo primitives as its other badges, so `hints.search_input_ui.*` means
what it says on all three; the badge is a display of the query the mode handler
already holds, and it never captures a key.

Neru sends the same distance on every backend; only the granularity of a step
differs — though on Wayland the animated path spends that distance as a
continuous delta where the unanimated one spends it as notches, and an
application may scale the two differently, so switching the animation on can
change how far a scroll reaches there. Wayland steps also declare axis source
`continuous` rather than `wheel`, because a wheel source invites a toolkit to
round the fraction back to a detent.

Measured on wlroots (sway) by
`TestScrollAtCursor_DeliversSubNotchStepsWithSmoothScroll`, which maps a real
`xdg-shell` window and reads what the compositor delivers to it. **The X11 and
KDE conclusions are read from the sources named above and are not measured on
hardware**, and neither is the uinput `REL_WHEEL_HI_RES` route — the
headless-sway job reads no input devices at all, so nothing written to a uinput
device reaches the compositor there.

⁶ Capture is taken per backend rather than through xdg-desktop-portal
ScreenCast everywhere, because a consent picker in front of what becomes a hint
refresh is a latency and consent-fatigue regression the blessed stack has no
need to pay ([ADR 0013](./adr/0013-parity-is-measured-in-words-not-subsystems.md)).
X11 reads the root window back with `XGetImage`; wlroots-family compositors
implement `wlr-screencopy-unstable-v1`, which needs no consent because the
client is already trusted with the session.

**KWin implements neither**, and its only pixel source is the portal's
ScreenCast session, which delivers frames over PipeWire — a required system
library Neru does not link and a build-dependency change beyond this row. KDE
Plasma therefore reports `CodeNotSupported` naming itself, and it is
[Known Gaps](#known-gaps) Linux entry 2. Capture is a **region** operation on
both implemented backends: the caller's rectangle is what gets read back, so
constraining detection to the focused window costs a window rather than a
display.

What comes back covers **exactly** the region asked for, and that is enforced
rather than hoped for: a rectangle that leaves the screen, that is degenerate,
or that spans two monitors on Wayland (`wlr-screencopy` captures one output)
**fails** instead of coming back clipped. A clipped frame carries nothing that
says where its own top-left is, so a caller could no longer map a pixel back to
a screen coordinate — and a caller asking for one window must never silently
receive the whole display.

One thing to know before reading a frame: on a scaled Wayland output the
compositor answers in **physical pixels**, so it can be larger than the logical
region by the output's scale factor — the same thing a Retina capture does on
macOS. The image's own bounds start at `(0, 0)`; the region passed in is what
places those pixels.

⁷ Linux `vision` is **text-only**, and permanently so. macOS runs three Vision
requests — text recognition, rectangle detection and saliency — and an OCR
engine answers the first. `hints.vision.detect_rectangles` and the four
`rectangle_*` options are therefore declared macOS-only rather than met with a
contour-detection library, which would be a heavy new required dependency for a
sub-feature of a non-default strategy
([ADR 0013](./adr/0013-parity-is-measured-in-words-not-subsystems.md)). The
other fourteen `hints.vision.*` options are read on Linux exactly as they are on
macOS.

The engine is **tesseract**, linked through `#cgo pkg-config: tesseract` like
every other native dependency here, and it is required rather than optional:
under dynamic linking a missing `libtesseract.so` stops the daemon before any
Neru code runs, so [LINUX_SETUP.md](./LINUX_SETUP.md) lists it with the rest.
Its **language data is a separate package** and is resolved at *use* rather than
at link time — `TESSDATA_PREFIX` first, then the paths distributions install
into. A machine with the library and no `eng.traineddata` gets
`CodeNotSupported` naming that file, from `VisionPort.Health` and from
`DetectElements`, rather than a strategy that silently finds nothing.

Recognition runs at word level for `neru hints --split-word` and at line level
otherwise, on the LSTM engine in sparse-text page segmentation — UI text is
scattered labels, not paragraphs. Detection is scoped to the region the caller
asks about, which is the focused window: full-display OCR takes seconds where
one window takes tens of milliseconds. Recognized text is screen content and is
treated as such — never logged, never written to disk, and cleared out of the
engine before each recognition returns.

### Notes on the ⚠️ entries

**Focused app on Wayland.** wlroots and KWin resolve the focused window through
`wlr-foreign-toplevel-management`, which exposes the window's **app_id** — used
as the bundle identifier for per-app config — but not its PID, because a Wayland
client cannot read another client's process credentials.
`SystemPort.FocusedApplicationPID` best-effort matches the app_id against
`/proc`; with no match it returns `CodeNotSupported` carrying the app_id rather
than a fabricated number.

**App watcher.** macOS gets focus changes pushed from an NSWorkspace observer.
Linux has no equivalent single API, so `appwatcher/platform_linux.go` subscribes
to a backend focus-change fd (`linux.SubscribeFocusedApp`: X11 event fd, or the
wlroots toplevel manager) and re-samples on each wake — near-instant per-app
hotkey re-registration — with a 3s safety re-sample against coalesced events.
When no fd is available it degrades to polling `FocusedAppID` every 400ms. The
identity is the **WM_CLASS** (X11) or **app_id** (Wayland). A sibling goroutine
watches a display-configuration fd and dispatches screen-parameter changes, so
monitor hotplug regenerates overlays like it does on macOS. Only
activate/deactivate/screen-params are emitted; launch, terminate, and Mission
Control events remain macOS-only.

**Global hotkeys on Wayland.** No Wayland protocol lets an ordinary client
register a global hotkey, so Neru reads `/dev/input/event*` directly with a
**passive** evdev listener — it does not grab devices or inject anything, so the
focused app still receives every key
([global_hotkey_cgo.go](../internal/adapter/eventtap/linux/global_hotkey_cgo.go)).
Two conditions apply: the process needs read access to `/dev/input` (add your
user to the `input` group), and it requires CGO — a `CGO_ENABLED=0` build gets a
stub whose `Start` reports `CodeNotSupported`
([global_hotkey_nocgo.go](../internal/adapter/eventtap/linux/global_hotkey_nocgo.go)).
Either way the listener cannot start, and Neru warns with the remediation that
fits. An unreadable `/dev/input` points at the `input` group; a no-cgo build
points at the build, and is warned about once, since no retry changes how the
binary was compiled. Both name the same fallback — bind `neru <mode>` as a
compositor keybinding. While a mode is active the in-mode event tap grabs the
same devices, so the listener naturally goes quiet until the mode exits.

**Native alerts on Linux.** Notifications and alerts both go to the session's
freedesktop notification daemon over D-Bus — the same session bus the tray's
StatusNotifierItem uses, in pure Go, so a `CGO_ENABLED=0` build shows them too.
An alert differs from a notification only in insistence: critical urgency and no
expiry, which the specification requires a daemon to leave on screen until it is
dismissed. What it is *not* is modal. macOS's `NSAlert` stops the world and
returns which button was pressed; no ordinary Wayland or X11 client can do that,
so a Linux alert informs rather than asks, and callers that would have branched
on the answer take the safe default. The two startup alerts are where that shows
up: a missing config file starts Neru on built-in defaults and says so, instead
of offering create / defaults / quit. Delivery depends on the session having a
notification daemon (mako, dunst, or the desktop's own) — either running, or
registered with the bus to be started on demand, which is how most desktops ship
theirs and which `neru doctor` counts as present. With none, `ShowNotification`
and `ShowAlert` report `CodeNotSupported` naming what is absent, `neru doctor`
probes the session and downgrades the notifications row with a line saying what
to install, and the two startup alerts fall back to stderr.

**Service management on Linux.** The mechanism is a **systemd user unit**
anchored on `graphical-session.target` — `After=` and `WantedBy=` it because
every backend needs a display server and starting before one exists would only
produce a crash loop, `PartOf=` it so a logout/login cycle restarts the daemon
instead of leaving an orphan attached to a display that is gone.

Coverage is **systemd and no other init system**: runit, OpenRC and s6 report
`CodeNotSupported` naming systemd, which
[ADR 0013](./adr/0013-parity-is-measured-in-words-not-subsystems.md) records as a
stated boundary rather than a gap. What answers the question is systemd's own
runtime marker, `/run/systemd/system`, not `systemctl` on `PATH` — that binary
ships in packages installed on machines running something else. `status` on a
machine where the unit was never installed says so instead of failing. Where the
unit is written and how a user drives it:
[LINUX_SETUP.md](./LINUX_SETUP.md#systemd-user-service).

**Smooth cursor animation on Linux.** Off by default; opt in with
`smooth_cursor.move_mouse_enabled` (the same cross-platform `SmoothCursorConfig`
macOS uses). When enabled, `SystemAdapter.MoveCursorToPoint` routes through
`smoothCursorAnimator` ([mouse_animator.go](../internal/adapter/platform/linux/mouse_animator.go)):
one worker goroutine samples the current position, then steps the per-backend
warp (XTest / `zwlr_virtual_pointer` / libei) toward the target by linear
interpolation, and `WaitForCursorIdle` blocks until it settles. This mirrors the
darwin animator (coalescing, latest-target-wins) but drives discrete warps
rather than a Quartz event stream, so there is no drag-event distinction. It
covers the same flows macOS animates — grid/recursive-grid cursor-follow,
`move_mouse`, selection moves; clicks stay instant. On Wayland the interpolation
start point comes from the client-side cursor cache, so a stale read only skews
the glide path, never the landing point.

Relative (hjkl) moves animate too, with the fixed per-move duration
`smooth_cursor.relative_movement_duration`, matching macOS. X11 and KDE extend
the absolute animator's pending endpoint; wlroots instead drains the delta in
integer chunks through native relative motion
([relative_animator.go](../internal/adapter/platform/linux/relative_animator.go)) —
the animation never reads the client position cache, preserving the exactness
that made wlroots apply deltas natively in the first place. Position-dependent
actions (clicks, scrolls) settle the in-flight animation before acting, so an
action fired mid-glide lands where the user aimed.

---

## Input Injection

Every action type in
[action.go](../internal/domain/action/action.go) — left/right/middle click,
per-button down/up/toggle, absolute and relative moves, drag-while-held, and
scroll — is dispatched through the shared `InfraAXClient.PerformAction`. The
dispatch, the action set, and the mode logic that drives it are platform-neutral
Go; only the final injection primitive differs:

| Platform              | Primitive                                                                    |
| --------------------- | ---------------------------------------------------------------------------- |
| macOS                 | `CGEventPost` (`kCGEventMouseMoved` / `*MouseDragged` for moves)              |
| Linux X11             | XTest (`XTestFakeMotionEvent`, buttons 1/2/3, scroll 4/5 vert. + 6/7 horiz.)  |
| Linux Wayland wlroots | `zwlr_virtual_pointer` (+ `/dev/uinput` for scroll)                           |
| Linux Wayland KDE     | libei via `org.freedesktop.portal.RemoteDesktop`                              |
| Windows               | `SendInput` / `SetCursorPos`                                                  |

**The one behavioral difference:** Windows `ScrollAtCursor` ignores `deltaX`, so
horizontal scrolling is a no-op there. Everything else behaves the same on all
three platforms.

**Modifiers on a scroll** reach the injection primitive by two different routes,
because only one of the primitives has a field for them. macOS stamps
`CGEventSetFlags` on the scroll event — always, the empty set included, because a
NULL-source event is born carrying whatever the combined session state currently
holds, so an unstamped scroll inherits ambient modifiers rather than carrying
none — and on every chunk of a smooth-scroll animation, since a zoom applied to
the first frame only is not a zoom. The other
three press the real key, scroll, and release it. On Wayland that forces a
choice: the modifier can only go out on the virtual keyboard (libei on KDE),
while the fast path for the scroll is the uinput device, so a modified scroll
skips the uinput batch entirely and goes out on the wlroots/libei seat. A path
with no backend to press through answers `CodeNotSupported`; none of them
scrolls unmodified and reports success.

**Held mouse buttons.** Press and release are separate actions, so every backend
must remember what it pressed — and that bookkeeping is shared, not
per-platform. Each adapter keeps a
[`mousestate.Tracker`](../internal/adapter/platform/mousestate/tracker.go)
recording which buttons are down, where, and with which modifiers. It drives
three behaviors identically everywhere: toggle actions resolve against it (held
→ release, free → press), `EnsureMouseUp` releases every held button when Neru
returns to idle, and on macOS it selects the drag event type for cursor moves.
macOS is the only backend needing that last distinction — Quartz requires
`kCGEventLeftMouseDragged` / `RightMouseDragged` / `OtherMouseDragged` with a
matching button number instead of `kCGEventMouseMoved`, while X11, Wayland, and
Windows simply warp the pointer and let the compositor or OS infer the drag.
When several buttons are held at once a macOS move is attributed to the
left-most held button, since one event cannot describe more.

---

## Keyboard Capture And Hotkeys

| Aspect                | macOS                  | Linux X11               | Linux Wayland                            | Windows                 |
| --------------------- | ---------------------- | ----------------------- | ---------------------------------------- | ----------------------- |
| **In-mode capture**   | `CGEventTapCreate`     | `XGrabKeyboard`         | evdev `EVIOCGRAB`, wl-keyboard fallback  | `WH_KEYBOARD_LL`        |
| **Global hotkeys**    | Per-key CGEventTap     | `XGrabKey`              | Passive evdev read                       | `RegisterHotKey`        |
| **CGO needed**        | Yes                    | Yes                     | Yes                                      | No                      |
| **Press/release**     | ✅ separate callbacks  | ✅ KeyPress/KeyRelease  | ⚠️ press-only in some configs            | ✅ `WM_HOTKEY` flags    |
| **Modifier passthrough** | ✅                  | ❌ grab is all-or-nothing | ✅ evdev only                          | ❌ no-op                |
| **`PostModifierEvent`** | ✅                   | ✅                      | ✅ (`zwp_virtual_keyboard_v1`)           | ❌ no-op                |
| **Sticky modifiers**  | ✅                     | ✅                      | ✅                                       | ✅                      |
| **Capture files**     | `eventtap/darwin/`     | `eventtap/linux/x11_cgo.go` | `eventtap/linux/wayland_cgo.go`, `evdev_cgo.go` | `eventtap/windows/` |
| **Hotkey files**      | `hotkeys/darwin/`      | `hotkeys/linux/x11_cgo.go`  | `hotkeys/linux/manager.go` + `eventtap/linux/global_hotkey_cgo.go` ³ | `hotkeys/windows/` |

³ There is no separate Wayland hotkey file — the Wayland path lives in the
common `hotkeys/linux/manager.go`, which delegates to the evdev listener in the
eventtap package.

**Modifier passthrough (Wayland evdev only).** While a mode is active Neru
captures the keyboard exclusively, so shortcuts it does not bind (`Ctrl+C`,
`Ctrl+Tab`) are normally swallowed. With `general.passthrough_unbounded_keys`,
unbound Ctrl/Alt/Cmd chords are re-injected to the focused app instead. This
works on the Wayland evdev backend because Neru holds `EVIOCGRAB` on the
physical device but injects through a *separate* `zwp_virtual_keyboard_v1`,
which bypasses that grab and reaches the app with no feedback loop (see
`handleWaylandEvdevEvent` → `passthroughEvdevChord`). It is **not** available on
X11 — an `XGrabKeyboard` routes Neru's own synthetic XTest events back to
itself, and `XSendEvent` is ignored by most apps — nor on the rare wl-keyboard
fallback, which has no injection path. Classification (blacklist,
mode-intercepted keys, the mode's own hotkeys) and the post-passthrough hint
refresh are shared in
[passthrough.go](../internal/app/modes/passthrough.go); only the final
re-injection is backend-specific. The blacklist keeps chosen chords consumed,
and `general.should_exit_after_passthrough` exits the mode after a passthrough.
Both lists are re-derived whenever a mode opens, the configuration is replaced,
hints refresh after a passthrough, or — where a per-app override could move
them — the focused application changes under an open mode. That last trigger is
what keeps per-app overrides meaningful after passing `Cmd+Tab` through and
carrying on in the application you landed in; it runs as soon as the mode
handler is free rather than in step with the focus change, so a chord pressed
in the same instant can still be routed by the lists the application you left
put in force.

---

## Accessibility And Hints

| Aspect                  | macOS                                       | Linux                                              | Windows                              |
| ----------------------- | ------------------------------------------- | -------------------------------------------------- | ------------------------------------ |
| **Backend**             | AXUIElement (CGO ObjC bridge)               | AT-SPI over D-Bus (pure Go)                        | UI Automation over COM (pure Go)     |
| **Client**              | `InfraAXClient` → ObjC bridge               | `ATSPIClient` → `org.a11y.atspi`                   | `UIAClient` → raw COM vtables        |
| **Files**               | `element_darwin.go`, `tree.go`              | `element_linux.go`, `atspi_linux.go`               | `element_windows.go`, `uia_windows.go`, `tree_windows.go` |
| **Traversal**           | Full recursive walk of the AXUIElement hierarchy | Recursive walk of the active frame's subtree, depth/node capped | Shallow walk of root-level nodes |
| **Sources collected**   | Frontmost + all windows, popovers, menubar, dock, notification center, Stage Manager, PIP | Active frame's subtree only          | Root element's children only         |
| **Filtering**           | Role matching, size/position heuristics, excluded apps, dedup | Native AT-SPI roles, `SHOWING` state, on-screen extents | `IsControlElement` + `IsContentElement`, non-zero bounds |
| **Strategies**          | `axtree` (default) and `vision`, incl. per-app overrides | `axtree` only                          | `axtree` only                        |
| **Popovers / menus**    | ✅ dedicated detection                      | ⚠️ only if inside the active frame's subtree       | 🟡                                   |

macOS builds the richest tree by a wide margin: it walks multiple window and
system sources, applies per-app strategy overrides, can fall back to the Vision
framework for OCR-discovered targets, and deduplicates overlapping elements.
Linux walks a single tree and has the OCR fallback beside it — tesseract, text
only, selected with `hints.strategy = vision`. Windows walks a single tree with
no fallback at all.

**Linux is ⚠️, not a stub.** Hints genuinely work: `ATSPIClient` enables
assistive-tech mode, finds the active frame, and walks it (`ClickableNodes`)
emitting native AT-SPI role names. Configured roles are resolved into that same
vocabulary at config load (`element.ResolveRoles`), so both sides of the filter
speak AT-SPI. This is the path the Linux adapter actually uses
(`platform_client_linux.go` → `Adapter.ClickableElements` → `client.ClickableNodes`);
the `TreeNode` / `BuildTree` stub in `tree_linux.go` is the macOS-style tree API
and is **not** on the Linux hints path. The ⚠️ is about coverage: it depends on
each app exposing AT-SPI. Qt and GTK apps do with accessibility enabled; some
toolkits expose almost nothing, and there is no Vision/OCR fallback.

**Chromium and Electron apps on Linux.** Chromium-based apps (Chrome, Electron,
forks such as Helium) do not expose their web-content tree over AT-SPI by
default — they gate it behind their own runtime detection, and unlike macOS
there is no per-app attribute Neru can toggle to force it (the macOS
`AXManualAccessibility` nudge in `electron.EnsureAccessibility` is a no-op on
Linux). The result is an AT-SPI frame with a single empty child, so hints find
nothing inside such windows. Launch the app with
`--force-renderer-accessibility` to force the full tree. Native GTK/Qt apps and
Firefox need no flag. This is Chromium behavior, not a Neru limitation.

**Picking the active frame on Wayland.** The AT-SPI `ACTIVE` state is unreliable
on wlroots compositors (niri, Sway, Hyprland) — the focused window can report
`ACTIVE=false` while background frames report `ACTIVE=true`. Neru therefore
matches the AT-SPI frame against the compositor's focused **app_id** (from
`wlr-foreign-toplevel-management`, the same source as the app watcher), falling
back to the `ACTIVE`/`SHOWING` heuristic only on X11 or when no app_id is
available. See `findActiveFrame` in `atspi_linux.go`.

**Window-origin offset on Wayland.** A Wayland client cannot know its own
on-screen position, so AT-SPI reports element coordinates relative to the
window. Neru offsets them by the focused window's screen origin, supplied by a
compositor-specific `windowOriginSource`
([window_origin.go](../internal/adapter/accessibility/atspi/window_origin.go)),
chosen from the backend `DetectLinuxBackend` reported — so nothing here starts on
a session that backend did not identify:

| Compositor | Source                                                       | Limits                                                                                                                    |
| ---------- | ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------- |
| KDE / KWin | KWin script pushing focused-window geometry over D-Bus       | —                                                                                                                          |
| niri       | `niri msg -j focused-window` / `focused-output`              | Floating and fullscreen windows only. **Tiled** windows — including a maximized column (`Mod+F`) — expose no on-screen position ([niri#2381](https://github.com/niri-wm/niri/issues/2381)), so hints are misaligned there. |
| Sway       | `swaymsg -t get_tree`, focused node `rect` + `window_rect`   | —                                                                                                                          |
| Hyprland   | `hyprctl -j activewindow` `at` / `size`                      | —                                                                                                                          |
| Anything else — X11, GNOME, other Wayland | none                                          | X11 needs none — AT-SPI already reports screen coordinates there. The rest report no origin, so hints stay window-relative. |

Each source verifies the reported window size matches the AT-SPI frame (a focus
change can race the query) and is best-effort: an unavailable origin degrades to
unoffset window-relative coordinates rather than misplacing hints.

---

## Overlay Rendering

### Architecture

The three platforms split responsibility differently, which is the single most
important thing to know before touching overlay code:

- **macOS** — each render component owns its own NSPanel. Files such as
  `adapter/overlay/render/hints/overlay_darwin.go` call the Objective-C bridge
  directly, and rendering is GPU-backed via CoreAnimation.
- **Linux and Windows** — the render components hold the shared `Style` and a
  thin wrapper; all real rendering happens in the overlay **manager**
  (`overlay/linux/x11_cgo.go`, `overlay/linux/wayland_cgo.go`,
  `overlay/windows/manager.go`), drawing every element into one shared
  surface.

### Implementation

| Aspect                | macOS                                    | Linux X11                              | Linux Wayland                                   | Windows                            |
| --------------------- | ---------------------------------------- | -------------------------------------- | ------------------------------------------------ | ---------------------------------- |
| **Window type**       | NSPanel, borderless non-activating       | override-redirect X11 window           | `wlr_layer_shell_v1` overlay surface             | layered `WS_POPUP` HWND            |
| **Rendering**         | CoreAnimation (CALayer, GPU)             | Cairo on an Xlib surface (CPU)         | Cairo into SHM buffers (CPU)                     | GDI + software SDF, BGRA (CPU)     |
| **Per-pixel alpha**   | clear color + non-opaque layer           | `CAIRO_OPERATOR_CLEAR`                 | `CAIRO_OPERATOR_CLEAR`                           | `AC_SRC_ALPHA` via `UpdateLayeredWindow` |
| **Click-through**     | `setIgnoresMouseEvents:YES`              | XFixes empty input region              | empty `wl_surface` input region                  | `WS_EX_TRANSPARENT`                |
| **Always on top**     | `NSScreenSaverWindowLevel`               | `_NET_WM_STATE_ABOVE` + `MapRaised`    | overlay layer                                    | `HWND_TOPMOST`                     |
| **Focus prevention**  | non-activating panel                     | `override_redirect=YES`                | controlled keyboard interactivity                | `WS_EX_NOACTIVATE`                 |
| **HiDPI**             | dynamic `contentsScale` + backing-change callback | `Xft.dpi`, one global factor  | `wl_output` scale + `wp_fractional_scale_v1` / `wp_viewporter` | not explicit           |
| **Multi-monitor**     | per-display clamping, screen-change tracking | all monitors enumerated, per-monitor render, live RandR hotplug | one `wl_surface` per output (max 16), live hotplug | cursor-screen tracking, separate indicator/sticky windows |
| **Buffers**           | layer-backed, OS-managed                 | single Cairo surface                   | triple-buffered SHM pool                         | single pixel buffer                |
| **Rounded rects / borders** | NSBezierPath                       | Cairo arc path + stroke                | Cairo arc path + stroke                          | software SDF fill + multi-pass stroke |
| **Text**              | NSFontManager                            | Cairo `select_font_face` / `show_text` | Cairo `select_font_face` / `show_text`           | GDI `CreateFontW` + `DrawTextW` + alpha composite |
| **Coordinate origin** | bottom-left (Y-flipped in the adapter)   | top-left                               | top-left                                         | top-left (negative DIB height)     |
| **Thread model**      | main-thread dispatch                     | `renderMu` mutex                       | `renderMu` mutex (also guards `wl_display`)      | dedicated UI thread (`LockOSThread`) |

### Animation

| Animation                    | macOS                                | Linux X11 / Wayland                | Windows                            |
| ---------------------------- | ------------------------------------ | ---------------------------------- | ---------------------------------- |
| **Grid transition**          | CoreAnimation, ease-in-out @120Hz    | goroutine, smoothstep @120fps      | ❌                                 |
| **Mouse action indicator**   | `CABasicAnimation` (scale + opacity) | goroutine, scale + opacity @120fps | goroutine, cubic easing @60fps     |
| **Smooth cursor**            | ✅ stepped linear interpolation      | ✅ stepped linear interpolation    | ❌                                 |
| **Smooth scroll**            | ✅ ease-out cubic                    | ❌                                 | ❌                                 |

---

## Mode Coverage

Mode logic — labelling, alphabets, matching, search filtering, grid subdivision,
recursion depth, scroll amounts, cell navigation — is pure domain Go under
`internal/domain/` and behaves **identically on all three platforms**. Only
the rows below differ, and every difference traces to rendering or element
discovery rather than the mode itself.

| Mode              | Feature                        | macOS                      | Linux                      | Windows                     |
| ----------------- | ------------------------------ | -------------------------- | -------------------------- | --------------------------- |
| **Hints**         | Element discovery              | ✅ full AX tree            | ⚠️ AT-SPI, toolkit-dependent | ⚠️ UIA, shallow tree      |
| **Hints**         | `vision` strategy + per-app overrides | ✅                  | ⚠️ tesseract; text only, no rectangles | ❌ macOS-only   |
| **Hints**         | Menubar / dock elements        | ✅                         | 🟡                         | 🟡                          |
| **Hints**         | Search input badge             | ✅                         | ✅ Cairo badge             | ✅                          |
| **Hints**         | Label arrow / tail             | ✅ NSBezierPath            | ✅ Cairo triangle          | ✅ sampled triangle, see below |
| **Hints**         | Label placement                | ✅ top / center / bottom   | ✅ top / center / bottom   | ✅ top / center / bottom   |
| **Grid**          | Transition animation           | ✅                         | ✅                         | ❌                          |
| **Grid**          | Virtual pointer indicator      | ✅                         | ❌ no-op                   | ❌ no-op                    |
| **Recursive grid**| Transition animation           | ✅                         | ✅                         | ❌                          |
| **Recursive grid**| Virtual pointer indicator      | ✅                         | ✅                         | ✅                          |
| **Recursive grid**| Sub-key preview                | ✅ mini-grid of next keys  | ✅ mini-grid of next keys  | ✅ mini-grid of next keys   |
| **Scroll**        | Smooth scroll animation        | ✅                         | ✅ (X11: whole notches)    | ❌                          |
| **Monitor select**| Whole mode                     | ✅ native panels           | ✅ Cairo panels            | 🟡 `CodeNotSupported`       |

Everything else is shared: multi-letter labels, label direction, hide-unmatched,
split-word, interactive search *behavior* (only the on-screen badge differs),
boundary highlight, mode indicator, sticky-modifier indicator, all pending
actions on grid cells, subgrid zoom, backtracking, and every scroll granularity.

> The **cursor-replacement virtual pointer** — the pointer drawn when the real
> cursor is hidden — is separate from the recursive-grid indicator above and is
> macOS-only: `virtualpointer.Overlay` is a no-op on every non-darwin build, and
> it is paired with `CGDisplayHideCursor`, which has no equivalent elsewhere.

> **`hints.ui.placement` means the same thing on all three platforms.** Each
> backend offsets the badge from the target point at the element's centre,
> keeping it horizontally centred there: `top` puts the badge above that point
> with a connector arrow pointing down at it, `center` over it with no arrow,
> `bottom` below it with an arrow pointing up (the default).
>
> The rule is shared; the exact pixels are not. Linux and Windows take the
> offsets and the arrow from one implementation
> (`adapter/overlay/render/badge.PlaceHint`), so a configured placement lands
> on the same pixel on both. macOS computes its own in Objective-C — the
> deliberate exception ADR 0007 records — with a shorter, wider arrow, so an
> offset badge sits a few pixels closer to its element there than it does on
> the other two.
>
> One detail of the arrow differs on Windows
> ([#1303](https://github.com/y3owk1n/neru/issues/1303), which is also where
> that backend started reading the option at all). macOS and Linux build the
> badge and the arrow as a single outline, so the border runs around both,
> while the Win32 surface has no path primitive: it draws the arrow as a
> triangle over a slightly larger one in the border colour, which borders its
> two slanted edges but leaves the badge's own edge running across the arrow's
> base.

> **`recursive_grid.ui.sub_key_preview` is one drawing on all three platforms**
> as of [#1297](https://github.com/y3owk1n/neru/issues/1297). Each backend
> divides the cell by the *next* level's grid dimensions and draws the key that
> selects each sub-cell in its own place, so the preview shows **where** each key
> lands; the center sub-cell of an odd-by-odd division is left blank, because the
> cell's own label is drawn there. None of them previews anything at the deepest
> level, where there is no next level to show.
>
> Windows drew a single label along the bottom of the cell until then, and its
> `sub_key_preview_autohide_multiplier` measured the **whole cell** to match.
> All three now measure a **sub-cell** — the cell divided by the next level's
> dimensions — which must reach `sub_key_preview_font_size × multiplier` in both
> width and height, from one implementation
> (`recursivegrid.Style.ShowSubKeyPreviewIn`, with the macOS copy held to it by
> `internal/architecture/sub_key_preview_autohide_rule_test.go`). **A Windows
> user's configured multiplier therefore hides the preview in larger cells than
> it used to**: with a 3×3 next level the preview now disappears at roughly three
> times the cell size it used to survive down to. Lower the multiplier to keep a
> preview in cells that small — it is the same number Linux and macOS have always
> read.

---

## Platform Support Per Word

Every option, mode flag and action carries a platform column, declared once
beside the vocabulary that owns it —
[`internal/config/platform_support.go`](../internal/config/platform_support.go),
[`internal/domain/modecmd/platform_support.go`](../internal/domain/modecmd/platform_support.go)
and
[`internal/domain/action/platform_support.go`](../internal/domain/action/platform_support.go).
The table below is a projection of those declarations, as are the warning the
daemon prints once at load and the `platform_support` row in `neru doctor`
([ADR 0013](./adr/0013-parity-is-measured-in-words-not-subsystems.md)).

It lists only the words whose column is narrower than every platform. The
several hundred that work everywhere are declared too — being supported
everywhere is written down rather than assumed — and
`internal/architecture/platform_support_test.go` fails the build when a word is
neither. Writing one of these where it is inert is never a config error: the
file loads, the daemon runs, and one warning says which lines mean nothing here,
so that one configuration can be carried between platforms
([ADR 0008](./adr/0008-a-vocabulary-has-one-home.md)).

This table answers a different question from the
[Capability Matrix](#capability-matrix). The matrix says whether a subsystem
works; this says whether a word a person wrote does anything. A subsystem can be
green in every cell while an option means nothing, which is exactly how
`smooth_scroll` shipped.

<!-- BEGIN GENERATED PLATFORM SUPPORT: edit the platform_support.go declarations, then run `just gensupportref` -->

| Word | Kind | macOS | Linux | Windows | Why |
| ---- | ---- | --- | --- | --- | --- |
| `general.hide_overlay_in_screen_share` | option | ✅ | ❌ | ❌ | hiding the overlay from a screen share is an NSWindow sharing level, a Quartz concept with no X11, Wayland or Win32 counterpart |
| `general.kb_layout_to_use` | option | ✅ | ❌ | ❌ | the keyboard layout is detected rather than chosen outside macOS |
| `general.passthrough_unbounded_keys` | option | ✅ | ✅ | ❌ | unbound modifier chords reach the focused application on macOS and on the Wayland evdev tap; X11 cannot pass them through at all and Windows does not |
| `general.passthrough_unbounded_keys_blacklist` | option | ✅ | ✅ | ❌ | unbound modifier chords reach the focused application on macOS and on the Wayland evdev tap; X11 cannot pass them through at all and Windows does not |
| `general.should_exit_after_passthrough` | option | ✅ | ✅ | ❌ | unbound modifier chords reach the focused application on macOS and on the Wayland evdev tap; X11 cannot pass them through at all and Windows does not |
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
| `hints.vision.detect_text` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `hints.vision.request_timeout_ms` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `hints.vision.minimum_confidence` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `hints.vision.merge_iou_threshold` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `hints.vision.button_min_confidence` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `hints.vision.button_min_aspect` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `hints.vision.button_max_aspect` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `hints.vision.button_icon_max_size` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `hints.vision.link_min_aspect` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `hints.vision.link_max_height` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `hints.vision.link_min_width` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `hints.vision.image_min_size` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `hints.vision.checkbox_max_size` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `hints.vision.generic_clickable_min_confidence` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `hints.vision.detect_rectangles` | option | ✅ | ❌ | ❌ | rectangle detection has no OCR answer, so it stays macOS-only even where the vision strategy lands; that half is text-only |
| `hints.vision.rectangle_max_candidates` | option | ✅ | ❌ | ❌ | rectangle detection has no OCR answer, so it stays macOS-only even where the vision strategy lands; that half is text-only |
| `hints.vision.rectangle_min_size` | option | ✅ | ❌ | ❌ | rectangle detection has no OCR answer, so it stays macOS-only even where the vision strategy lands; that half is text-only |
| `hints.vision.rectangle_min_aspect` | option | ✅ | ❌ | ❌ | rectangle detection has no OCR answer, so it stays macOS-only even where the vision strategy lands; that half is text-only |
| `hints.vision.rectangle_max_aspect` | option | ✅ | ❌ | ❌ | rectangle detection has no OCR answer, so it stays macOS-only even where the vision strategy lands; that half is text-only |
| `hints.strategy = vision` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `hints.app_configs.strategy = vision` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `grid.app_configs.strategy = vision` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `recursive_grid.app_configs.strategy = vision` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `scroll.app_configs.strategy = vision` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `app_configs.strategy = vision` | option | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so it finds nothing there and none of its settings are read; use axtree |
| `recursive_grid.animation.enabled` | option | ✅ | ✅ | ❌ | the Windows overlay backend has no grid transition animation |
| `recursive_grid.animation.duration_ms` | option | ✅ | ✅ | ❌ | the Windows overlay backend has no grid transition animation |
| `monitor_select.enabled` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `monitor_select.characters` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `monitor_select.ui.font_size` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `monitor_select.ui.font_family` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `monitor_select.ui.border_radius` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `monitor_select.ui.padding_x` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `monitor_select.ui.padding_y` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `monitor_select.ui.border_width` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `monitor_select.ui.subtitle_font_size` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `monitor_select.ui.subtitle_font_family` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `monitor_select.ui.background_color` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `monitor_select.ui.text_color` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `monitor_select.ui.matched_text_color` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `monitor_select.ui.border_color` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `monitor_select.ui.backdrop_color` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `monitor_select.ui.subtitle_text_color` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `mode_indicator.monitor_select.enabled` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `mode_indicator.monitor_select.text` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `mode_indicator.monitor_select.background_color` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `mode_indicator.monitor_select.text_color` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `mode_indicator.monitor_select.border_color` | option | ✅ | ✅ | ❌ | monitor_select needs the optional MonitorSelector overlay extension, which the Windows backend does not implement |
| `smooth_cursor.move_mouse_enabled` | option | ✅ | ✅ | ❌ | cursor movement is not animated on Windows |
| `smooth_cursor.steps` | option | ✅ | ✅ | ❌ | cursor movement is not animated on Windows |
| `smooth_cursor.max_duration` | option | ✅ | ✅ | ❌ | cursor movement is not animated on Windows |
| `smooth_cursor.duration_per_pixel` | option | ✅ | ✅ | ❌ | cursor movement is not animated on Windows |
| `smooth_cursor.relative_movement_duration` | option | ✅ | ✅ | ❌ | cursor movement is not animated on Windows |
| `smooth_scroll.enabled` | option | ✅ | ✅ | ❌ | the Windows scroll is injected in one step; macOS and Linux animate it, and on X11 the steps are whole wheel notches because X has no smaller scroll to send |
| `smooth_scroll.steps` | option | ✅ | ✅ | ❌ | the Windows scroll is injected in one step; macOS and Linux animate it, and on X11 the steps are whole wheel notches because X has no smaller scroll to send |
| `smooth_scroll.max_duration` | option | ✅ | ✅ | ❌ | the Windows scroll is injected in one step; macOS and Linux animate it, and on X11 the steps are whole wheel notches because X has no smaller scroll to send |
| `smooth_scroll.duration_per_pixel` | option | ✅ | ✅ | ❌ | the Windows scroll is injected in one step; macOS and Linux animate it, and on X11 the steps are whole wheel notches because X has no smaller scroll to send |
| `--split-word` | mode flag | ✅ | ✅ | ❌ | splitting detected text into words needs the vision strategy, which Windows has no engine for; there the flag is refused rather than ignored |
| `--strategy=vision` | mode flag | ✅ | ✅ | ❌ | the vision strategy needs an element-detection engine, which macOS has in the Vision framework and Linux in tesseract; Windows has neither, so detection returns nothing and no hints appear; use axtree |
| `hide_cursor` | action | ✅ | ❌ | ❌ | a Wayland client may not hide another client's cursor, and the blessed Linux stack is Wayland; Windows has no equivalent either |
| `show_cursor` | action | ✅ | ❌ | ❌ | a Wayland client may not hide another client's cursor, and the blessed Linux stack is Wayland; Windows has no equivalent either |
| `scroll_left` | action | ✅ | ✅ | ❌ | the Windows wheel event carries no horizontal delta, so a sideways scroll injects nothing |
| `scroll_right` | action | ✅ | ✅ | ❌ | the Windows wheel event carries no horizontal delta, so a sideways scroll injects nothing |
| `feed` | action | ✅ | ✅ | ❌ | Windows has no key-injection path yet, so the key it would post is never sent; the key_feed capability reports stub to match |

<!-- END GENERATED PLATFORM SUPPORT -->

---

## Platform Exclusives

Features available on exactly one platform, with why they do not port. This is
a **closed set** — anything not listed here is a gap rather than an exclusive,
whatever the [Capability Matrix](#capability-matrix) currently reports
([ADR 0013](./adr/0013-parity-is-measured-in-words-not-subsystems.md)).

| Feature                                   | Platform | Location                                                | Why it is exclusive                                          |
| ----------------------------------------- | -------- | ------------------------------------------------------- | ------------------------------------------------------------ |
| System cursor hide + virtual-pointer replacement | macOS | `app/modes/cursor_darwin.go`, `adapter/overlay/render/virtualpointer/overlay_darwin.go` | A Wayland client may not hide another client's cursor. X11 could (`xfixes` is already linked in `platform/linux/cgo.go`), but the blessed stack is Wayland, so shipping it on one backend would not be parity |
| Screen-sharing hide                       | macOS    | `platform/darwin/overlay_darwin.m`                      | NSWindow sharing level is a Quartz concept                    |
| Secure input detection                    | macOS    | `platform/darwin/secureinput.go`                        | `CGSessionCopyCurrentDictionary`, a private API; neither X11 nor Wayland has the concept |

Two entries left this table in ADR 0013 and neither is coming back.
**Smooth scroll animation** was recorded as needing "a synthesizable continuous
scroll event stream"; the spike found one on Wayland — a
`zwlr_virtual_pointer_v1` axis event with no discrete step count, and libei's
pixel-precise scroll delta on KWin — and it now animates on every Linux backend,
with X11 limited to whole notches for the reason footnote ⁴ of the
[Capability Matrix](#capability-matrix) gives. A limit on one backend is not an
exclusive: it is that backend's documented limit, which is what
[ADR 0013](./adr/0013-parity-is-measured-in-words-not-subsystems.md) says the
non-blessed stacks carry. The **Vision (OCR) hint
strategy** was recorded as needing macOS-only `VNRequest` APIs; the API is
macOS-only but the capability is not, so it is a Linux gap too — met by an OCR
engine linked the way every other native dependency here is, with its language
data resolved at use. Its rectangle-detection half has no OCR answer, so
`detect_rectangles` and the four `rectangle_*` options stay macOS-only and are
declared as such.

Linux and Windows have no exclusive *user-facing* features — their unique
elements (evdev, `zwlr_virtual_pointer`, libei, the Wayland sync-cursor surface,
`WH_KEYBOARD_LL`, `RegisterHotKey`, SDF rendering) are platform *mechanisms*
serving cross-platform features, and are listed in the
[Capability Matrix](#capability-matrix).

---

## Known Gaps

Work that is genuinely missing, as opposed to deliberately platform-specific.
A gap is anything a person can *write* — an option, a mode flag, an action, a
command — that means less here than it does on macOS, whether or not the
[Capability Matrix](#capability-matrix) reports its subsystem as supported
([ADR 0013](./adr/0013-parity-is-measured-in-words-not-subsystems.md)).

**Linux**

1. Screen capture on KDE — X11 (`XGetImage`) and the wlroots family
   (`wlr-screencopy-unstable-v1`) capture real pixels and honor a region, so
   the blessed stack is done. KWin implements no screencopy protocol Neru can
   use and reports `CodeNotSupported` naming itself. Its only pixel source is
   the portal's ScreenCast session, whose frames arrive over PipeWire — a new
   required system library, so closing this adds `libpipewire-0.3` to the Linux
   dependency list, the packaging and the CI images. It may share a solution
   with entry 4, which already holds a portal session. It is also what keeps
   the `vision` hint strategy off KDE: the engine is linked on every backend,
   and KWin is the one with no pixels to hand it
3. X11 unmodified scroll — a scroll with no `--modifier` presses nothing, so the
   `XTestFakeButtonEvent` still carries whatever the X server records the user as
   physically holding. Binding `Ctrl+J` to a plain `scroll_down` therefore sends
   ctrl+scroll for as long as ctrl is down. macOS forces the empty set onto the
   event instead; a real-key backend has no per-event field to zero, so closing
   this means reading the live key state through `XQueryKeymap` in the C bridge
4. KDE RemoteDesktop portal grant — does not survive a daemon restart, so the
   consent prompt returns on every start
5. Grid virtual-pointer indicator — a no-op on Linux, while recursive grid
   draws it on all three platforms
6. `FocusedWindowBounds` — returns not-found on KWin, so callers silently fall
   back to the active screen
7. Wayland global hotkeys — a setup requirement rather than missing code: they
   need `input`-group membership and a CGO build. Failing loudly with the
   remedy, and documenting it as a first-class setup step, is the work
8. Tail — the tray tooltip is a no-op (dbusmenu carries no such property), the
   tray has one icon for both running and paused states where macOS has two,
   and the `CGO_ENABLED=0` build should announce its boundary once at startup
   rather than failing feature by feature

The gaps above are the work; what a person writes and finds inert *today* is
[Platform Support Per Word](#platform-support-per-word), which is generated
from the declaration the load-time warning and `neru doctor` also read. A gap
closing is the same edit in both places: the code lands and the word's column
widens.

**Not Linux gaps**, and deliberately so: secure input detection and system
cursor hide are [Platform Exclusives](#platform-exclusives); GNOME/Mutter
Wayland and Cosmic are supported-desktop decisions, not capabilities; X11
modifier passthrough is impossible for the display server rather than unbuilt —
`XGrabKeyboard` is all-or-nothing and `XSendEvent` is ignored by most
applications; and `neru services` on a non-systemd init is a stated boundary,
not unfinished work.

**Windows**

1. App watcher — no foreground-window change notifications, so per-app config never re-applies
2. Display hotplug — no screen-parameter change events
3. Native notifications — no toast support
4. UIA tree depth — shallow walk; complex apps under-report clickable elements
5. Grid and recursive-grid transition animation — not implemented
6. Smooth cursor and smooth scroll animation — not implemented
7. Modifier passthrough and `PostModifierEvent` — no-ops
8. Horizontal scroll — `ScrollAtCursor` ignores `deltaX`
9. `monitor_select` mode — returns `CodeNotSupported`
10. Font resolution — alias mapping only, no system font enumeration
11. `neru services` — every subcommand returns `CodeNotSupported`, where macOS
    installs a launchd agent and Linux a systemd user unit

**macOS**

1. Named keys without a Carbon keycode — `Insert` and `F21`–`F24` validate but
   never fire, because Carbon declares no virtual key code for them. They stay
   in the shared key vocabulary so one config file works on every platform
   ([ADR 0008](./adr/0008-a-vocabulary-has-one-home.md)), and the absence is
   pinned by `internal/architecture/named_key_tables_test.go` — the day macOS
   grows a keycode, that test fails.

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

- [The Three Tiers](#the-three-tiers) — **start here**; it decides where your code goes
- [platform/profile.go](../internal/adapter/platform/profile.go) — per-subsystem backend family and CGO expectations
- [ports/system.go](../internal/ports/system.go) — the main OS contract, plus the optional-extension pattern
- [ports/capabilities.go](../internal/ports/capabilities.go) and [capability_presets.go](../internal/ports/capability_presets.go) — the capability registry `neru doctor` reports
- [ports/font.go](../internal/ports/font.go) — FontResolver port
- [architecture/platform_slots_test.go](../internal/architecture/platform_slots_test.go) — the file-layout rules, as executable checks
- [ARCHITECTURE.md](./ARCHITECTURE.md) and the root [AGENTS.md](../AGENTS.md) conventions

Contributing Linux support? Nothing is reserved and waiting for you — read the
Linux files the package already has before writing anything. Where they sit
depends on the package: a single-platform directory such as
`internal/adapter/platform/linux/` drops the OS token and splits by backend
(`system_x11_cgo.go`, `system_wayland_wlroots_cgo.go`), while a mixed package
carries it (`internal/adapter/platform/factory_linux.go`,
`internal/adapter/overlay/backend_linux.go`). The slot table below is the full
set.

## The Three Tiers

Before choosing a file, choose a **tier**. Every platform-varying capability in
Neru is expressed one of exactly three ways, and picking the wrong one is the
most common way platform code becomes hard to navigate.

The deciding question is **who needs the capability**:

| Tier                            | Use when                                                              | Mechanism                                                                  |
| ------------------------------- | --------------------------------------------------------------------- | -------------------------------------------------------------------------- |
| **1 — Port**                    | app, domain, or more than one adapter package needs it                  | interface in `internal/ports`, adapter in `internal/adapter`        |
| **2 — In-package dispatch**     | exactly one adapter package needs it                                    | build-tagged `platform_<os>.go` files, **unexported** functions             |
| **3 — Optional port extension** | only some platforms can offer it, and the caller has a real fallback  | interface **declared in `ports`**, reached by type assertion                |

### Tier 1 — Port

The app and domain layers must never import an adapter package to reach an OS
capability. If they need it, it is a port.

Requirements — all four, or it is not done:

1. Interface in `internal/ports`, documented with what each platform is
   expected to do and what a caller must do when it cannot.
2. Adapter in `internal/adapter/<subsystem>/`, with
   `var _ ports.XPort = (*Adapter)(nil)`.
3. Mock in `internal/ports/mocks/`. Hand-rolled fakes in `_test.go` files
   rot silently when the contract changes — the shared mock does not.
4. An entry in `ports.PlatformCapabilities` so `neru doctor` reports it.

Current ports: `SystemPort`, `AccessibilityPort`, `OverlayPort`, `EventTapPort`,
`HotkeyPort`, `IPCPort`, `VisionPort`, `TextInputPort`, `KeyFeedPort`,
`AppWatcherPort`, `SystrayPort`, `FontResolver`.

Optional extensions, reached by type assertion (Tier 3): `RelativeCursorMover`
and `CursorSynchronizer` on `SystemPort`, `HotkeyReleaseRegistrar` and
`HotkeyHealthReporter` on `HotkeyPort`, `OverlayKeyboardPassthroughReporter` on
`EventTapPort`, and `OverlayCapabilityReporter` on `OverlayPort`.

[`keyfeed`](../internal/adapter/keyfeed/) is the reference example: shared
normalization untagged in `keyfeed.go`, one unexported `postKey` per platform,
`Adapter` implementing the port, capability entry, mock, contract tests.

### Tier 2 — In-package dispatch

A capability only one adapter package uses does **not** become a port. Wrapping it
in an interface buys no test seam and no substitutability — just indirection.

Use build-tagged files inside that package with **unexported** functions:

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

### Tier 3 — Optional port extension

Some platforms can do a job better than shared code can, but not all can do it
at all — so it cannot go on the base port without forcing every adapter to carry
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
  undiscoverable — a contributor on another platform has no way to learn the
  extension exists. This is why `relativeCursorMover` and `cursorSyncer` moved
  out of `services` and `modes`.
- **The caller must have a working fallback.** An optional extension is an
  optimization or a platform-native shortcut, never the only path.

Adapters opting in should assert it: `var _ ports.RelativeCursorMover =
(*SystemAdapter)(nil)`. Callers reach these by type assertion, so a signature
drift would otherwise silently downgrade the platform to the generic path
instead of failing to compile.

### Not ports

Do not lift these behind interfaces: `platform/{darwin,linux,windows}`
internals, `wlr_protocol`, overlay drawing in `internal/adapter/overlay`, `logger`,
and IPC transport. They are implementation, reached through a port that already
exists.

### Dependency direction

The tiers only mean something if the arrows point one way. Three rules, all
enforced by
[layering_test.go](../internal/architecture/layering_test.go):

| Rule                                                | Why |
| --------------------------------------------------- | --- |
| `internal/domain` imports no adapter, app, or UI | domain is pure Go; a domain package that needs an OS cannot be unit-tested |
| `internal/{domain,ports,derrors,adapter}` never import `internal/app` | adapters implement ports; the hexagon has no upward edges |
| app code reaches adapters only through ports        | only the composition root knows which adapter exists |

The third rule has three deliberate escapes, all narrow:

- **Shared vocabulary** — `adapter/ipc` (the CLI/daemon wire protocol),
  `adapter/logger`, and `adapter/platform` (the SystemPort factory plus the
  `Profile` that `neru doctor` prints). These are data and plumbing, not OS
  behavior.
- **Composition root** — `wiring.go`, `startup_phases.go`, `cmd/neru/main.go`.
  Wiring adapters to ports is their job. `component_factory.go` was on this
  list until #1213 and came off it: with the overlay's render components no
  longer handed back to the app, it names no adapter at all.
- **Build-tagged dispatch** — any `*_darwin.go` / `*_linux*.go` /
  `*_windows.go` / `*_other.go` file in the app layer is Tier 2.

Anything else is a violation. `knownLayeringExceptions` exists for edges that
cannot be fixed in the same change; it is currently **empty**, and a second test
fails if an entry stops being a real violation, so the list can only shrink.

> `internal/adapter/overlay` was the worked example of an escape, and is now
> the worked example of retiring one. It carried a shared-vocabulary entry
> until #1213, because the app named its render models and its manager
> interface directly. The entry went when the things above it moved: the port
> took `ports.Frame` for transitions, the adapter took over resolving Styles
> and building its own render components, and the per-mode `Context` types —
> which were mode state, not drawing — moved up into
> `internal/app/components/`. Nothing about the render models moved down. The
> lesson is that an allowlist entry is retired by finding what does not belong
> on the other side of the line, not by relocating what does.

## File Layout Rules

Once the tier is settled, the filename declares the slot. These rules are
enforced by
[platform_slots_test.go](../internal/architecture/platform_slots_test.go), so a
violation fails `just test` rather than review:

| Suffix                            | Meaning                                                   |
| --------------------------------- | --------------------------------------------------------- |
| `*_darwin.go`                     | macOS                                                     |
| `*_windows.go`                    | Windows                                                   |
| `*_linux.go`                      | Linux, with no backend axis to split on                   |
| `*_other.go`                      | non-target fallback for dispatch-style packages           |
| `*_unix.go`                       | the `!windows` side of a split (established Go convention) |
| `*_linux_common.go`               | Linux-shared wrapper, fallback, or backend routing        |
| `*_linux_x11.go`                  | X11                                                       |
| `*_linux_wayland.go`              | Wayland                                                   |
| `*_linux_wayland_<compositor>.go` | one compositor family needing a distinct path             |
| `*_cgo.go` / `*_nocgo.go`         | CGO and pure-Go variants of the same slot                 |
| `*_integration_cgo.go`            | cgo scaffolding for an integration test, `//go:build … && integration` so it never ships |

Inside a package that is already one platform (`adapter/*/darwin`,
`adapter/*/linux`, `adapter/platform/windows`, …) the OS token is dropped —
the directory carries it. `overlay/linux/wayland_cgo.go` and
`platform/linux/system_x11_cgo.go` keep only the axes that still vary; a
`system_linux_x11_cgo.go` inside `platform/linux/` would say linux twice.

The `*_integration_cgo.go` row exists for one situation and should stay rare:
Go rejects `import "C"` in a `_test.go` file outright, so an integration test
that needs C — `accessibility/native/linux/scroll_probe_integration_cgo.go`
mapping a Wayland window to measure what a compositor delivers — has to put it
in a non-test file. The `integration` term is what keeps that file out of every
build the product is made from, and the C stays inline in the cgo preamble
rather than in a `.c` file beside it, because a `.c` file compiles into the
package unconditionally.

That is why the four Linux backend rows above hold no files today: every Linux
backend split in the tree lives inside a single-platform directory and has
dropped the token. The rows are the spelling to use if a mixed package ever
needs one — not files waiting to be opened.

What the guardrail test checks:

- A file constrained to exactly one GOOS must carry that OS as a name token.
  Go's implicit suffix rule already prevents the forward mistake; this catches
  the reverse — a `tree.go` that is secretly `//go:build darwin` is invisible to
  anyone scanning the directory.
- A file whose constraint is a pure negation is a fallback and must be named
  `*_other.go`. `_stub.go`, `_stubs.go`, `_default.go`, `_fallback.go`,
  `_noop.go` and friends are rejected — one slot, one spelling.
- A file gated on cgo must say so: `*_cgo.go` or `*_nocgo.go`. Before this rule
  a plain name usually meant the cgo variant, but in the overlay package it
  meant the opposite, and reading the build tag was the only way to tell.
- Every file in a **single-platform package** declares its OS tag. Such a
  package is exempt from the suffix rule — the directory carries the meaning —
  which only holds if nothing untagged leaks in. The set of exempt directories
  is derived from the tree, not listed: a directory earns the exemption when
  every file in it targets the same one OS.
- Every relative `#include` resolves
  ([cgo_includes_test.go](../internal/architecture/cgo_includes_test.go)). This
  one exists because a broken include is invisible to `go vet` and to
  `just check-cross` — `CGO_ENABLED=0` skips the file — and only surfaces when
  the target OS compiles with cgo on.

Two rules that save review cycles:

- **Do not invent new ad hoc platform filenames** when a slot already exists.
- **Do not create empty `darwin` / `linux` / `windows` files for symmetry.** Add
  a file only when there is a real implementation slot behind it.

## Backend Packages

Every OS capability is a contract plus one directory per operating system, and
each backend directory is named for its GOOS:

```
adapter/accessibility/{ax, atspi, native/{darwin,linux,windows}}
adapter/eventtap/{tap, darwin, linux, windows}
adapter/hotkeys/{darwin, linux, windows}
adapter/systray/{darwin, linux, windows, icon}
adapter/overlay/{manager, darwin, linux, windows}
```

The directory names the platform, so the filenames inside do not have to, and
`ls` answers "what do I touch for Wayland?". Because the word `darwin` means the
same thing in every one of them, the guardrails that key on it need no
per-package list.

The parent package keeps the port adapter and a small build-tagged factory —
usually ten lines — which is the only place that knows which implementation
exists.

### When a backend does not earn a package

The test is whether every platform has something substantial to say. If one does
and the others answer in eighty-line stubs, build-tagged files in a single
package are clearer: three directories where two hold stubs is ceremony rather
than navigation.

That is the case for
`overlay/render/{grid,hints,recursivegrid,modeindicator,stickyindicator}`. Each
is one real renderer plus small stubs, and `overlay_other.go` is already the
obvious file to open.

### Giving a capability its own packages

A package that reads as "shared code plus platform files" is usually one generic
shell specialised by build-tagged concrete types. It has no interface seam, so
there is nothing for a backend package to implement, and creating one is three
moves in order:

1. **Find the seam.** List the methods the shell calls on the platform type.
   For `eventtap` that is ten — small enough to write down in one sitting.
2. **Extract the contract into a leaf package** (`accessibility/ax`,
   `eventtap/tap`). It has to be a leaf: the backends import it to satisfy it
   and the factory imports the backends, so anything else is an import cycle.
3. **Move each platform into a package behind a build-tagged factory.**

When the shell talks to *package-level* symbols rather than to methods on a
value, there is a cheaper route: alias instead of abstracting.
`accessibility/native` works this way. Its shell is generic over `Element`,
`ElementInfo`, `TreeNode`, `TreeOptions` and roughly forty package-level
functions; each platform's files live in their own package and a build-tagged
file aliases the four types and binds the functions, leaving the shell itself
platform-agnostic without an interface.

Two traps worth knowing before you start:

- **Named function types do not interchange.** A method taking
  `darwin.Callback` does not satisfy an interface wanting `func(string)`, even
  though the underlying types are identical. Put callback types in the contract
  package and have the backends use them.
- **Typed nil.** A factory returning a concrete `*T` as an interface hands back
  a non-nil interface holding a nil pointer, and every caller's `if x != nil`
  silently passes. Check before returning; `staticcheck` reports this as
  SA4023.

### Where the render models live

`hints.Hint`, `grid.Style` and the other render models sit under
`adapter/overlay/render/` rather than in the domain, even though nothing about
them is platform-specific.

They stay there because `hints.Hint`, `hints.StyleMode` and `hints.Overlay` are
one concept, and every backend needs all three to draw. Splitting them by layer
produces two packages named `hints` — likewise `grid` and `recursivegrid` —
which every site touching both halves must then alias, and three of the six
render packages have no platform-neutral content at all.

Nothing above the overlay names them any more (#1213), so their home is now a
question about the adapter alone. What did move out was the per-mode `Context`
types, which sat in `render/hints`, `render/grid` and `render/recursivegrid`
without being render models at all: they are the state one mode session keeps,
they know no colour and no surface, and they live in
`internal/app/components/{hints,grid,recursivegrid}` beside the scroll context
that always did.

### Styles are one type per concept

`grid.Style`, `recursivegrid.Style` and `hints.StyleMode` are each declared once
for every platform. Their fields hold the values the configuration writes — hex
color strings, integer sizes — and the packed-ARGB and float forms that Cairo
and GDI want are accessors that convert at the point of use.

Keeping representation out of the struct is what lets `manager.Interface` name
these types in a signature every platform shares. When a type looks
platform-specific, check whether it differs in meaning or only in
representation; the second kind belongs in an accessor.

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

Worked examples:

- X11 hotkeys → [x11_cgo.go](../internal/adapter/hotkeys/linux/x11_cgo.go)
- Wayland keyboard capture → [wayland_cgo.go](../internal/adapter/eventtap/linux/wayland_cgo.go)
- shared Linux system fallbacks → [system_common.go](../internal/adapter/platform/linux/system_common.go)

## Build And Test Commands

Every build and test recipe is catalogued in
[DEVELOPMENT.md](./DEVELOPMENT.md#common-tasks). Two apply specifically to
platform work:

- `just build && just test-foundation` — the cross-platform-safe baseline to run
  before touching anything. Do that first, then find the slot you expect to
  change before writing code.
- `just release-ci-linux <arch> <version>` / `just release-ci-windows <arch>
  <version>` — the tagged release binaries CI produces.

Only the target OS can run `just test` meaningfully — integration tests are
tagged per-OS.

### `just build-linux` needs a Linux-targeting C compiler

`just build-windows` cross-compiles from any host, because Windows is a CGO-off
build. `just build-linux` does not: Linux needs CGO for the X11 and Wayland
backends, which drags in Go's own cgo runtime (`linux_syscall.c`,
`gcc_<arch>.S`). A macOS clang compiles that against the macOS SDK and fails.

The recipe checks the compiler's target triple up front and refuses with the
alternatives rather than failing inside Go's runtime. From a macOS host, use:

- `just lint-cross` — compiles and lints the linux/amd64 build with CGO on, in
  Docker
- `just check-cross` — a fast CGO-off type-check of the Linux and Windows
  builds, no Docker needed
- `CGO_ENABLED=0 GOOS=linux GOARCH=<arch> go build ./cmd/neru` — a pure-Go Linux
  binary. The CGO-only backends compile out, so it is not the shipped product

`just build-linux` still runs on a Linux host, and on any host whose `CC` is a
Linux cross toolchain. The guard fails open — it only refuses when the compiler
positively reports a non-Linux target — so it never blocks a build that would
have worked. The tagged Linux release binaries are built by CI on a native Linux
runner (`just release-ci-linux`).

### `just lint` only sees your own platform

golangci-lint honours build tags, so a `//go:build linux` file is invisible to
`just lint` on macOS. A change can be locally clean and still fail the Linux or
Windows lint job.

A *build* break in one of those files no longer gets that far — `just ci` runs
`just check-cross`, which catches it before you push. Lint findings are the
part that still needs reproducing, and for one of those:

```bash
CGO_ENABLED=0 GOOS=linux golangci-lint run ./internal/...
```

Read the output with care. Without cgo, the `*_cgo.go` files are excluded, so
anything they alone use is reported as `unused` and any helper they alone call
with a second value is reported by `unparam`. Those are artifacts of the
no-cgo build, not real findings — CI lints Linux with cgo enabled. Findings in
plain-`linux` files (`funcorder`, `godoclint`, `revive`, and similar) are real.

The cgo-only Linux paths need a real Linux toolchain, so the host cannot lint
them directly. `just lint-cross` runs them in the same container image the Linux
CI job uses; without Docker, CI is the check for those.

## Linux Backend Model

Linux is a backend *family*, not a single target. Keep two axes separate:

- **Compile-time axis (OS + CGO)** — expressed by build tags and file suffixes.
  Build tags cannot distinguish compositors: KDE and GNOME are both `linux` +
  Wayland at compile time. A suffix therefore never encodes a single desktop
  environment on its own.
- **Runtime axis (which compositor is live)** — expressed by the `LinuxBackend`
  family in [backend_linux.go](../internal/adapter/platform/backend_linux.go),
  detected from environment variables and routed by `factory.go` plus dispatch
  seams such as `system_wayland_input.go`.

Within the compile-time axis, choose the slot by purpose:

| Slot      | Use for                                                                    |
| --------- | -------------------------------------------------------------------------- |
| `common`  | shared Linux types, shared fallbacks, backend detection/routing, helpers    |
| `x11`     | X11 display enumeration, event capture, overlays, pointer queries and warps |
| `wayland` | compositor capture/overlay behavior, layer-shell, output enumeration        |

Not every package must implement both backends immediately — but new code should
land in the right slot from the start. Accessibility is the main exception: most
Linux accessibility stays shared around AT-SPI even where other subsystems split.

### Organize by mechanism, not by desktop

Desktop environments share mechanisms, so the axis that actually varies is
usually the mechanism:

- **Input** — KDE and GNOME both use libei (RemoteDesktop portal); wlroots and
  COSMIC use `zwlr_virtual_pointer`. One libei backend serves several DEs; do
  not duplicate it per DE.
- **Overlay** — layer-shell works on KDE, wlroots, and COSMIC; only GNOME/Mutter
  lacks it.
- **Genuinely DE-specific** — active-window geometry (KWin D-Bus vs Mutter
  D-Bus) and hotkey registration. These belong in DE-named files such as
  `internal/adapter/accessibility/atspi/kwin_geometry.go`.

Use a `*_linux_wayland_<compositor>.go` sub-slot only when a compositor family
needs a path no other family shares — spelled without the OS token inside
`internal/adapter/platform/linux/`, which is where every one of them lives
today: `system_wayland_wlroots_*.go` (virtual-pointer input) and
`system_wayland_kde_*.go` (libei input), with `system_wayland_input.go` as the
shared routing seam.

**To add a compositor** (COSMIC, say): add a `LinuxBackend` value and detection
in `backend_linux.go`, route it in the factory and the relevant dispatch seams,
and add a new compositor sub-slot *only* if it cannot reuse an existing
mechanism file.

Per-DE decisions, measured protocol support, and known issues live in
[LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md); host setup lives in
[LINUX_SETUP.md](./LINUX_SETUP.md).

## Windows Model

Windows is one backend family with alpha-level support. Prefer:

- `*_windows.go` as the implementation slot
- pure Go Win32 / COM bindings (via `x/sys/windows` or syscall) over CGO

Do not introduce additional Windows backend naming until there is a real reason.
See [Known Gaps](#known-gaps) for the current Windows to-do list — several
entries there are well-scoped starter tasks.

## CGO Guidance

**Do not decide CGO usage by OS alone.** CGO is a per-backend decision, and
[profile.go](../internal/adapter/platform/profile.go) is the source of truth.

Current intent:

- **macOS** — CGO required throughout (Objective-C bridge)
- **Linux** — backend-dependent; several backends already require it, and
  `*_nocgo.go` variants must still compile and degrade honestly
- **Windows** — pure Go first

Good default instincts:

- AT-SPI and freedesktop notifications should prefer pure Go / D-Bus paths
- X11 may be feasible in pure Go depending on library choice
- Wayland and compositor integrations often need CGO or native helpers
- Win32 hotkeys, hooks, monitor APIs, and UIA should prefer pure Go bindings

If you introduce a backend that changes the build story, update
[profile.go](../internal/adapter/platform/profile.go), the
[justfile](../justfile), and this document — and state the build assumption
explicitly in your PR description and the backend's package comments.

## Hotkeys And Modifiers

Shared code must not hard-code macOS conventions:

- use `Primary` when you mean "the main accelerator modifier"
- `Primary` maps to `Cmd` on macOS and `Ctrl` on Linux/Windows
- keep backend-specific key translation inside `adapter/platform` code
- never leak X11, Wayland, Carbon, or Win32 naming into shared app logic

Relevant files: [config.go](../internal/config/config.go),
[modifiers.go](../internal/domain/action/modifiers.go),
[binder.go](../internal/app/keybinding/binder.go).

On macOS, per-hotkey CGEventTaps are re-registered on keyboard-layout change
(via `NeruSetKeymapLayoutChangeCallback2`) because `NeruKeyNameToCode` maps key
names to layout-aware keycodes.

## Adding A New Capability

Start from [The Three Tiers](#the-three-tiers) — the tier decides everything
below.

**Tier 1, extending an existing port** (a new OS operation the app needs, and a
port already covers that subsystem — e.g. another screen query):

1. Add the method to the port, documenting what each platform should do
2. Implement it in the darwin adapter
3. Add a Linux shared fallback in `system_common.go`
4. Add a Windows implementation or explicit `CodeNotSupported` stub
5. Push backend-specific Linux behavior down into `system_x11_cgo.go` or
   `system_wayland.go`
6. Add the method to the mock in `internal/ports/mocks/`
7. Update capability reporting if the support surface changed

**Tier 1, a whole new port** (a subsystem no port covers yet): everything above,
plus a new `internal/ports/<name>.go`, an adapter package under
`internal/adapter/`, a `PlatformCapabilities` field **and** its `Entries()`
registration, and wiring in `startup_phases.go`. Copy the shape of
[`keyfeed`](../internal/adapter/keyfeed/).

**Tier 2, one adapter package only** (isolated platform behavior):

1. Keep the shared package code platform-agnostic
2. Use `platform_darwin.go` / `platform_other.go` dispatch files, unexported
3. Add Linux backend files inside that package rather than pushing detection up
   into shared app or service code

**Tier 3, an optional extension:** declare the interface in `ports` beside the
port it extends, implement it on the adapters that can, assert compliance with
`var _ ports.X = (*SystemAdapter)(nil)`, and give the caller a fallback.

### Adding a capability to `neru doctor`

`PlatformCapabilities` is a registry, not just a struct. Add the field, add a
`CapabilityKey` constant, and register the pair in `Entries()`. Every renderer
(`neru doctor`, the IPC info map) iterates `Entries()`, so that is the only
edit — and
[capabilities_test.go](../internal/ports/capabilities_test.go) fails if a
field is added without registering it. Then fill the entry in all three presets
in [capability_presets.go](../internal/ports/capability_presets.go).

## Errors And Capability Reporting

Unimplemented platform behavior returns `CodeNotSupported` — never a silent
no-op, unless the behavior is explicitly documented as best-effort:

```go
return derrors.New(derrors.CodeNotSupported, "ScreenBounds not yet implemented on linux")
```

Name the missing operation and the platform in the message. Callers degrade
gracefully via `derrors.IsNotSupported(err)`.

**A word is not the same question as a subsystem.** When the thing you shipped
or stubbed is an option, a mode flag or an action rather than a capability,
the answer goes in the `PlatformSupport()` declaration beside that vocabulary —
`internal/config/platform_support.go`,
`internal/domain/modecmd/platform_support.go`,
`internal/domain/action/platform_support.go` — and
`internal/architecture/platform_support_test.go` fails the build while a word
has no column. Regenerate the published table with `just gensupportref`. A
subsystem can be green in every cell of the matrix while an option a person
wrote means nothing
([ADR 0013](./adr/0013-parity-is-measured-in-words-not-subsystems.md)).

Capability reporting is part of the contract, not a user nicety — it is what
`neru doctor` prints. When you implement or partially implement a feature,
review [capabilities.go](../internal/ports/capabilities.go),
[capability_presets.go](../internal/ports/capability_presets.go), and
[info.go](../internal/app/ipcctrl/info.go). A stub must report `stub`, not
`supported` — and a shipped feature must stop reporting `stub`. When a feature
becomes real: replace the `CodeNotSupported` return, update the capability
detail, and delete TODO wording that no longer applies.

## Testing Checklist

- **unit tests** for shared parsing, normalization, routing, or config logic
  (`*_test.go`, using mocks from `internal/ports/mocks`)
- **contract tests** pinning `CodeNotSupported` behavior and capability semantics
- **integration tests** for real platform behavior, tagged per-OS
  (`*_integration_linux_test.go`, `*_integration_darwin_test.go`,
  `*_integration_windows_test.go`)

Questions your tests should answer:

- does the adapter return the right error when the feature is unsupported?
- does the capability matrix reflect the new state?
- does backend selection route to the intended Linux slot?
- does shared logic stay platform-neutral?

## Documentation Checklist

Land docs in the same PR as the platform work. Each fact has exactly one home —
update the one that owns it rather than restating it elsewhere:

| What changed                                    | Update                                                        |
| ----------------------------------------------- | ------------------------------------------------------------- |
| A capability's status or mechanism              | **this file** — the parity tables in Part 1                   |
| A gap closed or discovered                      | **this file** — [Known Gaps](#known-gaps)                     |
| Which platforms an option, mode flag or action does anything on | the `PlatformSupport()` declaration beside that vocabulary, then `just gensupportref` |
| Desktop-specific setup, protocol support, or a DE workaround | [LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md)         |
| Host dependencies, permissions, or deployment   | [LINUX_SETUP.md](./LINUX_SETUP.md) — keep DE-agnostic         |
| A layer boundary, port contract, or data flow   | [ARCHITECTURE.md](./ARCHITECTURE.md)                          |
| A build recipe or test tier                     | [DEVELOPMENT.md](./DEVELOPMENT.md)                            |
| Go style, logging, or naming                    | [AGENTS.md](../AGENTS.md) (Conventions)                       |
| What the project claims to support, at a glance | [README.md](../README.md)                                     |

ARCHITECTURE.md deliberately does **not** track per-platform support — it
describes shape, not status. Do not add a capability table there.

## Contributing Safely

**Good starter tasks:**

- improve capability detail text for an existing platform slice
- replace a Linux `CodeNotSupported` return with real X11 or AT-SPI behavior
- add a contract test for a currently stubbed feature
- pick a numbered item from [Known Gaps](#known-gaps)
- document missing backend assumptions in the package you are touching

**Higher-risk — open or link an issue first:**

- changing shared input semantics
- introducing CGO to a backend that was previously pure Go
- moving shared logic into platform packages
- mixing backend detection into app or service code

**A good platform PR** leaves the repo better in five ways: the implementation
sits in the intended file slot, unsupported paths stay explicit and honest,
capability reporting is updated, tests cover the new behavior or contract, and
the docs tell the next contributor what changed. That is the bar even for small
slices.
