# Platform support

What works on macOS, Linux, and Windows, and where a platform behaves
differently. Every claim here is derived from the code. If this page and the
code disagree, the code is right and this page has a bug.

## Platform status

### What the labels mean

**Stable**: fully featured *and* proven in use. A gap on this platform is a
bug.

**Beta**: ready for daily use. Every navigation mode works and behaves as it
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
[Known Gaps](#known-gaps) carries no entry for either
([ADR 0013](../adr/0013-parity-is-measured-in-words-not-subsystems.md)).
CI jobs on a headless Sway session and on native Windows gate every merge.

**Both stay Beta anyway**, because parity is a claim about coverage and Stable
is a claim about reliability. The Linux and Windows work is proven by CI rather
than by daily use. A Beta platform moves to Stable after six consecutive
releases in which no bug specific to it is filed, meaning one a macOS user would
not also hit.

### Per-platform

| Aspect               | macOS                       | Linux                                    | Windows                        |
| -------------------- | --------------------------- | ---------------------------------------- | ------------------------------ |
| **Status**           | **Stable**                  | **Beta**                                 | **Beta**                       |
| **Primary modifier** | `Cmd`                       | `Ctrl`                                   | `Ctrl`                         |
| **Display stack**    | Cocoa / Quartz              | X11, or Wayland (wlroots / KWin / COSMIC / Mutter) | Win32 / DWM           |
| **Accessibility**    | AXUIElement                 | AT-SPI over D-Bus                        | UI Automation                  |
| **Distributed as**   | `Neru.app`, codesigned      | Binary + install script                  | Binary                         |

### Linux backends

Linux is not one target. Neru picks its backend once at startup from
`XDG_CURRENT_DESKTOP`, `WAYLAND_DISPLAY`, and `DISPLAY`, and `neru doctor`
reports the result as `display_server`.

| Backend                | Detected when                                                          | Status            |
| ---------------------- | ---------------------------------------------------------------------- | ----------------- |
| `x11`                  | `DISPLAY` set, no `WAYLAND_DISPLAY`                                    | Supported         |
| `wayland-wlroots`      | Sway, Hyprland, niri, River, Wayfire, labwc, a `:wlroots` tag, or unset `XDG_CURRENT_DESKTOP` | Supported         |
| `wayland-kde`          | `XDG_CURRENT_DESKTOP` contains `KDE`                                   | Supported         |
| `wayland-cosmic`       | `XDG_CURRENT_DESKTOP` contains `COSMIC`                                | Supported         |
| `wayland-gnome`        | `XDG_CURRENT_DESKTOP` contains `GNOME`                                 | Supported with Xwayland |
| `wayland-other`        | Any other Wayland compositor                                           | **Not supported** |
| `unknown`              | Neither `WAYLAND_DISPLAY` nor `DISPLAY`                                | **Not supported** |

> [!NOTE]
> **The daemon refuses to start on `wayland-other` and `unknown`** rather than
> starting degraded. `wayland-gnome` is refused the same way when `DISPLAY` is
> unset, because Mutter implements no `wlr-layer-shell` and the overlay there
> is a window on Xwayland.
>
> **GNOME matches the KDE column below with four differences.** The overlay is
> X11 + Cairo on Xwayland. Focused-app identity, the app watcher and the window
> origin come from the Neru GNOME Shell extension, which works after one
> re-login once the daemon has installed it. The cursor position is re-learned
> through an Xwayland window. Keyboard capture is the evdev proxy alone.
> [Linux desktops](../guide/linux-desktops.md#gnome-wayland) has the detail.

## Capability Matrix

Status of each capability, with the mechanism that implements it. The KDE and
wlroots columns differ only where noted. Both are Wayland with
`wlr-layer-shell` overlays.

**Legend:** ✅ supported · ⚠️ works with known limits · 🟡 stub (`ERR_NOT_SUPPORTED`
or no effect) · ❌ no code path · ➖ macOS-only capability, exempt from parity
(see [Platform Exclusives](#platform-exclusives))

This table answers whether a *subsystem* works. Whether every option, mode
flag, action and command means the same thing on each platform is what
[Known Gaps](#known-gaps) and [Platform support per word](#platform-support-per-word)
answer.

| Capability                    | macOS                    | Linux X11              | Linux Wayland (wlroots)      | Linux Wayland (KDE)     | Windows                      |
| ----------------------------- | ------------------------ | ---------------------- | ---------------------------- | ----------------------- | ---------------------------- |
| **Screen bounds / enumeration** | ✅ Cocoa               | ✅ XRandR              | ✅ xdg-output                | ✅ xdg-output           | ✅ `EnumDisplayMonitors`     |
| **Display hotplug events**    | ✅ screen-params notif.  | ✅ RandR event fd      | ✅ `wl_output` events        | ✅ `wl_output` events   | ✅ `WM_DISPLAYCHANGE`        |
| **Focused app identity**      | ✅ NSWorkspace + AX      | ✅ `_NET_ACTIVE_WINDOW` / `WM_CLASS` | ⚠️ app_id only (see below) | ⚠️ app_id only     | ✅ `GetForegroundWindow`     |
| **App watcher (focus change)**| ✅ NSWorkspace observer  | ✅ event-driven        | ✅ event-driven              | ✅ event-driven         | ✅ `SetWinEventHook`         |
| **Keymap learns the focused app** | ✅ published by the watcher | ✅ published by the watcher | ✅ published by the watcher | ✅ published by the watcher | ✅ published by the watcher |
| **Cursor position**           | ✅ `CGEventGetLocation`  | ✅ `XQueryPointer`     | ✅ `hyprctl` on Hyprland, else sync-surface cache | ✅ sync-surface cache | ✅ `GetCursorPos` |
| **Cursor move**               | ✅ `CGEventPost` | ✅ XTest (`XTestFakeMotionEvent`) | ✅ `zwlr_virtual_pointer` | ✅ libei                | ✅ `SetCursorPos`, glided while a button is held |
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
| **Key feed (`action feed`)** | ✅ `CGEventPost`         | ✅ uinput               | ✅ uinput / virtual-keyboard | ✅ uinput               | ✅ `SendInput`               |
| **Service management (`neru services`)** | ✅ launchd user agent | ⚠️ systemd user unit only ² | ⚠️ systemd user unit only ² | ⚠️ systemd user unit only ² | ✅ Task Scheduler logon task |

¹ **Font resolution.** Every platform resolves font *families* through the OS,
and a named family resolves to **that name**. A family the platform can see is
missing goes to the sans baseline (DejaVu Sans on Linux, Segoe UI on Windows)
rather than to the platform's own substitute. macOS and the non-CGO Linux build
check nothing and let the renderer substitute at draw time. The generic names
`sans`, `serif`, `mono` (and their spelling variants, and the empty string)
resolve to each platform's own faces (ADR 0007).

**Text measurement** sizes every box drawn around text from the text itself, on
every platform: hint badges, the hint search badge, the mode and
sticky-modifier indicators, recursive-grid labels and the monitor picker's
badge. The non-CGO Linux build has no Cairo and falls back to an estimate.

² **Service management** is the one row whose limit is not the display server.
It needs **systemd**, on every Linux backend. runit, OpenRC and s6 get
`ERR_NOT_SUPPORTED` from every `neru services` subcommand, a stated boundary
rather than a gap. See the service management note below the matrix.

³ **Smooth scroll granularity.** `smooth_scroll` animates everywhere, but only
some backends can send a step shorter than a wheel notch: the wlroots virtual
pointer and libei carry fractional deltas, Windows counts 120ths of a notch,
and X11 core scrolling is one notch per event. Neru sends the same distance on
every backend. Linux paths that send notches convert at 30 px per notch and
round to the nearest, never fewer than one, so on X11 a `scroll_step` under 45
px is a single unanimated click and everything from two notches up gets the
same eased curve as elsewhere. The wlroots behaviour is measured in tests. The
X11 and KDE conclusions are read from the protocol sources and not measured on
hardware.

⁴ **Native hint-search field.** Only macOS has a platform text control that
owns keyboard focus and brings the system input method with it. Everywhere else
the query is read from the key stream, so dead keys and IME composition do not
work there. The search *badge* is a different thing. Every platform draws one,
and `hints.search_input_ui.*` means the same on all three.

⁵ **Screen capture** is taken per backend rather than through the desktop
portal everywhere, because a consent picker in front of a hint refresh would
cost latency. X11 uses `XGetImage`, wlroots `wlr-screencopy`, Windows `BitBlt`,
and none of them asks consent. **KWin implements neither**, so KDE uses the
portal: an `org.freedesktop.portal.ScreenCast` session over PipeWire. It is a
permission rather than a missing capability. The grant is remembered under
`$XDG_STATE_HOME/neru/`, and the consent dialog appears only when a mode that
needs capture starts. A capture region that leaves the screen, spans two
Wayland outputs, or lies on a monitor the user declined to share fails rather
than coming back clipped. Scaled outputs return physical pixels, as
Retina does on macOS.

⁶ **Vision on Linux and Windows is text-only**, and permanently so. macOS runs
three Vision requests (text, rectangles, saliency). An OCR engine answers only
the first, so `hints.vision.detect_rectangles` and the four `rectangle_*`
options are macOS-only. The `contour` strategy is a separate detector that works
on every platform. On Linux the engine is **tesseract**, with its language data
found through `TESSDATA_PREFIX`, then the distribution paths. A missing
`eng.traineddata` gets `ERR_NOT_SUPPORTED` naming that file. On Windows it is
**`Windows.Media.Ocr`**, which needs the OCR language pack for one of the
account's languages and reports no per-word confidence, so the three
`*_confidence` floors are inert there. Recognized text is screen content, so
Neru never logs it or writes it to disk.

⁷ **Modifiers on injected input (X11, Wayland and Windows).** An X11 pointer
event, a Wayland pointer event and a `SendInput` mouse event all carry whatever
modifiers the keyboard currently holds. Left alone, a hotkey chord still held
while a hint was chosen would turn the click into a ctrl+click. Neru reads the
live key state, releases the modifiers the injection would falsify, presses the
ones asked for, and undoes both when done. That state holds across every chunk
of an animated scroll, and across a drag until its release. On Wayland the keyboard the
compositor reads is the evdev proxy's, so the proxy does the releasing and
re-pressing. Without a forwarding proxy (no `/dev/uinput`, or the wl-keyboard
fallback) Neru has no modifier to release, since the compositor reads the
physical keyboards itself, and the click stays modified.

⁸ **Tray and notifications.** The tray icon shows the paused state on every
platform. Per-item menu tooltips exist nowhere on Linux, because
`com.canonical.dbusmenu` defines none. **Notifications on Windows are balloon
tips on the tray icon**, because WinRT toasts need an AppUserModelID an
unpackaged exe does not have. With `systray.enabled = false` there is nothing to
attach a tip to, so notifications report `ERR_NOT_SUPPORTED`. Alerts are
`MessageBoxW` and do not depend on the tray.

⁹ **Hyprland modified scroll.** With a virtual-keyboard modifier held, a
`zwlr_virtual_pointer` scroll produces no event on Hyprland, so there the
modifier goes out on the virtual keyboard and the scroll on the uinput wheel, in
whole notches.

¹⁰ **windows/arm64 overlay.** windows/arm64 builds draw with the GDI renderer
only, because the Direct2D binding is amd64-only.

### Notes on the ⚠️ entries

**Focused app on Wayland.** wlroots and KWin name the focused window by its
app_id, which is what per-app config keys on there. They do not expose the
process ID, so anything that needs one is matched best-effort from the app_id.

**App watcher.** On Linux and Windows the app watcher reports activation and
deactivation only. Launch, terminate and Mission Control events stay
macOS-only. What per-app config matches on each platform is in
[App identity across platforms](configuration.md#app-identity-across-platforms-bundle_id).

**Global hotkeys on Wayland.** No Wayland protocol lets an ordinary client
register a global hotkey, so Neru matches chords itself on an evdev keyboard
proxy ([ADR 0014](../adr/0014-the-wayland-keyboard-is-a-proxy.md)). With
`/dev/uinput` writable the proxy holds every keyboard from daemon launch and
re-emits it through a uinput device, so a matched chord is withheld from the
focused app. It never grabs a keyboard that has a key down, so no modifier is
left stuck. Without uinput it reads passively and the app receives the chord
too. It needs the `input` group and a CGO build. Either way Neru warns once with
the remedy and names the fallback, binding `neru <mode>` in the compositor.

**Native alerts on Linux.** Notifications and alerts both go to the session's
freedesktop notification daemon over D-Bus. An alert is a notification with
critical urgency and no expiry, and it is *not* modal. No Wayland or X11 client
can show a blocking dialog that returns the pressed button. With no notification daemon, both
report `ERR_NOT_SUPPORTED`, `neru doctor` says what to install, and the two
startup alerts fall back to stderr.

**Service management on Linux.** The service is a systemd user unit tied to
`graphical-session.target`, so a logout and login restarts the daemon rather
than orphaning it. Only systemd is covered.
[Linux setup](../guide/linux.md#systemd-user-service) says where the unit lives
and how to control it.

**Smooth cursor animation.** Off by default on every platform. Opt in with
`smooth_cursor.move_mouse_enabled`. Clicks stay instant, and actions that
depend on the cursor position wait for the animation to settle first.

## Input injection

Every action (click, press, release, toggle, absolute and relative moves, drag
and scroll) behaves the same on every platform. Only the API that sends it
differs, which matters when a desktop asks for a permission:

| Platform              | Injection API                                                                |
| --------------------- | ---------------------------------------------------------------------------- |
| macOS                 | `CGEventPost`                                                                |
| Linux X11             | XTest                                                                        |
| Linux Wayland wlroots | `zwlr_virtual_pointer` (+ `/dev/uinput` for scroll)                           |
| Linux Wayland KDE     | libei via `org.freedesktop.portal.RemoteDesktop` (+ `/dev/uinput` for scroll) |
| Windows               | `SendInput` / `SetCursorPos`                                                  |

Scrolling works on both axes everywhere. A modified scroll (`--modifier`) holds
the real key on Linux and Windows. A backend that cannot send the modifier
answers `ERR_NOT_SUPPORTED` rather than scrolling unmodified.

**Held mouse buttons** are released automatically when Neru returns to idle. On
Windows, a cursor move made while a button is held is sent as a short glide of
relative motion, because applications such as Windows Terminal select text only
from intermediate relative motion. The Linux backends warp the pointer and let
the compositor infer the drag.

## Keyboard capture and hotkeys

| Aspect                | macOS                  | Linux X11               | Linux Wayland                            | Windows                 |
| --------------------- | ---------------------- | ----------------------- | ---------------------------------------- | ----------------------- |
| **In-mode capture**   | `CGEventTapCreate`     | `XGrabKeyboard`         | evdev proxy (uinput re-emit), wl-keyboard fallback | `WH_KEYBOARD_LL`        |
| **Global hotkeys**    | Per-key CGEventTap     | `XGrabKey`              | Chord matcher on the evdev proxy         | `RegisterHotKey`        |
| **Modifier passthrough** | ✅                  | ❌ grab is all-or-nothing | ✅ evdev only                          | ✅                      |
| **Sticky modifiers**  | ✅                     | ✅                      | ✅                                       | ✅                      |

**A held global hotkey** is reported as one press and one release on every
platform, which is what `[held_repeat]` repeats between.

**A global chord while a mode is active.** A `[hotkeys]` binding keeps working
from inside a mode on every platform, and the mode's own binding for the same
chord wins. Only chords carrying Ctrl, Alt or Cmd fall back to the global
table, since a bare key inside a mode is a hint label or a grid cell key. On
Linux the mode owns the keyboard exclusively, so while a mode is open a chord
bound in the *compositor* rather than in `[hotkeys]` cannot fire.

**One key, one name.** Both Linux readers of `/dev/input` resolve a scan code
through the compositor's XKB keymap, and the X11 tap names the key from the
state-resolved **keysym** rather than the string `XLookupString` returns, so
all backends call `Shift+;` the same thing and XKB options like
`ctrl:swapcaps` reach Neru's own bindings. **On a non-QWERTY layout this
decides which physical key a `[hotkeys]` chord binds.** It is the key bearing
that character on the **reference layout**. A layout is ASCII-capable when its
letter row types ASCII. On X11 the reference is the active layout when it is
ASCII-capable, else the first ASCII-capable layout of the keymap, else the
active layout. The X11 tap reads the keymap when a mode starts, so a layout
added mid-mode applies from the next one. On Wayland the reference is the first
ASCII-capable layout of the compositor's keymap, else the active layout.
Wayland cannot prefer the active layout because the compositor tells only the
focused client which one it is, and Neru never holds keyboard focus. With two
Latin layouts, such as `us` and Dvorak, Wayland bindings therefore stay on the
first. Either way a `us,ru` or `ru,us` keymap keeps every binding, hint label
and grid key on its physical key while Russian is active. Shift, Caps Lock,
NumLock and AltGr still choose the level, and Neru identifies modifiers in the
active layout, which is where remaps like `ctrl:swapcaps` are defined.
`general.kb_layout_to_use` forces the reference by the layout's XKB name on
both backends, and the X11 `[hotkeys]` grab follows it too. A keysym is named
by the character it types when it types one, and by keysym name otherwise. The
one key XKB renames under Shift, `ISO_Left_Tab`, folds back to `Tab`, which is
what lets the default `Shift+Tab` hint binding fire.

**Windows names punctuation in the reference layout too.** Neru names letters
and digits by virtual-key code, and every layout keeps `VK_A` to `VK_Z` and
`VK_0` to `VK_9` on the keys of those names, Russian included. Punctuation sits
on the `VK_OEM_*` codes, whose characters come from the layout, so both naming
a key and parsing a `[hotkeys]` chord use the reference layout chosen by the
X11 rule. The active layout is the foreground window's, and the fallback list
is the user's installed layouts. So `` ` `` and `;` keep working while
Russian is active. `general.kb_layout_to_use` forces the reference by keyboard
layout identifier. Neru checks the reference layout once a second and registers
the hotkeys again when it changes.

**Modifier passthrough (Wayland evdev, and Windows).** While a mode is active
Neru captures the keyboard exclusively, so shortcuts it does not bind never
reach the focused app. With `general.passthrough_unbounded_keys`, unbound Ctrl/Alt/Cmd
chords reach the focused app instead. It is **not** available on X11 (an
`XGrabKeyboard` routes Neru's own XTest events back to itself, and
`XSendEvent` is ignored by most apps) nor on the wl-keyboard fallback. Turning
it on under X11 warns once at load, and `neru doctor` lists the option in its
`platform_support` row. `general.should_exit_after_passthrough` exits the mode
after a passthrough.

## Accessibility and hints

| Aspect                  | macOS                                       | Linux                                              | Windows                              |
| ----------------------- | ------------------------------------------- | -------------------------------------------------- | ------------------------------------ |
| **Backend**             | AXUIElement                                 | AT-SPI over D-Bus                                  | UI Automation                        |
| **Traversal**           | Full recursive walk of the accessibility tree | Recursive walk of the active window, depth and node capped | Control-view tree, any depth |
| **Sources collected**   | Frontmost + all windows, popovers, menubar, dock, notification center, Stage Manager, PIP | Active window only          | Foreground window's control view     |
| **Strategies**          | `axtree` (default), `vision` and `contour`, incl. per-app overrides | `axtree`, `vision` (text only) and `contour` | `axtree`, `vision` (text only) and `contour` |
| **Popovers / menus**    | ✅ dedicated detection                      | ⚠️ only if inside the active window                | 🟡                                   |

macOS builds the richest tree by a wide margin. On Linux, coverage depends on
each app exposing AT-SPI. On Windows, an element an app exposes only in UI
Automation's raw view is not hinted. Where the tree is thin, the `vision` and
`contour` strategies read the screen instead. The fix for a missing element is
a role or filter setting ([Clickable roles](configuration.md#clickable-roles)),
never a per-app workaround.

**Chromium and Electron apps on Linux** do not expose their web-content tree
over AT-SPI by default, and there is no per-app attribute Neru can toggle to
force it. Launch the app with `--force-renderer-accessibility`. Native GTK/Qt
apps and Firefox need no flag. This is Chromium behavior, not a Neru limitation.

**Picking the active window.** Neru matches the AT-SPI window against the
focused window's identity (app_id and title on Wayland, `WM_CLASS` and window
name on X11), because the AT-SPI "active" flag is unreliable across apps.

**Window-origin offset on Wayland.** A Wayland client cannot know its own
on-screen position, so AT-SPI reports element positions relative to the
window. Neru offsets them by the focused window's screen origin, which it asks
the compositor for:

| Compositor | Source                                                       | Limits                                                                                                                    |
| ---------- | ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------- |
| KDE / KWin | KWin script reporting focused-window geometry over D-Bus     | Reports on activation and on the focused window moving, resizing or going away. Neru reinstalls the script when KWin restarts. A drag reports its final rectangle. |
| niri       | `niri msg -j focused-window` / `focused-output`              | Floating and fullscreen windows only. **Tiled** windows expose no on-screen position ([niri#2381](https://github.com/niri-wm/niri/issues/2381)), so hints are misaligned there. |
| Sway       | `swaymsg -t get_tree`                                        | none                                                                                                                       |
| Hyprland   | `hyprctl -j activewindow`                                    | none                                                                                                                       |
| GNOME      | GNOME Shell extension reporting the focused window's frame over D-Bus | The daemon installs and enables the extension. The shell loads it at the next login. |
| Anything else (X11, River, Wayfire) | none                                                | X11 needs none, because AT-SPI already reports screen positions there. The rest report no origin, so hints stay window-relative. |

An unavailable origin leaves positions unoffset rather than guessing. Neru
warns when a compositor command could not be run or returned invalid output.

**The same sources answer "where is the focused window"**, which scopes vision
and contour detection, `neru action move_mouse --window`, and a grid, recursive
grid or bisect session started with `--capture-scope window`. A Wayland
compositor with no source (River, Wayfire) reports `ERR_NOT_SUPPORTED` there.

## Overlay rendering

| Aspect                | macOS                                    | Linux X11                              | Linux Wayland                                   | Windows                            |
| --------------------- | ---------------------------------------- | -------------------------------------- | ------------------------------------------------ | ---------------------------------- |
| **Window type**       | NSPanel, borderless non-activating       | override-redirect X11 window           | `wlr_layer_shell_v1` overlay surface             | `WS_POPUP` HWND                    |
| **Rendering**         | CoreAnimation (GPU)                      | Cairo (CPU)                            | Cairo (CPU)                                      | Direct2D on DirectComposition (GPU), GDI fallback (CPU) |
| **Click-through**     | yes                                      | yes                                    | yes                                              | yes                                |
| **Window identity**   | panel title `neru-overlay`               | `WM_CLASS` and name `neru-overlay`     | layer-shell namespace `neru-overlay`             | window class `neru-overlay`        |
| **HiDPI**             | per-display backing scale                | `Xft.dpi`, one global factor           | `wl_output` scale + fractional scale             | per-monitor DPI                    |
| **Multi-monitor**     | per-display, live screen changes         | every monitor, live RandR hotplug      | one surface per output (max 16), live hotplug    | live display hotplug               |

The window identity is what a window manager or compositor rule can match on,
for example to exclude the overlay from a screenshot tool or a tiling rule.

Grid cell sizes are planned in apparent units on every platform, so a grid
looks the same size on a scaled display. `recursive_grid.min_size_width` and
`min_size_height` are read the same way.

## Mode coverage

Mode logic (labelling, alphabets, matching, search filtering, grid subdivision,
recursion depth, scroll amounts, cell navigation) is shared and behaves
identically on all three platforms. Only the rows below differ, and every
difference traces to rendering or element discovery rather than the mode
itself.

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
> hidden, is separate from the two grid indicators above. It is macOS-only,
> because it pairs with hiding the system cursor, which has no equivalent
> elsewhere.

> **`hints.ui.placement` means the same thing on all three platforms.** macOS
> draws a shorter, wider arrow, so an offset badge sits a few pixels closer to
> its element there. On Windows the arrow is a triangle over a slightly larger
> one in the border colour.

> **Label sizing is the same on all three platforms.** Recursive-grid and grid
> labels are fitted to their cells, `recursive_grid.ui.sub_key_preview` is one
> drawing everywhere, and the monitor picker fits its key and the monitor's
> name to the badge.

## Platform support per word

Every option, mode flag and action declares the platforms it does something
on. The table below lists only the words whose answer is narrower than every
platform. The several hundred that work everywhere are not listed.

Writing one of these where it is inert is never a config error. The file loads,
the daemon runs, and one warning at load says which lines mean nothing here, so
one configuration can be carried between platforms
([ADR 0008](../adr/0008-a-vocabulary-has-one-home.md)). `neru doctor` lists the
inert words your own configuration writes in its `platform_support` row.

This table answers a different question from the
[Capability Matrix](#capability-matrix). The matrix says whether a subsystem
works. This says whether a word a person wrote does anything.

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

## Platform Exclusives

Features available on exactly one platform, and why they do not port. This is a
**closed set**: anything not listed here is a gap rather than an exclusive,
whatever the [Capability Matrix](#capability-matrix) currently reports
([ADR 0013](../adr/0013-parity-is-measured-in-words-not-subsystems.md)).

| Feature                                   | Platform | Why it is exclusive                                          |
| ----------------------------------------- | -------- | ------------------------------------------------------------ |
| System cursor hide + virtual-pointer replacement | macOS | A Wayland client may not hide another client's cursor. X11 could, but the supported Linux stack is Wayland, so shipping it on one backend would not be parity |
| Screen-sharing hide                       | macOS    | NSWindow sharing level is a Quartz concept                    |
| Secure input detection                    | macOS    | Read through a private macOS API. Neither X11 nor Wayland has the concept |

Smooth scroll and the `vision` hint strategy are not exclusives. Both work on
every platform, with the limits in footnotes ³ and ⁶.

Linux and Windows have no exclusive *user-facing* features.

## Known Gaps

Work that is missing, as opposed to deliberately platform-specific.
A gap is anything a person can *write* (an option, a mode flag, an action, a
command) that means less here than it does on macOS, whether or not the
[Capability Matrix](#capability-matrix) reports its subsystem as supported
([ADR 0013](../adr/0013-parity-is-measured-in-words-not-subsystems.md)).

**Linux**

None. Parity is complete on the supported stack.
[What the labels mean](#what-the-labels-mean) says why the label is still Beta.

The `input`-group membership Wayland global hotkeys need is a host setup step
rather than a gap. Neru warns with the remedy when the listener cannot start,
and [Linux setup](../guide/linux.md#install-time-environment-adjustments)
carries it.

**Not Linux gaps**, and deliberately so: secure input detection and system
cursor hide are [Platform Exclusives](#platform-exclusives). GNOME Wayland
needing a shell extension for its focused window is Mutter's decision. The X11
display server makes modifier passthrough impossible. `neru services` on a
non-systemd init is a stated boundary.

**The `CGO_ENABLED=0` Linux build is outside the boundary too**, and it reports
this itself. Cursor, clicks, scroll, hotkeys, keyboard capture, overlay, screen
enumeration, focused app, `action feed` and the `vision` strategy all report
`ERR_NOT_SUPPORTED` there, so the build states what kind of build it is once at
startup ([ADR 0012](../adr/0012-the-first-hour-must-not-lie.md)). The tray,
notifications and alerts keep working.

**Windows**

None. Parity is complete.

**Not a Windows gap**: the three `hints.vision.*_confidence` floors are inert
because `Windows.Media.Ocr` reports no per-word confidence, a boundary of the
engine in the same way X11 modifier passthrough is a boundary of the display
server. [What the labels mean](#what-the-labels-mean) says why the label is
Beta.

**Not a Windows gap either: windows running as administrator.** Windows stops
a normal process from sending input to, or reading keys typed into, a window
at a higher integrity level. While an elevated app has focus, Neru's global
hotkeys still fire and the cursor still moves, but the mode cannot see keys
typed into it and every click, scroll and injected key is dropped. Running
Neru elevated as well removes this limit, at the cost of an administrator token on a
process that sees every keystroke and a UAC prompt on each manual launch. The
secure desktop (UAC prompts, the lock screen) stays out of reach either way.

**macOS**

1. Named keys without a Carbon keycode: `Insert` and `F21` to `F24` validate but
   never fire, because Carbon declares no virtual key code for them. They stay
   in the shared key vocabulary so one config file works on every platform
   ([ADR 0008](../adr/0008-a-vocabulary-has-one-home.md)).

Otherwise none. macOS is the reference implementation.
