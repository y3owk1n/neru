# Configuration Guide

Neru uses TOML for configuration. No config file is required — Neru works out of the box with sensible defaults. When you're ready to customise, only define the options you want to change; all other defaults are preserved automatically.

> **Throughout this guide**, "the daemon" refers to the background process started with `neru launch`.

---

## Table of Contents

- [Quick Start](#quick-start)
- [Config File Location](#config-file-location)
- [Managing Your Config](#managing-your-config)
- [Hotkeys](#hotkeys)
- [Per-Mode Custom Hotkeys](#per-mode-custom-hotkeys)
- [Action Commands and Primitives](#action-commands-and-primitives)
- [General Settings](#general-settings)
- [Hint Mode](#hint-mode)
- [Grid Mode](#grid-mode)
- [Recursive Grid Mode](#recursive-grid-mode)
- [Scroll Mode](#scroll-mode)
- [Virtual Pointer](#virtual-pointer)
- [Mode Indicator](#mode-indicator)
- [Sticky Modifiers](#sticky-modifiers)
- [Theme Palette](#theme-palette)
- [Color Format](#color-format)
- [Smooth Cursor](#smooth-cursor)
- [Keyboard Layout](#keyboard-layout)
- [Font Configuration](#font-configuration)
- [Systray](#systray)
- [Logging](#logging)

---

## Quick Start

A minimal config for most users — copy this as a starting point:

```toml
[hints.hotkeys]
"Shift+L" = ["action left_click", "idle"]

[scroll]
scroll_step = 50
```

Generate a fully-commented starter file:

```bash
neru config init                          # Creates ~/.config/neru/config.toml
neru config init --force                  # Overwrite an existing file
neru config init -c /path/to/config.toml  # Create at a custom path
```

---

## Config File Location

> **Recommended location:** `~/.config/neru/config.toml`

Config files are loaded in this order (highest to lowest priority):

1. `$XDG_CONFIG_HOME/neru/config.toml` (if `XDG_CONFIG_HOME` is set)
2. `~/.config/neru/config.toml` _(recommended)_
3. `~/.neru.toml` _(legacy)_
4. `neru.toml` _(current directory)_
5. `config.toml` _(current directory)_

Use `--config` (or `-c`) to specify a custom path at launch:

```bash
neru launch -c /path/to/config.toml
```

---

## Managing Your Config

```bash
neru config validate    # Check for errors (no daemon needed)
neru config reload      # Apply changes to a running daemon
neru config dump        # Print the loaded configuration as JSON (daemon required)
```

To restart the daemon instead: `pkill neru && neru launch`

See [CLI.md — Configuration Management](CLI.md#configuration-management) for full flag documentation.

---

## Hotkeys

Global hotkeys trigger Neru navigation modes from anywhere on screen.

> [!TIP]
> User-defined hotkeys are **merged on top of defaults**. You only need to define the bindings you want to add or change — all other defaults are preserved.

### Merging behavior

| Scenario                              | Result                                                |
| ------------------------------------- | ----------------------------------------------------- |
| `[hotkeys]` section absent            | All defaults are used                                 |
| `[hotkeys]` section present but empty | All hotkeys disabled (for external daemons like skhd) |
| `[hotkeys]` with entries              | Entries are merged on top of defaults                 |

To **remove** a single default binding without affecting the rest, use the `__disabled__` sentinel:

```toml
[hotkeys]
"Cmd+Shift+S" = "__disabled__"   # removes the default scroll hotkey
"Ctrl+Space"  = "hints"          # adds a new binding; other defaults remain
```

### Syntax

**Format:** `"Modifier1+Modifier2+Key" = "action"`

**Modifiers:** `Cmd`, `Ctrl`, `Alt` / `Option`, `Shift`, `Primary` (case-insensitive).

`Primary` is a cross-platform alias that resolves to `Cmd` on macOS and `Ctrl` on Linux/Windows. Use this for hotkeys you want to work identically across platforms without changes.

**Actions can be:**

- A mode command: `"hints"`, `"grid"`, `"scroll"`, `"recursive_grid"`, `"idle"`
- An IPC action command: `"action left_click"`, `"action move_mouse_relative --dx=0 --dy=-10"`
- A shell command: `"exec open -a Terminal"`

### Multiple actions per hotkey

Bind multiple actions to a single hotkey using an array. Actions are executed sequentially; if one fails, the error is logged but remaining actions continue.

```toml
[hotkeys]
"PageUp" = ["action go_top", "action page_down"]
"Cmd+Shift+D" = ["hints", "exec echo 'hints activated'"]
```

Both `[hotkeys]` and `[<mode>.hotkeys]` support this array syntax.

---

## Per-Mode Custom Hotkeys

Define hotkeys that are only active while a specific mode is running.

> [!TIP]
> Like `[hotkeys]`, per-mode custom hotkeys are **merged on top of defaults**. You only need to define what you want to add, change, or remove.

### Merging behavior

| Scenario                                     | Result                              |
| -------------------------------------------- | ----------------------------------- |
| `[<mode>.hotkeys]` section absent            | All defaults for that mode are used |
| `[<mode>.hotkeys]` section present but empty | All bindings for that mode disabled |
| `[<mode>.hotkeys]` with entries              | Entries merged on top of defaults   |

To remove a single default binding, use `__disabled__`:

```toml
[scroll.hotkeys]
"h" = "__disabled__"             # removes default scroll_left on "h"
"x" = "action scroll_left"       # adds "x" for scroll_left; all other defaults remain
```

### Syntax

```toml
[hints.hotkeys]
"Escape" = "idle"
"Backspace" = "action backspace"
"Shift+L" = ["action left_click", "idle"]

[scroll.hotkeys]
"gg" = "action go_top"
"Cmd+Shift+T" = "exec open -a Terminal"
```

### Supported actions

All actions from `[hotkeys]` work here, plus these mode-specific ones:

- Mode commands: `idle`, `hints`, `grid`, `recursive_grid`, `scroll`
- Action subcommands: `action left_click`, `action scroll_down`, `action reset`, `action backspace`, `action wait_for_mode_exit`, `action save_cursor_pos`, `action restore_cursor_pos`
- Root toggle commands: `toggle-screen-share`, `toggle-cursor-follow-selection`
- Shell commands: `exec ...`

### Per-App Hint Hotkey Overrides

`[[hints.app_configs]]` overrides `[hints.hotkeys]` bindings for a specific app bundle ID. App hotkeys are merged on top of `[hints.hotkeys]`; missing keys inherit the global binding; `__disabled__` removes an inherited binding for that app only.

```toml
[hints.hotkeys]
"Return" = ["action left_click", "hints"]

[[hints.app_configs]]
bundle_id = "net.imput.helium"

[hints.app_configs.hotkeys]
"Return" = ["action left_click", "exec sleep 0.8", "hints"]
"Shift+L" = "__disabled__"
```

This is useful for apps that need different hint follow-up behavior, such as browser-like shells that need a short pause before hints are refreshed.

### Priority order

When a key is pressed inside a mode, Neru checks in this order:

1. Modifier toggle
2. Mode custom hotkeys (`<mode>.hotkeys`)
3. Mode-specific keys (hint/grid/recursive-grid character input)

### Multi-key sequences

Two-letter alphabetic sequences are supported in `hotkeys`:

```toml
[scroll.hotkeys]
"gg" = "action go_top"
```

Sequence timeout is `500ms`.

---

## Action Commands and Primitives

All one-shot actions are exposed through `action` subcommands and are valid in custom hotkeys as `"action <name>"`.

### Available actions

| Category    | Actions                                                       |
| ----------- | ------------------------------------------------------------- |
| Click       | `left_click`, `right_click`, `middle_click`                   |
| Mouse       | `mouse_down`, `mouse_up`, `move_mouse`, `move_mouse_relative` |
| Scroll      | `scroll_up`, `scroll_down`, `scroll_left`, `scroll_right`     |
| Page        | `page_up`, `page_down`, `go_top`, `go_bottom`                 |
| Mode        | `reset`, `backspace`                                          |
| Composition | `wait_for_mode_exit`, `save_cursor_pos`, `restore_cursor_pos` |

> [!NOTE]
> `reset`, `backspace`, `wait_for_mode_exit`, and `save_cursor_pos` / `restore_cursor_pos` are not valid mode `--action` values. Use them as `neru action ...` or in hotkeys as `"action ..."`. `toggle-cursor-follow-selection` is a root command.

> [!TIP]
> Point-targeted actions prefer the current mode selection by default. Use the `--bare` flag (e.g. `"action left_click --bare"`) to ignore the selection and target the current cursor position instead.

### Composition example

```toml
[hints.hotkeys]
"Enter" = ["action save_cursor_pos", "idle", "action wait_for_mode_exit", "action restore_cursor_pos"]
"Return" = ["action left_click", "exec sleep 0.5", "hints"]
```

---

## General Settings

Core behaviour settings that affect all Neru functionality.

| Option                                 | Type   | Default | Description                                            |
| -------------------------------------- | ------ | ------- | ------------------------------------------------------ |
| `excluded_apps`                        | array  | `[]`    | Bundle IDs where Neru won't activate                   |
| `accessibility_check_on_start`         | bool   | `true`  | Verify accessibility permissions on launch             |
| `kb_layout_to_use`                     | string | `""`    | Optional InputSourceID for layout mapping              |
| `hide_overlay_in_screen_share`         | bool   | `false` | Hide overlay in screen sharing apps                    |
| `passthrough_unbounded_keys`           | bool   | `false` | Let unbound modifier shortcuts pass through            |
| `should_exit_after_passthrough`        | bool   | `false` | Exit current mode after passthrough                    |
| `passthrough_unbounded_keys_blacklist` | array  | `[]`    | Shortcuts to keep consumed when passthrough is enabled |

---

## Hint Mode

Hint mode uses macOS Accessibility APIs to identify clickable UI elements and overlay short labels on them.

### Options

| Option                             | Type   | Default       | Description                                          |
| ---------------------------------- | ------ | ------------- | ---------------------------------------------------- |
| `enabled`                          | bool   | `true`        | Enable/disable hints mode                            |
| `hint_characters`                  | string | `"asdfghjkl"` | Characters used for labels                           |
| `max_depth`                        | int    | `50`          | Max accessibility tree depth (0 = unlimited)         |
| `parallel_threshold`               | int    | `20`          | Min children to trigger parallel tree building (≥ 1) |
| `include_menubar_hints`            | bool   | `false`       | Show hints on menubar items                          |
| `include_dock_hints`               | bool   | `false`       | Show hints on Dock items                             |
| `include_nc_hints`                 | bool   | `false`       | Show hints in Notification Center                    |
| `include_stage_manager_hints`      | bool   | `false`       | Show hints in Stage Manager                          |
| `detect_mission_control`           | bool   | `false`       | Auto-disable hints when in Mission Control           |
| `additional_menubar_hints_targets` | array  | see defaults  | Extra menubar bundle IDs                             |
| `clickable_roles`                  | array  | see defaults  | AX roles that generate hints                         |
| `ignore_clickable_check`           | bool   | `false`       | Skip clickability heuristic                          |

### Additional Accessibility Support

Enable framework-specific accessibility support for improved hint detection in Electron, Chromium, and Firefox apps:

```toml
[hints.additional_ax_support]
enable = true
additional_electron_bundles = []
additional_chromium_bundles = []
additional_firefox_bundles = []
```

To find a bundle ID:

```bash
osascript -e 'id of app "Safari"'
```

### UI Options

| Option               | Type   | Default | Description                          |
| -------------------- | ------ | ------- | ------------------------------------ |
| `font_size`          | int    | `10`    | Font size in points                  |
| `font_family`        | string | `""`    | Font family (empty = system default) |
| `border_radius`      | int    | `-1`    | Corner radius (-1 = auto)            |
| `padding_x`          | int    | `-1`    | Horizontal padding (-1 = auto)       |
| `padding_y`          | int    | `-1`    | Vertical padding (-1 = auto)         |
| `border_width`       | int    | `1`     | Border width in pixels               |
| `background_color`   | color  | derived | Background color with alpha          |
| `text_color`         | color  | derived | Text color                           |
| `matched_text_color` | color  | derived | Text color for matched characters    |
| `border_color`       | color  | derived | Border color                         |

```toml
[hints.ui]
font_size = 10
font_family = ""
border_radius = -1
padding_x = -1
padding_y = -1
border_width = 1
background_color = "#F2465FBC"
text_color = "#17327A"
matched_text_color = "#0B2377"
border_color = "#0B2377"
```

---

## Grid Mode

Grid mode divides the screen into a labelled coordinate grid.

### Options

| Option              | Type   | Default              | Description                   |
| ------------------- | ------ | -------------------- | ----------------------------- |
| `enabled`           | bool   | `true`               | Enable/disable grid mode      |
| `characters`        | string | see default config   | Primary grid labels           |
| `sublayer_keys`     | string | same as `characters` | Subgrid labels                |
| `live_match_update` | bool   | `true`               | Highlight cells as you type   |
| `hide_unmatched`    | bool   | `true`               | Hide non-matching cells       |
| `prewarm_enabled`   | bool   | `true`               | Pre-compute grid on startup   |
| `enable_gc`         | bool   | `false`              | Periodic memory cleanup       |
| `row_labels`        | string | `""`                 | Optional custom row labels    |
| `col_labels`        | string | `""`                 | Optional custom column labels |

Runtime cursor behavior is chosen per invocation rather than in config:

```toml
[hotkeys]
"Cmd+Shift+G" = "grid --cursor-selection-mode follow"
"Cmd+Alt+G"   = "grid --cursor-selection-mode hold"
```

Default grid hotkeys include ``"`" = "toggle-cursor-follow-selection"`` so you can flip cursor follow behavior mid-session.

### UI Options

| Option                     | Type   | Default | Description                          |
| -------------------------- | ------ | ------- | ------------------------------------ |
| `font_size`                | int    | `10`    | Font size in points                  |
| `font_family`              | string | `""`    | Font family (empty = system default) |
| `border_width`             | int    | `1`     | Border width in pixels               |
| `background_color`         | color  | derived | Cell background color with alpha     |
| `text_color`               | color  | derived | Label text color                     |
| `matched_text_color`       | color  | derived | Text color for matched cells         |
| `matched_background_color` | color  | derived | Background color for matched cells   |
| `matched_border_color`     | color  | derived | Border color for matched cells       |
| `border_color`             | color  | derived | Default border color                 |

```toml
[grid.ui]
font_size = 10
font_family = ""
border_width = 1
background_color = "#B3465FBC"
text_color = "#E8EEFF"
matched_text_color = "#F8FAFF"
matched_background_color = "#465FBC"
matched_border_color = "#0B2377"
border_color = "#99465FBC"
```

---

## Recursive Grid Mode

Recursive grid narrows the active area with each keypress for precise cursor placement.

### Options

| Option            | Type   | Default  | Description                                      |
| ----------------- | ------ | -------- | ------------------------------------------------ |
| `enabled`         | bool   | `true`   | Enable/disable mode                              |
| `grid_cols`       | int    | `2`      | Number of columns (≥ 1; total cells must be ≥ 2) |
| `grid_rows`       | int    | `2`      | Number of rows (≥ 1; total cells must be ≥ 2)    |
| `keys`            | string | `"uijk"` | Cell selection keys                              |
| `min_size_width`  | int    | `25`     | Minimum cell width in pixels                     |
| `min_size_height` | int    | `25`     | Minimum cell height in pixels                    |
| `max_depth`       | int    | `10`     | Maximum recursion levels (1–20)                  |
| `layers`          | array  | `[]`     | Optional per-depth layout overrides              |

Like `grid`, recursive-grid uses `--cursor-selection-mode follow|hold` at launch time. Default hotkeys also include ``"`" = "toggle-cursor-follow-selection"``.

### Animation Options

| Option        | Type | Default | Description                                               |
| ------------- | ---- | ------- | --------------------------------------------------------- |
| `enabled`     | bool | `false` | Opt in to native depth transitions on supported platforms |
| `duration_ms` | int  | `180`   | Depth transition duration in milliseconds                 |

```toml
[recursive_grid.animation]
enabled = false
duration_ms = 180
```

### UI Options

| Option                                | Type   | Default | Description                          |
| ------------------------------------- | ------ | ------- | ------------------------------------ |
| `font_size`                           | int    | `10`    | Font size in points                  |
| `font_family`                         | string | `""`    | Font family (empty = system default) |
| `line_width`                          | int    | `1`     | Grid line width in pixels            |
| `line_color`                          | color  | derived | Grid line color                      |
| `highlight_color`                     | color  | derived | Selected cell highlight color        |
| `text_color`                          | color  | derived | Label text color                     |
| `label_background`                    | bool   | `false` | Show background behind labels        |
| `label_background_color`              | color  | derived | Label background color               |
| `label_background_padding_x`          | int    | `-1`    | Horizontal label padding (-1 = auto) |
| `label_background_padding_y`          | int    | `-1`    | Vertical label padding (-1 = auto)   |
| `label_background_border_radius`      | int    | `-1`    | Label corner radius (-1 = auto)      |
| `label_background_border_width`       | int    | `1`     | Label border width                   |
| `sub_key_preview`                     | bool   | `false` | Show sub-key previews in cells       |
| `sub_key_preview_font_size`           | int    | `8`     | Sub-key preview font size            |
| `sub_key_preview_autohide_multiplier` | float  | `1.5`   | Autohide threshold multiplier        |
| `sub_key_preview_text_color`          | color  | derived | Sub-key preview text color           |

```toml
[recursive_grid.ui]
line_color = "#000000"
line_width = 1
highlight_color = "#465FBC"
text_color = "#FFFFFF"
font_size = 10
font_family = ""
label_background = false
```

---

## Scroll Mode

Scroll mode provides keyboard-driven scrolling.

### Options

| Option             | Type | Default   | Description                        |
| ------------------ | ---- | --------- | ---------------------------------- |
| `scroll_step`      | int  | `50`      | Pixels for line scroll actions     |
| `scroll_step_half` | int  | `500`     | Pixels for half-page actions       |
| `scroll_step_full` | int  | `1000000` | Pixels for top/bottom jump actions |

```toml
[scroll]
scroll_step = 50
scroll_step_half = 500
scroll_step_full = 1000000
```

Default scroll hotkeys:

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
"d"       = "action page_down"
```

---

## Virtual Pointer

When grid or recursive-grid runs in `--cursor-selection-mode hold`, Neru renders a small dot at the active selection so you can track the target while the real cursor stays still.

### Options

| Option    | Type | Default | Description                        |
| --------- | ---- | ------- | ---------------------------------- |
| `enabled` | bool | `true`  | Enable/disable the virtual pointer |

### UI Options

| Option  | Type  | Default       | Description          |
| ------- | ----- | ------------- | -------------------- |
| `size`  | int   | `3`           | Dot radius in points |
| `color` | color | Theme-derived | Dot color            |

```toml
[virtual_pointer]
enabled = true

[virtual_pointer.ui]
size = 4
color = { light = "#0B2377", dark = "#8FA2F0" }
```

---

## Mode Indicator

A floating label that follows the cursor and shows the current mode.

### Per-mode Options

| Option             | Type   | Default | Description                       |
| ------------------ | ------ | ------- | --------------------------------- |
| `enabled`          | bool   | varies  | Show/hide indicator for this mode |
| `text`             | string | varies  | Label shown in indicator          |
| `background_color` | color  | derived | Custom background color           |
| `text_color`       | color  | derived | Custom text color                 |
| `border_color`     | color  | derived | Custom border color               |

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
```

### UI Options

| Option               | Type   | Default | Description                             |
| -------------------- | ------ | ------- | --------------------------------------- |
| `font_size`          | int    | `10`    | Font size in points                     |
| `font_family`        | string | `""`    | Font family (empty = system default)    |
| `background_color`   | color  | derived | Background color with alpha             |
| `text_color`         | color  | derived | Text color                              |
| `border_color`       | color  | derived | Border color                            |
| `border_width`       | int    | `1`     | Border width in pixels                  |
| `padding_x`          | int    | `8`     | Horizontal padding                      |
| `padding_y`          | int    | `4`     | Vertical padding                        |
| `border_radius`      | int    | `4`     | Corner radius                           |
| `indicator_x_offset` | int    | `20`    | X offset from cursor (positive = right) |
| `indicator_y_offset` | int    | `20`    | Y offset from cursor (positive = down)  |

```toml
[mode_indicator.ui]
font_size = 10
font_family = ""
border_width = 1
padding_x = 8
padding_y = 4
border_radius = 4
indicator_x_offset = 20
indicator_y_offset = 20
```

---

## Sticky Modifiers

Tap modifiers inside a mode to make them sticky for subsequent actions.

### Options

| Option             | Type | Default | Description                                           |
| ------------------ | ---- | ------- | ----------------------------------------------------- |
| `enabled`          | bool | `true`  | Enable sticky modifiers                               |
| `tap_max_duration` | int  | `300`   | Max hold duration (ms) for tap detection (0 = always) |

### UI Options

| Option               | Type   | Default | Description                                      |
| -------------------- | ------ | ------- | ------------------------------------------------ |
| `font_size`          | int    | `10`    | Font size in points                              |
| `font_family`        | string | `""`    | Font family (empty = system default)             |
| `background_color`   | color  | derived | Background color with alpha                      |
| `text_color`         | color  | derived | Text color                                       |
| `border_color`       | color  | derived | Border color                                     |
| `border_width`       | int    | `1`     | Border width in pixels                           |
| `padding_x`          | int    | `-1`    | Horizontal padding (-1 = auto)                   |
| `padding_y`          | int    | `-1`    | Vertical padding (-1 = auto)                     |
| `border_radius`      | int    | `-1`    | Corner radius (-1 = auto)                        |
| `indicator_x_offset` | int    | `-40`   | X offset from cursor (negative = left of cursor) |
| `indicator_y_offset` | int    | `20`    | Y offset from cursor (positive = down)           |

```toml
[sticky_modifiers]
enabled = true
tap_max_duration = 300
```

---

## Theme Palette

Neru exposes a top-level `[theme]` palette. All built-in defaults for hints, grid, recursive grid, the virtual pointer, the mode indicator, and sticky modifiers are derived from these base colors with component-specific alpha applied automatically.

Use solid colors only in `[theme.light]` / `[theme.dark]` — use `#RGB` or `#RRGGBB` format without alpha.

| Key             | Role                                                        |
| --------------- | ----------------------------------------------------------- |
| `surface`       | Translucent fills, badges, and indicator backgrounds        |
| `accent`        | Borders, lines, and primary chrome                          |
| `accent_alt`    | Active/emphasis states, highlights, and the virtual pointer |
| `on_accent_alt` | Foreground text/icon color on top of `accent_alt` surfaces  |
| `text`          | Readable foreground text on `surface` and calm backgrounds  |

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

If you leave a component color unset, Neru derives it from `[theme]`. If you set a component color explicitly, that value wins for the specified variant.

---

## Color Format

> **Read this before customising any colors.** Color values appear throughout this guide — this section explains the format.

Neru colors use hex notation with optional alpha transparency.

### Supported formats

| Format      | Example     | Alpha | Description                         |
| ----------- | ----------- | ----- | ----------------------------------- |
| `#AARRGGBB` | `#FF000000` | Yes   | 8-char: Alpha + RGB _(recommended)_ |
| `#RRGGBB`   | `#FF0000`   | No    | 6-char: RGB only (fully opaque)     |
| `#RGB`      | `#F00`      | No    | 3-char shorthand                    |

### Alpha channel reference

| Opacity | Hex  | Common use                  |
| ------- | ---- | --------------------------- |
| 100%    | `FF` | Solid colors, high contrast |
| 95%     | `F2` | Hint labels (default)       |
| 70%     | `B3` | Grid cell backgrounds       |
| 60%     | `99` | Grid borders                |
| 30%     | `4D` | Subtle highlights           |
| 0%      | `00` | Invisible                   |

To calculate any alpha value: `round(opacity_fraction × 255)` → convert to hex.

### Light and dark mode

Colors can be a single string (same for both themes) or a dictionary with `light` and `dark` keys:

```toml
# Same color for both themes:
background_color = "#FF0000AA"

# Different colors per theme:
background_color = { light = "#FF0000AA", dark = "#00FF00AA" }
```

When a color is omitted or set to empty, Neru uses its built-in theme-aware defaults. Colors update in real time when you switch system themes.

---

## Smooth Cursor

| Option               | Type  | Default | Description                        |
| -------------------- | ----- | ------- | ---------------------------------- |
| `move_mouse_enabled` | bool  | `false` | Enable animated mouse movement     |
| `steps`              | int   | `10`    | Number of animation steps          |
| `max_duration`       | int   | `200`   | Max animation duration in ms       |
| `duration_per_pixel` | float | `0.1`   | Ms per pixel for adaptive duration |

```toml
[smooth_cursor]
move_mouse_enabled = false
steps = 10
max_duration = 200
duration_per_pixel = 0.1
```

---

## Keyboard Layout

Neru uses a reference keyboard layout for key translation so hotkeys and mode keys stay stable when you switch active input sources.

```toml
[general]
kb_layout_to_use = "com.apple.keylayout.ABC"
```

To find available layout IDs on macOS:

```bash
defaults read com.apple.HIToolbox AppleEnabledInputSources
```

---

## Font Configuration

Use `font_family` in any UI section (`hints.ui`, `grid.ui`, `recursive_grid.ui`, `mode_indicator.ui`, `sticky_modifiers.ui`). An empty string uses the system default.

---

## Systray

| Option    | Type | Default | Description                |
| --------- | ---- | ------- | -------------------------- |
| `enabled` | bool | `true`  | Show/hide the systray icon |

```toml
[systray]
enabled = true
```

> [!NOTE]
> Changing systray behavior requires a daemon restart.

---

## Logging

| Option                 | Type   | Default  | Description                                     |
| ---------------------- | ------ | -------- | ----------------------------------------------- |
| `log_level`            | string | `"info"` | Log level: `debug`, `info`, `warn`, `error`     |
| `log_file`             | string | `""`     | Custom log file path (empty = default location) |
| `structured_logging`   | bool   | `true`   | Enable structured JSON logging                  |
| `disable_file_logging` | bool   | `true`   | Disable file logging (stdout only)              |
| `max_file_size`        | int    | `10`     | Max log file size in MB before rotation         |
| `max_backups`          | int    | `5`      | Number of old log files to retain               |
| `max_age`              | int    | `30`     | Days to retain old log files                    |

```toml
[logging]
log_level = "info"
log_file = ""
structured_logging = true
disable_file_logging = true
max_file_size = 10
max_backups = 5
max_age = 30
```

Use `neru doctor` and runtime logs for troubleshooting configuration issues.
