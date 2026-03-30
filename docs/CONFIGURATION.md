# Configuration Guide

Neru uses TOML for configuration. This guide covers all available options with explanations, use cases, and examples.

> **Throughout this guide**, "the daemon" refers to the background process started with `neru launch`.

---

## Table of Contents

- [Configuration Overview](#configuration-overview)
- [Color Format](#color-format)
- [Starter Config](#starter-config)
- [Hotkeys](#hotkeys)
- [Per-Mode Custom Hotkeys](#per-mode-custom-hotkeys)
- [Keyboard Layout Requirements](#keyboard-layout-requirements)
- [General Settings](#general-settings)
- [Hint Mode](#hint-mode)
- [Grid Mode](#grid-mode)
- [Recursive Grid Mode](#recursive-grid-mode)
- [Virtual Pointer](#virtual-pointer)
- [Scroll Mode](#scroll-mode)
- [Action Commands and Primitives](#action-commands-and-primitives)
- [Mode Indicator](#mode-indicator)
- [Sticky Modifiers](#sticky-modifiers)
- [Smooth Cursor](#smooth-cursor)
- [Systray](#systray)
- [Font Configuration](#font-configuration)
- [Logging](#logging)

---

## Configuration Overview

> **Recommended location:** `~/.config/neru/config.toml`

Config files are loaded in this order (highest to lowest priority):

1. `$XDG_CONFIG_HOME/neru/config.toml` (if `XDG_CONFIG_HOME` is set)
2. `~/.config/neru/config.toml` _(recommended)_
3. `~/.neru.toml` _(legacy)_
4. `neru.toml` _(current directory)_
5. `config.toml` _(current directory)_

This search order is the same on macOS, Linux, and Windows.

### Creating a config file

Neru works out of the box with sensible defaults — no config file is required. When you're ready to customise, generate a fully-commented starter file:

```bash
neru config init                          # Creates ~/.config/neru/config.toml
neru config init --force                  # Overwrite an existing file
neru config init -c /path/to/config.toml  # Create at a custom path
```

You can also copy manually from [default-config.toml](../configs/default-config.toml).

### Managing your config

```bash
neru config validate    # Check for errors (no daemon needed)
neru config reload      # Apply changes to a running daemon
neru config dump        # Print the loaded configuration as JSON (daemon required)
```

To restart the daemon instead: `pkill neru && neru launch`

See [CLI.md — Configuration Management](CLI.md#configuration-management) for full flag documentation.

### Global flag

Use `--config` (or `-c`) when launching to specify a custom config file path:

```bash
neru launch -c /path/to/config.toml
```

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

### `#AARRGGBB` breakdown

| Position | Component | Range | Description          |
| -------- | --------- | ----- | -------------------- |
| 1–2      | AA        | 00–FF | Alpha (transparency) |
| 3–4      | RR        | 00–FF | Red                  |
| 5–6      | GG        | 00–FF | Green                |
| 7–8      | BB        | 00–FF | Blue                 |

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

Colors can be specified as either a single string (same for both themes) or a dictionary with `light` and `dark` keys:

```toml
# Same color for both themes:
[hints.ui]
background_color = "#FF0000AA"

# Different colors per theme:
[hints.ui]
background_color = { light = "#FF0000AA", dark = "#00FF00AA" }
```

When a color is omitted or set to empty, Neru uses its built-in theme-aware defaults. Colors update in real time when you switch system themes. Setting an explicit value always overrides the default for that variant.

---

## Starter Config

A minimal config for most users — copy this as a starting point:

```toml
[hints.hotkeys]
"Shift+L" = ["action left_click", "idle"]

[scroll]
scroll_step = 50
```

> [!NOTE]
> You only need to define the settings you want to change — all defaults are preserved automatically.

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

**Modifiers:** `Cmd`, `Ctrl`, `Alt` / `Option`, `Shift` (case-insensitive).

**Actions can be:**

- A mode command: `"hints"`, `"grid"`, `"scroll"`, `"recursive_grid"`, `"idle"`
- An IPC action command: `"action left_click"`, `"action move_mouse_relative --dx=0 --dy=-10"`
- A shell command: `"exec open -a Terminal"`

### Multiple actions per hotkey

You can bind multiple actions to a single hotkey by using an array:

```toml
[hotkeys]
"PageUp" = ["action go_top", "action page_down"]
"Cmd+Shift+D" = ["hints", "exec echo 'hints activated'"]
```

Actions are executed sequentially in order. If an action fails, the error is logged but remaining actions continue.

Both `[hotkeys]` and `[<mode>.hotkeys]` support this array syntax.

---

## Per-Mode Custom Hotkeys

Define hotkeys that are only active while a specific mode is running.

> [!TIP]
> Like `[hotkeys]`, per-mode custom hotkeys are **merged on top of defaults**. You only need to define what you want to add, change, or remove.

### Merging behavior

| Scenario                                            | Result                              |
| --------------------------------------------------- | ----------------------------------- |
| `[<mode>.hotkeys]` section absent            | All defaults for that mode are used |
| `[<mode>.hotkeys]` section present but empty | All bindings for that mode disabled |
| `[<mode>.hotkeys]` with entries              | Entries merged on top of defaults   |

To remove a single default binding, use `__disabled__`:

```toml
[scroll.hotkeys]
"h" = "__disabled__"             # removes default scroll_left on "h"
"x" = "action scroll_left"      # adds "x" for scroll_left; all other defaults remain
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

All actions from `[hotkeys]` work here, including:

- Mode commands: `idle`, `hints`, `grid`, `recursive_grid`, `scroll`
- Action subcommands: `action left_click`, `action scroll_down`, `action reset`, `action backspace`, `action wait_for_mode_exit`, `action save_cursor_pos`, `action restore_cursor_pos`
- Root toggle commands: `toggle-screen-share`, `toggle-cursor-follow-selection`
- Shell commands: `exec ...`

### Per-App Hint Hotkey Overrides

`[[hints.app_configs]]` can override the same `[hints.hotkeys]` bindings for a specific app bundle ID.

- App hotkeys are merged on top of `[hints.hotkeys]`
- Missing keys inherit the global hint binding
- `__disabled__` removes an inherited hint binding for that app only

```toml
[hints.hotkeys]
"Return" = ["action left_click", "hints"]

[[hints.app_configs]]
bundle_id = "net.imput.helium"

[hints.app_configs.hotkeys]
"Return" = ["action left_click", "exec sleep 0.8", "hints"]
"Shift+L" = "__disabled__"
```

This is useful for apps that need different hint follow-up behavior without adding special-purpose config fields. A common case is apps like Helium or browser-like shells that need a short pause before hints are refreshed.

### Priority order

When a key is pressed inside a mode, Neru checks in this order:

1. modifier toggle
2. mode custom hotkeys (`<mode>.hotkeys`)
3. mode-specific keys (hint/grid/recursive-grid character input)

### Multi-key sequences

Two-letter alphabetic sequences are supported in `hotkeys`:

```toml
[scroll.hotkeys]
"gg" = "action go_top"
```

Sequence timeout is `500ms`.

---

## Keyboard Layout Requirements

Neru uses a reference keyboard layout for key translation so hotkeys and mode keys stay stable even when you switch active input sources.

### `general.kb_layout_to_use` (optional)

```toml
[general]
kb_layout_to_use = "com.apple.keylayout.ABC"
```

To find available layout IDs on macOS:

```bash
defaults read com.apple.HIToolbox AppleEnabledInputSources
```

---

## General Settings

Core behaviour settings that affect all Neru functionality.

### Option reference

| Option                                 | Type   | Default | Description                                            |
| -------------------------------------- | ------ | ------- | ------------------------------------------------------ |
| `excluded_apps`                        | array  | `[]`    | Bundle IDs where Neru won't activate                   |
| `accessibility_check_on_start`         | bool   | `true`  | Verify accessibility permissions on launch             |
| `kb_layout_to_use`                     | string | `""`    | Optional InputSourceID for layout mapping              |
| `hide_overlay_in_screen_share`         | bool   | `false` | Hide overlay in screen sharing apps                    |
| `passthrough_unbounded_keys`           | bool   | `false` | Let unbound modifier shortcuts pass through            |
| `should_exit_after_passthrough`        | bool   | `false` | Exit current mode after passthrough                    |
| `passthrough_unbounded_keys_blacklist` | array  | `[]`    | Shortcuts to keep consumed when passthrough is enabled |

> [!NOTE]
> Legacy fields `restore_cursor_position`, `center_cursor_position`, and `mode_exit_keys` were removed. Exit/cursor behavior is now composed with custom hotkey action arrays.

---

## Hint Mode

Hint mode uses macOS Accessibility APIs to identify clickable UI elements and overlay short labels on them.

### Basic configuration

| Option                             | Type   | Default       | Description                                          |
| ---------------------------------- | ------ | ------------- | ---------------------------------------------------- |
| `enabled`                          | bool   | `true`        | Enable/disable hints mode                            |
| `hotkeys`                   | table  | `{}`          | Per-mode hotkeys                                     |
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

> [!NOTE]
> `auto_exit_actions`, `mode_exit_keys`, and `backspace_key` were removed. Use `hotkeys` arrays like `"Shift+L" = ["action left_click", "idle"]` and `"Backspace" = "action backspace"`.

---

## Grid Mode

Grid mode divides the screen into a labelled coordinate grid.

### Basic configuration

| Option              | Type   | Default              | Description                   |
| ------------------- | ------ | -------------------- | ----------------------------- |
| `enabled`           | bool   | `true`               | Enable/disable grid mode      |
| `hotkeys`    | table  | `{}`                 | Per-mode hotkeys              |
| `characters`        | string | see default config   | Primary grid labels           |
| `sublayer_keys`     | string | same as `characters` | Subgrid labels                |
| `live_match_update` | bool   | `true`               | Highlight cells as you type   |
| `hide_unmatched`    | bool   | `true`               | Hide non-matching cells       |
| `prewarm_enabled`   | bool   | `true`               | Pre-compute grid on startup   |
| `enable_gc`         | bool   | `false`              | Periodic memory cleanup       |
| `row_labels`        | string | `""`                 | Optional custom row labels    |
| `col_labels`        | string | `""`                 | Optional custom column labels |

> [!NOTE]
> `auto_exit_actions`, `mode_exit_keys`, `reset_key`, and `backspace_key` were removed. Use `hotkeys` (for example `"Space" = "action reset"`, `"Backspace" = "action backspace"`).

Default grid hotkeys also include ``"`" = "toggle-cursor-follow-selection"`` so you can flip between follow and hold behavior mid-session.

Runtime cursor behavior is now chosen per invocation instead of in config:

```toml
[hotkeys]
"Cmd+Shift+G" = "grid --cursor-selection-mode follow"
"Cmd+Alt+G" = "grid --cursor-selection-mode hold"
```

---

## Recursive Grid Mode

Recursive grid narrows the active area with each keypress for precise cursor placement.

### Basic configuration

| Option            | Type   | Default  | Description                         |
| ----------------- | ------ | -------- | ----------------------------------- |
| `enabled`         | bool   | `true`   | Enable/disable mode                 |
| `hotkeys`  | table  | `{}`     | Per-mode hotkeys                    |
| `grid_cols`       | int    | `2`      | Number of columns (≥ 2)             |
| `grid_rows`       | int    | `2`      | Number of rows (≥ 2)                |
| `keys`            | string | `"uijk"` | Cell selection keys                 |
| `min_size_width`  | int    | `25`     | Minimum cell width in pixels        |
| `min_size_height` | int    | `25`     | Minimum cell height in pixels       |
| `max_depth`       | int    | `10`     | Maximum recursion levels (1–20)     |
| `layers`          | array  | `[]`     | Optional per-depth layout overrides |

> [!NOTE]
> `auto_exit_actions`, `mode_exit_keys`, `reset_key`, and `backspace_key` were removed. Use `hotkeys` (for example `"Space" = "action reset"`, `"Backspace" = "action backspace"`).

Default recursive-grid hotkeys also include ``"`" = "toggle-cursor-follow-selection"`` for toggling cursor follow behavior without leaving the mode.

Like `grid`, recursive-grid uses `--cursor-selection-mode follow|hold` at launch time instead of a persistent config field.

---

## Virtual Pointer

When grid or recursive-grid runs in `--cursor-selection-mode hold`, Neru can render a small virtual pointer dot at the active selection so you can keep track of the target while the real cursor stays still.

### Basic configuration

| Option | Type | Default | Description |
| ------ | ---- | ------- | ----------- |
| `enabled` | bool | `true` | Enable/disable the virtual pointer |

### UI configuration

| Option | Type | Default | Description |
| ------ | ---- | ------- | ----------- |
| `size` | int | `3` | Dot radius in points |
| `color_light` | string | `"#FF007A9E"` | Light-mode dot color |
| `color_dark` | string | `"#FF00CFCF"` | Dark-mode dot color |

Example:

```toml
[virtual_pointer]
enabled = true

[virtual_pointer.ui]
size = 4
color_light = "#FF007A9E"
color_dark = "#FF00CFCF"
```

---

## Scroll Mode

Scroll mode provides keyboard-driven scrolling behavior.

### Basic configuration

| Option             | Type  | Default   | Description                             |
| ------------------ | ----- | --------- | --------------------------------------- |
| `scroll_step`      | int   | `50`      | Pixels for line scroll actions          |
| `scroll_step_half` | int   | `500`     | Pixels for half-page actions            |
| `scroll_step_full` | int   | `1000000` | Pixels for top/bottom jump actions      |
| `hotkeys`   | table | `{}`      | Per-mode hotkeys (includes scroll keys) |

> [!NOTE]
> `auto_exit_actions`, `mode_exit_keys`, and `[scroll.key_bindings]` were removed. Bind scroll keys in `[scroll.hotkeys]` instead.

Example:

```toml
[scroll.hotkeys]
"Escape" = "idle"
"k" = "action scroll_up"
"j" = "action scroll_down"
"h" = "action scroll_left"
"l" = "action scroll_right"
"gg" = "action go_top"
"Shift+G" = "action go_bottom"
"u" = "action page_up"
"d" = "action page_down"
```

---

## Action Commands and Primitives

All one-shot actions are exposed through `action` subcommands and are valid in custom hotkeys as `"action <name>"`.

### Action subcommands

- `left_click`, `right_click`, `middle_click`
- `mouse_down`, `mouse_up`
- `move_mouse`, `move_mouse_relative`
- `scroll_up`, `scroll_down`, `scroll_left`, `scroll_right`
- `page_up`, `page_down`, `go_top`, `go_bottom`
- `reset`, `backspace`
- `wait_for_mode_exit`
- `save_cursor_pos`
- `restore_cursor_pos`

### New composition primitives

These are action subcommands used to compose advanced array hotkeys:

- `action save_cursor_pos`
- `action wait_for_mode_exit`
- `action restore_cursor_pos`

Example:

```toml
[hints.hotkeys]
"Enter" = ["action save_cursor_pos", "idle", "action wait_for_mode_exit", "action restore_cursor_pos"]
"Return" = ["action left_click", "exec sleep 0.5", "hints"]
```

> [!NOTE]
> `reset`, `backspace`, `wait_for_mode_exit`, and `save_cursor_pos` / `restore_cursor_pos` are not valid mode `--action` values. Use them as `neru action ...` or in hotkeys as `"action ..."`. `toggle-cursor-follow-selection` is a root command.

> [!TIP]
> Point-targeted actions prefer the current mode selection by default. Use `"action left_click --bare"`, `"action scroll_down --bare"`, or `"action move_mouse --bare"` when you want to ignore the selection and target the current cursor position instead.

---

## Mode Indicator

A floating label that follows the cursor and shows the current mode.

Per-mode and UI options remain unchanged from previous versions; refer to `configs/default-config.toml` for the full, current set.

---

## Sticky Modifiers

Tap modifiers inside a mode to make them sticky for subsequent actions.

Configuration remains:

```toml
[sticky_modifiers]
enabled = true
tap_max_duration = 300
```

UI customization remains under `[sticky_modifiers.ui]`.

---

## Smooth Cursor

Optional cursor animation settings remain under `[smooth_cursor]`.

```toml
[smooth_cursor]
move_mouse_enabled = false
steps = 10
max_duration = 200
duration_per_pixel = 0.1
```

---

## Systray

```toml
[systray]
enabled = true
```

> [!NOTE]
> Changing systray behavior requires daemon restart.

---

## Font Configuration

Use `font_family` in each UI section (`hints.ui`, `grid.ui`, `recursive_grid.ui`, `mode_indicator.ui`, `sticky_modifiers.ui`). Empty string uses system defaults.

---

## Logging

Logging settings remain under `[logging]`.

```toml
[logging]
log_level = "info"
structured_logging = true
disable_file_logging = true
```

Use `neru doctor` and runtime logs for troubleshooting configuration issues.
