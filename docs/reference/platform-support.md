# Platform support

What works on macOS, Linux and Windows, and where a platform behaves
differently. If this page and the code disagree, the code is right.

## Platform status

### What the labels mean

- **Stable**: fully featured and proven in daily use. A gap here is a bug.
- **Beta**: ready for daily use, and every mode works as on a stable platform.
  Something is still missing, or what is there is proven only by CI.
- **Alpha**: core navigation works, but hint coverage is incomplete and per-app
  config does not re-apply on focus change.

Linux and Windows have full parity
([ADR 0013](../adr/0013-parity-is-measured-in-words-not-subsystems.md)) but
stay Beta, because CI on headless Sway and native Windows proves them rather
than daily use. A Beta platform becomes Stable after six consecutive releases
with no bug filed that only it has.

### Per-platform

| Aspect               | macOS                       | Linux                                    | Windows                        |
| -------------------- | --------------------------- | ---------------------------------------- | ------------------------------ |
| **Status**           | **Stable**                  | **Beta**                                 | **Beta**                       |
| **Primary modifier** | `Cmd`                       | `Ctrl`                                   | `Ctrl`                         |
| **Display stack**    | Cocoa / Quartz              | X11, or Wayland (wlroots / KWin / COSMIC / Mutter) | Win32 / DWM           |
| **Accessibility**    | AXUIElement                 | AT-SPI over D-Bus                        | UI Automation                  |
| **Distributed as**   | `Neru.app`, codesigned      | Binary + install script                  | Binary                         |

### Linux backends

Neru picks one backend at startup from `XDG_CURRENT_DESKTOP`,
`WAYLAND_DISPLAY` and `DISPLAY`. `neru doctor` reports it as `display_server`.

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
> **The daemon refuses to start on `wayland-other` and `unknown`.** It also
> refuses `wayland-gnome` when `DISPLAY` is unset, because the GNOME overlay
> is a window on Xwayland.
>
> **GNOME matches the KDE column below except in four places.** The overlay is
> X11 + Cairo on Xwayland, the cursor position comes from an Xwayland window,
> and keyboard capture is the evdev proxy alone. Focused-app identity, the app
> watcher and the window origin come from the Neru GNOME Shell extension,
> which the daemon installs and which works after one re-login. See
> [Linux desktops](../guide/linux-desktops.md#gnome-wayland).

## Capability Matrix

Whether each subsystem works, and what implements it. KDE and wlroots are both
Wayland with `wlr-layer-shell` overlays. Whether each word you write means the
same everywhere is in [Known Gaps](#known-gaps) and
[Platform support per word](#platform-support-per-word).

**Legend:** ✅ supported · ⚠️ works with known limits · 🟡 stub (`ERR_NOT_SUPPORTED`
or no effect) · ❌ no code path · ➖ macOS-only capability, exempt from parity
(see [Platform Exclusives](#platform-exclusives))

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

¹ **Font resolution.** A named font family resolves to that name on every
platform. On Linux and Windows, a family the system cannot find falls back to
DejaVu Sans or Segoe UI. macOS and the non-CGO Linux build let the renderer
substitute at draw time. `sans`, `serif`, `mono` and the empty string resolve
to each platform's own faces. **Text measurement** sizes boxes from the text,
except on the non-CGO Linux build, which has no Cairo and estimates.

² **Service management** needs systemd on every Linux backend. runit, OpenRC
and s6 get `ERR_NOT_SUPPORTED` from every `neru services` subcommand. The unit
is tied to `graphical-session.target`, so logging out and in restarts the
daemon ([Linux setup](../guide/linux.md#systemd-user-service)).

³ **Smooth scroll granularity.** `smooth_scroll` animates everywhere and sends
the same distance. wlroots and libei send fractional deltas, Windows sends
120ths of a notch, and X11 sends whole notches. Linux notch paths use 30 px per
notch, rounded, at least one, so on X11 a `scroll_step` under 45 px is one
unanimated click. The X11 and KDE behaviour is untested on hardware.

⁴ **Native hint-search field.** Only macOS has a native search field with
keyboard focus and the system input method. Elsewhere Neru reads the query from
the key stream, so dead keys and IME composition do not work.

⁵ **Screen capture.** X11, wlroots and Windows capture without a consent
prompt. KWin supports neither protocol, so KDE uses the
`org.freedesktop.portal.ScreenCast` portal over PipeWire. Its consent dialog
appears when a mode that needs capture first starts, and the grant is kept
under `$XDG_STATE_HOME/neru/`. A region that leaves the screen, spans two
Wayland outputs, or lies on a monitor you declined to share fails rather than
coming back clipped. Scaled outputs return physical pixels, as on Retina.

⁶ **Vision on Linux and Windows is text-only.** OCR finds text but not
rectangles, so `hints.vision.detect_rectangles` and the four `rectangle_*`
options are macOS-only. The `contour` strategy works everywhere. Linux uses
tesseract, which finds language data through `TESSDATA_PREFIX` and then the
distribution paths, and a missing `eng.traineddata` gets `ERR_NOT_SUPPORTED`
naming that file. Windows uses `Windows.Media.Ocr`, which needs the OCR
language pack for one of the account's languages. It reports no per-word
confidence, so the three `*_confidence` floors are inert there. Neru never
logs or stores recognized text.

⁷ **Modifiers on injected input.** On X11, Wayland and Windows, an injected
pointer event carries whatever modifiers the keyboard holds, so a chord still
held when you pick a hint would make a ctrl+click. Neru releases those
modifiers, presses the requested ones, and restores both afterwards, across an
animated scroll and until a drag's release. On Wayland the evdev proxy does
this. Without a forwarding proxy (no `/dev/uinput`, or the wl-keyboard
fallback) the click stays modified.

⁸ **Tray and notifications.** The tray icon shows the paused state on every
platform. Linux tray menus have no item tooltips, because
`com.canonical.dbusmenu` defines none. Windows notifications are balloon tips
on the tray icon, so with `systray.enabled = false` they report
`ERR_NOT_SUPPORTED`. Windows alerts are `MessageBoxW` and need no tray.

⁹ **Hyprland modified scroll** goes out on the uinput wheel in whole notches,
because Hyprland drops a virtual-pointer scroll while a modifier is held.

¹⁰ **windows/arm64 overlay.** windows/arm64 builds draw with GDI only, because
the Direct2D binding is amd64-only.

### Notes on the ⚠️ entries

**Focused app on Wayland.** wlroots and KWin identify the focused window by
app_id, and per-app config keys on it. They expose no process ID, so anything
that needs one is matched best-effort from the app_id.

**App watcher.** On Linux and Windows it reports activation and deactivation
only. Launch, terminate and Mission Control events are macOS-only. See
[App identity across platforms](configuration.md#app-identity-across-platforms-bundle_id).

**Global hotkeys on Wayland** use an evdev keyboard proxy
([ADR 0014](../adr/0014-the-wayland-keyboard-is-a-proxy.md)) that needs the
`input` group and a CGO build. With `/dev/uinput` writable, the focused app
never sees a matched chord. Without it, the app gets the chord too. If either
requirement is missing, Neru warns once with the remedy and the fallback,
binding `neru <mode>` in the compositor.

**Native alerts on Linux** are critical notifications with no expiry, sent to
the session's notification daemon over D-Bus, and are not modal. With no
notification daemon, alerts and notifications report `ERR_NOT_SUPPORTED`,
`neru doctor` says what to install, and the two startup alerts go to stderr.

**Smooth cursor animation** is off by default on every platform
(`smooth_cursor.move_mouse_enabled`). Clicks stay instant, and actions that
depend on the cursor position wait for it to finish.

## Input injection

Every click, press, release, move, drag and scroll behaves the same on every
platform. The [Capability Matrix](#capability-matrix) names the API each one
uses, and KDE reaches libei through the
`org.freedesktop.portal.RemoteDesktop` portal. A modified scroll holds the real
key on Linux and Windows, and a backend that cannot send the modifier answers
`ERR_NOT_SUPPORTED`. Neru releases held mouse buttons when it returns to idle.
On Windows, a move while a button is held is sent as a short relative glide,
because apps such as Windows Terminal select text only from that motion. Linux
backends warp the pointer and let the compositor infer the drag.

## Keyboard capture and hotkeys

| Aspect                | macOS                  | Linux X11               | Linux Wayland                            | Windows                 |
| --------------------- | ---------------------- | ----------------------- | ---------------------------------------- | ----------------------- |
| **In-mode capture**   | `CGEventTapCreate`     | `XGrabKeyboard`         | evdev proxy (uinput re-emit), wl-keyboard fallback | `WH_KEYBOARD_LL`        |
| **Global hotkeys**    | Per-key CGEventTap     | `XGrabKey`              | Chord matcher on the evdev proxy         | `RegisterHotKey`        |
| **Modifier passthrough** | ✅                  | ❌ grab is all-or-nothing | ✅ evdev only                          | ✅                      |
| **Sticky modifiers**  | ✅                     | ✅                      | ✅                                       | ✅                      |

**A held global hotkey** is one press and one release on every platform, which
is what `[held_repeat]` repeats between.

**A global chord while a mode is active.** `[hotkeys]` bindings keep working
inside a mode, and the mode's own binding for the same chord wins. Only chords
with Ctrl, Alt or Cmd fall through, because a bare key is a hint label or grid
key. On Linux the mode owns the keyboard, so a chord bound in the compositor
cannot fire while a mode is open.

**One key, one name.** Every backend names a key by the character it types,
or by keysym name when it types none, so `Shift+;` has one name everywhere. On
a non-QWERTY layout, a `[hotkeys]` chord binds the key bearing that character
on the reference layout. A layout is ASCII-capable when its letter row types
ASCII.

- **X11**: the active layout if ASCII-capable, else the first ASCII-capable
  layout in the keymap, else the active layout. Read when a mode starts.
- **Wayland**: the first ASCII-capable layout of the compositor's keymap, else
  the active layout. The compositor shows the active layout only to the
  focused client, so with two Latin layouts such as `us` and Dvorak, bindings
  stay on the first.
- **Windows**: letters and digits use virtual-key codes, fixed on every
  layout. Punctuation follows the X11 rule, with the foreground window's layout
  as active and the installed layouts as fallback. Neru re-registers hotkeys
  within a second of a change.
- `general.kb_layout_to_use` forces the reference: an XKB layout name on Linux,
  which the X11 `[hotkeys]` grab also follows, or a keyboard layout identifier
  on Windows.

With `us,ru` or `ru,us`, every binding, hint label and grid key stays on its
physical key while Russian is active. Shift, Caps Lock, NumLock and AltGr
still pick the level, and modifiers are read from the active layout, where
remaps such as `ctrl:swapcaps` live. `ISO_Left_Tab` folds back to `Tab`, so
the default `Shift+Tab` hint binding fires.

**Modifier passthrough.** In a mode Neru captures the keyboard, so unbound
shortcuts never reach the app. `general.passthrough_unbounded_keys` lets
unbound Ctrl, Alt and Cmd chords through, and
`general.should_exit_after_passthrough` exits the mode afterwards. Neither
X11 nor the Wayland wl-keyboard fallback supports it. On X11 it warns once at
load, and `neru doctor` lists it under `platform_support`.

## Accessibility and hints

| Aspect                  | macOS                                       | Linux                                              | Windows                              |
| ----------------------- | ------------------------------------------- | -------------------------------------------------- | ------------------------------------ |
| **Backend**             | AXUIElement                                 | AT-SPI over D-Bus                                  | UI Automation                        |
| **Traversal**           | Full recursive walk of the accessibility tree | Recursive walk of the active window, depth and node capped | Control-view tree, any depth |
| **Sources collected**   | Frontmost + all windows, popovers, menubar, dock, notification center, Stage Manager, PIP | Active window only          | Foreground window's control view     |
| **Strategies**          | `axtree` (default), `vision` and `contour`, incl. per-app overrides | `axtree`, `vision` (text only) and `contour` | `axtree`, `vision` (text only) and `contour` |
| **Popovers / menus**    | ✅ dedicated detection                      | ⚠️ only if inside the active window                | 🟡                                   |

On Linux, coverage depends on each app exposing AT-SPI. On Windows, an element
an app exposes only in UI Automation's raw view gets no hint. Where the tree is
thin, use the `vision` or `contour` strategy. Fix a missing element with a role
or filter setting ([Clickable roles](configuration.md#clickable-roles)).

**Chromium and Electron apps on Linux** hide their web content from AT-SPI by
default. Launch them with `--force-renderer-accessibility`. Native GTK and Qt
apps and Firefox need no flag.

**Window-origin offset on Wayland.** AT-SPI positions are window-relative on
Wayland, so Neru asks the compositor for the window's screen origin:

| Compositor | Source                                                       | Limits                                                                                                                    |
| ---------- | ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------- |
| KDE / KWin | KWin script reporting focused-window geometry over D-Bus     | Reports on activation and on the focused window moving, resizing or going away. Neru reinstalls the script when KWin restarts. A drag reports its final rectangle. |
| niri       | `niri msg -j focused-window` / `focused-output`              | Floating and fullscreen windows only. **Tiled** windows expose no on-screen position ([niri#2381](https://github.com/niri-wm/niri/issues/2381)), so hints are misaligned there. |
| Sway       | `swaymsg -t get_tree`                                        | none                                                                                                                       |
| Hyprland   | `hyprctl -j activewindow`                                    | none                                                                                                                       |
| GNOME      | GNOME Shell extension reporting the focused window's frame over D-Bus | The daemon installs and enables the extension. The shell loads it at the next login. |
| Anything else (X11, River, Wayfire) | none                                                | X11 needs none, because AT-SPI already reports screen positions there. The rest report no origin, so hints stay window-relative. |

Without an origin, positions stay unoffset. Neru warns when a compositor
command fails or returns invalid output. The same sources scope vision and
contour detection, `neru action move_mouse --window`, and grid, recursive grid
or bisect with `--capture-scope window`. River and Wayfire report
`ERR_NOT_SUPPORTED` for these.

## Overlay rendering

| Aspect                | macOS                                    | Linux X11                              | Linux Wayland                                   | Windows                            |
| --------------------- | ---------------------------------------- | -------------------------------------- | ------------------------------------------------ | ---------------------------------- |
| **Window type**       | NSPanel, borderless non-activating       | override-redirect X11 window           | `wlr_layer_shell_v1` overlay surface             | `WS_POPUP` HWND                    |
| **Rendering**         | CoreAnimation (GPU)                      | Cairo (CPU)                            | Cairo (CPU)                                      | Direct2D on DirectComposition (GPU), GDI fallback (CPU) |
| **Click-through**     | yes                                      | yes                                    | yes                                              | yes                                |
| **Window identity**   | panel title `neru-overlay`               | `WM_CLASS` and name `neru-overlay`     | layer-shell namespace `neru-overlay`             | window class `neru-overlay`        |
| **HiDPI**             | per-display backing scale                | `Xft.dpi`, one global factor           | `wl_output` scale + fractional scale             | per-monitor DPI                    |
| **Multi-monitor**     | per-display, live screen changes         | every monitor, live RandR hotplug      | one surface per output (max 16), live hotplug    | live display hotplug               |

Window identity is what a window manager or compositor rule can match, for
example to exclude the overlay from a screenshot tool or tiling rule. Grid
cell sizes and `recursive_grid.min_size_width` and `min_size_height` are in
apparent units, so a grid looks the same size on a scaled display.

## Mode coverage

Mode logic is shared. Only these rows differ, by rendering or element discovery.

| Mode              | Feature                        | macOS                      | Linux                      | Windows                     |
| ----------------- | ------------------------------ | -------------------------- | -------------------------- | --------------------------- |
| **Hints**         | Element discovery              | ✅ full AX tree            | ⚠️ AT-SPI, toolkit-dependent | ⚠️ UIA, control view only |
| **Hints**         | `vision` strategy + per-app overrides | ✅                  | ⚠️ tesseract; text only, no rectangles | ⚠️ `Windows.Media.Ocr`; text only, no rectangles, no confidence |
| **Hints**         | Menubar / dock elements        | ✅                         | 🟡                         | 🟡                          |
| **Hints**         | Label arrow / tail             | ✅ NSBezierPath            | ✅ Cairo triangle          | ✅ triangle |
| **Scroll**        | Smooth scroll animation        | ✅                         | ✅ (X11: whole notches)    | ✅ (120ths of a notch)     |
| **Monitor select**| Whole mode                     | ✅ native panels           | ✅ Cairo panels            | ✅ one layered window per display |

Everything else is the same on all three, including label placement, the
search badge, label sizing, virtual pointer indicators, transition animations,
the recursive-grid sub-key preview, and what an open subgrid shows.
The cursor-replacement virtual pointer is a [Platform Exclusive](#platform-exclusives).

## Platform support per word

Options, mode flags and actions that do not work on every platform. Writing
one where it is inert loads with one warning
([ADR 0008](../adr/0008-a-vocabulary-has-one-home.md)), and `neru doctor`
lists the inert words in your config under `platform_support`.

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

Features on exactly one platform. Anything else missing is a gap
([ADR 0013](../adr/0013-parity-is-measured-in-words-not-subsystems.md)).

| Feature                                   | Platform | Why it is exclusive                                          |
| ----------------------------------------- | -------- | ------------------------------------------------------------ |
| System cursor hide + virtual-pointer replacement | macOS | A Wayland client may not hide another client's cursor. X11 could, but the supported Linux stack is Wayland, so shipping it on one backend would not be parity |
| Screen-sharing hide                       | macOS    | NSWindow sharing level is a Quartz concept                    |
| Secure input detection                    | macOS    | Read through a private macOS API. Neither X11 nor Wayland has the concept |

Linux and Windows have no exclusives. Smooth scroll and `vision` work
everywhere, with the limits in footnotes ³ and ⁶.

## Known Gaps

A gap is anything you can write that means less on a platform than on macOS
([ADR 0013](../adr/0013-parity-is-measured-in-words-not-subsystems.md)).

**Linux**: none. These are boundaries, not gaps:

- Wayland global hotkeys need the `input` group
  ([Linux setup](../guide/linux.md#install-time-environment-adjustments)).
- GNOME Wayland needs a shell extension for the focused window. X11 cannot do
  modifier passthrough. `neru services` needs systemd.
- The `CGO_ENABLED=0` build reports `ERR_NOT_SUPPORTED` for cursor, clicks,
  scroll, hotkeys, keyboard capture, overlay, screen enumeration, focused app,
  `action feed` and `vision`, and says so once at startup
  ([ADR 0012](../adr/0012-the-first-hour-must-not-lie.md)). The tray,
  notifications and alerts still work.

**Windows**: none. These are boundaries, not gaps:

- The three `hints.vision.*_confidence` floors are inert, because
  `Windows.Media.Ocr` reports no per-word confidence.
- **Windows running as administrator.** While an elevated app has focus,
  hotkeys fire and the cursor moves, but the mode sees no keys and every
  click, scroll and injected key is dropped. Running Neru elevated fixes this,
  at the cost of an admin token on a process that sees every keystroke and a
  UAC prompt per manual launch. The secure desktop is out of reach either way.

**macOS**: one gap. `Insert` and `F21` to `F24` validate but never fire,
because Carbon has no key code for them. They stay in the shared vocabulary so
one config works everywhere
([ADR 0008](../adr/0008-a-vocabulary-has-one-home.md)).
