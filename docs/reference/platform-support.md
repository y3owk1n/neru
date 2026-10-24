# Platform support

What works on macOS, Linux and Windows, and what does not. How each platform
implements it is in the contributor docs.

## Platform status

| Platform | Status     | Default `Primary` | Ships as                          |
| -------- | ---------- | ---------------- | --------------------------------- |
| macOS 14+     | **Stable** | `Cmd`            | `Neru.app`, codesigned            |
| Linux    | **Beta**   | `Ctrl`           | Binary and install script         |
| Windows 10+  | **Beta**   | `Ctrl`           | Binary and install script         |

On Linux, Neru supports X11 and the Wayland desktops listed in
[Linux setup](../guide/linux.md#supported-desktops).

### What the labels mean

- **Stable**: fully featured and proven in daily use. A gap here is a bug.
- **Beta**: ready for daily use, and every mode works as on a stable platform.
  Something is still missing, or what is there is proven only by CI.
- **Alpha**: core navigation works, but hint coverage is incomplete and per-app
  config does not re-apply on focus change.

Windows has no known gaps, and Linux has one, under [Known gaps](#known-gaps).
Both stay Beta because CI proves them rather than daily use. A Beta platform
becomes Stable after six releases in a row with no bug filed against it alone.

## Capability matrix

**Legend:** ✅ works · ⚠️ works with a limit, see the note · ❌ not available ·
➖ macOS only, see [Platform exclusives](#platform-exclusives)

| Capability                     | macOS | Linux X11 | Wayland (wlroots) | Wayland (KDE) | Windows |
| ------------------------------ | :---: | :-------: | :---------------: | :-----------: | :-----: |
| Grid, recursive grid, bisect   | ✅    | ✅        | ✅                | ✅            | ✅      |
| Hints from the accessibility tree | ✅ | ⚠️ ¹     | ⚠️ ¹              | ⚠️ ¹          | ⚠️ ¹    |
| Hints from OCR (`vision`)      | ✅    | ⚠️ ²      | ⚠️ ²              | ⚠️ ²          | ⚠️ ²    |
| Hints from shapes (`contour`)  | ✅    | ✅        | ✅                | ✅            | ✅      |
| Hints from layout (`segment`)  | ✅    | ✅        | ✅                | ✅            | ✅      |
| Hint search                    | ✅    | ✅        | ✅                | ⚠️ ³          | ⚠️ ³    |
| Menu bar, Dock and other system surfaces in hints | ✅ | ➖ | ➖          | ➖            | ➖      |
| Scroll mode                    | ✅    | ✅        | ✅                | ✅            | ✅      |
| Monitor select                 | ✅    | ✅        | ✅                | ✅            | ✅      |
| Clicks, drag, modified clicks  | ✅    | ✅        | ✅ ⁴              | ✅ ⁴          | ✅      |
| Smooth cursor and held-key glide | ✅  | ✅        | ✅                | ✅            | ✅      |
| Smooth scroll                  | ✅    | ⚠️ ⁵      | ✅ ⁵              | ⚠️ ⁵          | ✅      |
| Global hotkeys                 | ✅    | ✅        | ⚠️ ⁶              | ⚠️ ⁶          | ✅      |
| Modifier passthrough           | ✅    | ❌        | ⚠️ ⁶              | ⚠️ ⁶          | ✅      |
| Sticky modifiers, `action feed` | ✅   | ✅        | ✅                | ✅            | ✅      |
| Per-app config                 | ✅    | ✅        | ⚠️ ⁷              | ⚠️ ⁷          | ✅      |
| Screen capture for hints       | ✅    | ✅        | ✅                | ⚠️ ⁸          | ✅      |
| Tray icon                      | ✅    | ✅        | ✅                | ✅            | ✅      |
| Notifications                  | ✅    | ⚠️ ⁹      | ⚠️ ⁹              | ⚠️ ⁹          | ⚠️ ¹¹   |
| Alerts                         | ✅    | ⚠️ ⁹      | ⚠️ ⁹              | ⚠️ ⁹          | ✅      |
| Login service (`neru services`) | ✅   | ⚠️ ¹⁰     | ⚠️ ¹⁰             | ⚠️ ¹⁰         | ✅      |
| Hide the system cursor         | ✅    | ➖        | ➖                | ➖            | ➖      |
| Hide overlays from screen share | ✅   | ➖        | ➖                | ➖            | ➖      |

COSMIC matches the KDE column. GNOME matches it too, except that per-app
config and window positions need the Neru GNOME Shell extension, see
[GNOME](../guide/linux-desktops.md#gnome-wayland).

### Notes

¹ **Hint coverage depends on the app.** Linux reads AT-SPI, and each app
decides what it exposes. Chromium and Electron apps need
`--force-renderer-accessibility`. Windows misses elements an app exposes only
in UI Automation's raw view. Where hints are thin, use `vision`, `contour`,
`segment` or a grid mode. On some Wayland compositors hints in native apps can be offset,
see [wlroots compositors](../guide/linux-desktops.md#wlroots-compositors).
The `identifier` that `neru query hints` reports is empty on Linux for GTK 3
apps, which set no AT-SPI `AccessibleId`. GTK 4 apps report no screen positions,
so Neru places their hints only where it can read the window's position. On
X11, and on Wayland wherever Neru gets no window position, their hints stack
at the top-left corner. `hints.visible_check_enabled` keeps every element in
GTK 4 apps, which answer no hit-test. Elsewhere on Linux it drops only
elements something else in the same window covers. The app answers the hit-test
itself, so it never sees other windows, on X11 or Wayland.

² **Text and shapes come from two engines.** Linux (tesseract) and Windows
(`Windows.Media.Ocr`) read the words, and the `contour` detector finds button
shapes in the same frame. It uses its shipped thresholds, not
`[hints.contour]`, and scores every shape one, so the `*_confidence` options
never drop a shape. Windows also needs an OCR language pack, and ignores the
`*_confidence` options for words too. Neru never logs or stores recognized
text.

³ **Hint search opens a text field that takes the keyboard**, as on macOS, on
X11 and on Wayland compositors with a layer shell. An input method reaches it
through XIM on X11, so `XMODIFIERS` has to name one, as it does for any X
application. KDE has the protocols the field needs, but it has not been tested
there. On Windows, search opens a text field too when Windows lets Neru take
the foreground, and reads raw keys when it does not. GNOME has no layer shell,
so search reads raw keys there. Dead keys and input-method composition do not
work, and shifted symbols such as `!` are dropped, because the raw key does
not say which symbol the layout puts there. `neru doctor` reports the native
search field as a stub on GNOME and on Linux builds without cgo. It does not
report a field that failed to open on one search, such as when Windows refused
the foreground.

⁴ **Modified clicks on Wayland** need a writable `/dev/uinput`, see
[Linux setup](../guide/linux.md#setup-steps).

⁵ **Smooth scroll granularity.** X11 scrolls in whole wheel notches, so a
`scroll_step` under 45 px is one unanimated click. X11 and KDE scrolling is
untested on hardware. On Hyprland a scroll with a modifier held moves in whole notches.

⁶ **Wayland hotkeys and passthrough** need the `input` group, see
[Linux setup](../guide/linux.md#setup-steps). Without it, bind modes in your
compositor.

⁷ **Wayland identifies apps by `app_id`**, which is what `bundle_id` matches,
see [App identity](configuration.md#app-identity-across-platforms-bundle_id).

⁸ **KDE asks once before the first screen capture**, see
[Screen-sharing consent](../guide/linux-desktops.md#screen-sharing-consent).

⁹ **Linux alerts and notifications need a notification daemon**, such as mako
or dunst, running or D-Bus activatable. Alerts arrive as notifications, not
dialogs. Without a daemon, both report that they are not supported, and the two
startup alerts go to stderr.

¹⁰ **The login service needs systemd.** On runit, OpenRC or s6, run
`neru launch` from your session.

¹¹ **Windows notifications are balloons on the tray icon**, so with
`systray.enabled = false` they report that they are not supported. Alerts are
dialogs and need no tray.

**Windows on ARM** draws overlays with GDI, the CPU fallback, instead of
Direct2D.

**Window rules.** Every overlay window is named `neru-overlay`: the panel
title on macOS, `WM_CLASS` and window name on X11, the layer-shell namespace
on Wayland, and the window class on Windows. Match it to keep overlays out of
a tiling layout or a screenshot tool.

## Platform support per word

Options, mode flags and actions that do nothing on some platform. Writing one
there is not an error, so one config works everywhere. Neru warns once at
load, and `neru doctor` lists them under `platform_support`.

<!-- BEGIN GENERATED PLATFORM SUPPORT: edit the platform_support.go declarations, then run `just gensupportref` -->

| Words | macOS | Linux | Windows | Why |
| ---- | --- | --- | --- | --- |
| `general.hide_overlay_in_screen_share`, `hooks.on_screen_share_hide` | ✅ | ❌ | ❌ | hiding the overlay from a screen share is an NSWindow sharing level, a Quartz concept with no X11, Wayland or Win32 counterpart |
| `hints.include_menubar_hints`, `hints.additional_menubar_hints_targets`, `hints.include_dock_hints`, `hints.include_nc_hints`, `hints.include_stage_manager_hints`, `hints.include_pip_hints`, `hints.include_screen_capture_hints` | ✅ | ❌ | ❌ | the menu bar, the Dock, Notification Center, Stage Manager, picture-in-picture and the screen-capture chrome are macOS surfaces with no counterpart |
| `hints.detect_mission_control`, `hints.on_mission_control_activated`, `hints.on_mission_control_deactivated`, `hooks.on_mission_control_activated`, `hooks.on_mission_control_deactivated` | ✅ | ❌ | ❌ | Mission Control is a macOS concept, so the detection never fires and the hooks never run |
| `grid.prewarm_enabled` | ✅ | ❌ | ❌ | only the darwin grid overlay prewarms its layers; the other backends draw on demand |
| `hints.vision.minimum_confidence`, `hints.vision.button_min_confidence`, `hints.vision.generic_clickable_min_confidence` | ✅ | ✅ | ❌ | Windows.Media.Ocr reports no per-word confidence, so every word scores one there and a floor keeps everything; the Vision framework and tesseract score each word |
| `hide_cursor` (action), `show_cursor` (action) | ✅ | ❌ | ❌ | a Wayland client may not hide another client's cursor, and the blessed Linux stack is Wayland; Windows has no equivalent either |

<!-- END GENERATED PLATFORM SUPPORT -->

## Platform exclusives

Features on exactly one platform, because the others have no way to do them.
Anything else missing is a gap.

| Feature                                   | Platform | Why it is exclusive                                          |
| ----------------------------------------- | -------- | ------------------------------------------------------------ |
| System cursor hide + virtual-pointer replacement | macOS | A Wayland client may not hide another client's cursor. X11 could, but the supported Linux stack is Wayland, so shipping it on one backend would not be parity |
| Screen-sharing hide                       | macOS    | NSWindow sharing level is a Quartz concept                    |
| Secure input detection                    | macOS    | Read through a private macOS API. Neither X11 nor Wayland has the concept |

Linux and Windows have no exclusives.

## Known gaps

A gap is anything you can write that means less on a platform than on macOS.
A boundary is a requirement the platform imposes, not something Neru lacks.

**Linux**: one gap.

- **A Wayland monitor's resolution or scale change** needs a
  [daemon restart](../guide/troubleshooting.md#restart-the-daemon) before
  overlays fit it again. Plugging in or removing a monitor is tracked live.

These are boundaries, not gaps:

- Wayland global hotkeys need the `input` group
  ([Linux setup](../guide/linux.md#setup-steps)).
- On wlroots compositors other than Sway, Hyprland and niri, such as River,
  Wayfire and labwc, the compositor does not report where the focused window
  is. Hints in native apps are offset there, and `--capture-scope window`
  covers the whole screen, see
  [wlroots compositors](../guide/linux-desktops.md#wlroots-compositors).
- GNOME Wayland needs a shell extension for the focused window. X11 cannot do
  modifier passthrough. `neru services` needs systemd.
- A source build with `CGO_ENABLED=0` cannot move the cursor, click, scroll,
  capture keys, draw overlays or run `vision`, and says so once at startup.
  The Linux release binary is built with CGO.

**Windows**: none. These are boundaries, not gaps:

- **Windows running as administrator.** While an elevated app has focus,
  hotkeys fire and the cursor moves, but the mode sees no keys and every
  click, scroll and injected key is dropped. Running Neru elevated fixes this,
  at the cost of an admin token on a process that sees every keystroke and a
  UAC prompt per manual launch. The secure desktop is out of reach either way.

**macOS**: one gap. `Insert` and `F21` to `F24` validate but never fire,
because macOS has no key code for them. They stay valid so one config works
everywhere.
