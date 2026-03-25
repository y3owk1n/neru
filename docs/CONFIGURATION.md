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
- [Scroll Mode](#scroll-mode)
- [Mouse Movement Actions](#mouse-movement-actions)
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

All color options come in `_light` and `_dark` variants:

- `*_light` — used when macOS is in Light Mode
- `*_dark` — used when macOS is in Dark Mode

When a color is omitted or set to `""`, Neru uses its built-in theme-aware defaults. Colors update in real time when you switch system themes. Setting an explicit value always overrides the default for that variant.

```toml
[hints.ui]
background_color_light = "#FF0000AA"  # Custom for light mode
# background_color_dark is omitted — uses built-in dark default
```

### Common default values

```toml
# Hints (cyan / deep teal)
background_color_light = "#F200CFCF"    # Cyan, 95% opacity
background_color_dark  = "#F2007A9E"    # Deep teal, 95% opacity

# Grid cell background
background_color_light = "#9900B4D8"    # Sky blue, 60% opacity
background_color_dark  = "#99003554"    # Deep navy, 60% opacity

# Recursive grid lines
line_color_light = "#FF007A9E"          # Deep teal, fully opaque
line_color_dark  = "#FF00CFCF"          # Bright cyan, fully opaque
```

---

## Starter Config

A minimal config for most users — copy this as a starting point:

```toml
[hotkeys]
"Cmd+Shift+Space" = "hints --action left_click"
"Cmd+Shift+G"     = "grid"
"Cmd+Shift+C"     = "recursive_grid"
"Cmd+Shift+S"     = "scroll"

[hints]
auto_exit_actions = ["left_click"]

[scroll]
scroll_step = 50
```

> [!NOTE]
> This config overrides all built-in hotkey defaults. All four default modes are included above — remove any you don't use, or add `--action left_click` to a mode to make it click automatically on selection.

---

## Hotkeys

Global hotkeys trigger Neru navigation modes from anywhere on screen.

> [!WARNING]
> Defining **any** custom hotkey **replaces ALL defaults**. You must explicitly define every hotkey you want to keep.
>
> The built-in defaults are:
>
> ```toml
> [hotkeys]
> "Cmd+Shift+Space" = "hints"
> "Cmd+Shift+G"     = "grid"
> "Cmd+Shift+C"     = "recursive_grid"
> "Cmd+Shift+S"     = "scroll"
> ```
>
> Copy these into your config and modify from there.

### Syntax

**Format:** `"Modifier1+Modifier2+Key" = "action"`

**Modifiers:** `Cmd`, `Ctrl`, `Alt` / `Option`, `Shift` (case-insensitive). Right/left-prefixed variants are accepted (e.g. `RightCmd`, `LeftShift`) — see note below.

**Actions can be:**

- A Neru command: `"hints"`, `"grid"`, `"scroll"`, `"recursive_grid"`
- A command with flags: `"hints --action left_click"`
- A shell command: `"exec open -a Terminal"`

> [!NOTE]
> Right/left modifier prefixes (`RightCmd`, `LeftShift`, etc.) are aliases — they map to the same modifier flag as the unprefixed form. The macOS Carbon API does not distinguish left vs right modifier keys. These prefixed names exist for readability when using key remappers like Karabiner-Elements.

### Common configurations

**With auto-click (recommended):**

```toml
[hotkeys]
"Cmd+Shift+Space" = "hints --action left_click"
"Cmd+Shift+G"     = "grid --action left_click"
"Cmd+Shift+C"     = "recursive_grid"
"Cmd+Shift+S"     = "scroll"
```

**Execute shell commands:**

```toml
[hotkeys]
"Cmd+Alt+T" = "exec open -a Terminal"
"Cmd+Alt+C" = "exec open -a 'Visual Studio Code'"
"Cmd+Alt+S" = "exec ~/scripts/screenshot.sh"
"Cmd+Alt+N" = "exec osascript -e 'display notification \"Hello!\" with title \"Neru\"'"
```

### Multiple actions per hotkey

You can bind multiple actions to a single hotkey by using an array:

```toml
[hotkeys]
"PageUp" = ["action go_top", "action scroll_down"]
"Cmd+Shift+D" = ["hints", "exec echo 'hints activated'"]
```

Actions are executed sequentially in order. If an action fails, the error is logged but the remaining actions still run. This is useful when you want to perform multiple operations with a single hotkey, such as scrolling and then performing an action.

> [!WARNING]
> Shell commands (`exec …`) **block** until they finish (or the 30-second timeout expires). In a multi-action binding like `["exec sleep 10", "hints"]`, the `hints` action won't run until the shell command completes. Place `exec` actions last when possible, or run long-running commands in the background with `"exec my-script &"`.

Both `[hotkeys]` and `[<mode>.custom_hotkeys]` support this array syntax.

### Disabling all hotkeys

To disable **all** built-in hotkeys (e.g. when using an external hotkey daemon like skhd), provide an empty `[hotkeys]` section:

```toml
[hotkeys]
# No bindings — all defaults are cleared.
# Trigger modes via CLI: neru hints, neru grid, etc.
```

### Alternative: external hotkey manager

For more complex setups, use [skhd](https://github.com/koekeishiya/skhd) or [Karabiner-Elements](https://karabiner-elements.pqrs.org/):

```bash
# ~/.config/skhd/skhdrc
ctrl - f : neru hints
ctrl - g : neru grid
ctrl - r : neru hints --action right_click
```

---

## Per-Mode Custom Hotkeys

Define hotkeys that are only active while a specific mode is running. These use the **exact same action syntax** as `[hotkeys]` but are processed through the event tap instead of the Carbon API — they only fire while the mode's overlay is active.

### When to use

- Execute shell commands or IPC actions without leaving a mode
- Trigger `action move_mouse --center` to re-center the cursor mid-mode
- Run scripts that interact with the focused app while hints/grid/scroll is active

### Syntax

Add a `[<mode>.custom_hotkeys]` table to any mode section. The key format and action format are identical to `[hotkeys]`.

```toml
[recursive_grid.custom_hotkeys]
"Cmd+Shift+G" = "action move_mouse --center"
"Cmd+Shift+L" = "exec bash /path/script.sh"

[hints.custom_hotkeys]
"Cmd+Shift+R" = "exec open -a Safari"

[grid.custom_hotkeys]
"Ctrl+C"      = "action move_mouse --center"

[scroll.custom_hotkeys]
"Cmd+Shift+T" = "exec open -a Terminal"
```

### Supported actions

All actions from `[hotkeys]` work here:

| Action type     | Example                                    | Description                 |
| --------------- | ------------------------------------------ | --------------------------- |
| Mode activation | `"hints"`, `"grid"`, `"scroll"`            | Switch to another mode      |
| IPC command     | `"action move_mouse --center"`             | Execute an IPC action       |
| Scroll action   | `"action scroll_down"`                     | Scroll at cursor position   |
| Shell command   | `"exec bash /path/script.sh"`              | Run a shell command         |
| With flags      | `"action move_mouse --center --monitor 2"` | IPC action with extra flags |

### Priority order

When a key is pressed inside a mode, Neru checks in this order:

1. **Exit keys** (`general.mode_exit_keys` + per-mode `mode_exit_keys`)
2. **Custom hotkeys** (`<mode>.custom_hotkeys`)
3. **Mode-specific keys** (hint characters, grid characters, scroll bindings, action key bindings)
   Custom hotkeys override mode-specific key handling but never override exit keys.

### How it differs from `[hotkeys]`

| Aspect        | `[hotkeys]`                   | `[<mode>.custom_hotkeys]`                  |
| ------------- | ----------------------------- | ------------------------------------------ |
| Registration  | Carbon API (system-wide)      | Event tap (mode-active only)               |
| When active   | Always (when Neru is enabled) | Only while the specific mode is active     |
| Excluded apps | Unregistered in excluded apps | N/A — mode can't activate in excluded apps |
| Scope         | Global across all modes       | Scoped to one mode                         |

### Validation

- Each key must be a valid hotkey format (same rules as `[hotkeys]`)
- Each action must be non-empty
- Invalid entries cause a startup validation error

---

## Keyboard Layout Requirements

Neru uses a reference keyboard layout for key translation so hotkeys and mode keys stay stable even when you switch active input sources (for example EN/RU).

### `general.kb_layout_to_use` (optional)

#### macOS

Set an `InputSourceID` to force a specific layout:

```toml
[general]
kb_layout_to_use = "com.apple.keylayout.ABC"
```

To find available layout IDs:

```bash
defaults read com.apple.HIToolbox AppleEnabledInputSources
```

#### Linux / Windows

_(Planned)_ This setting will allow specifying a fallback XKB layout or Windows input locale.

### Automatic fallback

If `kb_layout_to_use` is not set, Neru selects the first available layout in this order:

1. **macOS:** `com.apple.keylayout.ABC`, `com.apple.keylayout.US`, or any English layout
2. **Linux:** _(Planned)_ `us`, `abc`, or current layout
3. **Windows:** _(Planned)_ `en-US` or current layout
4. Current active keyboard layout (last resort)

---

## General Settings

Core behaviour settings that affect all Neru functionality.

### Option reference

| Option                         | Type   | Default      | Description                                |
| ------------------------------ | ------ | ------------ | ------------------------------------------ |
| `excluded_apps`                | array  | `[]`         | Bundle IDs where Neru won't activate       |
| `accessibility_check_on_start` | bool   | `true`       | Verify accessibility permissions on launch |
| `restore_cursor_position`      | bool   | `false`      | Return cursor to pre-mode position on exit |
| `center_cursor_position`       | bool   | `false`      | Center cursor on current screen on exit    |
| `kb_layout_to_use`             | string | `""`         | Optional InputSourceID for layout mapping  |
| `mode_exit_keys`               | array  | `["Escape"]` | Keys that exit any active mode             |
| `hide_overlay_in_screen_share` | bool   | `false`      | Hide overlay in screen sharing apps        |

### Passthrough options

| Option                                 | Type  | Default | Description                                             |
| -------------------------------------- | ----- | ------- | ------------------------------------------------------- |
| `passthrough_unbounded_keys`           | bool  | `false` | Let unbound Cmd/Ctrl/Alt shortcuts reach macOS          |
| `should_exit_after_passthrough`        | bool  | `false` | Exit the current mode after a shortcut passes through   |
| `passthrough_unbounded_keys_blacklist` | array | `[]`    | Shortcuts to keep consumed while passthrough is enabled |

---

### excluded_apps

Prevent Neru from activating in specific applications.

```toml
[general]
excluded_apps = [
    "com.apple.Terminal",       # Terminal
    "com.googlecode.iterm2",    # iTerm2
    "com.microsoft.rdc.macos",  # Microsoft Remote Desktop
]
```

To find an app's bundle ID:

```bash
osascript -e 'id of app "Safari"'
# Output: com.apple.Safari
```

### accessibility_check_on_start

```toml
[general]
accessibility_check_on_start = true  # default
```

- `true`: Show an error if accessibility permissions are missing
- `false`: Skip the check (may cause silent runtime errors)

### restore_cursor_position

Return the cursor to its position before entering a navigation mode.

```toml
[general]
restore_cursor_position = false  # default
```

### center_cursor_position

Center the cursor on the current screen when exiting a navigation mode.

```toml
[general]
center_cursor_position = false  # default
```

> [!NOTE]
> `restore_cursor_position` and `center_cursor_position` are mutually exclusive — only one can be `true` at a time.

### mode_exit_keys

Keys that exit any active mode (hints, grid, scroll, recursive_grid). Additional mode-specific exit keys can be added in each mode's own section.

> [!NOTE]
> This array cannot be empty — at least one exit key must be defined.

```toml
[general]
mode_exit_keys = ["Escape"]           # default
# mode_exit_keys = ["Ctrl+C"]         # Vim-style
# mode_exit_keys = ["Escape", "q"]    # Multiple keys
```

**Valid key formats:** plain named keys (`Escape`, `Return`, `Tab`, `Space`, `Backspace`, `Delete`, `Home`, `End`, `PageUp`, `PageDown`, arrow keys, `F1`–`F20`), modifier combos (`Ctrl+C`, `Cmd+Q`), or single characters. Key name casing is flexible: `"escape"`, `"Escape"`, and `"ESCAPE"` all work.

### passthrough_unbounded_keys

Allow unbound Cmd/Ctrl/Alt shortcuts to keep reaching macOS while a mode is active.

```toml
[general]
passthrough_unbounded_keys = false  # default
```

- `true`: Neru passes through modifier shortcuts the active mode doesn't use (e.g. `Cmd+Tab`, `Cmd+Space`, `Cmd+W`)
- `false`: Neru consumes all key presses while a mode is active

> [!NOTE]
> Shortcuts that Neru actively uses are always consumed, even with passthrough enabled. For example, scroll bindings like `Cmd+Down` still work when configured in the active mode.

### should_exit_after_passthrough

Exit the active mode after an unbound modifier shortcut is passed through.

```toml
[general]
passthrough_unbounded_keys = true
should_exit_after_passthrough = false  # default
```

Useful when shortcuts like `Cmd+Tab` or `Cmd+Space` should both reach macOS and dismiss the overlay.

> [!NOTE]
> Only has an effect when `passthrough_unbounded_keys` is enabled.

### passthrough_unbounded_keys_blacklist

Shortcuts that should stay consumed by Neru even when passthrough is enabled.

```toml
[general]
passthrough_unbounded_keys = true
passthrough_unbounded_keys_blacklist = ["Cmd+W", "Cmd+Q"]
```

> [!NOTE]
> Each entry must include at least one of `Cmd`, `Ctrl`, `Alt`, or `Option` as a modifier. Plain keys or Shift-only combos are not valid here.

### hide_overlay_in_screen_share

Hide Neru overlays during screen sharing.

```toml
[general]
hide_overlay_in_screen_share = false  # default
```

> [!NOTE]
> Uses the macOS `NSWindow.sharingType` API. See [CLI.md — Screen Sharing](CLI.md#screen-sharing) for version compatibility details. You can also toggle this at runtime with `neru toggle-screen-share`.

---

## Hint Mode

Hint mode uses macOS Accessibility APIs to identify clickable UI elements and overlay short labels on them. Type a label to move the cursor to that element.

### When to use

- Clicking buttons, links, and menus in standard macOS applications
- Forms and dialogs with multiple clickable elements
- Any app with standard macOS UI elements

### Basic configuration

| Option                             | Type   | Default       | Description                                            |
| ---------------------------------- | ------ | ------------- | ------------------------------------------------------ |
| `enabled`                          | bool   | `true`        | Enable/disable hints mode                              |
| `auto_exit_actions`                | array  | `[]`          | Actions that auto-exit after execution                 |
| `mode_exit_keys`                   | array  | `[]`          | Additional keys that exit hints mode                   |
| custom_hotkeys                     | table  | {}            | Per-mode hotkeys (same syntax as [hotkeys])            |
| `hint_characters`                  | string | `"asdfghjkl"` | Characters used for labels                             |
| `backspace_key`                    | string | `"Backspace"` | Key for input correction                               |
| `mouse_action_refresh_delay`       | int    | `0`           | ms delay before refreshing hints after click (0–10000) |
| `max_depth`                        | int    | `50`          | Max accessibility tree depth (0 = unlimited)           |
| `parallel_threshold`               | int    | `20`          | Min children to trigger parallel tree building (≥ 1)   |
| `include_menubar_hints`            | bool   | `false`       | Show hints on menubar items                            |
| `include_dock_hints`               | bool   | `false`       | Show hints on Dock items                               |
| `include_nc_hints`                 | bool   | `false`       | Show hints in Notification Center                      |
| `include_stage_manager_hints`      | bool   | `false`       | Show hints in Stage Manager                            |
| `detect_mission_control`           | bool   | `false`       | Auto-disable hints when in Mission Control             |
| `additional_menubar_hints_targets` | array  | see below     | Extra menubar bundle IDs shown by default              |
| `clickable_roles`                  | array  | see below     | AX roles that generate hints                           |
| `ignore_clickable_check`           | bool   | `false`       | Skip clickability heuristic                            |

### Visual options (`[hints.ui]`)

| Option                     | Type   | Default       | Description                      |
| -------------------------- | ------ | ------------- | -------------------------------- |
| `font_size`                | int    | `10`          | Label font size (6–72)           |
| `font_family`              | string | `""`          | Font name (empty = system)       |
| `border_radius`            | int    | `-1`          | Border radius (`-1` = auto pill) |
| `border_width`             | int    | `1`           | Border width in pixels (≥ 0)     |
| `padding_x`                | int    | `-1`          | Horizontal padding (`-1` = auto) |
| `padding_y`                | int    | `-1`          | Vertical padding (`-1` = auto)   |
| `background_color_light`   | string | `"#F200CFCF"` | Label background (light mode)    |
| `background_color_dark`    | string | `"#F2007A9E"` | Label background (dark mode)     |
| `text_color_light`         | string | `"#FF003554"` | Label text (light mode)          |
| `text_color_dark`          | string | `"#FFFFFFFF"` | Label text (dark mode)           |
| `matched_text_color_light` | string | `"#FFAAEEFF"` | Typed text color (light mode)    |
| `matched_text_color_dark`  | string | `"#FF003554"` | Typed text color (dark mode)     |
| `border_color_light`       | string | `"#FF008A8A"` | Border color (light mode)        |
| `border_color_dark`        | string | `"#FF00B4D8"` | Border color (dark mode)         |

### auto_exit_actions

Actions that cause hints mode to exit automatically after execution. When an action key (configured in `[action.key_bindings]`) is pressed and matches an entry in this list, the mode exits immediately after performing the action.

> [!WARNING]
> Do not add `move_mouse_relative` to `auto_exit_actions`. It causes the mode to exit on every arrow-key nudge, which is almost always undesirable. See [Mouse Movement Actions](#mouse-movement-actions).

```toml
[hints]
auto_exit_actions = ["left_click", "middle_click"]
```

### mode_exit_keys

Keys that exit hints mode, merged with `general.mode_exit_keys`.

> [!NOTE]
> `hints.mode_exit_keys` must not conflict with `hints.hint_characters`.

### hint_characters

Characters used to generate hint labels. Order matters for ergonomics.

```toml
[hints]
hint_characters = "asdfghjkl"    # Home row (default, recommended)
# hint_characters = "hjklasdfg"  # Vim-style
# hint_characters = "fjdksla;g"  # Center columns
```

**Requirements:** At least 2 unique ASCII characters. No duplicates (case-insensitive).

### backspace_key

Key used to delete the last typed character in hints mode.

```toml
[hints]
backspace_key = "Backspace"  # default
# backspace_key = "Delete"   # macOS forward-delete
# backspace_key = "Ctrl+H"   # modifier combo
```

> [!NOTE]
> `hints.backspace_key` must not conflict with `hints.hint_characters` or `action.key_bindings`. Action keys are checked before the backspace key, so a conflict means backspace will never fire.

### Visibility options

By default, hints only appear on UI elements in the currently focused app. Enable additional system areas as needed:

```toml
[hints]
include_menubar_hints       = true   # Menubar items
include_dock_hints          = true   # Dock icons
include_nc_hints            = true   # Notification Center
include_stage_manager_hints = true   # Stage Manager windows
```

**`additional_menubar_hints_targets`** — the default config pre-populates three common menubar targets. Add more bundle IDs for other menubar apps:

```toml
[hints]
additional_menubar_hints_targets = [
    "com.apple.TextInputMenuAgent",  # Input menu (default)
    "com.apple.controlcenter",       # Control Center (default)
    "com.apple.systemuiserver",      # System UI (default)
    "com.example.MyMenubarApp",      # Add your own
]
```

> [!WARNING]
> `detect_mission_control` is only supported on macOS 26 and later. Enabling it on earlier versions causes false positives that prevent hints from appearing in the active app.

### Clickable elements

**`clickable_roles`** controls which macOS accessibility roles generate hint labels. The defaults cover most standard UI elements. Add custom roles for specific apps using `[[hints.app_configs]]` (see [Per-app configuration](#per-app-configuration) below).

**Default clickable roles:**

```toml
[hints]
clickable_roles = [
    "AXButton", "AXComboBox", "AXCheckBox", "AXRadioButton",
    "AXLink", "AXPopUpButton", "AXTextField", "AXSlider",
    "AXTabButton", "AXSwitch", "AXDisclosureTriangle",
    "AXTextArea", "AXMenuButton", "AXMenuItem", "AXCell", "AXRow",
]
```

Set `ignore_clickable_check = true` to show hints on all elements of the listed roles, even if Neru's heuristic doesn't consider them interactive. Useful for non-standard apps (e.g. some Adobe tools).

### Performance notes

- **`mouse_action_refresh_delay`** — increase this (e.g. `500`) for apps or browsers with dynamic content that takes time to update after a click. `0` means hints refresh immediately.
- **`max_depth`** — limits how deep Neru traverses the accessibility tree. Increase if hints are missing in deeply nested UIs; decrease if Neru is slow in complex apps.
- **`parallel_threshold`** — lower values increase parallelisation for small trees; higher values reduce overhead for tiny subtrees. The default of `20` is suitable for most apps.

### Per-app configuration

Override settings for specific applications.

```toml
# Chrome: add tab groups
[[hints.app_configs]]
bundle_id = "com.google.Chrome"
additional_clickable_roles = ["AXTabGroup"]

# Adobe apps: custom roles and skip clickability check
[[hints.app_configs]]
bundle_id = "com.adobe.illustrator"
additional_clickable_roles = ["AXStaticText", "AXImage"]
ignore_clickable_check = true

# Safari: delay for dynamic content
[[hints.app_configs]]
bundle_id = "com.apple.Safari"
mouse_action_refresh_delay = 1000
```

### Enhanced browser support

Enable additional accessibility features for web browsers and Electron apps.

- **Electron apps**: sets the `AXManualAccessibility` attribute
- **Chromium/Firefox browsers**: sets the `AXEnhancedUserInterface` attribute

**Auto-detected Electron apps:**

| Bundle ID                   | Application        |
| --------------------------- | ------------------ |
| `com.microsoft.VSCode`      | Visual Studio Code |
| `com.exafunction.windsurf`  | Windsurf           |
| `com.tinyspeck.slackmacgap` | Slack              |
| `com.spotify.client`        | Spotify            |
| `md.obsidian`               | Obsidian           |

**Auto-detected Chromium browsers:**

| Bundle ID                    | Application   |
| ---------------------------- | ------------- |
| `com.google.Chrome`          | Google Chrome |
| `com.brave.Browser`          | Brave         |
| `net.imput.helium`           | Helium        |
| `company.thebrowser.Browser` | Arc           |

**Auto-detected Firefox browsers:**

| Bundle ID             | Application |
| --------------------- | ----------- |
| `org.mozilla.firefox` | Firefox     |
| `app.zen-browser.zen` | Zen Browser |

Auto-detected apps work without any config. To add a custom app:

```toml
[hints.additional_ax_support]
enable = true
additional_electron_bundles = ["com.your.electronapp"]
additional_chromium_bundles = ["com.your.browser"]
additional_firefox_bundles  = ["com.your.firefox"]
```

---

## Grid Mode

Grid mode divides the screen into a labelled coordinate grid. Type a row+column combination to jump to that position.

### When to use

- Apps with no accessibility support (hints won't work)
- Precise cursor positioning independent of UI elements
- Universal — works anywhere on screen

### Basic configuration

| Option              | Type   | Default              | Description                                 |
| ------------------- | ------ | -------------------- | ------------------------------------------- |
| `enabled`           | bool   | `true`               | Enable/disable grid mode                    |
| `auto_exit_actions` | array  | `[]`                 | Actions that auto-exit after execution      |
| `mode_exit_keys`    | array  | `[]`                 | Additional keys that exit grid mode         |
| custom_hotkeys      | table  | {}                   | Per-mode hotkeys (same syntax as [hotkeys]) |
| `characters`        | string | see below            | Primary grid labels                         |
| `sublayer_keys`     | string | same as `characters` | Subgrid labels (≥ 9 chars for 3×3 subgrid)  |
| `reset_key`         | string | `" "`                | Key to clear input and start over           |
| `backspace_key`     | string | `"Backspace"`        | Key for input correction                    |
| `live_match_update` | bool   | `true`               | Highlight cells as you type                 |
| `hide_unmatched`    | bool   | `true`               | Hide non-matching cells                     |
| `prewarm_enabled`   | bool   | `true`               | Pre-compute grid on startup (~1.5 MB RAM)   |
| `enable_gc`         | bool   | `false`              | Periodic memory cleanup (adds CPU overhead) |

**Default characters:** `abcdefghijklmnpqrstuvwxyz`

### Visual options (`[grid.ui]`)

| Option                           | Type   | Default       | Description                          |
| -------------------------------- | ------ | ------------- | ------------------------------------ |
| `font_size`                      | int    | `10`          | Label font size (6–72)               |
| `font_family`                    | string | `""`          | Font name (empty = system)           |
| `border_width`                   | int    | `1`           | Cell border width (≥ 0)              |
| `background_color_light`         | string | `"#9900B4D8"` | Cell background (light mode)         |
| `background_color_dark`          | string | `"#99003554"` | Cell background (dark mode)          |
| `text_color_light`               | string | `"#FF003554"` | Label text (light mode)              |
| `text_color_dark`                | string | `"#FFB3E8F5"` | Label text (dark mode)               |
| `matched_text_color_light`       | string | `"#FFAAEEFF"` | Matched cell text (light mode)       |
| `matched_text_color_dark`        | string | `"#FFFFFFFF"` | Matched cell text (dark mode)        |
| `matched_background_color_light` | string | `"#B300CFCF"` | Matched cell background (light mode) |
| `matched_background_color_dark`  | string | `"#B300B4D8"` | Matched cell background (dark mode)  |
| `matched_border_color_light`     | string | `"#B300CFCF"` | Matched cell border (light mode)     |
| `matched_border_color_dark`      | string | `"#B300B4D8"` | Matched cell border (dark mode)      |
| `border_color_light`             | string | `"#9900B4D8"` | Cell border (light mode)             |
| `border_color_dark`              | string | `"#99003554"` | Cell border (dark mode)              |

> [!NOTE]
> Omitting a color value (or setting it to `""`) causes Neru to use its built-in theme-aware default, which updates automatically when you switch system themes.

### auto_exit_actions

```toml
[grid]
auto_exit_actions = ["left_click"]
```

> [!WARNING]
> Do not add `move_mouse_relative` to `auto_exit_actions` — it causes the mode to exit on every arrow-key nudge. See [Mouse Movement Actions](#mouse-movement-actions).

### mode_exit_keys

Keys that exit grid mode (merged with `general.mode_exit_keys`).

> [!NOTE]
> `grid.mode_exit_keys` must not conflict with `grid.characters`, `grid.row_labels`, `grid.col_labels`, `grid.sublayer_keys`, `grid.reset_key`, or `grid.backspace_key`.

### Custom row/column labels

Override labels for rows and columns independently. These are optional — if not set, `characters` is used for both axes.

```toml
[grid]
row_labels = "123456789"
col_labels = "abcdefghij"
```

### Reset key

Key to clear current input and start over.

```toml
[grid]
reset_key = " "         # default (space)
# reset_key = "Ctrl+R"  # modifier combo
# reset_key = "Home"    # named key
```

### Backspace key

```toml
[grid]
backspace_key = "Backspace"  # default
# backspace_key = "Ctrl+H"
```

> [!NOTE]
> `grid.backspace_key` must not conflict with `grid.characters`, `grid.row_labels`, `grid.col_labels`, `grid.sublayer_keys`, or `action.key_bindings`.

---

## Recursive Grid Mode

Recursive grid divides the screen into cells and narrows the active area with each keypress, enabling very precise positioning anywhere on screen.

### When to use

- Extremely precise cursor positioning
- Large or high-resolution screens where a flat grid would have too many cells
- Repetitive workflows where you build muscle memory for common screen regions

### How it works

1. The screen is divided into an N×N grid (default 2×2).
2. Press a cell key to zoom into that region.
3. The selected region divides again recursively.
4. This continues until the cell is smaller than `min_size_width` / `min_size_height`.

### Basic configuration

| Option              | Type   | Default       | Description                                     |
| ------------------- | ------ | ------------- | ----------------------------------------------- |
| `enabled`           | bool   | `true`        | Enable/disable mode                             |
| `auto_exit_actions` | array  | `[]`          | Actions that auto-exit after execution          |
| `mode_exit_keys`    | array  | `[]`          | Additional keys that exit this mode             |
| custom_hotkeys      | table  | {}            | Per-mode hotkeys (same syntax as [hotkeys])     |
| `grid_cols`         | int    | `2`           | Number of columns (≥ 2)                         |
| `grid_rows`         | int    | `2`           | Number of rows (≥ 2)                            |
| `keys`              | string | `"uijk"`      | Cell selection keys (exactly cols × rows chars) |
| `backspace_key`     | string | `"Backspace"` | Go up one recursion level                       |
| `min_size_width`    | int    | `25`          | Minimum cell width in pixels (≥ 10)             |
| `min_size_height`   | int    | `25`          | Minimum cell height in pixels (≥ 10)            |
| `max_depth`         | int    | `10`          | Maximum recursion levels (1–20)                 |
| `reset_key`         | string | `" "`         | Reset to initial screen center                  |

### auto_exit_actions

```toml
[recursive_grid]
auto_exit_actions = ["left_click", "right_click"]
```

> [!WARNING]
> Do not add `move_mouse_relative` to `auto_exit_actions` — it causes the mode to exit on every arrow-key nudge. See [Mouse Movement Actions](#mouse-movement-actions).

### mode_exit_keys

> [!NOTE]
> `recursive_grid.mode_exit_keys` must not conflict with `recursive_grid.keys`, `recursive_grid.reset_key`, or `recursive_grid.backspace_key`.

### Default key layout

```
┌───────┬───────┐
│   u   │   i   │   u = upper-left
├───────┼───────┤   i = upper-right
│   j   │   k   │   j = lower-left
└───────┴───────┘   k = lower-right
```

### Key behaviour

| Key                                       | Action                          |
| ----------------------------------------- | ------------------------------- |
| Cell keys (`u`, `i`, `j`, `k` by default) | Narrow to that cell             |
| `Backspace` or `Delete`                   | Go up one level                 |
| Reset key (Space by default)              | Return to initial screen center |
| `Esc`                                     | Exit mode                       |

### Grid dimensions

```toml
[recursive_grid]
# 2×2 (default, 4 cells)
grid_cols = 2
grid_rows = 2
keys = "uijk"

# 3×3 (9 cells, more precision per step)
grid_cols = 3
grid_rows = 3
keys = "gcrhtnmwv"

# 3×2 (non-square, 6 cells)
grid_cols = 3
grid_rows = 2
keys = "gcrhtn"
```

### Per-depth layers

Override grid dimensions at specific recursion depths. Unspecified depths use the top-level defaults.

> [!NOTE]
> Depths are 0-indexed. You don't have to start at depth 0 — layers can be configured at any depth, and unspecified depths fall back to the top-level defaults.

```toml
[recursive_grid]
grid_cols = 2
grid_rows = 2
keys = "uijk"

# Depth 0: wide 4×2 grid for coarse navigation
[[recursive_grid.layers]]
depth = 0
grid_cols = 4
grid_rows = 2
keys = "qwerasdf"

# Depth 1: 3×3 for medium precision
[[recursive_grid.layers]]
depth = 1
grid_cols = 3
grid_rows = 3
keys = "qweasdzxc"

# Depth 2+: falls back to the 2×2 defaults
```

Each layer must specify `grid_cols`, `grid_rows`, and `keys`. The `keys` string must have exactly `grid_cols × grid_rows` unique ASCII characters. Duplicate depths are not allowed.

### Backspace key

```toml
[recursive_grid]
backspace_key = "Backspace"  # default
# backspace_key = "Ctrl+H"
```

> [!NOTE]
> `recursive_grid.backspace_key` must not conflict with `recursive_grid.keys` or `action.key_bindings`.

### Visual options (`[recursive_grid.ui]`)

| Option                                | Type   | Default       | Description                                                     |
| ------------------------------------- | ------ | ------------- | --------------------------------------------------------------- |
| `line_color_light`                    | string | `"#FF007A9E"` | Grid line color (light mode)                                    |
| `line_color_dark`                     | string | `"#FF00CFCF"` | Grid line color (dark mode)                                     |
| `line_width`                          | int    | `1`           | Line thickness (≥ 0)                                            |
| `highlight_color_light`               | string | `"#4D007A9E"` | Cell highlight (light mode)                                     |
| `highlight_color_dark`                | string | `"#4D00CFCF"` | Cell highlight (dark mode)                                      |
| `text_color_light`                    | string | `"#FF007A9E"` | Cell label color (light mode)                                   |
| `text_color_dark`                     | string | `"#FF00CFCF"` | Cell label color (dark mode)                                    |
| `font_size`                           | int    | `10`          | Font size for labels (6–72)                                     |
| `font_family`                         | string | `""`          | Font family (empty = system)                                    |
| `label_background`                    | bool   | `false`       | Add rounded backgrounds behind labels                           |
| `label_background_color_light`        | string | `"#FFAAEEFF"` | Label background (light mode)                                   |
| `label_background_color_dark`         | string | `"#FF003554"` | Label background (dark mode)                                    |
| `label_background_padding_x`          | int    | `-1`          | Horizontal badge padding (`-1` = auto)                          |
| `label_background_padding_y`          | int    | `-1`          | Vertical badge padding (`-1` = auto)                            |
| `label_background_border_radius`      | int    | `-1`          | Badge border radius (`-1` = auto)                               |
| `label_background_border_width`       | int    | `1`           | Badge border width (≥ 0; `0` disables)                          |
| `sub_key_preview`                     | bool   | `false`       | Draw miniature key grid inside each cell                        |
| `sub_key_preview_font_size`           | int    | `8`           | Font size for sub-key labels (4–72)                             |
| `sub_key_preview_autohide_multiplier` | float  | `1.5`         | Hide preview when sub-cell < font_size × this; `0` = never hide |
| `sub_key_preview_text_color_light`    | string | `"#66007A9E"` | Sub-key label color (light mode)                                |
| `sub_key_preview_text_color_dark`     | string | `"#6600CFCF"` | Sub-key label color (dark mode)                                 |

> [!NOTE]
> Omitting a color (or setting it to `""`) uses Neru's built-in theme-aware default, which updates in real time when you switch system themes.

When `label_background = true`, each cell label gets a rounded badge background. Badge geometry is controlled via `label_background_padding_x`, `label_background_padding_y`, `label_background_border_radius`, and `label_background_border_width` — values of `-1` use automatic sizing.

When `sub_key_preview = true`, each cell shows a miniature version of the next key grid, useful when learning a non-default layout. For odd-sized grids, the exact center slot is left empty to avoid overlapping the main cell label.

---

## Scroll Mode

Vim-style scrolling at the cursor position, with fully configurable key bindings.

### When to use

- Scrolling documents and web pages without touching the mouse
- Code review in terminals or editors
- Any situation where reaching for the mouse to scroll feels slow

### Basic configuration

| Option              | Type  | Default   | Description                                 |
| ------------------- | ----- | --------- | ------------------------------------------- |
| `scroll_step`       | int   | `50`      | Pixels per j/k press (≥ 1)                  |
| `scroll_step_half`  | int   | `500`     | Pixels for d/u half-page (≥ 1)              |
| `scroll_step_full`  | int   | `1000000` | Pixels for gg/G jump to top/bottom (≥ 1)    |
| `auto_exit_actions` | array | `[]`      | Actions that auto-exit after execution      |
| `mode_exit_keys`    | array | `[]`      | Additional keys that exit scroll mode       |
| custom_hotkeys      | table | {}        | Per-mode hotkeys (same syntax as [hotkeys]) |

### auto_exit_actions

```toml
[scroll]
auto_exit_actions = ["left_click"]
```

> [!WARNING]
> Do not add `move_mouse_relative` to `auto_exit_actions` — it causes the mode to exit on every arrow-key nudge. See [Mouse Movement Actions](#mouse-movement-actions).

### mode_exit_keys

> [!NOTE]
> `scroll.mode_exit_keys` must not conflict with `scroll.key_bindings` keys or `action.key_bindings`.

### Key bindings

Each action can have multiple keys assigned.

```toml
[scroll.key_bindings]
scroll_up    = ["k"]
scroll_down  = ["j"]
scroll_left  = ["h"]
scroll_right = ["l"]
go_top       = ["gg"]
go_bottom    = ["Shift+G"]
page_up      = ["u", "PageUp"]
page_down    = ["d", "PageDown"]
```

### Key format reference

| Format             | Example                      | Description                        |
| ------------------ | ---------------------------- | ---------------------------------- |
| Single key         | `"j"`, `"k"`                 | Direct press                       |
| Named key          | `"Up"`, `"Down"`, `"PageUp"` | Arrow and special keys             |
| Modifier combo     | `"Ctrl+Z"`, `"Cmd+Down"`     | Modifier + key                     |
| Function key       | `"F1"`, `"F12"`              | F1 through F20                     |
| Multi-key sequence | `"gg"`                       | Typed in sequence (500 ms timeout) |

### Customisation examples

**With arrow keys for scrolling** (disables arrow-key mouse movement in scroll mode):

```toml
[scroll.key_bindings]
scroll_up    = ["k", "Up"]
scroll_down  = ["j", "Down"]
scroll_left  = ["h", "Left"]
scroll_right = ["l", "Right"]
go_top       = ["gg", "Cmd+Up"]
go_bottom    = ["Shift+G", "Cmd+Down"]
page_up      = ["u", "PageUp"]
page_down    = ["d", "PageDown"]
```

> [!NOTE]
> Arrow keys (`Up`/`Down`/`Left`/`Right`) are used by default for `move_mouse_*` action bindings. If you add them to scroll key bindings, the action bindings take priority (action keys are checked first). To use arrow keys for scrolling, clear the corresponding `action.key_bindings` entries or rebind them.

---

## Mouse Movement Actions

Keyboard-driven cursor nudging and direct mouse actions, available while inside hints, grid, recursive grid, or scroll mode.

### When to use

- Fine-tuning cursor position after selecting a hint or grid cell
- Performing clicks or mouse movements while scrolling without exiting scroll mode
- Adjusting before clicking without exiting the mode

### Configuration

| Option            | Type | Default | Description                      |
| ----------------- | ---- | ------- | -------------------------------- |
| `move_mouse_step` | int  | `10`    | Pixels per arrow key press (≥ 1) |

```toml
[action]
move_mouse_step = 10  # ~1 mm on a typical display
```

### Default key bindings

```toml
[action.key_bindings]
left_click        = "Shift+L"
right_click       = "Shift+R"
middle_click      = "Shift+M"
mouse_down        = "Shift+I"
mouse_up          = "Shift+U"
move_mouse_up     = "Up"
move_mouse_down   = "Down"
move_mouse_left   = "Left"
move_mouse_right  = "Right"
```

### Typical workflow

1. Enter hints, grid, or scroll mode
2. Type to select a position (or scroll to the desired area)
3. Use arrow keys to nudge the cursor
4. Press an action key (e.g. `Shift+L`) to click

### Scroll actions via CLI

Scroll at the current cursor position without entering scroll mode. These are available as `neru action` subcommands and can be used from hotkeys or custom hotkeys.

| Subcommand     | Scroll amount             | Description              |
| -------------- | ------------------------- | ------------------------ |
| `scroll_up`    | `scroll.scroll_step`      | Scroll up by one line    |
| `scroll_down`  | `scroll.scroll_step`      | Scroll down by one line  |
| `scroll_left`  | `scroll.scroll_step`      | Scroll left by one line  |
| `scroll_right` | `scroll.scroll_step`      | Scroll right by one line |
| `go_top`       | `scroll.scroll_step_full` | Scroll to top of page    |
| `go_bottom`    | `scroll.scroll_step_full` | Scroll to bottom of page |
| `page_up`      | `scroll.scroll_step_half` | Scroll up by half page   |
| `page_down`    | `scroll.scroll_step_half` | Scroll down by half page |

**CLI examples:**

```bash
neru action scroll_down    # Scroll down by scroll_step pixels
neru action page_up        # Scroll up by scroll_step_half pixels
neru action go_top         # Jump to top (scroll_step_full)
```

**Hotkey examples:**

```toml
[hotkeys]
"Cmd+Down" = "action scroll_down"
"Cmd+Up"   = "action scroll_up"

[scroll.custom_hotkeys]
"Cmd+Shift+J" = "action page_down"
"Cmd+Shift+K" = "action page_up"
```

---

## Mode Indicator

A small floating label that follows the cursor and shows the current active mode.

### Per-mode settings (`[mode_indicator.<mode>]`)

Available modes: `scroll`, `hints`, `grid`, `recursive_grid`.

| Option                   | Type   | Default | Description                               |
| ------------------------ | ------ | ------- | ----------------------------------------- |
| `enabled`                | bool   | varies  | Show indicator in this mode               |
| `text`                   | string | varies  | Label text (empty string hides the label) |
| `background_color_light` | string | `""`    | Per-mode background override (light mode) |
| `background_color_dark`  | string | `""`    | Per-mode background override (dark mode)  |
| `text_color_light`       | string | `""`    | Per-mode text color override (light mode) |
| `text_color_dark`        | string | `""`    | Per-mode text color override (dark mode)  |
| `border_color_light`     | string | `""`    | Per-mode border override (light mode)     |
| `border_color_dark`      | string | `""`    | Per-mode border override (dark mode)      |

Color overrides are optional. When left empty, the value from `[mode_indicator.ui]` is used.

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

**Customisation examples:**

```toml
# Emoji label
[mode_indicator.scroll]
enabled = true
text = "📜 Scroll"

# Different color per mode
[mode_indicator.hints]
enabled = true
text = "Hints"
background_color_light = "#F2FF6B6B"
background_color_dark  = "#F2CC3333"

# Show indicator but hide label text
[mode_indicator.grid]
enabled = true
text = ""
```

### Shared appearance (`[mode_indicator.ui]`)

| Option                   | Type   | Default       | Description                      |
| ------------------------ | ------ | ------------- | -------------------------------- |
| `font_size`              | int    | `10`          | Text size (≥ 1)                  |
| `font_family`            | string | `""`          | Font (empty = system)            |
| `background_color_light` | string | `"#F200CFCF"` | Background (light mode)          |
| `background_color_dark`  | string | `"#F200CFCF"` | Background (dark mode)           |
| `text_color_light`       | string | `"#FF003554"` | Text (light mode)                |
| `text_color_dark`        | string | `"#FF003554"` | Text (dark mode)                 |
| `border_color_light`     | string | `"#FF007A9E"` | Border (light mode)              |
| `border_color_dark`      | string | `"#FF007A9E"` | Border (dark mode)               |
| `border_width`           | int    | `1`           | Border width (≥ 0)               |
| `padding_x`              | int    | `-1`          | Horizontal padding (`-1` = auto) |
| `padding_y`              | int    | `-1`          | Vertical padding (`-1` = auto)   |
| `border_radius`          | int    | `-1`          | Border radius (`-1` = auto pill) |
| `indicator_x_offset`     | int    | `20`          | X offset from cursor             |
| `indicator_y_offset`     | int    | `20`          | Y offset from cursor             |

---

## Sticky Modifiers

Tap a modifier key while in any navigation mode to "stick" it. The modifier is applied to the next mouse action without physically holding the key.

### How it works

1. Enter any navigation mode.
2. Tap `Shift` once (press and release without pressing any other key) — a `⇧` indicator appears near the cursor.
3. Perform an action (e.g. left click) — it executes as `Shift+click`.
4. Tap `Shift` again to remove it, or tap `Cmd` to add `⌘⇧`.
5. Modifiers reset automatically when you exit the mode.

> [!NOTE]
> Neru physically injects the modifier key-down event into macOS when a sticky modifier becomes active. This ensures full compatibility with third-party applications (like window managers or ghostty) that rely on reading physical keyboard states. Your normal Neru hotkey combinations will still work transparently.

> [!NOTE]
> A modifier tap is only detected when the key is pressed and released cleanly with no other key in between. Pressing `Shift+L` (a common action binding) does **not** toggle `Shift` as sticky.

### Configuration

| Option             | Type | Default | Description                                                                      |
| ------------------ | ---- | ------- | -------------------------------------------------------------------------------- |
| `enabled`          | bool | `true`  | Enable sticky modifiers                                                          |
| `tap_max_duration` | int  | `300`   | Max hold duration (ms) for a tap to register. `0` = always toggle.               |
| `tap_cooldown`     | int  | `0`     | Min quiet period (ms) after a key press before a tap can toggle. `0` = disabled. |

```toml
[sticky_modifiers]
enabled = true
tap_max_duration = 300
tap_cooldown = 0
```

If a modifier is held longer than `tap_max_duration` before being released, it does **not** toggle the sticky state. This prevents accidental toggles when you hold a modifier intending to use it as a chord and then change your mind.

#### Karabiner-Elements users

If you use [Karabiner-Elements](https://karabiner-elements.pqrs.org/) to remap modifier+key combos (e.g. `Option+h → Left`), you may see false sticky toggles. This happens because Karabiner clears the modifier flag _before_ sending the remapped key, so Neru sees a clean modifier tap.
Set `tap_cooldown` to suppress these ghost toggles:

```toml
[sticky_modifiers]
tap_cooldown = 500   # 500 ms works well for most Karabiner setups as far as I tested
```

When `tap_cooldown > 0`, a modifier tap is ignored if a regular key was pressed within that many milliseconds **before** the modifier went down. This catches rapid Karabiner-remapped sequences without affecting normal usage — intentional modifier taps after a brief pause still work.

> [!NOTE]
> A 50 ms debounce is always active regardless of `tap_cooldown`. It delays the toggle just long enough for a Karabiner-remapped key to arrive and cancel the pending toggle. The cooldown is an additional layer for rapid-fire scenarios where the remapped key arrives _before_ the next modifier press.

### Visual options (`[sticky_modifiers.ui]`)

The sticky modifier indicator appears to the **left** of the cursor by default; the mode indicator appears to the **right**.

```
          ⌘⇧  ↖ cursor ↗  Scroll
     (-40, 20)           (20, 20)
```

| Option                   | Type   | Default       | Description                            |
| ------------------------ | ------ | ------------- | -------------------------------------- |
| `font_size`              | int    | `10`          | Text size (≥ 1)                        |
| `font_family`            | string | `""`          | Font (empty = system)                  |
| `background_color_light` | string | `"#F200CFCF"` | Background (light mode)                |
| `background_color_dark`  | string | `"#F200CFCF"` | Background (dark mode)                 |
| `text_color_light`       | string | `"#FF003554"` | Text (light mode)                      |
| `text_color_dark`        | string | `"#FF003554"` | Text (dark mode)                       |
| `border_color_light`     | string | `"#FF007A9E"` | Border (light mode)                    |
| `border_color_dark`      | string | `"#FF007A9E"` | Border (dark mode)                     |
| `border_width`           | int    | `1`           | Border width (≥ 0)                     |
| `padding_x`              | int    | `-1`          | Horizontal padding (`-1` = auto)       |
| `padding_y`              | int    | `-1`          | Vertical padding (`-1` = auto)         |
| `border_radius`          | int    | `-1`          | Border radius (`-1` = auto pill)       |
| `indicator_x_offset`     | int    | `-40`         | X offset from cursor (negative = left) |
| `indicator_y_offset`     | int    | `20`          | Y offset from cursor                   |

To move the sticky indicator below the cursor instead:

```toml
[sticky_modifiers.ui]
indicator_x_offset = 20
indicator_y_offset = 40
```

---

## Smooth Cursor

Animates cursor movement between positions. Duration adapts to distance — short moves animate quickly, long moves take longer.

### Configuration

| Option               | Type  | Default | Description                        |
| -------------------- | ----- | ------- | ---------------------------------- |
| `move_mouse_enabled` | bool  | `false` | Enable smooth mouse movement       |
| `steps`              | int   | `10`    | Number of intermediate positions   |
| `max_duration`       | int   | `200`   | Maximum animation duration (ms)    |
| `duration_per_pixel` | float | `0.1`   | Ms per pixel for adaptive duration |

```toml
[smooth_cursor]
move_mouse_enabled = false  # default (instant)
steps = 10
max_duration = 200
duration_per_pixel = 0.1
```

The animation runs asynchronously, so key presses are never blocked. Steps are automatically reduced when needed to stay within the computed duration.

> [!NOTE]
> Smooth cursor adds a small delay between cursor position and click. If you use hints or grid to click rapidly, keep this disabled for a snappier feel.

---

## Systray

Controls the system tray (menu bar) icon.

| Option    | Type | Default | Description               |
| --------- | ---- | ------- | ------------------------- |
| `enabled` | bool | `true`  | Show icon in the menu bar |

```toml
[systray]
enabled = true     # Show icon (default)
# enabled = false  # Headless mode — control via hotkeys and CLI only
```

> [!NOTE]
> Changing this setting requires a full daemon restart — `neru config reload` will not apply it.

---

## Font Configuration

Customise fonts used in overlays.

### Default behaviour

When `font_family = ""` (empty string):

- **Hints**: bold system font
- **Grid / Scroll**: regular system font

### Font resolution order

1. **PostScript name** (exact match) — e.g. `"SFMono-Bold"`, `"Menlo-Regular"`
2. **Family name** (looser match) — e.g. `"SF Mono"`, `"JetBrains Mono"`
3. **Fallback** to system font

### Common fonts

```toml
[hints.ui]
font_family = ""                 # System default
# font_family = "SF Mono"
# font_family = "JetBrains Mono"
# font_family = "Fira Code"
# font_family = "Menlo"
# font_family = "SFMono-Bold"   # PostScript name with variant
```

### Finding available fonts

```bash
system_profiler SPFontsDataType | grep "Family:"
# Or open Font Book:
open -a "Font Book"
```

> [!TIP]
> Family names (e.g. `"SF Mono"`) are more portable than PostScript names across machines.

---

## Logging

Control how the daemon logs information for debugging and monitoring.

### Configuration

| Option                 | Type   | Default  | Description                       |
| ---------------------- | ------ | -------- | --------------------------------- |
| `log_level`            | string | `"info"` | Minimum level to log              |
| `log_file`             | string | `""`     | Custom log file path              |
| `structured_logging`   | bool   | `true`   | JSON-formatted output             |
| `disable_file_logging` | bool   | `true`   | Write to console only (no file)   |
| `max_file_size`        | int    | `10`     | MB per file before rotation       |
| `max_backups`          | int    | `5`      | Number of old files to keep       |
| `max_age`              | int    | `30`     | Days before old files are deleted |

> [!NOTE]
> With `disable_file_logging = true` (the default), logs are written to the console only. Set it to `false` to enable the log file at `~/Library/Logs/neru/app.log`.

### Log levels

| Level   | What gets logged     |
| ------- | -------------------- |
| `debug` | Everything (verbose) |
| `info`  | Normal operations    |
| `warn`  | Warnings and errors  |
| `error` | Errors only          |

### Common configurations

**Default:**

```toml
[logging]
log_level = "info"
structured_logging = true
```

**Debug mode:**

```toml
[logging]
log_level = "debug"
disable_file_logging = false
```

**Log to file (persistent):**

```toml
[logging]
disable_file_logging = false
log_file = ""  # Uses ~/Library/Logs/neru/app.log
```

**Custom log path:**

```toml
[logging]
disable_file_logging = false
log_file = "/var/log/neru.log"
```

### Viewing logs

```bash
tail -f ~/Library/Logs/neru/app.log           # Real-time stream
tail -50 ~/Library/Logs/neru/app.log          # Last 50 lines
grep ERROR ~/Library/Logs/neru/app.log        # Errors only
grep "com.apple.Safari" ~/Library/Logs/neru/app.log  # App-specific
```

### Log rotation

Logs rotate automatically based on your settings:

- **`max_file_size`** — triggers rotation when the current file exceeds this size
- **`max_backups`** — number of rotated files to retain
- **`max_age`** — deletes files older than this many days
