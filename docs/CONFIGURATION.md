# Configuration Reference

Complete reference for every Neru configuration option.

Neru is configured in TOML. No config file is required: Neru runs on built-in
defaults. Define only the options you want to change — every option left out
keeps its default. "The daemon" below means the process started by
`neru launch`.

**Related:** [CLI Reference](CLI.md) · [Tips & Tricks](TIPS_TRICKS.md) ·
[Troubleshooting](TROUBLESHOOTING.md)

---

## Table of Contents

**Getting started**

- [Quick Start](#quick-start)
- [Config File Location](#config-file-location)
- [Managing Your Config](#managing-your-config)
- [Runtime Config Changes](#runtime-config-changes)

**Shared concepts**

- [Color Format](#color-format)
- [Hotkeys](#hotkeys)

**Sections**

| Section                                       | Controls                                     |
| --------------------------------------------- | -------------------------------------------- |
| [`[macros]`](#macros)                         | Named action sequences reused across bindings |
| [`[general]`](#general)                       | Global behaviour, passthrough, `exec` shell   |
| [`[theme]`](#theme)                           | Base palette all components derive from       |
| [`[hints]`](#hints)                           | Hints mode and element discovery              |
| [`[grid]`](#grid)                             | Grid mode                                     |
| [`[recursive_grid]`](#recursive_grid)         | Recursive grid mode                           |
| [`[scroll]`](#scroll)                         | Scroll mode and step sizes                    |
| [`[monitor_select]`](#monitor_select)         | Display picker                                |
| [`[virtual_pointer]`](#virtual_pointer)       | On-screen pointer indicator                   |
| [`[mouse_action_indicator]`](#mouse_action_indicator) | Click feedback animation              |
| [`[mode_indicator]`](#mode_indicator)         | Active-mode badge                             |
| [`[sticky_modifiers]`](#sticky_modifiers)     | Sticky modifier badge                         |
| [`[smooth_cursor]`](#smooth_cursor)           | Animated cursor movement                      |
| [`[smooth_scroll]`](#smooth_scroll)           | Animated scrolling                            |
| [`[held_repeat]`](#held_repeat)               | Key-repeat while held                         |
| [`[systray]`](#systray)                       | System tray icon                              |
| [`[logging]`](#logging)                       | Log level, file, rotation                     |

---

## How to read this reference

Each section below documents one TOML table. Options are listed in a table with
this shape:

| Column        | Meaning                                                   |
| ------------- | --------------------------------------------------------- |
| `Option`      | The key, written as it appears in the TOML table           |
| `Type`        | `bool`, `int`, `float`, `string`, `array`, `color`, `map`  |
| `Default`     | The built-in value used when the key is absent             |
| `Description` | What the option does                                       |

Nested tables are written with their full path, so `[hints.ui]` means a `[ui]`
table inside `[hints]`. Options marked as overridable per app can also appear
inside an `app_configs` entry for that section.

**Platform support.** Some sections below carry a `Platforms:` line, but the
complete answer is the one the daemon itself reads —
[Platform Support Per Word](CROSS_PLATFORM.md#platform-support-per-word), which
is generated from that declaration rather than kept by hand. An option missing
from it behaves the same on macOS, Linux, and Windows. Unsupported options are
still accepted and validated rather than rejected, so one config file can be
shared across machines, and the daemon warns once at load about the lines that
mean nothing here. What each one does instead is in
[Platform-specific options](#platform-specific-options).

On Linux, "supported" means an X11 session or a Wayland session on wlroots or
KWin. GNOME Wayland is not supported at all; the daemon exits at startup.

---

## Platform-specific options

Every option not listed below behaves the same on all three platforms.

The list itself lives in
[Platform Support Per Word](CROSS_PLATFORM.md#platform-support-per-word), which
is generated from the declaration the daemon reads: writing one of those options
on a platform that ignores it produces one warning at load and a row in
`neru doctor`, and never a refusal — the file loads, so one configuration can be
carried between machines
([ADR 0013](adr/0013-parity-is-measured-in-words-not-subsystems.md)). It was kept
by hand here until that declaration existed, which is exactly how a
cross-platform `[smooth_scroll]` block came to be documented while only macOS
read it.

Three cases the platform column cannot state, because support is partial rather
than absent:

- `general.passthrough_unbounded_keys` and `general.should_exit_after_passthrough`
  reach the focused application on macOS and on the Wayland evdev tap, but not
  on X11 — the display server cannot pass a grabbed chord through at all — and
  not on Windows.
- `[virtual_pointer]` styles two things. The standalone overlay is macOS-only
  (it pairs with system cursor hiding, a
  [platform exclusive](CROSS_PLATFORM.md#platform-exclusives)), while these same
  options style the recursive-grid in-frame pointer on every platform.
- `hints.strategy = "vision"` works everywhere. On Linux and Windows it is
  **text-only** — tesseract OCR on Linux and `Windows.Media.Ocr` on Windows
  answer the text half of the strategy, so `detect_rectangles` and the four
  `rectangle_*` options below are macOS-only. Linux needs the tesseract English
  language data installed ([Linux setup](LINUX_SETUP.md#build-dependencies));
  Windows needs an OCR language pack for one of the account's languages, which
  a language's *Basic typing* feature installs. `Windows.Media.Ocr` reports no
  per-word confidence, so the three `*_confidence` floors are inert there.

Accessibility coverage for hints also differs in kind rather than by option; see
[Accessibility and hints](CROSS_PLATFORM.md#accessibility-and-hints).

---

# Getting started

## Quick Start

```toml
[hints.hotkeys]
"Shift+L" = ["action left_click", "idle"]

[scroll]
scroll_step = 50
```

Generate a fully-commented starter file:

```bash
neru config init                          # Creates ~/.config/neru/config.toml
neru config init --force                  # Overwrite existing
neru config init -c /path/to/config.toml  # Custom path
```

---

## Config File Location

> **Recommended:** `~/.config/neru/config.toml`

Loaded in priority order (highest first):

1. `$XDG_CONFIG_HOME/neru/config.toml`
2. `%APPDATA%\neru\config.toml` (Windows only)
3. `~/.config/neru/config.toml`
4. `~/.neru.toml` (legacy)
5. `neru.toml` (current directory)
6. `config.toml` (current directory)

Override at launch: `neru launch -c /path/to/config.toml`

On Windows, `neru config init` writes to `%APPDATA%\neru\config.toml` — the
platform convention — but `~/.config/neru/config.toml` is still read, so a config
kept in a cross-platform dotfiles repo works as-is. Set `XDG_CONFIG_HOME` if you
want `neru config init` and `neru config set` to write there too.

### Config Layering

Neru loads configuration in layers. Each layer overrides the previous one:

```
Defaults → config.toml → config.override.toml → Runtime (in-memory)
```

- **Defaults**: Built-in sensible defaults for your platform.
- **config.toml**: Your hand-crafted configuration file.
- **Override file**: Runtime changes from `neru config set` (persistent). Named after your config file (e.g. `config.override.toml` for `config.toml`).
- **Runtime**: In-memory changes that are lost on restart.

This means `neru config set` changes survive restarts without modifying your `config.toml`. To revert, edit or remove the override file.

---

## Managing Your Config

```bash
neru config validate    # Check syntax (no daemon needed)
neru config reload      # Apply changes to running daemon
neru config dump        # Print loaded config as JSON (daemon required)
neru config init        # Create default config file
neru config set <key> <value>   # Change a single value at runtime (see below)
neru config reset <key>         # Remove a single override (reverts to base config)
```

See [CLI.md](CLI.md#configuration-commands) for full flag documentation.

---

## Runtime Config Changes

Neru supports changing individual configuration values at runtime without restarting the daemon or re-reading the config file from disk.

```bash
neru config set hints.hint_characters "qwerty"
neru config set scroll.scroll_step 25
neru config set general.passthrough_unbounded_keys true
```

### How it works

1. The CLI validates the path and value locally before sending to the daemon.
2. The daemon deep-copies the current in-memory config, applies the change, and validates the result.
3. Services, overlays, and hotkeys are reconfigured automatically — the same internal path used by `neru config reload`.
4. The change is automatically persisted to `config.override.toml` alongside your config file. This file is loaded on every start, so changes survive restarts.

### Batch changes with `--no-reload`

Use `--no-reload` when setting or resetting multiple interdependent fields (e.g. `recursive_grid.grid_cols` + `recursive_grid.keys`). Each change persists to the override file without disrupting active hotkeys or exiting the current mode. Run `neru config reload` once after all changes to apply them.

```bash
neru config set --no-reload recursive_grid.grid_cols 3
neru config set --no-reload recursive_grid.grid_rows 3
neru config set --no-reload recursive_grid.keys "gcrhtnmwv"
neru config reload
```

### Resetting overrides with `config reset`

To revert a single field to its base config value, use `neru config reset <key>`. Like `config set`, it supports `--no-reload` for batch operations.

```bash
neru config reset recursive_grid.grid_cols
neru config reset --no-reload recursive_grid.grid_rows
neru config reload
```

### Config override file

Runtime changes via `neru config set` and `neru config reset` are written to an override file alongside your main config file. The override filename is derived from your config file's name: `config.toml` produces `config.override.toml`, `my-neru.toml` produces `my-neru.override.toml`, etc.

The file uses the same TOML format and follows the same layering:

```
Defaults → config.toml → config.override.toml → Runtime (in-memory)
```

To revert all overrides at once, delete the override file and run `neru config reload`.

### Supported field types

| Type    | Example                                   |
| ------- | ----------------------------------------- |
| string  | `neru config set hints.hint_characters qwerty` |
| integer | `neru config set hints.ui.font_size 14`        |
| boolean | `neru config set scroll.invert_scroll true`    |
| float   | `neru config set hints.vision.minimum_confidence 0.3` |
| color   | `neru config set hints.ui.background_color "#FF0000AA"` |
| array   | `neru config set hints.clickable_roles "button,link"` |

> **Tip:** Use `neru config dump | jq` to explore the full config structure and find the dotted path for any setting.

### Limitations

- **Hotkeys**: Cannot be set via `neru config set` (edit `config.toml` directly instead). However, `config set` and `config reset` can be used *as hotkey actions* to change other config fields at runtime.
- **Struct fields**: Sections like `[theme]` must be set via their leaf sub-paths, not as an object.
- **`app_configs`**: Per-app overrides can't be set via `config set` (edit `config.toml` directly).
- **Override file hotkeys**: If you manually edit the override file to add a `[hotkeys]` section, those bindings won't be loaded. The override file is intended for typed field overrides from `config set` — hotkey changes belong in `config.toml`.
- **Array replacement**: Array fields are replaced wholesale, not appended.
- **Derived values**: Settings computed from other settings — the theme colors filled in from `[theme]`, and `grid.row_labels` / `grid.col_labels` / `grid.sublayer_keys` inferred from `grid.characters` — are recomputed by `config set`, including when you set the setting they are computed *from*. `neru config set grid.characters "qwerty"` relabels the grid immediately, and `neru config set theme.light.surface "#1E1E2E"` recolors immediately. Setting a derived value directly still wins: labels and keys you wrote are kept, and only the ones you left empty are inferred. Note `sublayer_keys` ships with a value, so it is only inferred once you blank it — `neru config set grid.sublayer_keys ""` makes the subgrid follow `grid.characters` from then on.
- **`config reload`**: Re-reading from disk re-applies the override file, but any in-memory-only changes (e.g. before this feature existed) are lost.

---

# Shared concepts

## Color Format

Colors use hex notation with optional alpha transparency.

| Format      | Example     | Alpha | Notes              |
| ----------- | ----------- | ----- | ------------------ |
| `#AARRGGBB` | `#FF000000` | Yes   | Recommended format |
| `#RRGGBB`   | `#FF0000`   | No    | Fully opaque       |
| `#RGB`      | `#F00`      | No    | Shorthand          |

### Alpha Reference

| Opacity | Hex  | Common Use                  |
| ------- | ---- | --------------------------- |
| 100%    | `FF` | Solid colors, high contrast |
| 95%     | `F2` | Hint labels (default)       |
| 70%     | `B3` | Grid cell backgrounds       |
| 60%     | `99` | Grid borders                |
| 30%     | `4D` | Subtle highlights           |
| 0%      | `00` | Invisible                   |

Calculate: `round(opacity_fraction × 255)` → hex.

### Light / Dark Mode

Colors can be a single string or a dictionary with `light` / `dark` keys:

```toml
# Same for both themes
background_color = "#FF0000AA"

# Per-theme
background_color = { light = "#FF0000AA", dark = "#00FF00AA" }
```

Omitted colors inherit Neru's theme-derived defaults and update in real time when you switch system themes.

---

## Hotkeys

### Global Hotkeys

```toml
[hotkeys]
"Primary+Shift+Space" = "hints"
```

**Syntax:** `"Mod1+Mod2+Key" = "action"`

**Defaults by platform.** macOS and Windows ship four launcher bindings:
`Primary+Shift+Space` (hints), `Primary+Shift+G` (grid), `Primary+Shift+C`
(recursive grid), and `Primary+Shift+S` (scroll). `monitor_select` has no
default binding on any platform.

**Linux ships no default global hotkeys at all.** They are cleared during
startup because the defaults collide with common terminal and desktop shortcuts
such as `Ctrl+Shift+C` (copy) and `Ctrl+Shift+V` (paste). Define the bindings
you want in `[hotkeys]`, or bind `neru <mode>` in your compositor — see
[Global hotkeys on Wayland](LINUX_DESKTOPS.md#global-hotkeys-on-wayland) for
which of the two applies to your session.

| Modifier  | Aliases                                 |
| --------- | --------------------------------------- |
| `Cmd`     | `Command`, `Super`, `Meta`              |
| `Ctrl`    | `Control`                               |
| `Alt`     | `Option`                                |
| `Shift`   |                                         |
| `Primary` | `Cmd` on macOS, `Ctrl` on Linux/Windows |

**Shift and a symbol: write the character Shift produces.** On Linux a key is
named by what the active layout makes it mean, so `Shift` plus the `;` key is
`"Shift+:"` and not `"Shift+;"` — the same for `"Shift+\""` over `"Shift+'"`, and
so on for every symbol with a shifted twin. Letters are unaffected: `"Shift+L"` is
right either way.

**Available keys** (the `Key` part after modifiers):

| Category   | Keys                                                                                                    |
| ---------- | ------------------------------------------------------------------------------------------------------- |
| Letters    | `a`–`z`, `A`–`Z`                                                                                        |
| Numbers    | `0`–`9`                                                                                                 |
| Symbols    | `` ` ``, `-`, `=`, `[`, `]`, `\`, `;`, `'`, `,`, `.`, `/`                                               |
| Named      | `Space`, `Return`, `Enter`, `Escape`, `Tab`, `Delete`, `Backspace`                                      |
| Navigation | `Up`, `Down`, `Left`, `Right`, `Home`, `End`, `PageUp`, `PageDown`, `Insert` (Linux and Windows only)   |
| Function   | `F1`–`F24` (`F21`–`F24` on Linux and Windows only)                                                      |

`Delete` and `Backspace` both name the backspace key, the one that erases to the
left, on every platform. The forward-delete key has no hotkey name.

See [CLI.md](CLI.md#neru-action-feed) for a full key reference with key codes and platform behavior.

Multi-key sequences (e.g. `gg`, `ab`) are supported for per-mode hotkeys with a 500ms timeout.

**Action values** can be a single string or an array:

```toml
[hotkeys]
"Primary+Shift+D" = ["hints", "exec echo 'hints activated'"]
"PageUp"          = ["action go_top", "action page_down"]
"Cmd+8"           = [
    "config set recursive_grid.grid_cols 3 --no-reload",
    "config set recursive_grid.grid_rows 3 --no-reload",
    "config set recursive_grid.keys gcrhtnmwv",
]
"Cmd+9"           = [
    "config reset recursive_grid.grid_cols --no-reload",
    "config reset recursive_grid.grid_rows --no-reload",
    "config reset recursive_grid.keys",
]
```

**Shell commands** use the `exec` prefix: `"Primary+T" = "exec open -a Terminal"`

**Mode commands take the same flags they take on the command line**, and are
read the same way: `"Primary+Shift+Space" = "hints --action left_click --repeat"`
behaves exactly as `neru hints --action left_click --repeat` does. A flag the
named mode does not accept, a mistyped flag, and a flag whose partner is
missing — `--on-exit` without `--action`, for instance — are refused when the
key is pressed rather than dropped in silence, with the message the CLI gives
for the same mistake.

**The flags are read when the config loads**, so a mistake is found by
`neru config validate` rather than by pressing the key. What happens next
depends on what the mistake costs:

| In a binding                                                              | Result                                   |
| ------------------------------------------------------------------------- | ---------------------------------------- |
| A flag no mode has (`hints --serach`), or a value no flag takes (`--strategy=nonsense`) | The config fails to load and Neru runs on defaults |
| A flag the named mode does not accept (`grid --search`)                   | Loads, and `neru config validate` warns  |
| A flag whose partner is missing (`hints --repeat` with no `--action`)     | Loads, and `neru config validate` warns  |

The first row is a binding that could not have activated anything either way,
and it fails the load exactly as an unknown *command* in a binding already
does. The other two describe a binding that works minus one flag, and losing
your whole configuration over one of those would be worse than the flag doing
nothing — so they are reported and left alone.

The check reaches every table a binding can be written in, and a macro body.
A step nested inside another one — the steps of a `run`, or of an `--on-exit` —
is checked for the command it names, and its flags are read when it runs. A
macro body step carrying a `$1` placeholder is left for the same reason: what
fills it is only known when the macro is called.

#### Merging Behavior

| Config                 | Result                    |
| ---------------------- | ------------------------- |
| Section absent         | All defaults used         |
| Section present, empty | All hotkeys disabled      |
| Section has entries    | Merged on top of defaults |

Use `__disabled__` to remove individual defaults:

```toml
[hotkeys]
"Primary+Shift+S" = "__disabled__"   # removes default scroll binding
"Ctrl+Space"      = "hints"          # adds binding; other defaults unchanged
```

When a mode is disabled (`enabled = false`), its default launcher hotkey is removed automatically.

**Mode toggling:** Append `--toggle` to turn a hotkey into a toggle — activates the mode on first press, exits to idle on the second. Works with any mode: `"Ctrl+F" = "grid --toggle"`.

#### Per-App Global Hotkey Overrides

`[[app_configs]]` overrides `[hotkeys]` bindings for specific apps. Use this when you want different launcher hotkeys depending on which app is focused.

```toml
[hotkeys]
"Cmd+Shift+Space" = "hints"

[[app_configs]]
bundle_id = "com.apple.Terminal"
hotkeys = {
    "Cmd+Space" = "hints",
    "Cmd+Shift+Space" = "__disabled__"
}

[[app_configs]]
bundle_id = "com.apple.Safari"
hotkeys = {
    "Cmd+Shift+F" = "hints",
    "Cmd+Shift+Space" = "__disabled__"
}
```

The same [merging rules](#merging-behavior) apply: app hotkeys merge on top of the base `[hotkeys]` bindings, and `__disabled__` removes an inherited binding. When no `[[app_configs]]` entry matches the focused app, the base `[hotkeys]` bindings are used as-is.

#### App identity across platforms (`bundle_id`)

The `bundle_id` key selects which app an override applies to — for both `[[app_configs]]` and every `[[<mode>.app_configs]]`. What it matches is platform-specific, even though the field is named `bundle_id` everywhere for config compatibility:

| Platform | Identity Neru matches | How to find it |
| --- | --- | --- |
| macOS | Bundle ID, reverse-DNS (e.g. `com.apple.Safari`) | `osascript -e 'id of app "Safari"'` |
| Linux · X11 | Window `WM_CLASS` — the *class* field | `xprop WM_CLASS`, then click the window |
| Linux · Wayland (wlroots: Sway/Hyprland/niri/COSMIC, and KWin/KDE) | Toplevel `app_id` | `swaymsg -t get_tree` (Sway), `hyprctl activewindow` (Hyprland), `niri msg windows` (niri), or your compositor's window inspector |
| Windows | Full path of the focused window's executable (e.g. `C:\Program Files\Google\Chrome\Application\chrome.exe`) | Task Manager, Details tab, right-click the process, **Open file location**, or `(Get-Process chrome).Path` in PowerShell |

On Linux, put the `WM_CLASS` or `app_id` in the `bundle_id` field. Matching is case-insensitive but exact — no globbing or partial matches.

On Windows, put the executable path in the `bundle_id` field, and use a TOML literal string (`'C:\...'`) or double every backslash so the path survives parsing. Matching is case-insensitive but otherwise exact, so the same program installed somewhere else (a per-user install under `%LOCALAPPDATA%`, a portable copy) is a different identity and needs its own entry. Packaged apps from the Microsoft Store all present their window through `ApplicationFrameHost.exe`, so they share one identity and cannot be told apart today.

> **Heads up:** Linux identity strings vary by toolkit and distribution. GTK, Qt, Electron, and XWayland apps often report a `WM_CLASS`/`app_id` you would not guess (e.g. `Google-chrome`, `code`, `org.kde.konsole`). Always confirm with the commands above rather than assuming a reverse-DNS name.

**GNOME/Mutter on Wayland is not supported at all** — the daemon exits at startup rather than running without per-app support, partly because Mutter implements no focused-app protocol (no `wlr-foreign-toplevel-management`) for Neru to identify the focused window with. GNOME on **X11** works normally, as do all wlroots compositors and KWin/KDE on Wayland. See [CROSS_PLATFORM.md](./CROSS_PLATFORM.md) for the backend matrix.

Where the compositor or X11 exposes a focus-change signal, Neru applies per-app overrides the instant you switch windows; otherwise it re-checks the focused app a few times per second.

### Per-Mode Hotkeys

Each mode can define hotkeys active only while that mode is running. Follows the same [merging rules](#merging-behavior) as global hotkeys.

**Precedence while a mode is open.** The mode's own table is asked first, and a
Ctrl/Alt/Cmd chord it does not bind falls back to `[hotkeys]` — so a global
`"Super+;" = "recursive_grid --toggle"` still toggles the mode off from inside it,
however you got there. Bind the chord in `[<mode>.hotkeys]` to give it a different
meaning in that mode; `__disabled__` does not silence it there, it removes the
*mode's* binding and hands the key back to the global one. Bare keys and
`Shift`-only combos are never taken this way — inside a mode those are its input.
How each platform delivers a chord to an open mode, and what that costs on X11,
is in [CROSS_PLATFORM.md](CROSS_PLATFORM.md#keyboard-capture-and-hotkeys).

```toml
[hints.hotkeys]
"Escape"    = "idle"
"Backspace" = "action backspace"
"Shift+L"   = ["action left_click", "idle"]

[scroll.hotkeys]
"gg"                   = "action go_top"      # two-letter sequence
"Primary+Shift+T"      = "exec open -a Terminal"
```

Multi-key alphabetic sequences (e.g. `gg`) use a 500ms timeout.

#### Per-App Hotkey Overrides

Both global and per-mode hotkeys support per-app overrides via `[[app_configs]]` and `[[<mode>.app_configs]]`.

- Global: `[[app_configs]]` overrides `[hotkeys]` bindings for specific apps
- Per-mode: `[[<mode>.app_configs]]` overrides `<mode>.hotkeys` for specific apps

Supported modes for per-mode overrides are `hints`, `grid`, `recursive_grid`, and `scroll`. App hotkeys merge on top of base hotkeys; `__disabled__` removes an inherited binding.

```toml
# Per-app global hotkey overrides (root-level)
[[app_configs]]
bundle_id = "com.apple.Terminal"
hotkeys = {
    "Cmd+Space" = "hints",
    "Cmd+Shift+Space" = "__disabled__"
}

# Per-app mode hotkey overrides
[[hints.app_configs]]
bundle_id = "com.brave.Browser"
hotkeys = {
    "Return" = "action left_click",
    "Shift+L" = "__disabled__"
}
```

**Priority order** when a key is pressed while Neru is running:

| Context                                      | Resolution                                                                                                                                                                 |
| -------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Idle (no mode active)**                    | `[hotkeys]` bindings merged with per-app `[[app_configs]]` overrides for the focused app                                                                                   |
| **Inside a mode**                            | `[<mode>.hotkeys]` merged with per-app `[[<mode>.app_configs]]` overrides, checked before the mode's built-in keys                                                         |
| **Global hotkey conflicts with mode hotkey** | Mode hotkey override wins (e.g., a global `Cmd+Shift+F = "hints"` launcher is replaced by `[hints.hotkeys]` `"Cmd+Shift+F" = "recursive_grid"` while hints mode is active) |

Inside a mode, the dispatch order is:

1. Modifier toggle
2. `<mode>.hotkeys` + per-app overrides
3. Mode built-in keys (hint/grid character input)

The merged result is resolved once — when the mode opens, when the focused app
changes, or when the configuration is replaced — and every keystroke consults
it. The focused app is learned by being told, so switching apps mid-mode puts
the new app's overrides in force on your next key. See
[Keymap learns the focused app](CROSS_PLATFORM.md#capability-matrix).

With `general.passthrough_unbounded_keys` on, which chords Neru consumes rather
than passes to the focused app follows the same change — see
[Keyboard capture and hotkeys](CROSS_PLATFORM.md#keyboard-capture-and-hotkeys).

### Action Reference

All actions available in hotkeys. These also work as `neru action <name>` — see [CLI.md](CLI.md#actions) for full flag documentation.

| Category    | Actions                                                                                |
| ----------- | -------------------------------------------------------------------------------------- |
| Click       | `left_click`, `right_click`, `middle_click`                                            |
| Button hold | `left_mouse_down`, `left_mouse_up`, `right_mouse_down`, `right_mouse_up`, `middle_mouse_down`, `middle_mouse_up` |
| Button toggle | `left_mouse_toggle`, `right_mouse_toggle`, `middle_mouse_toggle`                     |
| Mouse       | `move_mouse`, `move_mouse_relative`                                                    |
| Scroll      | `scroll_up`, `scroll_down`, `scroll_left`, `scroll_right`                              |
| Page        | `page_up`, `page_down`, `go_top`, `go_bottom`                                          |
| Keyboard    | `feed`                                                                                 |
| Hints       | `search_hints`, `cycle_hint`, `cycle_hint --backward`                                  |
| Delay       | `sleep <duration>` — plain numbers are seconds (`0.5`), explicit units: `500ms`, `1s`  |
| Mode        | `reset`, `backspace`, `move_cell --direction <dir>`                                    |
| Composition | `wait_for_mode_exit` (with optional `--bail`), `save_cursor_pos`, `restore_cursor_pos` |
| Cursor      | `hide_cursor`, `show_cursor`                                                           |

- Click actions accept `--state down` / `--state up` to press and release the button as separate hotkeys, and `--toggle` to do whichever comes next from a single hotkey. `"action right_click --state down"` and `"action right_mouse_down"` are the same action written two ways; the flag form is the documented spelling, and the name form is what a mode `--action` takes (`hints --action right_mouse_down`)
- Click actions can be chained with commas to produce multi-click sequences at one target point: `"action left_click,left_click"` double-clicks, `"action left_click,left_click,left_click"` triple-clicks. Only mouse button actions are allowed in a chain, and the names must not be separated by spaces (see [CLI.md](CLI.md#neru-action-left_click-right_click-middle_click))
- Any button Neru is holding is released automatically when it returns to idle
- `mouse_down` and `mouse_up` are the original spellings of `left_mouse_down` and `left_mouse_up`. They still work in configs, but new configs should use the explicit names
- Use `--bare` (e.g. `"action left_click --bare"`) to target the cursor position instead of the current mode selection (see [CLI.md](CLI.md#neru-action-left_click-right_click-middle_click))
- `scroll_up` / `scroll_down` support `--steps` (e.g. `"action scroll_down --steps 200"`) to override `scroll_step` (see [CLI.md](CLI.md#neru-action-scroll_up-scroll_down-scroll_left-scroll_right))
- `move_cell` slides the grid or recursive-grid selection to a neighbouring cell on the same layer, e.g. `"action move_cell --direction=right"`. It takes an optional `--count`, and repeats while the key is held when [`[held_repeat]`](#held_repeat) is enabled (see [CLI.md](CLI.md#neru-action-move_cell))
- `reset`, `backspace`, `move_cell`, `search_hints`, `cycle_hint`, `sleep`, `wait_for_mode_exit`, `save_cursor_pos`, `restore_cursor_pos`, `hide_cursor`, and `show_cursor` are not valid mode `--action` values — use `neru action ...` or in hotkeys as `"action ..."`
- `sleep` is the exception among those: it works only in hotkey bindings (`"action sleep 0.5"`), **not** as a terminal command, and it cannot appear in a comma-separated chain. See [CLI.md](CLI.md#action-sleep-hotkey-bindings-only)

#### Feed Keys

```toml
[hotkeys]
"Primary+Y"       = "action feed h e l l o return"
"Primary+Shift+C" = "action feed ctrl+c"

[hints.hotkeys]
"o"               = ["idle", "action feed o"]

# Feed into Neru's own mode system (--mode)
"Cmd+3"           = [
    "hints --role radio --text design --action left_click",
    "action feed --mode a",
]
```

Use `--mode` to route keys through Neru's active mode/action pipeline instead of the OS. See [CLI.md](CLI.md#neru-action-feed) for syntax, supported key names, and platform behavior.

#### Composition Example

```toml
[hints.hotkeys]
"Enter"  = ["action save_cursor_pos", "idle", "action wait_for_mode_exit", "action restore_cursor_pos"]
"Return" = ["action left_click", "action sleep 0.5", "hints"]

# Bail: abort the chain if the user cancels (Escape) instead of making a selection
"Ctrl+Z" = ["monitor_select", "action wait_for_mode_exit --bail", "recursive_grid"]
```

Use `--bail` to abort the chain when the mode exits without a selection (e.g., user presses Escape). Without `--bail`, `wait_for_mode_exit` always succeeds and the chain continues.

A step that fails for any other reason is logged and the chain continues. End that step with `--bail-on-error` to stop the chain there instead — useful when a later step should not happen unless the earlier one landed:

```toml
[hints.hotkeys]
# Exit only if the click actually landed.
"Shift+L" = ["action left_click --bail-on-error", "idle"]
```

`--bail-on-error` is a sequencing directive rather than a flag of the action, so it must come last in the step. See [CLI.md](CLI.md#failure-policy).

An array like the ones above is an *action sequence*. The same sequence, with the same rules, can also be written as a mode's `--on-exit` (repeat the flag once per step) or run from a script with [`neru run`](CLI.md#neru-run) — one executor backs all three, so a sequence that works in one place works in the others.

---

# Sections

## [macros]

Named action sequences. A macro is written once and invoked from any binding
with `macro <name> [args...]`, which keeps a sequence used by several keys in
one place instead of copied across them.

These are written, not recorded: there is nothing to capture and replay, unlike
the macros of a text editor. A macro is a named list of the same steps you would
otherwise inline into the binding, with positional arguments.

```toml
[macros]
# No arguments.
click_and_exit = ["action left_click --bail-on-error", "idle"]

# $1 and $2 are the first and second argument of the call.
window_click = [
    "action move_mouse --window --x -1000 --y -1000",
    "action sleep 0.1",
    "action move_mouse_relative --dx $1 --dy $2",
    "action left_click",
]

[hints.hotkeys]
"Enter" = "macro click_and_exit"

[[app_configs]]
bundle_id = "com.anthropic.claudefordesktop"
hotkeys = { "Cmd+1" = "macro window_click 100 70" }
```

**Names** use letters, digits, `_` and `-`, and start with a letter.

**Arguments** are positional: `$1`, `$2`, and so on, with `$$` for a literal
dollar sign. Substitution is textual and happens before the step is split into
arguments, so quote a placeholder that may contain spaces — `exec say "$1"` —
exactly as you would in a shell.

**Arity is checked when the config loads.** A call must pass exactly as many
arguments as the body uses; an unknown name or the wrong count fails
`neru config validate` rather than doing nothing when the key is pressed.

The check reaches everywhere an action can be written: `[hotkeys]`, every
`[<mode>.hotkeys]` table, the per-app overrides of both, the
[Mission Control hooks](#hints), a macro body, and the steps carried inside a
`run` or an `--on-exit`. So does the ordinary check that a step names a real
command — a step is validated at whatever depth it appears.

**A placeholder belongs in an argument, not in the command word.** A body step
like `"$1 --action left_click"` is rejected at load, because a step whose
command is only known at call time could not be validated at all.

**A macro body is a sequence** like any other, so it can use
[`--bail-on-error`](CLI.md#failure-policy), and it can call another macro. A
macro that reaches itself is stopped by the same nesting limit that bounds
`run` — five levels — rather than recursing.

**A macro runs as a nested sequence**, so a failure inside it is reported to the
caller as that one step failing. Mark the call with `--bail-on-error` when the
rest of the calling sequence should not continue past it.

Macros are not accepted as a mode's `--action`, which takes a mouse button
name; use `--on-exit` for a sequence that follows the action.

**A macro is also reachable from outside**, with
[`neru macro <name> [args...]`](CLI.md#neru-macro). The daemon runs it exactly
as a binding does, so an external driver (skhd, Hammerspoon, a shell script)
shares the same definition rather than keeping its own copy:

```bash
neru macro window_click 100 70
```

## [general]

Global behaviour that is not tied to a single mode: app exclusions, keyboard
layout, shortcut passthrough, and the shell used by `exec` hotkeys.

| Option                                 | Type   | Default       | Description                                                                                       |
| -------------------------------------- | ------ | ------------- | ------------------------------------------------------------------------------------------------- |
| `excluded_apps`                        | array  | `[]`          | Bundle IDs where Neru won't activate                                                              |
| `kb_layout_to_use`                     | string | `""`          | Force keyboard layout InputSourceID bundle ID (auto if empty). E.g. `com.apple.keylayout.Colemak` |
| `hide_overlay_in_screen_share`         | bool   | `false`       | Hide overlay in screen sharing apps                                                               |
| `passthrough_unbounded_keys`           | bool   | `false`       | Let unbound Cmd/Ctrl/Alt shortcuts pass through                                                   |
| `should_exit_after_passthrough`        | bool   | `false`       | Exit mode after a passthrough shortcut                                                            |
| `passthrough_unbounded_keys_blacklist` | array  | `[]`          | Shortcuts to keep consumed when passthrough is on                                                 |
| `exec_shell`                           | string | `"/bin/bash"` | Shell binary used for `exec` hotkey commands                                                      |
| `exec_shell_args`                      | array  | `["-lc"]`     | Shell arguments; command string is appended last                                                  |

Find available `kb_layout_to_use` IDs on macOS:

```bash
# get all enabled input sources
defaults read com.apple.HIToolbox AppleEnabledInputSources

# get the current keyboard layout that is active (e.g. if you use dvorak, it should be `com.apple.keylayout.Dvorak`)
defaults read com.apple.HIToolbox AppleCurrentKeyboardLayoutInputSourceID
```

---

## [theme]

Base colors used to derive all component defaults. Use solid `#RRGGBB` or `#RGB` (no alpha).

| Key             | Role                                                |
| --------------- | --------------------------------------------------- |
| `surface`       | Translucent fills, badges, indicator backgrounds    |
| `accent`        | Borders, lines, primary chrome                      |
| `accent_alt`    | Active/emphasis states, highlights, virtual pointer |
| `on_accent_alt` | Foreground text/icon on `accent_alt` surfaces       |
| `text`          | Foreground text on `surface` backgrounds            |

```toml
[theme.light]
surface       = "#EEF2FF"
accent        = "#465FBC"
accent_alt    = "#0B2377"
on_accent_alt = "#F8FAFF"
text          = "#17327A"

[theme.dark]
surface       = "#0A1338"
accent        = "#6E82D6"
accent_alt    = "#8FA2F0"
on_accent_alt = "#081022"
text          = "#E8EEFF"
```

Explicit component colors override theme derivation. Omitted colors inherit from the palette.

---

## [hints]

Labels clickable UI elements with short overlay labels. By default uses the platform accessibility tree (`axtree` strategy). Two screen-capture strategies exist for apps whose accessibility tree is too thin to hint from; both scan the focused window by default (`capture_scope` widens that to the whole screen), and both add the system surfaces the `include_*` options ask for from the accessibility tree:

- `vision`: on-screen recognition. The Vision framework on macOS (text plus rectangles), tesseract OCR on Linux and `Windows.Media.Ocr` on Windows (text only). Detected text becomes the element's title, so hint search (`--search`) and `--split-word` work. Costs an ML or OCR pass per activation; Linux needs tesseract installed and Windows an OCR language pack.
- `contour`: edge and contour analysis of the window pixels, an algorithm ported from [wl-kbptr](https://github.com/moverest/wl-kbptr). Finds anything with a visible outline (buttons, icons, toolbar items, text runs) in a few milliseconds with no external dependency. Elements carry no text, so search and word splitting do not apply, and `hints.vision.*` is not read.

Pick `vision` when you want to type what you see, or the app is text-heavy. Pick `contour` when latency matters, the targets are icons rather than words, or OCR is not installed. Both are overridable per-app.

Press `/` to text-search elements. `Space` for multi-word queries. `Return` confirms filtered hints (first is auto-selected). `Escape` cancels search.

Start with search visible: `neru hints --search` (see [CLI.md](CLI.md#neru-hints))

### Options

| Option                             | Type         | Default                 | Description                                                                                                                                                                                                                                                                                                                          |
| ---------------------------------- | ------------ | ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `enabled`                          | bool         | `true`                  | Enable/disable hints mode                                                                                                                                                                                                                                                                                                            |
| `strategy`                         | string       | `"axtree"`              | Element detection strategy: `"axtree"` (the platform accessibility tree), `"vision"` (screen recognition — Vision framework on macOS, tesseract OCR on Linux, Windows.Media.Ocr on Windows), or `"contour"` (edge and contour analysis ported from wl-kbptr, every platform). Both capture strategies detect the focused window content from a screen capture; see the section intro for when to pick which. Overridable per-app via `[hints.app_configs]`. |
| `capture_scope`                    | string       | `"window"`              | Region the `vision` and `contour` strategies scan: `"window"` (the focused window, or the whole screen when nothing is focused) or `"screen"` (the whole active screen, so notifications, panels and adjacent tiled windows get hints too, at the cost of a bigger capture). Ignored by `axtree`. Overridable per-app via `[hints.app_configs]` and per-activation via `neru hints --capture-scope`. |
| `hint_characters`                  | string       | `"asdfghjkl"`           | Characters used for labels                                                                                                                                                                                                                                                                                                           |
| `label_direction`                  | string       | `"normal"`              | Hint label algorithm: `"normal"` (default, prefix-avoidance greedy) or `"reverse"` (reverse-order tiers). Empty value defaults to `"normal"`. Overridable per-app via `[hints.app_configs]` and per-activation via the `neru hints --label-direction` CLI flag. See [Choosing a label direction](#choosing-a-label-direction) below. |
| `max_depth`                        | int          | `50`                    | Max accessibility tree depth (0 = unlimited)                                                                                                                                                                                                                                                                                         |
| `include_menubar_hints`            | bool         | `false`                 | Show hints on menubar items                                                                                                                                                                                                                                                                                                          |
| `include_dock_hints`               | bool         | `false`                 | Show hints on Dock items                                                                                                                                                                                                                                                                                                             |
| `include_nc_hints`                 | bool         | `false`                 | Show hints in Notification Center                                                                                                                                                                                                                                                                                                    |
| `include_stage_manager_hints`      | bool         | `false`                 | Show hints in Stage Manager                                                                                                                                                                                                                                                                                                          |
| `include_pip_hints`                | bool         | `false`                 | Show hints on Picture in Picture controls                                                                                                                                                                                                                                                                                            |
| `include_screen_capture_hints`     | bool         | `false`                 | Show hints on Screen Capture controls                                                                                                                                                                                                                                                                                                |
| `detect_mission_control`           | bool         | `false`                 | Enable Mission Control state detection                                                                                                                                                                                                                                                                                               |
| `on_mission_control_activated`     | string/array | none                    | Action(s) to execute when Mission Control opens                                                                                                                                                                                                                                                                                      |
| `on_mission_control_deactivated`   | string/array | none                    | Action(s) to execute when Mission Control closes                                                                                                                                                                                                                                                                                     |
| `additional_menubar_hints_targets` | array        | macOS-specific defaults | Extra menubar bundle IDs                                                                                                                                                                                                                                                                                                             |
| `clickable_roles`                  | array        | shared semantic defaults | Roles that generate hints. See [Clickable roles](#clickable-roles)                                                                                                                                                                                                                                                                  |
| `ignore_clickable_check`           | bool         | `false`                 | Skip clickability heuristic                                                                                                                                                                                                                                                                                                          |
| `visible_check_enabled`            | bool         | `false`                 | Enable visibility hit-test (slower but fewer noisy hints)                                                                                                                                                                                                                                                                            |

### Clickable roles

`hints.clickable_roles` decides which accessibility elements get a hint. Each platform
exposes a different role vocabulary — macOS uses AX roles, Linux uses AT-SPI role names,
Windows uses UI Automation control types — so entries are written in neru's own semantic
vocabulary and resolved to the native names of whichever platform neru is running on.

```toml
[hints]
clickable_roles = ["button", "link", "text_field"]
```

The same list works on every platform. Run `neru roles` to see the vocabulary and how each
name resolves here, and `neru roles --explain` to see how your own config resolves.

#### Semantic roles

| Semantic       | macOS (`ax:`)          | Linux (`atspi:`)                        | Windows (`uia:`)      |
| -------------- | ---------------------- | --------------------------------------- | --------------------- |
| `button`       | `AXButton`             | `push button`, `button`, `toggle button` | `Button`, `SplitButton` |
| `menu_button`  | `AXMenuButton`         | `push button menu`                      | —                     |
| `popup_button` | `AXPopUpButton`        | `combo box`                             | `ComboBox`            |
| `combo_box`    | `AXComboBox`           | `combo box`                             | `ComboBox`            |
| `link`         | `AXLink`               | `link`                                  | `Hyperlink`           |
| `checkbox`     | `AXCheckBox`           | `check box`, `check menu item`          | `CheckBox`            |
| `radio`        | `AXRadioButton`        | `radio button`, `radio menu item`       | `RadioButton`         |
| `switch`       | `AXSwitch` †           | `switch`, `toggle button`               | —                     |
| `disclosure`   | `AXDisclosureTriangle` | —                                       | —                     |
| `text_field`   | `AXTextField`          | `entry`, `password text`                | `Edit`                |
| `text_area`    | `AXTextArea`           | `entry`                                 | `Edit`                |
| `search_field` | `AXSearchField` †      | `entry`                                 | `Edit`                |
| `slider`       | `AXSlider`             | `slider`                                | `Slider`              |
| `stepper`      | `AXIncrementor`        | `spin button`                           | `Spinner`             |
| `tab`          | `AXTabButton` †        | `page tab`                              | `TabItem`             |
| `menu_item`    | `AXMenuItem`           | `menu item`                             | `MenuItem`            |
| `menubar_item` | `AXMenuBarItem`        | —                                       | —                     |
| `dock_item`    | `AXDockItem`           | —                                       | —                     |
| `cell`         | `AXCell`               | `table cell`                            | `DataItem`            |
| `row`          | `AXRow`                | `table row`                             | `TreeItem`            |
| `list_item`    | `AXRow`                | `list item`                             | `ListItem`            |
| `image`        | `AXImage`              | `image`, `icon`                         | `Image`               |
| `static_text`  | `AXStaticText`         | `static`, `label`, `text`               | `Text`                |
| `heading`      | `AXHeading`            | `heading`                               | —                     |
| `color_well`   | `AXColorWell`          | `color chooser`                         | —                     |
| `toolbar_button` | `AXToolbarButton` †  | —                                       | —                     |

A `—` means the platform has no equivalent; that entry is ignored there. `neru config
validate` warns about one as soon as your `clickable_roles` differs from the shipped list
— the shipped list itself stays silent, since it is one list for every platform — and
`neru roles --explain` and `neru doctor` report it either way. An application's
`additional_clickable_roles` are always your own, so those are always reported.

† A subrole, not a role: AppKit reports these names in the element's *subrole* while the
role stays generic — a search field is an `AXTextField` with subrole `AXSearchField`, a
SwiftUI toggle an `AXCheckBox` with subrole `AXSwitch`. Neru matches configured names
against both the role and the subrole, so these entries work as written.

#### Native roles

The semantic vocabulary is deliberately not exhaustive. Any native role can be addressed
directly through its vocabulary prefix:

```toml
clickable_roles = [
    "button",
    "ax:AXDisclosureTriangle",   # macOS only
    "atspi:page tab list",       # Linux only
    "uia:Custom",                # Windows only
]
```

Prefixed entries that belong to another platform are ignored rather than rejected, so one
config file can serve several machines. This is the escape hatch for toolkits that expose
little role information — many legacy Win32 and WinForms controls surface only as
`uia:Pane`, `uia:Custom` or `uia:Document`, and hinting them requires naming them directly.

Unprefixed entries must be semantic roles. An unrecognised one is a configuration error,
which is what makes typos visible:

```
hints.clickable_roles: unknown role "AXButton": use "button"
```

### UI

| Option               | Type   | Default    | Description                                |
| -------------------- | ------ | ---------- | ------------------------------------------ |
| `font_size`          | int    | `10`       | Font size in points                        |
| `font_family`        | string | `""`       | Font family; [generic aliases](CROSS_PLATFORM.md#capability-matrix) accepted, empty among them — it asks for the platform's sans family |
| `border_radius`      | int    | `-1`       | Corner radius (-1 = auto)                  |
| `padding_x`          | int    | `-1`       | Horizontal padding (-1 = auto)             |
| `padding_y`          | int    | `-1`       | Vertical padding (-1 = auto)               |
| `border_width`       | int    | `1`        | Border width in pixels                     |
| `placement`          | string | `"bottom"` | Label placement relative to the element: `top`, `center`, `bottom`; `top` and `bottom` also draw a [connector arrow](CROSS_PLATFORM.md#mode-coverage) |
| `background_color`   | color  | derived    | Background color                           |
| `text_color`         | color  | derived    | Text color                                 |
| `matched_text_color` | color  | derived    | Text color for matched characters          |
| `border_color`       | color  | derived    | Border color                               |

```toml
[hints.ui]
font_size = 10
border_radius = -1
padding_x = -1
padding_y = -1
border_width = 1
placement = "bottom"
```

### Boundary Highlight

Optional element outlines for dense layouts. Off by default.

| Option             | Type  | Default | Description                    |
| ------------------ | ----- | ------- | ------------------------------ |
| `enabled`          | bool  | `false` | Draw element boundaries        |
| `border_width`     | int   | `1`     | Stroke width in pixels         |
| `border_radius`    | int   | `-1`    | Corner radius (-1 = auto pill) |
| `background_color` | color | derived | Element fill color             |
| `border_color`     | color | derived | Element stroke color           |

```toml
[hints.boundary_highlight]
enabled = false
border_width = 1
border_radius = -1
```

### Search Input UI

| Option     | Type   | Default           | Description                                                                                             |
| ---------- | ------ | ----------------- | ------------------------------------------------------------------------------------------------------- |
| `position` | string | `"bottom_center"` | Anchor: `top_left`, `top_center`, `top_right`, `center`, `bottom_left`, `bottom_center`, `bottom_right` |
| `x_offset` | int    | `0`               | Horizontal offset from anchor                                                                           |
| `y_offset` | int    | `24`              | Vertical offset from anchor                                                                             |
| `width`    | int    | `320`             | Width in pixels                                                                                         |

Also supports all [hints UI](#ui) visual options except `matched_text_color`.

```toml
[hints.search_input_ui]
position = "bottom_center"
x_offset = 0
y_offset = 24
width = 320
```

### Vision

Tunable settings for Vision-based hint detection (only used when `hints.strategy` or the app-specific `strategy` override is set to `"vision"`).

The engine differs by platform and the options mostly do not: macOS runs the
Vision framework, Linux runs tesseract OCR and Windows runs `Windows.Media.Ocr`,
each over a screen capture of the focused window. Two consequences are visible.
**Rectangle detection is macOS-only** — an OCR engine answers text and nothing
else — so `detect_rectangles` and the four `rectangle_*` options are read on
macOS alone and warn once at load if written elsewhere. And `Windows.Media.Ocr`
reports no per-word confidence, so `minimum_confidence`,
`button_min_confidence` and `generic_clickable_min_confidence` are inert on
Windows: every word scores one there. Every other option below is read on all
three.

| Option                             | Type  | Default | Description                                                                                       |
| ---------------------------------- | ----- | ------- | ------------------------------------------------------------------------------------------------- |
| `detect_text`                      | bool  | `true`  | Enable text detection. With this off, Linux detects nothing at all.                               |
| `detect_rectangles`                | bool  | `true`  | Enable rectangle detection.                                                       |
| `request_timeout_ms`               | int   | `5000`  | Timeout in milliseconds for one analysis request (one OCR pass on Linux).                         |
| `minimum_confidence`               | float | `0.0`   | Minimum confidence score (0.0 to 1.0) for keeping an observation.                                 |
| `merge_iou_threshold`              | float | `0.5`   | Intersection-over-Union (IoU) overlap threshold for merging redundant overlapping bounding boxes. |
| `rectangle_max_candidates`         | int   | `100`   | Maximum number of rectangle candidate observations to evaluate.                   |
| `rectangle_min_size`               | float | `0.01`  | Minimum normalized size of detected rectangles (e.g. `0.01` is 1% of screen/window dimensions). |
| `rectangle_min_aspect`             | float | `0.3`   | Minimum aspect ratio (width/height) for rectangle elements.                       |
| `rectangle_max_aspect`             | float | `10.0`  | Maximum aspect ratio (width/height) for rectangle elements.                       |
| `button_min_confidence`            | float | `0.3`   | Minimum confidence score threshold for classifying a rectangle as a button.                       |
| `button_min_aspect`                | float | `0.8`   | Minimum aspect ratio for button elements.                                                         |
| `button_max_aspect`                | float | `8.0`   | Maximum aspect ratio for button elements.                                                         |
| `button_icon_max_size`             | int   | `48`    | Maximum width/height in pixels for square button or icon elements.                                |
| `link_min_aspect`                  | float | `5.0`   | Minimum aspect ratio for text link elements.                                                      |
| `link_max_height`                  | int   | `40`    | Maximum height in pixels for text link elements.                                                  |
| `link_min_width`                   | int   | `50`    | Minimum width in pixels for text link elements.                                                   |
| `image_min_size`                   | int   | `48`    | Minimum width/height in pixels for image elements.                                                |
| `checkbox_max_size`                | int   | `32`    | Maximum width/height in pixels for checkbox elements.                                             |
| `generic_clickable_min_confidence` | float | `0.5`   | Minimum confidence threshold for generic clickable elements.                                      |

```toml
[hints.vision]
detect_text = true
detect_rectangles = true
request_timeout_ms = 5000
minimum_confidence = 0.0
merge_iou_threshold = 0.5
rectangle_max_candidates = 100
rectangle_min_size = 0.01
rectangle_min_aspect = 0.3
rectangle_max_aspect = 10.0
button_min_confidence = 0.3
button_min_aspect = 0.8
button_max_aspect = 8.0
button_icon_max_size = 48
link_min_aspect = 5.0
link_max_height = 40
link_min_width = 50
image_min_size = 48
checkbox_max_size = 32
generic_clickable_min_confidence = 0.5
```

### Choosing a label direction

The `label_direction` setting controls how multi-character hint labels are enumerated once the single-character pool is exhausted. With a 4-character alphabet (`asdf`) and 5 hinted elements, the two algorithms produce visibly different label sequences:

| Direction          | Sequence         | Notes                                                                              |
| ------------------ | ---------------- | ---------------------------------------------------------------------------------- |
| `normal` (default) | `A S D FA FS`    | Keeps 3 single-char labels, then expands the 4th alphabet slot into 2-char labels. |
| `reverse`          | `AA SA DA FA AS` | Fills the 2-char tier uniformly from the first alphabet character.                 |

**When to prefer `normal` (default):**

- Most workflows — fewer keystrokes for the common case where 1- or 2-character labels are enough.
- Hint characters are scarce (e.g. a 2- or 3-character alphabet), so single-char labels stay usable longer.

**When to prefer `reverse`:**

- Many hints clustered in one region of the screen. `reverse` spreads the _first_ character of each label evenly across the alphabet, so labels rarely share a prefix and the hint key (the visible character) is less likely to be occluded by another element.
- Workflows that consistently need more than `len(hint_characters)` hints.

You can also mix directions per-app via `[hints.app_configs]` or per-activation via `neru hints --label-direction`. See the [per-app config table](#per-app-config) and [CLI reference](CLI.md#neru-hints).

### Per-App Config

| Field                        | Type   | Description                                                                                                                                                                               |
| ---------------------------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bundle_id`                  | string | App bundle ID                                                                                                                                                                             |
| `strategy`                   | string | Override element detection strategy for this app (`"axtree"`, `"vision"`, or `"contour"`). Empty string = use global `hints.strategy`.                                                                  |
| `capture_scope`              | string | Override the region the `vision` and `contour` strategies scan for this app (`"window"` or `"screen"`). Empty string = use global `hints.capture_scope`. |
| `label_direction`            | string | Override hint label algorithm for this app (`"normal"` or `"reverse"`). Empty string = use global `hints.label_direction`. See [Choosing a label direction](#choosing-a-label-direction). |
| `additional_clickable_roles` | array  | Extra roles to treat as clickable, same vocabulary as [`clickable_roles`](#clickable-roles)                                                                                              |
| `ignore_clickable_check`     | bool   | Skip clickability heuristic for this app                                                                                                                                                  |
| `visible_check_enabled`      | bool   | Enable visibility hit-test for this app                                                                                                                                                   |
| `hotkeys`                    | map    | [per-app hotkey overrides](#per-app-hotkey-overrides)                                                                                                                                     |

```toml
[[hints.app_configs]]
bundle_id = "com.apple.Safari"
strategy = "vision"
label_direction = "reverse"
additional_clickable_roles = ["link"]
ignore_clickable_check = true
visible_check_enabled = true
```

---

## [grid]

Divides the screen into a labelled coordinate grid.

Cursor behavior is chosen per invocation: `neru grid --cursor-selection-mode follow|hold` (see [CLI.md](CLI.md#neru-grid)). Default hotkeys include `` ` `` for `toggle-cursor-follow-selection`.

### Options

| Option              | Type   | Default                       | Description                 |
| ------------------- | ------ | ----------------------------- | --------------------------- |
| `enabled`           | bool   | `true`                        | Enable/disable grid mode    |
| `characters`        | string | `"abcdefghijklmnpqrstuvwxyz"` | Primary grid labels         |
| `sublayer_keys`     | string | `"abcdefghijklmnpqrstuvwxyz"` | Subgrid labels; empty is resolved at load time to the characters the grid is labelled with, the same ones `row_labels` is inferred from. Only the first 9 are used — the subgrid is 3×3 |
| `max_label_length`  | int    | `4`                           | Maximum coarse-grid label length (2–4). The default preserves the legacy automatic 2–4-key layout. When a limit of 2 shortens an automatically longer label, the coarse grid is enlarged and spatially rebalanced while still covering the screen; the following subgrid refinement remains one keypress |
| `row_labels`        | string | `""`                          | Custom row labels; empty is resolved at load time to the labels inferred from `characters` |
| `col_labels`        | string | `""`                          | Custom column labels; empty is resolved the same way as `row_labels`                       |
| `live_match_update` | bool   | `true`                        | Highlight cells as you type |
| `hide_unmatched`    | bool   | `true`                        | Hide non-matching cells     |
| `prewarm_enabled`   | bool   | `true`                        | Pre-compute grid on startup |
| `enable_gc`         | bool   | `false`                       | Periodic memory cleanup     |

**The label sets are checked when the config loads.** `characters`, `row_labels`
and `col_labels` all name cells you then have to type, so `neru config validate`
warns when one of them holds a single character, the same character twice (case
is folded — `aA` is one character written twice), or a character that cannot be
typed: whitespace or a control character. None of these stops a grid being built
— a short `characters` is replaced by `a-z`, short labels cap the grid to the
cells they can name, and a repeat is dropped — so the configuration still loads
and the warning is where you hear about it. A label left empty is checked as the
`characters` it is inferred from and reported under that name, so one mistake is
reported once.

**A repeat is dropped, never drawn.** Every set a grid is labelled from is read as
its distinct characters, `sublayer_keys` included, so the grid is built from `ab`
whether you wrote `ab`, `aab` or `aAb`. That is why the repeat is worth a warning
rather than a refusal: what it costs is a shorter alphabet, not a cell you can see
and cannot click. Note that dropping repeats is also what can leave a set too
short — `characters = "aa"` has one usable character, so it falls back to `a-z`,
and both facts are reported.

Two neighbouring rules are older and stricter, and refuse the file rather than
warn: `characters` cannot be empty, and neither `characters` nor `sublayer_keys`
may contain a character outside ASCII. `row_labels` and `col_labels` warn about
non-ASCII instead. `sublayer_keys` is not checked for the three faults above:
only its first 9 characters are drawn, so which of them matter depends on the
subgrid rather than on the option — a set of exactly 9 with a repeat in it leaves
one subgrid cell unlabelled, which is visible on screen in a way a warning is not.

### UI

| Option                     | Type   | Default | Description                          |
| -------------------------- | ------ | ------- | ------------------------------------ |
| `font_size`                | int    | `10`    | Font size in points                  |
| `font_family`              | string | `""`    | Font family; [generic aliases](CROSS_PLATFORM.md#capability-matrix) accepted, empty among them — it asks for the platform's sans family |
| `border_width`             | int    | `1`     | Border width in pixels               |
| `background_color`         | color  | derived | Cell background                      |
| `text_color`               | color  | derived | Label text                           |
| `matched_text_color`       | color  | derived | Matched cell text                    |
| `matched_background_color` | color  | derived | Matched cell background              |
| `matched_border_color`     | color  | derived | Matched cell border                  |
| `border_color`             | color  | derived | Default cell border                  |

```toml
[grid.ui]
font_size = 10
border_width = 1
```

### Per-App Config

```toml
[[grid.app_configs]]
bundle_id = "com.brave.Browser"
hotkeys = { "Return" = "action left_click" }
```

See [per-app hotkey overrides](#per-app-hotkey-overrides).

---

## [recursive_grid]

Narrows the active area with each keypress for precise cursor placement.

Cursor behavior: `neru recursive_grid --cursor-selection-mode follow|hold` (see [CLI.md](CLI.md#neru-recursive_grid)). Auto-zoom to a specific depth on activation with `--zoom-to-depth <n>` (e.g. `neru recursive_grid --zoom-to-depth 3`). Default hotkeys include `` ` `` for `toggle-cursor-follow-selection`.

### Options

| Option            | Type   | Default       | Description                                                      |
| ----------------- | ------ | ------------- | ---------------------------------------------------------------- |
| `enabled`         | bool   | `true`        | Enable/disable mode                                              |
| `grid_cols`       | int    | `3`           | Columns (≥ 1; total cells ≥ 2)                                   |
| `grid_rows`       | int    | `3`           | Rows (≥ 1; total cells ≥ 2)                                      |
| `keys`            | string | `"rtyfghvbn"` | Cell selection keys (must be `grid_cols × grid_rows` characters) |
| `min_size_width`  | int    | `1`           | Minimum cell width in pixels                                     |
| `min_size_height` | int    | `1`           | Minimum cell height in pixels                                    |
| `max_depth`       | int    | `10`          | Maximum recursion levels (1–20)                                  |
| `layers`          | array  | `[]`          | Per-depth layout overrides (see below)                           |

#### Layers

Each entry overrides the grid dimensions and keys for a specific depth:

| Field       | Type   | Default        | Description                                 |
| ----------- | ------ | -------------- | ------------------------------------------- |
| `depth`     | int    | required       | Recursion depth to override (0 based index) |
| `grid_cols` | int    | same as parent | Columns at this depth                       |
| `grid_rows` | int    | same as parent | Rows at this depth                          |
| `keys`      | string | same as parent | Selection keys at this depth                |

```toml
[recursive_grid]
layers = [
  { depth = 0, grid_cols = 2, grid_rows = 2, keys = "crtn," },
  { depth = 1, grid_cols = 3, grid_rows = 3, keys = "gcrhtnmwv" },
]
```

### Animation

| Option        | Type | Default | Description                                     |
| ------------- | ---- | ------- | ----------------------------------------------- |
| `enabled`     | bool | `true`  | Native depth transitions on supported platforms |
| `duration_ms` | int  | `50`    | Transition duration in milliseconds             |

### UI

| Option                                | Type   | Default | Description                                                                  |
| ------------------------------------- | ------ | ------- | ---------------------------------------------------------------------------- |
| `font_size`                           | int    | `10`    | Font size                                                                    |
| `font_family`                         | string | `""`    | Font family; [generic aliases](CROSS_PLATFORM.md#capability-matrix) accepted, empty among them — it asks for the platform's sans family |
| `line_width`                          | int    | `1`     | Grid line width                                                              |
| `line_color`                          | color  | derived | Grid line color                                                              |
| `highlight_color`                     | color  | derived | Selected cell highlight                                                      |
| `text_color`                          | color  | derived | Label text                                                                   |
| `label_background`                    | bool   | `false` | Background behind labels                                                     |
| `label_background_color`              | color  | derived | Label background                                                             |
| `label_background_padding_x`          | int    | `-1`    | Horizontal label padding (-1 = auto)                                         |
| `label_background_padding_y`          | int    | `-1`    | Vertical label padding (-1 = auto)                                           |
| `label_background_border_radius`      | int    | `-1`    | Label corner radius (-1 = auto)                                              |
| `label_background_border_width`       | int    | `1`     | Label border width                                                           |
| `label_char`                          | string | `""`    | Override all cell labels with a single character (e.g. `·`); empty = use key |
| `label_autohide_multiplier`           | float  | `1.5`   | Hide labels when cell < fontSize × multiplier (0 = disable)                  |
| `sub_key_preview`                     | bool   | `false` | Show a mini-grid of the next level's keys inside each cell                    |
| `sub_key_preview_font_size`           | int    | `8`     | Sub-key preview font size                                                    |
| `sub_key_preview_autohide_multiplier` | float  | `1.5`   | Autohide threshold multiplier, measured against one sub-cell of that mini-grid |
| `sub_key_preview_text_color`          | color  | derived | Sub-key preview text color                                                   |
| `sub_key_preview_label_char`          | string | `""`    | Override sub-key labels with a single character (e.g. `·`); empty = use key  |

```toml
[recursive_grid.ui]
line_width = 1
font_size = 10
label_background = false
sub_key_preview = false
```

### Per-App Config

```toml
[[recursive_grid.app_configs]]
bundle_id = "com.brave.Browser"
hotkeys = { "u" = "action left_click" }
```

See [per-app hotkey overrides](#per-app-hotkey-overrides).

---

## [scroll]

Keyboard-driven scrolling.

### Options

| Option             | Type | Default   | Description                                                                                     |
| ------------------ | ---- | --------- | ----------------------------------------------------------------------------------------------- |
| `scroll_step`      | int  | `50`      | Pixels per line scroll action                                                                   |
| `scroll_step_half` | int  | `500`     | Pixels per half-page action                                                                     |
| `scroll_step_full` | int  | `1000000` | Pixels for top/bottom jump actions                                                              |
| `invert_scroll`    | bool | `false`   | Invert scroll direction (useful when using tools like Mos that reverse synthetic scroll events) |

### Default Hotkeys

```toml
[scroll.hotkeys]
"Escape"  = "idle"
"k"       = "action scroll_up"
"j"       = "action scroll_down"
"h"       = "action scroll_left"
"l"       = "action scroll_right"
"gg"      = "action go_top"
"Shift+G" = "action go_bottom"
"u"       = "action page_up"
"PageUp"  = "action page_up"
"d"       = "action page_down"
"PageDown"= "action page_down"
```

### Per-App Config

| Field              | Type   | Description                                           |
| ------------------ | ------ | ----------------------------------------------------- |
| `bundle_id`        | string | App bundle ID                                         |
| `scroll_step`      | int    | Optional app-specific scroll step override            |
| `scroll_step_half` | int    | Optional app-specific scroll step half override       |
| `scroll_step_full` | int    | Optional app-specific scroll step full override       |
| `hotkeys`          | map    | [per-app hotkey overrides](#per-app-hotkey-overrides) |

```toml
[[scroll.app_configs]]
bundle_id = "com.apple.Safari"
scroll_step = 25
scroll_step_half = 200
scroll_step_full = 1000
hotkeys = { "k" = "action scroll_up", "j" = "action scroll_down" }
```

---

## [monitor_select]

Interactive display picking mode. Shows per-monitor overlay badges labelled with selectable characters. Monitors are sorted in a fixed spatial order (top-to-bottom, left-to-right).

**Platforms:** macOS · Linux. Not implemented on Windows.

| Option       | Type   | Default       | Description                        |
| ------------ | ------ | ------------- | ---------------------------------- |
| `enabled`    | bool   | `false`       | Enable interactive monitor picking |
| `characters` | string | `"123456789"` | Characters used for monitor labels |

### UI

| Key                    | Default       | Description                       |
| ---------------------- | ------------- | --------------------------------- |
| `font_size`            | `96`          | Badge label font size             |
| `font_family`          | `""` (sans)   | Badge label font family; [generic aliases](CROSS_PLATFORM.md#capability-matrix) accepted, empty among them — it asks for the platform's sans family |
| `subtitle_font_size`   | `18`          | Monitor name subtitle font size   |
| `subtitle_font_family` | `""` (label's) | Subtitle font family, defaulting to the label's; [generic aliases](CROSS_PLATFORM.md#capability-matrix) accepted |
| `border_radius`        | `-1` (auto)   | Badge corner radius               |
| `padding_x`            | `-1` (auto)   | Horizontal padding                |
| `padding_y`            | `-1` (auto)   | Vertical padding                  |
| `border_width`         | `0`           | Badge border width                |
| `background_color`     | derived       | Badge fill color                  |
| `text_color`           | derived       | Label text color                  |
| `matched_text_color`   | derived       | Partially-typed label text color  |
| `border_color`         | derived       | Badge border color                |
| `backdrop_color`       | `""` (none)   | Per-monitor overlay backdrop tint |
| `subtitle_text_color`  | derived       | Subtitle text color               |

### Hotkeys

| Key      | Default | Description               |
| -------- | ------- | ------------------------- |
| `Escape` | `idle`  | Cancel and return to idle |

```toml
[monitor_select]
enabled = false
characters = "123456789"

[monitor_select.ui]
font_size = 96
font_family = ""
subtitle_font_size = 18
subtitle_font_family = ""
border_radius = -1
padding_x = -1
padding_y = -1
border_width = 0
backdrop_color = ""

[monitor_select.hotkeys]
"Escape" = "idle"
```

---

## [virtual_pointer]

A small character rendered at the cursor position when the system cursor is
hidden — the standalone virtual-pointer overlay.

**Platforms:** macOS only for that standalone overlay. It pairs with
`hide_cursor`, which has no cross-platform equivalent, so it is a no-op on Linux
and Windows. The separate virtual-pointer indicator drawn inside the
recursive-grid overlay works on all platforms, and it takes its character, size,
family and color from these same `[virtual_pointer.ui]` options — there is no
pointer option under `[recursive_grid]`. Grid mode draws its in-frame indicator
from them too; which platforms draw which is in the
[mode coverage matrix](CROSS_PLATFORM.md#mode-coverage).

### UI

| Option        | Type   | Default | Description                  |
| ------------- | ------ | ------- | ---------------------------- |
| `char`        | string | `"●"`   | Character to display         |
| `font_size`   | int    | `8`     | Font size in points          |
| `font_family` | string | `""`    | Font family; [generic aliases](CROSS_PLATFORM.md#capability-matrix) accepted, empty among them — it asks for the platform's sans family |
| `text_color`  | color  | derived | Character color              |

```toml
[virtual_pointer.ui]
char = "●"
font_size = 8
font_family = ""
```

---

## [mouse_action_indicator]

Transient visual marker drawn at the point of a mouse action.

**Platforms:** all. The animation is implemented natively per platform —
CoreAnimation on macOS, a 120fps goroutine on Linux, and a 60fps goroutine on
Windows — so timing may differ slightly.

| Option    | Type     | Default                                       | Description        |
| --------- | -------- | --------------------------------------------- | ------------------ |
| `enabled` | bool     | `false`                                       | Enable indicators  |
| `actions` | string[] | every click, press, release, and toggle action | Triggering actions |

`actions` accepts any mouse button action from the [Action Reference](#action-reference):
`left_click`, `right_click`, `middle_click`, the six `*_mouse_down` / `*_mouse_up`
actions, and the three `*_mouse_toggle` actions. The deprecated `mouse_down` and
`mouse_up` spellings still match the left button's press and release.

### UI

| Option             | Type   | Default    | Description          |
| ------------------ | ------ | ---------- | -------------------- |
| `size`             | int    | `36`       | Diameter in points   |
| `border_width`     | int    | `2`        | Border width         |
| `background_color` | color  | derived    | Fill color           |
| `border_color`     | color  | derived    | Stroke color         |
| `shape`            | string | `"circle"` | `circle` or `square` |

### Animation

| Option          | Type   | Default      | Description                                    |
| --------------- | ------ | ------------ | ---------------------------------------------- |
| `duration_ms`   | int    | `260`        | Animation duration in ms                       |
| `start_scale`   | float  | `0.55`       | Starting scale                                 |
| `end_scale`     | float  | `1.35`       | Ending scale                                   |
| `start_opacity` | float  | `0.85`       | Starting opacity                               |
| `end_opacity`   | float  | `0.0`        | Ending opacity                                 |
| `easing`        | string | `"ease_out"` | `linear`, `ease_in`, `ease_out`, `ease_in_out` |

```toml
[mouse_action_indicator]
enabled = false
actions = ["left_click", "right_click"]

[mouse_action_indicator.ui]
size = 36
shape = "circle"

[mouse_action_indicator.animation]
duration_ms = 260
easing = "ease_out"
```

---

## [mode_indicator]

A floating label that follows the cursor and displays the current mode name.

### Per-Mode

| Option             | Type   | Default        | Description                       |
| ------------------ | ------ | -------------- | --------------------------------- |
| `enabled`          | bool   | varies by mode | Show/hide indicator for this mode |
| `text`             | string | varies by mode | Label text                        |
| `background_color` | color  | derived        | Override background color         |
| `text_color`       | color  | derived        | Override text color               |
| `border_color`     | color  | derived        | Override border color             |

```toml
[mode_indicator.scroll]
enabled = true
text = "Scroll"

[mode_indicator.hints]
enabled = false
text = "Hints"

[mode_indicator.grid]
enabled = false
text = "Grid"

[mode_indicator.recursive_grid]
enabled = false
text = "Recursive Grid"

[mode_indicator.monitor_select]
enabled = false
text = "Monitor Select"
```

### UI

| Option               | Type   | Default | Description                             |
| -------------------- | ------ | ------- | --------------------------------------- |
| `font_size`          | int    | `10`    | Font size                               |
| `font_family`        | string | `""`    | Font family; [generic aliases](CROSS_PLATFORM.md#capability-matrix) accepted, empty among them — it asks for the platform's sans family |
| `background_color`   | color  | derived | Background with alpha                   |
| `text_color`         | color  | derived | Text color                              |
| `border_color`       | color  | derived | Border color                            |
| `border_width`       | int    | `1`     | Border width                            |
| `padding_x`          | int    | `-1`    | Horizontal padding (-1 = auto)          |
| `padding_y`          | int    | `-1`    | Vertical padding (-1 = auto)            |
| `border_radius`      | int    | `-1`    | Corner radius (-1 = auto)               |
| `indicator_x_offset` | int    | `20`    | X offset from cursor (positive = right) |
| `indicator_y_offset` | int    | `20`    | Y offset from cursor (positive = down)  |

```toml
[mode_indicator.ui]
font_size = 10
border_width = 1
padding_x = -1
padding_y = -1
border_radius = -1
indicator_x_offset = 20
indicator_y_offset = 20
```

---

## [sticky_modifiers]

Tap modifiers inside a mode to make them sticky for subsequent actions.

| Option             | Type | Default | Description                                         |
| ------------------ | ---- | ------- | --------------------------------------------------- |
| `enabled`          | bool | `true`  | Enable sticky modifiers                             |
| `tap_max_duration` | int  | `300`   | Max hold (ms) for tap detection (0 = always toggle) |

### UI

| Option               | Type   | Default | Description                            |
| -------------------- | ------ | ------- | -------------------------------------- |
| `font_size`          | int    | `10`    | Font size                              |
| `font_family`        | string | `""`    | Font family; [generic aliases](CROSS_PLATFORM.md#capability-matrix) accepted, empty among them — it asks for the platform's sans family |
| `background_color`   | color  | derived | Background with alpha                  |
| `text_color`         | color  | derived | Text color                             |
| `border_color`       | color  | derived | Border color                           |
| `border_width`       | int    | `1`     | Border width                           |
| `padding_x`          | int    | `-1`    | Horizontal padding (-1 = auto)         |
| `padding_y`          | int    | `-1`    | Vertical padding (-1 = auto)           |
| `border_radius`      | int    | `-1`    | Corner radius (-1 = auto)              |
| `indicator_x_offset` | int    | `-40`   | X offset from cursor (negative = left) |
| `indicator_y_offset` | int    | `20`    | Y offset from cursor (down)            |

```toml
[sticky_modifiers]
enabled = true
tap_max_duration = 300

[sticky_modifiers.ui]
font_size = 10
indicator_x_offset = -40
indicator_y_offset = 20
```

On Linux, the indicator renders the symbols `❖⇧⌥⌃`. If they appear as `[][][][]`, set `font_family` to a font with those glyphs.

---

## [smooth_cursor]

Animates cursor movement between positions. Supported on macOS and Linux
(X11, Wayland wlroots/KDE); Windows falls back to instant movement.

| Option               | Type  | Default | Description                        |
| -------------------- | ----- | ------- | ---------------------------------- |
| `move_mouse_enabled` | bool  | `false` | Enable animated mouse movement     |
| `steps`              | int   | `10`    | Number of animation steps          |
| `max_duration`       | int   | `200`   | Max animation duration in ms       |
| `duration_per_pixel` | float | `0.1`   | Ms per pixel for adaptive duration |
| `relative_movement_duration` | int | `50` | Fixed duration per relative move in ms (>= 10) |

```toml
[smooth_cursor]
move_mouse_enabled = false
steps = 10
max_duration = 200
duration_per_pixel = 0.1
relative_movement_duration = 50
```

`relative_movement_duration` applies to relative (keyboard-driven) movement —
`move_mouse_relative`, i.e. the default hjkl bindings. Jumps derive their
duration from the distance (`duration_per_pixel`), which yields constant
velocity — fine for a one-shot jump, but under held-key repeat it makes a 50px
step no faster than a 10px one. Relative moves therefore animate with a
**fixed duration per move**, so cursor speed scales with the delta. While an
animation is still in flight, a new relative move extends its endpoint instead
of restarting from the current position, so no part of a delta is lost under
key repeat.

Relative animation is supported on macOS and Linux (X11 and the Wayland
wlroots/KDE backends; on wlroots the animation is applied as native relative
motion, never as position warps). On Windows relative moves always warp
instantly. It composes with
[`held_repeat` acceleration](#held_repeat): the accelerated deltas pass
through unchanged, and since animation speed scales with the delta, the cursor
speeds up over the ramp exactly as without animation — just smoothly.

---

## [smooth_scroll]

Splits scroll deltas into chunked ease-out events for visual feedback.

**Platforms:** macOS and Linux. Windows falls back to instant scrolling, and on
Linux X11 a step can be no finer than a wheel notch — what that means for a
given scroll is in the
[capability matrix](CROSS_PLATFORM.md#capability-matrix).

Neru sends the same distance whether the animation is on or off. On Wayland it
sends it in a different *currency*: a continuous delta rather than a count of
wheel notches, and an application is free to scale those two differently. So
turning the animation on can change how far a `scroll_down` reaches in a given
application, and `scroll.scroll_step` is the setting to trim if it does.

The same holds for a key held down under [`[held_repeat]`](#held_repeat). A
repeat tick preempts the animation still in flight, and folds whatever that
animation had not yet sent into its own, so N repeats travel as far as N
discrete presses — on X11, within a wheel notch per repeat, since nothing there
can express a fraction of one. A binding with a *different* modifier set still
cancels outright: a plain `scroll_down` arriving mid-zoom finishes unmodified,
and the zoom's remaining distance is dropped rather than sent as a plain scroll.

| Option               | Type  | Default | Description                        |
| -------------------- | ----- | ------- | ---------------------------------- |
| `enabled`            | bool  | `false` | Enable smooth scrolling            |
| `steps`              | int   | `20`    | Number of animation steps          |
| `max_duration`       | int   | `180`   | Max animation duration in ms       |
| `duration_per_pixel` | float | `1.0`   | Ms per pixel for adaptive duration |

```toml
[smooth_scroll]
enabled = false
steps = 20
max_duration = 180
duration_per_pixel = 1.0
```

---

## [held_repeat]

Repeatedly dispatches scroll, page, and `move_cell` actions while the key is held, with a configurable initial delay and repeat interval, and glides the cursor for a held `move_mouse_relative` (see [Glide](#glide)). Disable held-key behaviour entirely by setting `enabled = false`.

| Option                 | Type     | Default                   | Description                                      |
| ---------------------- | -------- | ------------------------- | ------------------------------------------------ |
| `enabled`              | bool     | `false`                   | Master toggle for held-key repeat and the glide  |
| `initial_delay_ms`     | int      | `50`                      | Delay before first repeat fires (ms)             |
| `interval_ms`          | int      | `50`                      | Interval between subsequent repeats (ms)         |
| `accel_enabled`        | bool     | `false`                   | Ramp the glide's speed up the longer the key stays held |
| `accel_ramp_ms`        | int      | `500`                     | Hold time to reach `accel_max_multiplier` (ms)   |
| `accel_max_multiplier` | float    | `4.0`                     | Speed multiplier at full ramp                    |
| `accel_targets`        | string[] | `["move_mouse_relative"]` | Action names eligible for acceleration           |

```toml
[held_repeat]
enabled = false
initial_delay_ms = 50
interval_ms = 50
accel_enabled = false
accel_ramp_ms = 500
accel_max_multiplier = 4.0
accel_targets = ["move_mouse_relative"]
```

### Glide

With `enabled = true`, holding a key bound to a lone `move_mouse_relative` does
not repeat its step. The key contributes a direction, read from the sign of its
`--dx`/`--dy`, to one continuous glide: the cursor starts moving the moment the
key goes down at the binding's step per `interval_ms` (the default 10px step
every 50ms is 200px/s) and keeps that speed until release. Two keys held
together move diagonally at the same speed as one key moves straight, and
opposite keys cancel; when held keys have different steps the larger one sets
the speed. A short tap still travels about one step.

The glide runs on its own 10ms tick from the moment a key goes down until the
last direction key is released, with a subpixel position so slow speeds stay
smooth, so `initial_delay_ms` does not apply to it. It works across monitors,
and a click or any other action fired while moving acts at the cursor's live
position. Scroll, page and `move_cell` keep their fixed step and repeat on
`interval_ms` as before.

### Acceleration

With `accel_enabled = true`, the glide's speed ramps linearly to
`accel_max_multiplier` times the binding's speed over `accel_ramp_ms`, then holds
there until release. With the values above a 10px binding is at 500px/s after
250ms and 800px/s from 500ms onward. Leave it off for a glide at constant speed.

Acceleration only applies to actions listed in `accel_targets`. Only
`move_mouse_relative` glides, so that is currently the sole valid entry:
anything else is a config error rather than a binding that silently never
accelerates. An empty `accel_targets` while `accel_enabled = true` is rejected
for the same reason.

`accel_enabled = true` while `enabled = false` is not an error: acceleration
shapes a glide, so with no glide to shape it simply does nothing, and refusing
the file would stop you turning held-key behaviour off without also unwinding
the settings under it. `neru config validate` reports it as a warning instead.

---

## [systray]

The system tray icon and its menu. Changing `enabled` requires a full daemon
restart; `neru config reload` does not create or remove the icon.

| Option    | Type | Default | Description                |
| --------- | ---- | ------- | -------------------------- |
| `enabled` | bool | `true`  | Show/hide the systray icon |

> Changing this option requires a daemon restart.

---

## [logging]

Log level, destination, and rotation. File paths per platform are listed in
[TROUBLESHOOTING.md](TROUBLESHOOTING.md#log-file-locations).

| Option                 | Type   | Default  | Description                                       |
| ---------------------- | ------ | -------- | ------------------------------------------------- |
| `log_level`            | string | `"info"` | Level: `debug`, `info`, `warn`, `error`           |
| `log_file`             | string | `""`     | Custom log file path (empty = default location)   |
| `disable_file_logging` | bool   | `true`   | Console only (no file); file logs always use JSON |
| `max_file_size`        | int    | `10`     | MB before rotation                                |
| `max_backups`          | int    | `5`      | Old log files to keep                             |
| `max_age`              | int    | `30`     | Days to retain old logs                           |

When `log_file` is empty, Neru writes to a platform default location:

| Platform | Default log file                  |
| -------- | --------------------------------- |
| macOS    | `~/Library/Logs/neru/app.log`     |
| Linux    | `~/.local/state/neru/log/app.log` |
| Windows  | `%LOCALAPPDATA%\neru\log\app.log` |

At the default `info` level, logs focus on lifecycle, configuration, mode activation, and actionable operational events. Use `debug` temporarily when investigating key routing, hint generation, accessibility collection, overlay redraws, or IPC action flow. Debug logs intentionally avoid typed UI text, feed-key payloads, exec output, and full configuration values.

---

Use `neru doctor` and runtime logs for troubleshooting configuration issues.
