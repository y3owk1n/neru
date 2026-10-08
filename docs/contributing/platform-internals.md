# Platform internals

How each capability is implemented on each platform and backend. This page
owns mechanism. Status, labels and gaps live in
[platform support](../reference/platform-support.md), and where platform code
goes is in the [porting guide](porting.md).

## Linux backend detection

Neru picks one backend at startup from `XDG_CURRENT_DESKTOP`,
`WAYLAND_DISPLAY` and `DISPLAY`. `neru doctor` reports it as `display_server`.

| Backend           | Detected when                                                                                  |
| ----------------- | ---------------------------------------------------------------------------------------------- |
| `x11`             | `DISPLAY` set, no `WAYLAND_DISPLAY`                                                            |
| `wayland-wlroots` | Sway, Hyprland, niri, River, Wayfire, labwc, a `:wlroots` tag, or unset `XDG_CURRENT_DESKTOP` |
| `wayland-kde`     | `XDG_CURRENT_DESKTOP` contains `KDE`                                                           |
| `wayland-cosmic`  | `XDG_CURRENT_DESKTOP` contains `COSMIC`                                                        |
| `wayland-gnome`   | `XDG_CURRENT_DESKTOP` contains `GNOME`                                                         |
| `wayland-other`   | Any other Wayland compositor                                                                   |
| `unknown`         | Neither `WAYLAND_DISPLAY` nor `DISPLAY`                                                        |

Which backends are supported is in
[Linux setup](../guide/linux.md#supported-desktops). On GNOME the overlay is
X11 and Cairo on Xwayland, the cursor position comes from an Xwayland window,
keyboard capture is the evdev proxy alone, and the focused app and window
origin come from the Neru GNOME Shell extension. Otherwise GNOME matches the
KDE column below.

## Implementation matrix

The API or protocol behind each capability. KDE and wlroots are both Wayland
with `wlr-layer-shell` overlays. Whether each capability works is in the user
[capability matrix](../reference/platform-support.md#capability-matrix), and
the footnotes below give the mechanism behind its limits.

| Capability                    | macOS                    | Linux X11              | Linux Wayland (wlroots)      | Linux Wayland (KDE)     | Windows                      |
| ----------------------------- | ------------------------ | ---------------------- | ---------------------------- | ----------------------- | ---------------------------- |
| **Screen bounds / enumeration** | Cocoa               | XRandR              | xdg-output                | xdg-output           | `EnumDisplayMonitors`     |
| **Display hotplug events**    | screen-params notif.  | RandR event fd      | `wl_output` events        | `wl_output` events   | `WM_DISPLAYCHANGE`        |
| **Focused app identity**      | NSWorkspace + AX      | `_NET_ACTIVE_WINDOW` / `WM_CLASS` | app_id only (see below) | app_id only     | `GetForegroundWindow`     |
| **App watcher (focus change)**| NSWorkspace observer  | event-driven        | event-driven              | event-driven         | `SetWinEventHook`         |
| **Keymap learns the focused app** | published by the watcher | published by the watcher | published by the watcher | published by the watcher | published by the watcher |
| **Cursor position**           | `CGEventGetLocation`  | `XQueryPointer`     | `hyprctl` on Hyprland, else sync-surface cache | sync-surface cache | `GetCursorPos` |
| **Cursor move**               | `CGEventPost` | XTest (`XTestFakeMotionEvent`) | `zwlr_virtual_pointer` | libei                | `SetCursorPos`, glided while a button is held |
| **Mouse buttons / drag**      | `CGEventPost`         | XTest ⁷             | `zwlr_virtual_pointer` ⁷  | libei ⁷              | `SendInput` ⁷             |
| **Scroll injection**          | both axes             | both axes ⁷         | both axes (uinput, virtual-pointer fallback) | both axes (uinput, libei fallback) | both axes ⁷               |
| **Modified scroll (`--modifier`)** | `CGEventSetFlags` on every chunk | XTest key hold ⁷ | virtual keyboard + virtual pointer (uinput on Hyprland ⁹) | libei | `SendInput` key hold ⁷ |
| **Smooth cursor animation**   | incl. relative, opt-in | incl. relative, opt-in | incl. relative, opt-in | incl. relative, opt-in | incl. relative, opt-in |
| **Held-key glide (`held_repeat`)** | `CGEventPost` per tick | XTest per tick | virtual-pointer relative motion per tick | libei per tick | `SetCursorPos` per tick |
| **Smooth scroll animation**   | continuous axis       | whole notches only ³ | continuous axis ³ (whole notches when modified on Hyprland ⁹) | libei scroll delta, unverified ³ | 120ths of a notch ³ |
| **Element discovery (hints)** | AXUIElement           | AT-SPI walk         | AT-SPI walk               | AT-SPI walk          | UIA, control view only    |
| **Overlay**                   | NSPanel + CoreAnimation | X11 + Cairo       | layer-shell + Cairo       | layer-shell + Cairo  | DirectComposition + Direct2D (GDI fallback; windows/arm64 is GDI only ¹⁰) |
| **Global hotkeys**            | per-key CGEventTap    | `XGrabKey`          | evdev proxy (`input` group) | evdev proxy (`input` group) | `RegisterHotKey`          |
| **Keyboard capture**          | CGEventTap            | `XGrabKeyboard`     | evdev proxy (uinput; wl-keyboard fallback) | evdev proxy (uinput) | `WH_KEYBOARD_LL`          |
| **Modifier passthrough**      | CGEventTap forwards or blocks per event | none, the grab is all-or-nothing | evdev backend only        | evdev backend only   | `WH_KEYBOARD_LL` forwards or blocks per event |
| **Dark mode detection**       | Cocoa appearance      | xdg appearance portal | xdg appearance portal   | kdeglobals + portal  | registry                  |
| **Font resolution**           | NSFont                | fontconfig          | fontconfig                | fontconfig           | GDI `EnumFontFamiliesExW` ¹ |
| **Text measurement**          | CoreText              | Cairo text extents ¹ | Cairo text extents ¹      | Cairo text extents ¹ | GDI `GetTextExtentPoint32W` ¹ |
| **System tray**               | NSStatusItem ⁸        | D-Bus StatusNotifierItem ⁸ | StatusNotifierItem ⁸      | StatusNotifierItem ⁸ | Win32 notification area ⁸ |
| **Native alerts**             | NSAlert               | D-Bus, not modal    | D-Bus, not modal          | D-Bus, not modal     | `MessageBoxW`             |
| **Native notifications**      | UNNotification        | `org.freedesktop.Notifications` | `org.freedesktop.Notifications` | `org.freedesktop.Notifications` | Tray balloon tips ⁸ |
| **Secure input detection**    | private macOS API     | always false        | always false              | always false         | always false              |
| **System cursor hide**        | `CGDisplayHideCursor` | none                | none                      | none                 | none                      |
| **`monitor_select` mode**     | native panels         | Cairo panels        | Cairo panels              | Cairo panels         | layered panels            |
| **Native hint-search field**  | NSTextField overlay   | key-stream input ⁴  | key-stream input ⁴        | key-stream input ⁴   | key-stream input ⁴        |
| **Screen capture**            | ScreenCaptureKit      | `XGetImage`         | `wlr-screencopy`          | portal ScreenCast, consent ⁵ | `BitBlt` ⁵        |
| **Vision / OCR detection**    | Vision framework      | tesseract, text only ⁶ | tesseract, text only ⁶ | tesseract, text only ⁶ | `Windows.Media.Ocr`, text only ⁶ |
| **Key feed (`action feed`)** | `CGEventPost`         | uinput               | uinput / virtual-keyboard | uinput               | `SendInput`               |
| **Service management (`neru services`)** | launchd user agent | systemd user unit only ² | systemd user unit only ² | systemd user unit only ² | Task Scheduler logon task |

Each footnote gives the mechanism. The effect a user sees is in the
[user notes](../reference/platform-support.md#notes) and the [font rules](../reference/configuration.md#fonts).

¹ **Font resolution.** Linux and Windows resolve a family at load and fall back
when it is missing. macOS and the non-CGO Linux build let the renderer
substitute at draw time. **Text measurement** sizes boxes from the text,
except on the non-CGO Linux build, which has no Cairo and estimates.

² **Service management** writes a systemd user unit tied to
`graphical-session.target`, so logging out and in restarts the daemon. Other
init systems get `ERR_NOT_SUPPORTED`
([Linux setup](../guide/linux.md#systemd-user-service)).

³ **Smooth scroll granularity.** Every backend sends the same total distance.
wlroots and libei send fractional deltas, Windows sends 120ths of a notch, and
X11 sends whole notches. Linux notch paths use 30 px per notch, rounded, at
least one.

⁴ **Native hint-search field.** macOS draws an `NSTextField` with keyboard
focus and the system input method. Elsewhere the query is read from the key
stream.

⁵ **Screen capture.** X11, wlroots and Windows capture without a prompt. KWin
supports neither protocol, so KDE uses the `org.freedesktop.portal.ScreenCast`
portal over PipeWire, with the grant kept under `$XDG_STATE_HOME/neru/`. A
region that leaves the screen, spans two Wayland outputs, or lies on an
unshared monitor fails rather than coming back clipped. Scaled outputs return
physical pixels, as on Retina.

⁶ **Vision.** Linux uses tesseract, which finds language data through
`TESSDATA_PREFIX` and then the distribution paths, and a missing
`eng.traineddata` gets `ERR_NOT_SUPPORTED` naming that file. Windows uses
`Windows.Media.Ocr`, which reports no per-word confidence. Neither reports
rectangles. Recognized text is never logged or stored.

⁷ **Modifiers on injected input.** On X11, Wayland and Windows, an injected
pointer event carries whatever modifiers the keyboard holds, so a chord still
held when you pick a hint would make a ctrl+click. Neru releases those
modifiers, presses the requested ones, and restores both afterwards, across an
animated scroll and until a drag's release. On Wayland the evdev proxy does
this. Without a forwarding proxy (no `/dev/uinput`, or the wl-keyboard
fallback) the click stays modified.

⁸ **Tray and notifications.** Linux tray menus have no item tooltips, because
`com.canonical.dbusmenu` defines none. Windows notifications are balloon tips
on the tray icon, so with no tray they report `ERR_NOT_SUPPORTED`. Windows
alerts are `MessageBoxW` and need no tray.

⁹ **Hyprland modified scroll** goes out on the uinput wheel, because Hyprland
drops a virtual-pointer scroll while a modifier is held.

¹⁰ **windows/arm64 overlay** is GDI only, because the Direct2D binding is
amd64-only.

### Notes on the limited entries

**Focused app on Wayland.** wlroots and KWin identify the focused window by
app_id, and per-app config keys on it. They expose no process ID, so anything
that needs one is matched best-effort from the app_id.

**App watcher.** On Linux and Windows it reports activation and deactivation
only. Launch, terminate and Mission Control events are macOS-only. See
[App identity across platforms](../reference/configuration.md#app-identity-across-platforms-bundle_id).
On macOS, Neru detects Mission Control from the Dock's accessibility tree. It
checks once a second and on every Space change while
`hints.detect_mission_control` is on. Mission Control is up while the Dock has
a child group with the identifier `mc`, which App Expose, Show Desktop and the
Apps launcher never add. The window list cannot tell, because the Dock keeps a
full-display window on screen whenever the Dock itself is visible.

**Global hotkeys on Wayland** use an evdev keyboard proxy
([ADR 0014](../adr/0014-the-wayland-keyboard-is-a-proxy.md)) that needs the
`input` group and a CGO build. With `/dev/uinput` writable, the focused app
never sees a matched chord. Without it, the app gets the chord too. If either
requirement is missing, Neru warns once with the remedy and the fallback,
binding `neru <mode>` in the compositor.

**Native alerts on Linux** are critical notifications with no expiry, sent to
the session's notification daemon over D-Bus. With no notification daemon they
report `ERR_NOT_SUPPORTED`, and `neru doctor` says what to install.

**Smooth cursor animation** is off by default on every platform
(`smooth_cursor.move_mouse_enabled`). Clicks stay instant, and actions that
depend on the cursor position wait for it to finish.

## Input injection

Every click, press, release, move, drag and scroll behaves the same on every
platform. The [implementation matrix](#implementation-matrix) names the API each one
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
| **Modifier passthrough** | event tap forwards or blocks | none, grab is all-or-nothing | evdev proxy only              | `WH_KEYBOARD_LL` forwards or blocks |

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
| **Popovers / menus**    | dedicated detection                      | only if inside the active window                | none                              |

On Linux, coverage depends on each app exposing AT-SPI. On Windows, an element
an app exposes only in UI Automation's raw view gets no hint. Where the tree is
thin, use the `vision` or `contour` strategy. Fix a missing element with a role
or filter setting ([Clickable roles](../reference/configuration.md#clickable-roles)).

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
| **Hints**         | Element discovery              | full AX tree            | AT-SPI, toolkit-dependent | UIA, control view only |
| **Hints**         | `vision` strategy + per-app overrides | Vision framework, text and rectangles | tesseract; text only, no rectangles | `Windows.Media.Ocr`; text only, no rectangles, no confidence |
| **Hints**         | Label arrow / tail             | NSBezierPath            | Cairo triangle          | triangle |
| **Scroll**        | Smooth scroll animation        | continuous              | continuous (X11: whole notches) | 120ths of a notch |
| **Monitor select**| Whole mode                     | native panels           | Cairo panels            | one layered window per display |

Everything else is the same on all three, including label placement, the
search badge, label sizing, virtual pointer indicators, transition animations,
the recursive-grid sub-key preview, and what an open subgrid shows.
The cursor-replacement virtual pointer is a [platform exclusive](../reference/platform-support.md#platform-exclusives).


## Wayland desktops

How each Wayland desktop reaches the capabilities Neru needs, as measured on
the versions named. The user-side setup is in
[Linux desktops](../guide/linux-desktops.md).

### KDE Plasma (KWin 6.6.4)

Pointer and keyboard input go through libei and the RemoteDesktop portal, and
screen capture through the ScreenCast portal over PipeWire. Neru installs a
KWin script that reports focused-window geometry, and reinstalls it when KWin
restarts. The PipeWire stream is open only for the length of a capture.

| Protocol                           | Purpose                   | KWin 6.6.4 |
| ---------------------------------- | ------------------------- | ---------- |
| `zwlr_layer_shell_v1`              | Overlay surfaces          | yes (v5)   |
| `zxdg_output_manager_v1`           | Screen geometry           | yes (v3)   |
| `zwlr_foreign_toplevel_manager_v1` | Focused-app app_id        | yes (v3)   |
| `zwlr_virtual_pointer_v1`          | Pointer move / click      | **no**     |
| `zwp_virtual_keyboard_manager_v1`  | Sticky-modifier injection | **no**     |
| `org_kde_kwin_fake_input`          | KWin-native emulation     | **no**     |
| `zwlr_screencopy_manager_v1`       | Screen capture            | **no**     |

To verify injected input, run
`qdbus org.kde.KWin /KWin org.kde.KWin.showDebugConsole`. Its Input Events tab
shows Neru's events with an "Unknown" device, which is libei.

### COSMIC (cosmic-comp 1.6.0)

Overlays, screen geometry and sticky modifiers use the shared Wayland client.
The focused app and window geometry come from `ext_foreign_toplevel_list_v1`
and `zcosmic_toplevel_info_v1`. Pointer input and screen capture use the
portals, as on KDE.

| Protocol                           | Purpose                   | cosmic-comp |
| ---------------------------------- | ------------------------- | ----------- |
| `zwlr_layer_shell_v1`              | Overlay surfaces          | yes (v5)    |
| `zxdg_output_manager_v1`           | Screen geometry           | yes (v3)    |
| `zwp_virtual_keyboard_manager_v1`  | Sticky-modifier injection | yes         |
| `ext_foreign_toplevel_list_v1`     | Focused-app app_id, title | yes (v1)    |
| `zcosmic_toplevel_info_v1`         | Activated state, geometry | yes (v3)    |
| `zwlr_virtual_pointer_v1`          | Pointer move / click      | **no**      |
| `zwlr_foreign_toplevel_manager_v1` | (wlr toplevel manager)    | **no**      |
| `zwlr_screencopy_manager_v1`       | Screen capture            | **no**      |

### wlroots compositors

Neru caches the pointer position and sets a known one at startup with a brief
pointer wiggle. On Hyprland it reads `hyprctl` instead. Modified scroll on
Hyprland goes out on the uinput wheel.

### GNOME (Mutter 50.4)

Pointer input and screen capture use the portals. GNOME grants the keyboard
with the pointer. Keyboard capture and hotkeys use the evdev proxy only, with
no compositor fallback. The overlay is an X11 window on Xwayland. The Neru
GNOME Shell extension `neru@y3owk1n.github.io` reports the focused window's
app id, title and frame over D-Bus. It reads only
`global.display.focus_window`.

| Protocol                           | Purpose                   | Mutter 50.4 |
| ---------------------------------- | ------------------------- | ----------- |
| `zxdg_output_manager_v1`           | Screen geometry           | yes (v3)    |
| `zwlr_layer_shell_v1`              | Overlay surfaces          | **no**      |
| `zwlr_virtual_pointer_v1`          | Pointer move / click      | **no**      |
| `zwp_virtual_keyboard_manager_v1`  | Sticky-modifier injection | **no**      |
| `zwlr_foreign_toplevel_manager_v1` | Focused-app app_id        | **no**      |
| `ext_foreign_toplevel_list_v1`     | Focused-app app_id        | **no**      |
| `zwlr_screencopy_manager_v1`       | Screen capture            | **no**      |
