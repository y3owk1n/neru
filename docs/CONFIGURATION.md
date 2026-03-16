# Configuration Guide

Neru uses TOML for configuration. This guide covers all available options with explanations, use cases, and examples.

---

## Table of Contents

- [Configuration Overview](#configuration-overview)
- [Hotkeys](#hotkeys)
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
- [Color Format](#color-format)
- [Logging](#logging)

---

## Configuration Overview

Neru uses TOML configuration files. Configuration is loaded from (in order of priority):

1. **`$XDG_CONFIG_HOME/neru/config.toml`** (if `XDG_CONFIG_HOME` is set)
2. **`~/.config/neru/config.toml`** (recommended)
3. **`~/.neru.toml`** (legacy)
4. **`neru.toml`** (current directory)
5. **`config.toml`** (current directory)

This search order is the same on all platforms (macOS, Linux, Windows).

### Global Flag

- **`--config` or `-c` flag**: Specify a custom config file path when launching.

**No config file?** Neru works out of the box with sensible defaults. Copy from [default-config.toml](../configs/default-config.toml) to customize.

**Reload config changes:**

```bash
neru config reload
```

Or restart the daemon: `pkill neru && neru launch`

---

## Hotkeys

Global hotkeys trigger Neru navigation modes from anywhere.

### Why Configure Hotkeys?

- Activate navigation modes without CLI
- Customize key combinations to your preference
- Execute shell commands directly from keyboard

### Syntax

**Modifiers:** `Cmd`, `Ctrl`, `Alt`/`Option`, `Shift` (case-insensitive). Right/Left-prefixed variants are also accepted (e.g., `RightCmd`, `LeftShift`, `RightOption`, `RightCtrl`).

> [!NOTE]
> The Right/Left-prefixes are **aliases** — they map to the same modifier flag as the unprefixed form.
> The macOS Carbon API does not distinguish left vs right modifier keys, so `RightCmd+Space` and `Cmd+Space` register the exact same hotkey.
> The prefixed names exist for readability when using key remappers like Karabiner-Elements that produce side-specific modifiers.

**Format:** `"Modifier1+Modifier2+Key" = "action"`

**Actions can be:**

- **Neru commands**: `"Cmd+Shift+Space" = "hints"`
- **Commands with actions**: `"Cmd+Shift+G" = "grid --action left_click"`
- **Shell commands**: `"Cmd+Alt+T" = "exec open -a Terminal"`

### Important Note

> Defining **any** custom hotkey **replaces ALL defaults**. You must explicitly define every hotkey you want.

Defaults that will be disabled:

- `Cmd+Shift+Space` → hints
- `Cmd+Shift+G` → grid
- `Cmd+Shift+C` → recursive_grid
- `Cmd+Shift+S` → scroll

### Common Configurations

**Basic (enable only what you need):**

```toml
[hotkeys]
"Cmd+Shift+Space" = "hints"
```

**With auto-click (recommended):**

```toml
[hotkeys]
"Cmd+Shift+Space" = "hints --action left_click"
"Cmd+Shift+G" = "grid --action left_click"
"Cmd+Shift+S" = "scroll"
```

**Execute shell commands:**

```toml
[hotkeys]
# Open Terminal
"Cmd+Alt+T" = "exec open -a Terminal"

# Open specific app
"Cmd+Alt+C" = "exec open -a 'Visual Studio Code'"

# Run script
"Cmd+Alt+S" = "exec ~/scripts/screenshot.sh"

# Show notification
"Cmd+Alt+N" = "exec osascript -e 'display notification \"Hello!\" with title \"Neru\"'"
```

### Alternative: External Hotkey Manager

Use [skhd](https://github.com/koekeishiya/skhd) or [Karabiner](https://karabiner-elements.pqrs.org/) for more complex hotkey setups:

```bash
# ~/.config/skhd/skhdrc
ctrl - f : neru hints
ctrl - g : neru grid
ctrl - r : neru hints --action right_click
```

---

## Keyboard Layout Requirements

Neru uses a reference keyboard layout for key translation so hotkeys and mode keys
stay stable even when you switch active input sources (for example EN/RU).

### `general.kb_layout_to_use` (optional)

#### macOS

Set an `InputSourceID` to force a specific layout:

```toml
[general]
kb_layout_to_use = "com.apple.keylayout.ABC"
```

Use `defaults read com.apple.HIToolbox AppleEnabledInputSources` to inspect available IDs.

#### Linux / Windows

(Planned) This setting will allow specifying a fallback XKB layout or Windows input locale.

### Automatic fallback

Neru auto-selects the first available layout in this order:

1. **macOS**: `com.apple.keylayout.ABC`, `com.apple.keylayout.US`, or any English layout.
2. **Linux**: (Planned) `us`, `abc`, or current layout.
3. **Windows**: (Planned) `en-US` or current layout.
4. Current keyboard layout (last resort).

---

## General Settings

Core behavior settings that affect all Neru functionality.

### Option Reference

| Option                                 | Type   | Default      | Description                                             |
| -------------------------------------- | ------ | ------------ | ------------------------------------------------------- |
| `excluded_apps`                        | array  | `[]`         | Bundle IDs where Neru won't activate                    |
| `accessibility_check_on_start`         | bool   | `true`       | Verify accessibility permissions on launch              |
| `restore_cursor_position`              | bool   | `false`      | Return cursor to pre-mode position on exit              |
| `center_cursor_position`               | bool   | `false`      | Center cursor on current screen on exit                 |
| `kb_layout_to_use`                     | string | `""`         | Optional InputSourceID for layout mapping               |
| `mode_exit_keys`                       | array  | `["Escape"]` | Keys that exit any active mode                          |
| `passthrough_unbounded_keys`           | bool   | `false`      | Let unbound Cmd/Ctrl/Alt shortcuts reach macOS          |
| `should_exit_after_passthrough`        | bool   | `false`      | Exit the current mode after a shortcut passes through   |
| `passthrough_unbounded_keys_blacklist` | array  | `[]`         | Shortcuts to keep consumed while passthrough is enabled |
| `hide_overlay_in_screen_share`         | bool   | `false`      | Hide overlay in screen sharing apps                     |

### excluded_apps

Prevent Neru from activating in specific applications.

```toml
[general]
excluded_apps = [
    "com.apple.Terminal",        # Terminal
    "com.googlecode.iterm2",     # iTerm2
    "com.microsoft.rdc.macos",   # Microsoft Remote Desktop
    "com.apple.finder",          # Finder (optional)
]
```

**Finding Bundle IDs:**

```bash
osascript -e 'id of app "Safari"'
# Output: com.apple.Safari
```

**Use case:** Exclude terminal apps where you prefer keyboard-only workflows, or exclude screen sharing apps.

### accessibility_check_on_start

Ensures Neru has accessibility permissions before running.

```toml
[general]
accessibility_check_on_start = true  # default
```

- `true`: Show error if permissions missing
- `false`: Skip check (may cause runtime errors)

### restore_cursor_position

Return cursor to where it was before entering a navigation mode.

```toml
[general]
restore_cursor_position = false  # default
```

- `true`: Cursor returns to original position after mode exits
- `false`: Cursor stays at last navigated position

**Use case:** Set to `true` if you prefer the cursor not moving unexpectedly.

### center_cursor_position

Center the cursor on the current screen when exiting a navigation mode.

```toml
[general]
center_cursor_position = false  # default
```

- `true`: Cursor is centered on the current screen after mode exits
- `false`: Cursor stays at last navigated position

> [!NOTE]
> `restore_cursor_position` and `center_cursor_position` are mutually exclusive. Only one can be enabled at a time.

**Use case:** Set to `true` if you want a predictable cursor location after navigation.

### kb_layout_to_use

Optional reference layout InputSourceID used for key translation.

```toml
[general]
kb_layout_to_use = ""                            # auto fallback (default)
# kb_layout_to_use = "com.apple.keylayout.ABC"  # force ABC
```

Use this when you want physical keys interpreted consistently across multiple active input sources.

### mode_exit_keys

Keys that exit any active mode (hints, grid, scroll, recursive_grid). Note that this affects all modes. You can add more mode-specific exit keys in each mode's section.

> [!NOTE]
> This array **cannot be empty** — at least one exit key must be defined.

```toml
[general]
mode_exit_keys = ["Escape"]           # default
# mode_exit_keys = ["Ctrl+C"]         # Vim-style
# mode_exit_keys = ["Escape", "q"]    # Multiple keys
```

**Valid keys:**

- Plain: `Escape`, `Return`, `Tab`, `Space`, `Backspace`, `Delete`
- Navigation: `Home`, `End`, `PageUp`, `PageDown`, `Up`, `Down`, `Left`, `Right`
- Function keys: `F1` through `F20`
- With modifiers: `Ctrl+C`, `Cmd+Q`, `Alt+X`
- Single characters: any single letter or digit

> [!NOTE]
> Named key casing is flexible — `"escape"`, `"Escape"`, and `"ESCAPE"` all work. Title case (e.g. `"Escape"`) is the recommended convention.

### passthrough_unbounded_keys

Allow unbound Cmd/Ctrl/Alt shortcuts to keep reaching macOS while a mode is active.

```toml
[general]
passthrough_unbounded_keys = false  # default
```

- `true`: Neru passes through modifier shortcuts that the active mode does not use
- `false`: Neru consumes all active-mode key presses

**Examples that can pass through:** `Cmd+Tab`, `Cmd+Space`, `Cmd+W`, `Alt+Tab`

> [!NOTE]
> Modifier shortcuts that Neru actively uses are still consumed. For example, scroll bindings like `Ctrl+D` or `Cmd+Down` continue working when they are configured in the active mode.

### should_exit_after_passthrough

Exit the active mode after an unbound modifier shortcut is passed through.

```toml
[general]
passthrough_unbounded_keys = true
should_exit_after_passthrough = false  # default
```

- `true`: After a passthrough shortcut is detected, Neru exits the mode that observed it
- `false`: Neru stays in the current mode after passthrough

This is useful when shortcuts like `Cmd+Tab` or `Cmd+Space` should both reach macOS and dismiss the overlay.

> [!NOTE]
> This setting only has an effect when `passthrough_unbounded_keys` is enabled.

### passthrough_unbounded_keys_blacklist

Optional list of modifier shortcuts that should stay consumed by Neru even when `passthrough_unbounded_keys` is enabled.

```toml
[general]
passthrough_unbounded_keys = true
passthrough_unbounded_keys_blacklist = ["Cmd+W", "Cmd+Q"]
```

Use this when you want macOS shortcuts to keep working in general, but you want to suppress a few of them while a mode is active.

> [!NOTE]
> Each entry must include at least one of `Cmd`, `Ctrl`, `Alt`, or `Option` as a modifier. Plain keys or Shift-only combos are not valid here.

### hide_overlay_in_screen_share

Hide Neru overlays during screen sharing.

```toml
[general]
hide_overlay_in_screen_share = false  # default
```

- `true`: Overlay hidden in shared screens (visible locally)
- `false`: Overlay always visible

> [!NOTE]
> Uses macOS `NSWindow.sharingType` API. Reliability varies:
>
> - macOS 14 and earlier: Works well with most apps
> - macOS 15.4+: Limited effectiveness with ScreenCaptureKit-based apps
> - Test with your specific video conferencing software

---

## Hint Mode

Hint mode uses macOS Accessibility APIs to identify clickable UI elements and overlay short labels on them.

### When to Use

- **Clicking buttons, links, menus** in applications
- **Forms and dialogs** with multiple clickable elements
- **Any application** with standard macOS UI elements

### Basic Configuration

| Option              | Type   | Default       | Description                                        |
| ------------------- | ------ | ------------- | -------------------------------------------------- |
| `enabled`           | bool   | `true`        | Enable/disable hints mode                          |
| `auto_exit_actions` | array  | `[]`          | Actions that auto-exit after execution (see below) |
| `mode_exit_keys`    | array  | `[]`          | Additional keys that exit hints mode               |
| `hint_characters`   | string | `"asdfghjkl"` | Characters used for labels                         |
| `backspace_key`     | string | `"Backspace"` | Key for input correction (see below)               |

### Visual Options (`[hints.ui]`)

| Option                     | Type   | Default       | Description                                         |
| -------------------------- | ------ | ------------- | --------------------------------------------------- |
| `font_size`                | int    | `10`          | Label font size (6–72)                              |
| `font_family`              | string | `""`          | Font name (empty = system)                          |
| `border_radius`            | int    | `-1`          | Border radius (`-1` = auto pill)                    |
| `border_width`             | int    | `1`           | Border width in pixels (≥ 0)                        |
| `padding_x`                | int    | `-1`          | Horizontal padding (`-1` = auto based on font size) |
| `padding_y`                | int    | `-1`          | Vertical padding (`-1` = auto based on font size)   |
| `background_color_light`   | string | `"#F200CFCF"` | Label background for Light Mode (theme-aware)       |
| `background_color_dark`    | string | `"#F2007A9E"` | Label background for Dark Mode (theme-aware)        |
| `text_color_light`         | string | `"#FF003554"` | Label text for Light Mode (theme-aware)             |
| `text_color_dark`          | string | `"#FFFFFFFF"` | Label text for Dark Mode (theme-aware)              |
| `matched_text_color_light` | string | `"#FFAAEEFF"` | Typed text color for Light Mode (theme-aware)       |
| `matched_text_color_dark`  | string | `"#FF003554"` | Typed text color for Dark Mode (theme-aware)        |
| `border_color_light`       | string | `"#FF008A8A"` | Border color for Light Mode (theme-aware)           |
| `border_color_dark`        | string | `"#FF00B4D8"` | Border color for Dark Mode (theme-aware)            |

### auto_exit_actions

Actions that cause hints mode to exit automatically after execution. When a direct action key (configured in `[action.key_bindings]`) is pressed and matches an action in this list, the mode will exit immediately after performing the action. See [Mouse Movement Actions](#mouse-movement-actions) for available action names.

> [!WARNING]
> Including `move_mouse_relative` will cause the mode to exit on every arrow-key nudge (Up/Down/Left/Right), which is usually undesirable. This action is primarily intended for fine-tuning cursor position while staying in the mode.

```toml
[hints]
# Exit hints mode after left or middle click
auto_exit_actions = ["left_click", "middle_click"]
```

### mode_exit_keys

Keys that exit hints mode (merged with `general.mode_exit_keys`).

> [!NOTE]
> `hints.mode_exit_keys` must not conflict with `hints.hint_characters`

### hint_characters

Characters used to generate hint labels. Order matters for ergonomics.

```toml
[hints]
hint_characters = "asdfghjkl"    # Home row (default, recommended)
# hint_characters = "jkl;asdfg"  # Alternative home row
# hint_characters = "fjdksla;g"  # Center columns
# hint_characters = "hjklasdfg"  # Vim-style
```

**Requirements:**

- At least 2 unique ASCII characters
- No duplicates (case-insensitive)

### backspace_key

Key used for input correction (deleting the last typed character) in hints mode.

```toml
[hints]
backspace_key = "Backspace"  # default
# backspace_key = "Delete"   # macOS forward-delete key
# backspace_key = "x"        # single character
# backspace_key = "Ctrl+H"   # modifier combo
```

When empty (`""`), defaults to the standard Backspace/Delete key.

> [!NOTE]
> `hints.backspace_key` must not conflict with `hints.hint_characters` or `action.key_bindings`. At runtime, action keys are checked before the backspace key, so a conflict means backspace will never fire.

### Visibility Options

Control which system areas show hints.

| Option                             | Type  | Default   | Description                                           |
| ---------------------------------- | ----- | --------- | ----------------------------------------------------- |
| `include_menubar_hints`            | bool  | `false`   | Show hints on menubar items                           |
| `include_dock_hints`               | bool  | `false`   | Show hints on Dock items                              |
| `include_nc_hints`                 | bool  | `false`   | Show hints in Notification Center                     |
| `include_stage_manager_hints`      | bool  | `false`   | Show hints in Stage Manager                           |
| `detect_mission_control`           | bool  | `false`   | Auto-disable active app hints when in Mission Control |
| `additional_menubar_hints_targets` | array | see below | Extra menubar bundle IDs                              |

```toml
[hints]
# Enable system area hints
include_menubar_hints = true
include_dock_hints = true
include_nc_hints = false
include_stage_manager_hints = false

# Mission Control detection (macOS 26+ only!)
# WARNING: Do NOT enable on macOS < 26.x - causes false positives
# detect_mission_control = true

# Add specific menubar apps
additional_menubar_hints_targets = [
    "com.apple.TextInputMenuAgent",     # Input menu
    "com.apple.controlcenter",          # Control Center
    "com.apple.systemuiserver",         # System UI
]
```

### Clickable Elements

Define which UI element types generate hints.

| Option                   | Type  | Default     | Description                 |
| ------------------------ | ----- | ----------- | --------------------------- |
| `clickable_roles`        | array | (see below) | AX roles that are clickable |
| `ignore_clickable_check` | bool  | `false`     | Skip clickability heuristic |

**Default clickable roles:**

```toml
[hints]
clickable_roles = [
    "AXButton",
    "AXComboBox",
    "AXCheckBox",
    "AXRadioButton",
    "AXLink",
    "AXPopUpButton",
    "AXTextField",
    "AXSlider",
    "AXTabButton",
    "AXSwitch",
    "AXDisclosureTriangle",
    "AXTextArea",
    "AXMenuButton",
    "AXMenuItem",
    "AXCell",
    "AXRow",
]
```

**Use case:** Add custom roles for specific apps:

```toml
[hints]
# Add tab groups for Chrome
[[hints.app_configs]]
bundle_id = "com.google.Chrome"
additional_clickable_roles = ["AXTabGroup"]
```

### Performance Tuning

| Option                       | Type | Default | Description                                      |
| ---------------------------- | ---- | ------- | ------------------------------------------------ |
| `mouse_action_refresh_delay` | int  | `0`     | Delay after click before refresh in ms (0–10000) |
| `max_depth`                  | int  | `50`    | Max accessibility tree depth (0 = unlimited)     |
| `parallel_threshold`         | int  | `20`    | Min children for parallel processing (≥ 1)       |

```toml
[hints]
# Delay before refreshing hints after an action (for slow-loading apps or browser dynamic contents)
# 0 = immediate, 10000 = max 10 seconds
mouse_action_refresh_delay = 500  # 0.5 seconds

# Tree depth limit (prevents stack overflow on complex UIs)
# 0 = unlimited, 50 = default
max_depth = 50

# Parallel processing threshold
# Lower = more parallelization for small trees
# Higher = less overhead for tiny subtrees
parallel_threshold = 20
```

### Per-App Configuration

Override settings for specific applications.

```toml
# Chrome: add tab groups
[[hints.app_configs]]
bundle_id = "com.google.Chrome"
additional_clickable_roles = ["AXTabGroup"]

# Adobe apps: may need custom roles
[[hints.app_configs]]
bundle_id = "com.adobe.illustrator"
additional_clickable_roles = ["AXStaticText", "AXImage"]
ignore_clickable_check = true

# Safari: delay for dynamic content
[[hints.app_configs]]
bundle_id = "com.apple.Safari"
mouse_action_refresh_delay = 1000  # 1 second
```

### Enhanced Browser Support

Enable additional accessibility features for web browsers and Electron apps. These require special handling because they don't expose standard macOS accessibility APIs by default.

**How it works:**

- **Electron apps**: Sets `AXManualAccessibility` attribute
- **Chromium/Firefox browsers**: Sets `AXEnhancedUserInterface` attribute

#### Default Auto-Detected Bundles

**Electron apps** (require `AXManualAccessibility`):

| Bundle ID                   | Application        |
| --------------------------- | ------------------ |
| `com.microsoft.VSCode`      | Visual Studio Code |
| `com.exafunction.windsurf`  | Windsurf           |
| `com.tinyspeck.slackmacgap` | Slack              |
| `com.spotify.client`        | Spotify            |
| `md.obsidian`               | Obsidian           |

**Chromium browsers** (require `AXEnhancedUserInterface`):

| Bundle ID                    | Application   |
| ---------------------------- | ------------- |
| `com.google.Chrome`          | Google Chrome |
| `com.brave.Browser`          | Brave Browser |
| `net.imput.helium`           | Helium        |
| `company.thebrowser.Browser` | Arc Browser   |

**Firefox browsers** (require `AXEnhancedUserInterface`):

| Bundle ID             | Application     |
| --------------------- | --------------- |
| `org.mozilla.firefox` | Mozilla Firefox |
| `app.zen-browser.zen` | Zen Browser     |

#### Configuration

```toml
[hints.additional_ax_support]
enable = false  # default

# Auto-detected bundles work automatically
# Add custom bundles if needed:
additional_electron_bundles = ["com.example.electronapp"]
additional_chromium_bundles = ["com.example.browser"]
additional_firefox_bundles = ["com.example.firefox"]
```

#### Enabling for Custom Apps

If you have an Electron/Chromium/Firefox app not in the default list:

```toml
[hints.additional_ax_support]
enable = true

# Add your custom app
additional_electron_bundles = ["com.your.customapp"]
additional_chromium_bundles = ["com.your.customapp"]
additional_firefox_bundles = ["com.your.customapp"]
```

---

## Grid Mode

Grid mode divides the screen into a coordinate-based grid for direct position selection.

### When to Use

- **No accessibility support** in target app
- **Precise cursor positioning** needed
- **Fast navigation** without scanning elements
- **Anywhere on screen** - works universally

### Basic Configuration

| Option              | Type   | Default              | Description                                        |
| ------------------- | ------ | -------------------- | -------------------------------------------------- |
| `enabled`           | bool   | `true`               | Enable/disable grid mode                           |
| `auto_exit_actions` | array  | `[]`                 | Actions that auto-exit after execution (see below) |
| `mode_exit_keys`    | array  | `[]`                 | Additional keys that exit grid mode                |
| `characters`        | string | (see below)          | Primary grid labels                                |
| `sublayer_keys`     | string | (same as characters) | Subgrid labels (≥ 9 chars for 3×3 subgrid)         |
| `reset_key`         | string | `" "`                | Key to clear input                                 |
| `backspace_key`     | string | `"Backspace"`        | Key for input correction                           |

### Visual Options (`[grid.ui]`)

| Option                           | Type   | Default       | Description                                          |
| -------------------------------- | ------ | ------------- | ---------------------------------------------------- |
| `font_size`                      | int    | `10`          | Label font size (6–72)                               |
| `font_family`                    | string | `""`          | Font name (empty = system)                           |
| `border_width`                   | int    | `1`           | Cell border width (≥ 0)                              |
| `background_color_light`         | string | `"#9900B4D8"` | Cell background for Light Mode (theme-aware)         |
| `background_color_dark`          | string | `"#99003554"` | Cell background for Dark Mode (theme-aware)          |
| `text_color_light`               | string | `"#FF003554"` | Label text for Light Mode (theme-aware)              |
| `text_color_dark`                | string | `"#FFB3E8F5"` | Label text for Dark Mode (theme-aware)               |
| `matched_text_color_light`       | string | `"#FFAAEEFF"` | Matched cell text for Light Mode (theme-aware)       |
| `matched_text_color_dark`        | string | `"#FFFFFFFF"` | Matched cell text for Dark Mode (theme-aware)        |
| `matched_background_color_light` | string | `"#B300CFCF"` | Matched cell background for Light Mode (theme-aware) |
| `matched_background_color_dark`  | string | `"#B300B4D8"` | Matched cell background for Dark Mode (theme-aware)  |
| `matched_border_color_light`     | string | `"#B300CFCF"` | Matched cell border for Light Mode (theme-aware)     |
| `matched_border_color_dark`      | string | `"#B300B4D8"` | Matched cell border for Dark Mode (theme-aware)      |
| `border_color_light`             | string | `"#9900B4D8"` | Cell border for Light Mode (theme-aware)             |
| `border_color_dark`              | string | `"#99003554"` | Cell border for Dark Mode (theme-aware)              |

> [!NOTE]
> Theme-aware colors:
> When these are not set in your config file (empty string `""`),
> Neru automatically uses sensible defaults that adapt to your system appearance.
> The colors update in real time when you switch system themes. If you explicitly
> set a value, it is always used regardless of the system theme.

**Default characters:** `abcdefghijklmnpqrstuvwxyz`

### auto_exit_actions

Actions that cause grid mode to exit automatically after execution. See [Mouse Movement Actions](#mouse-movement-actions) for available action names.

> [!WARNING]
> Including `move_mouse_relative` will cause the mode to exit on every arrow-key nudge, which is usually undesirable.

```toml
[grid]
# Exit grid mode after left click
auto_exit_actions = ["left_click"]
```

### mode_exit_keys

Keys that exit grid mode (merged with `general.mode_exit_keys`).

> [!NOTE]
> `grid.mode_exit_keys` must not conflict with `grid.characters`, `grid.row_labels`, `grid.col_labels`, `grid.sublayer_keys`, `grid.reset_key`, or `grid.backspace_key`

### Character Requirements

- At least 2 unique ASCII characters
- No duplicates (case-insensitive)
- Cannot contain the reset key (` ` / space by default)

```toml
[grid]
# Default (all lowercase letters)
characters = "abcdefghijklmnpqrstuvwxyz"

# Custom character sets
# characters = "asdfghjkl"          # Home row
# characters = "1234567890"         # Numbers only
# characters = "hjklasdfg"          # Vim-style
```

### Custom Row/Column Labels

Override labels for rows and columns separately.

```toml
[grid]
# Numbers for rows, letters for columns
row_labels = "123456789"
col_labels = "abcdefghij"

# Dvorak-style
row_labels = "',.pyfgcrl/"
col_labels = "aoeuidhtns"

# Symbols (advanced keyboard layouts)
row_labels = "7890gcrlhtnsmwvz"
col_labels = "7890gcrlhtnsmwvz"
```

### Grid Behavior

| Option              | Type | Default | Description                 |
| ------------------- | ---- | ------- | --------------------------- |
| `live_match_update` | bool | `true`  | Highlight cells as you type |
| `hide_unmatched`    | bool | `true`  | Hide non-matching cells     |
| `prewarm_enabled`   | bool | `true`  | Pre-compute grid on startup |
| `enable_gc`         | bool | `false` | Periodic memory cleanup     |

```toml
[grid]
# Real-time feedback
live_match_update = true   # Highlight as you type
hide_unmatched = true      # Dim non-matching cells

# Performance
prewarm_enabled = true     # Faster first use (~1.5MB RAM)
enable_gc = false          # Enable if memory is tight
```

### Reset Key

Key to clear current input and start over.

```toml
[grid]
reset_key = " "         # default (space)
# reset_key = ","       # comma
# reset_key = "."       # period
# reset_key = "Ctrl+R"  # modifier combo
# reset_key = "Home"    # named key
# reset_key = "F1"      # function key
```

**Valid formats:** single character, named key (`Home`, `End`, `Tab`, `F1`–`F20`, etc.), or modifier combo (`Ctrl+R`).

### Backspace Key

Key used for input correction (deleting the last typed character) in grid mode.

```toml
[grid]
backspace_key = "Backspace"  # default
# backspace_key = "Ctrl+H"   # modifier combo
```

When empty (`""`), defaults to the standard Backspace/Delete key.

> [!NOTE]
> `grid.backspace_key` must not conflict with `grid.characters`, `grid.row_labels`, `grid.col_labels`, `grid.sublayer_keys`, or `action.key_bindings`.

---

## Recursive Grid Mode (Recommended)

Recursive grid divides the screen into cells, narrowing selection with each keypress.

### When to Use

- **Extremely precise** cursor positioning
- **Large screens** where grid would have too many cells
- **Repetitive workflows** - muscle memory for common areas

### How It Works

1. Screen divided into NxN cells (default 2x2)
2. Press a cell key to narrow to that region
3. Selected region divides again recursively
4. Continues until cell is small enough (min_size_width/height)

### Basic Configuration

| Option              | Type   | Default       | Description                                        |
| ------------------- | ------ | ------------- | -------------------------------------------------- |
| `enabled`           | bool   | `true`        | Enable/disable mode                                |
| `auto_exit_actions` | array  | `[]`          | Actions that auto-exit after execution (see below) |
| `mode_exit_keys`    | array  | `[]`          | Additional keys that exit recursive-grid mode      |
| `grid_cols`         | int    | `2`           | Number of columns (≥ 2)                            |
| `grid_rows`         | int    | `2`           | Number of rows (≥ 2)                               |
| `keys`              | string | `"uijk"`      | Cell selection keys (exactly cols × rows chars)    |
| `backspace_key`     | string | `"Backspace"` | Key for backtracking (go up one level)             |
| `min_size_width`    | int    | `25`          | Min cell width in pixels (≥ 10)                    |
| `min_size_height`   | int    | `25`          | Min cell height in pixels (≥ 10)                   |
| `max_depth`         | int    | `10`          | Maximum recursion levels (1–20)                    |
| `reset_key`         | string | `" "`         | Key to reset to start                              |

### auto_exit_actions

Actions that cause recursive-grid mode to exit automatically after execution. See [Mouse Movement Actions](#mouse-movement-actions) for available action names.

> [!WARNING]
> Including `move_mouse_relative` will cause the mode to exit on every arrow-key nudge, which is usually undesirable.

```toml
[recursive_grid]
# Exit recursive-grid mode after left or right click
auto_exit_actions = ["left_click", "right_click"]
```

### mode_exit_keys

Keys that exit recursive-grid mode (merged with `general.mode_exit_keys`).

> [!NOTE]
> `recursive_grid.mode_exit_keys` must not conflict with `recursive_grid.keys`, `recursive_grid.reset_key`, or `recursive_grid.backspace_key`

### Grid Dimensions

```toml
[recursive_grid]
# 2x2 (default, 4 cells)
grid_cols = 2
grid_rows = 2
keys = "uijk"  # u=TL, i=TR, j=BL, k=BR

# 3x3 (9 cells)
grid_cols = 3
grid_rows = 3
keys = "gcrhtnmwv"  # 9 unique characters

# 3x2 (non-square, 6 cells)
grid_cols = 3
grid_rows = 2
keys = "gcrhtn"  # 6 unique characters
```

### Per-Depth Layers

Override grid dimensions and keys at specific recursion depths. Depths without an entry use the top-level defaults.

> [!NOTE]
> Depths are `0`-indexed, so the first layer is depth `0`, and you don't have to start with `0`, it cen be configured at any depth, and the rest fallback to the top-level defaults.

```toml
[recursive_grid]
grid_cols = 2
grid_rows = 2
keys = "uijk"

# Depth 0: wide 4×2 grid
[[recursive_grid.layers]]
depth = 0
grid_cols = 4
grid_rows = 2
keys = "qwerasdf"

# Depth 1: 3×3 grid
[[recursive_grid.layers]]
depth = 1
grid_cols = 3
grid_rows = 3
keys = "qweasdzxc"

# Depth 2+: falls back to the 2×2 / "uijk" defaults
```

Each layer must specify all three fields (`grid_cols`, `grid_rows`, `keys`). The `keys` string must have exactly `grid_cols × grid_rows` unique ASCII characters. Duplicate depths are not allowed.

### Key Behavior

| Key                                    | Action                   |
| -------------------------------------- | ------------------------ |
| Cell keys (`u`,`i`,`j`,`k` by default) | Narrow to that cell      |
| `Backspace` or `Delete`                | Go up one level          |
| Reset key (` ` / space by default)     | Return to initial center |
| `Esc`                                  | Exit mode                |

### Cell Key Mapping

Default 2x2 layout:

```
u   →   i          u = Upper-left
                   i = Upper-right
j   →   k          j = Lower-left
                   k = Lower-right
```

### Visual Options (`[recursive_grid.ui]`)

| Option                                | Type   | Default       | Description                                                            |
| ------------------------------------- | ------ | ------------- | ---------------------------------------------------------------------- |
| `line_color_light`                    | string | `"#FF007A9E"` | Grid line color for Light Mode (theme-aware)                           |
| `line_color_dark`                     | string | `"#FF00CFCF"` | Grid line color for Dark Mode (theme-aware)                            |
| `line_width`                          | int    | `1`           | Line thickness (≥ 0)                                                   |
| `highlight_color_light`               | string | `"#4D007A9E"` | Cell highlight for Light Mode (theme-aware)                            |
| `highlight_color_dark`                | string | `"#4D00CFCF"` | Cell highlight for Dark Mode (theme-aware)                             |
| `text_color_light`                    | string | `"#FF007A9E"` | Cell text color for Light Mode (theme-aware)                           |
| `text_color_dark`                     | string | `"#FF00CFCF"` | Cell text color for Dark Mode (theme-aware)                            |
| `font_size`                           | int    | `10`          | Font size for labels (6–72)                                            |
| `font_family`                         | string | `""`          | Font family for labels (empty = system)                                |
| `label_background`                    | bool   | `false`       | Add rounded backgrounds behind labels                                  |
| `label_background_color_light`        | string | `"#FFAAEEFF"` | Label background for Light Mode                                        |
| `label_background_color_dark`         | string | `"#FF003554"` | Label background for Dark Mode                                         |
| `label_background_padding_x`          | int    | `-1`          | Horizontal badge padding (`-1` = auto)                                 |
| `label_background_padding_y`          | int    | `-1`          | Vertical badge padding (`-1` = auto)                                   |
| `label_background_border_radius`      | int    | `-1`          | Badge border radius (`-1` = auto)                                      |
| `label_background_border_width`       | int    | `1`           | Badge border width (≥ 0, `0` disables)                                 |
| `sub_key_preview`                     | bool   | `false`       | Draw miniature key grid inside each cell                               |
| `sub_key_preview_font_size`           | int    | `8`           | Font size for sub-key preview labels (4–72)                            |
| `sub_key_preview_autohide_multiplier` | float  | `1.5`         | Preview hides when sub-cell < font size × this value; `0` = never hide |
| `sub_key_preview_text_color_light`    | string | `"#66007A9E"` | Sub-key label color for Light Mode                                     |
| `sub_key_preview_text_color_dark`     | string | `"#6600CFCF"` | Sub-key label color for Dark Mode                                      |

> [!NOTE]
> Theme-aware colors:
> When these are not set in your config file (empty string `""`),
> Neru automatically uses sensible defaults that adapt to your system appearance.
> The colors update in real time when you switch system themes. If you explicitly
> set a value, it is always used regardless of the system theme.

When `label_background = true`, Neru keeps the normal recursive-grid cell fill and adds a separate rounded label background behind each letter. The configured alpha is used as-is, so you can control the transparency precisely with `label_background_color_light` and `label_background_color_dark`. Badge geometry is configurable with `label_background_padding_x`, `label_background_padding_y`, `label_background_border_radius`, and `label_background_border_width`. A value of `-1` keeps the automatic sizing behavior for padding and border radius.

When `sub_key_preview = true`, each cell in the recursive grid shows a miniature version of the key grid, giving you a visual preview of which key maps to which sub-region. This is helpful when learning the key layout or using non-default grid dimensions. The preview labels use `sub_key_preview_font_size` and `sub_key_preview_text_color_light`/`sub_key_preview_text_color_dark` for styling. The default colors are set at 40% opacity to keep them subtle. For odd-sized preview grids, Neru leaves the exact center preview slot empty so it does not overlap the main cell label.

### Backspace Key

Key used for backtracking (going up one recursion level) in recursive-grid mode.

```toml
[recursive_grid]
backspace_key = "Backspace"  # default
# backspace_key = "Ctrl+H"   # modifier combo
```

When empty (`""`), defaults to the standard Backspace/Delete key.

> [!NOTE]
> `recursive_grid.backspace_key` must not conflict with `recursive_grid.keys` or `action.key_bindings`.

---

## Scroll Mode

Vim-style scrolling at the cursor position.

### When to Use

- **Scrolling documents** without reaching for mouse
- **Code review** in terminal or editors
- **Long web pages** in browsers

### Basic Configuration

| Option             | Type  | Default   | Description                           |
| ------------------ | ----- | --------- | ------------------------------------- |
| `scroll_step`      | int   | `50`      | Pixels per j/k press (≥ 1)            |
| `scroll_step_half` | int   | `500`     | Pixels for Ctrl+D/U (≥ 1)             |
| `scroll_step_full` | int   | `1000000` | Pixels for gg/G (top/bottom, ≥ 1)     |
| `mode_exit_keys`   | array | `[]`      | Additional keys that exit scroll mode |

```toml
[scroll]
scroll_step = 50            # j/k: 50 pixels
scroll_step_half = 500      # Ctrl+D/U: half page
scroll_step_full = 1000000  # gg/G: jump to top/bottom
```

### mode_exit_keys

Keys that exit scroll mode (merged with `general.mode_exit_keys`).

> [!NOTE]
> `scroll.mode_exit_keys` must not conflict with `scroll.key_bindings` keys

### Key Bindings

Configure which keys perform each action. Each action can have multiple keys.

```toml
[scroll.key_bindings]
# Movement (one line)
scroll_up = ["k", "Up"]
scroll_down = ["j", "Down"]
scroll_left = ["h", "Left"]
scroll_right = ["l", "Right"]

# Navigation (jump to position)
go_top = ["gg", "Cmd+Up"]
go_bottom = ["Shift+G", "Cmd+Down"]

# Page movement
page_up = ["Ctrl+U", "PageUp"]
page_down = ["Ctrl+D", "PageDown"]
```

### Key Format

| Format         | Example                       | Description                  |
| -------------- | ----------------------------- | ---------------------------- |
| Single key     | `"j"`, `"k"`, `"g"`           | Direct press                 |
| Arrow keys     | `"Up"`, `"Down"`              | Named keys                   |
| Modifier combo | `"Ctrl+U"`, `"Cmd+Down"`      | Modifier + key               |
| Special keys   | `"PageUp"`, `"Home"`, `"End"` | Named specials               |
| Function keys  | `"F1"`, `"F12"`, `"F20"`      | Function keys F1 through F20 |
| Multi-key      | `"gg"`                        | Sequence (500ms timeout)     |

### Customization Examples

**Vim-style:**

```toml
[scroll.key_bindings]
scroll_up = ["k"]
scroll_down = ["j"]
scroll_left = ["h"]
scroll_right = ["l"]
go_top = ["gg"]
go_bottom = ["Shift+G"]
```

**Mac-style:**

```toml
[scroll.key_bindings]
scroll_up = ["k", "Up"]
scroll_down = ["j", "Down"]
go_top = ["Cmd+Up"]
go_bottom = ["Cmd+Down"]
page_up = ["PageUp"]
page_down = ["PageDown"]
```

**Home/End navigation:**

```toml
[scroll.key_bindings]
scroll_up = ["k"]
scroll_down = ["j"]
go_top = ["Home"]
go_bottom = ["End"]
page_up = ["PageUp"]
page_down = ["PageDown"]
```

---

## Mouse Movement Actions

Keyboard-driven cursor movement within hints/grid modes.

### When to Use

- **Fine-grained positioning** after selecting a hint/grid cell
- **Adjusting position** before clicking
- **Mouse-free navigation** combined with clicking

### Configuration

| Option            | Type | Default | Description                      |
| ----------------- | ---- | ------- | -------------------------------- |
| `move_mouse_step` | int  | `10`    | Pixels per arrow key press (≥ 1) |

**Default keybindings:**

```toml
[action]
move_mouse_step = 10  # pixels per press

[action.key_bindings]
left_click = "Shift+L"
right_click = "Shift+R"
middle_click = "Shift+M"
mouse_down = "Shift+I"
mouse_up = "Shift+U"
move_mouse_up = "Up"
move_mouse_down = "Down"
move_mouse_left = "Left"
move_mouse_right = "Right"
```

### Step Size

```toml
[action]
move_mouse_step = 10   # default, ~1mm on typical display
move_mouse_step = 5    # finer control
move_mouse_step = 20   # faster movement
```

### Usage

1. Enter hints or grid mode
2. Type to select a position
3. Use arrow keys to adjust
4. Click or press action key

---

## Mode Indicator

A small label that follows the cursor showing the current mode.

### When to Use

- **Visual feedback** on which mode is active
- **Multiple monitors** - helps track which screen
- **Beginners** learning Neru

### Per-Mode Settings (`[mode_indicator.<mode>]`)

Each mode has its own sub-table under `[mode_indicator]`. Available modes: `scroll`, `hints`, `grid`, `recursive_grid`.

| Option                   | Type   | Default | Description                                    |
| ------------------------ | ------ | ------- | ---------------------------------------------- |
| `enabled`                | bool   | varies  | Show indicator in this mode                    |
| `text`                   | string | varies  | Label text (empty string `""` hides the label) |
| `background_color_light` | string | `""`    | Per-mode background override for Light Mode    |
| `background_color_dark`  | string | `""`    | Per-mode background override for Dark Mode     |
| `text_color_light`       | string | `""`    | Per-mode text color override for Light Mode    |
| `text_color_dark`        | string | `""`    | Per-mode text color override for Dark Mode     |
| `border_color_light`     | string | `""`    | Per-mode border color override for Light Mode  |
| `border_color_dark`      | string | `""`    | Per-mode border color override for Dark Mode   |

Color overrides are optional. When left empty (`""`), the value from `[mode_indicator.ui]` is used.

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

**Customization examples:**

```toml
# Custom label text
[mode_indicator.scroll]
enabled = true
text = "📜 Scroll"
# Different color for hints mode
[mode_indicator.hints]
enabled = true
text = "Hints"
background_color_light = "#F2FF6B6B"
background_color_dark = "#F2CC3333"
# Hide label text but keep indicator enabled (empty text = hidden)
[mode_indicator.grid]
enabled = true
text = ""
```

### Shared Appearance (`[mode_indicator.ui]`)

| Option                   | Type   | Default       | Description                                         |
| ------------------------ | ------ | ------------- | --------------------------------------------------- |
| `font_size`              | int    | `10`          | Text size (≥ 1)                                     |
| `font_family`            | string | `""`          | Font (empty = system)                               |
| `background_color_light` | string | `"#F200CFCF"` | Background for Light Mode (theme-aware)             |
| `background_color_dark`  | string | `"#F200CFCF"` | Background for Dark Mode (theme-aware)              |
| `text_color_light`       | string | `"#FF003554"` | Text for Light Mode (theme-aware)                   |
| `text_color_dark`        | string | `"#FF003554"` | Text for Dark Mode (theme-aware)                    |
| `border_color_light`     | string | `"#FF007A9E"` | Border for Light Mode (theme-aware)                 |
| `border_color_dark`      | string | `"#FF007A9E"` | Border for Dark Mode (theme-aware)                  |
| `border_width`           | int    | `1`           | Border width (≥ 0)                                  |
| `padding_x`              | int    | `-1`          | Horizontal padding (`-1` = auto based on font size) |
| `padding_y`              | int    | `-1`          | Vertical padding (`-1` = auto based on font size)   |
| `border_radius`          | int    | `-1`          | Border radius (`-1` = auto pill)                    |
| `indicator_x_offset`     | int    | `20`          | X offset from cursor                                |
| `indicator_y_offset`     | int    | `20`          | Y offset from cursor                                |

---

## Sticky Modifiers

Tap a modifier key while in any navigation mode to "stick" it — the modifier will be applied to the next mouse action without physically holding the key.

### When to Use

- **Shift+click** to select ranges without holding Shift
- **Cmd+click** to open links in new tabs
- **Combining modifiers** like Cmd+Shift for complex actions

### How It Works

1. Enter any navigation mode (hints, grid, recursive grid, scroll)
2. Tap Shift once (press and release without pressing any other key) — "⇧" appears near the cursor
3. Perform an action (e.g., left click) — it executes as Shift+click
4. Tap Shift again to remove it, or tap Cmd to add "⌘⇧"
5. Modifiers reset automatically when exiting the mode
    > [!NOTE]
    > A modifier tap is only detected when the key is pressed and released cleanly without any other key in between. Pressing Shift+L (a common action binding) does **not** toggle Shift as sticky.

### Configuration

| Option             | Type | Default | Description                                                                                  |
| ------------------ | ---- | ------- | -------------------------------------------------------------------------------------------- |
| `enabled`          | bool | `true`  | Enable sticky modifier feature                                                               |
| `tap_max_duration` | int  | `300`   | Max hold duration (ms) for a tap to toggle sticky state. `0` = always toggle (no threshold). |

```toml
[sticky_modifiers]
enabled = true
tap_max_duration = 300  # ms; 0 = always toggle
```

If a modifier key is held longer than `tap_max_duration` before being released, the release will **not** toggle the sticky modifier. This prevents accidental toggles when you hold a modifier intending to use it as a chord (e.g., holding Ctrl for Ctrl+C) and then change your mind. Set to `0` to disable the threshold and preserve the previous always-toggle behavior.

### Visual Options (`[sticky_modifiers.ui]`)

A small indicator near the cursor shows active sticky modifiers (e.g., "⌘⇧"). By default it appears to the **left** of the cursor, while the mode indicator appears to the **right**.

| Option                   | Type   | Default       | Description                                         |
| ------------------------ | ------ | ------------- | --------------------------------------------------- |
| `font_size`              | int    | `10`          | Text size (≥ 1)                                     |
| `font_family`            | string | `""`          | Font (empty = system)                               |
| `background_color_light` | string | `"#F200CFCF"` | Background for Light Mode (theme-aware)             |
| `background_color_dark`  | string | `"#F200CFCF"` | Background for Dark Mode (theme-aware)              |
| `text_color_light`       | string | `"#FF003554"` | Text for Light Mode (theme-aware)                   |
| `text_color_dark`        | string | `"#FF003554"` | Text for Dark Mode (theme-aware)                    |
| `border_color_light`     | string | `"#FF007A9E"` | Border for Light Mode (theme-aware)                 |
| `border_color_dark`      | string | `"#FF007A9E"` | Border for Dark Mode (theme-aware)                  |
| `border_width`           | int    | `1`           | Border width (≥ 0)                                  |
| `padding_x`              | int    | `-1`          | Horizontal padding (`-1` = auto based on font size) |
| `padding_y`              | int    | `-1`          | Vertical padding (`-1` = auto based on font size)   |
| `border_radius`          | int    | `-1`          | Border radius (`-1` = auto pill)                    |
| `indicator_x_offset`     | int    | `-40`         | X offset from cursor (negative = left)              |
| `indicator_y_offset`     | int    | `20`          | Y offset from cursor                                |

### Positioning

The sticky indicator and mode indicator are positioned relative to the cursor using their respective offsets:

```
          ⌘⇧  ↖ cursor ↗  Scroll
     (-40, 20)           (20, 20)
```

To move the sticky indicator below the cursor instead:

```toml
[sticky_modifiers.ui]
indicator_x_offset = 20
indicator_y_offset = 40
```

---

## Smooth Cursor

Animate cursor movement between positions. Uses adaptive duration based on distance - short moves animate quickly, long moves take longer.

### When to Use

- **Visual tracking** - see where cursor is going
- **Aesthetic preference** - smooth motion looks nicer
- **Large jumps** - helps track movement

### Configuration

| Option               | Type  | Default | Description                                    |
| -------------------- | ----- | ------- | ---------------------------------------------- |
| `move_mouse_enabled` | bool  | `false` | Enable smooth mouse movement                   |
| `steps`              | int   | `10`    | Number of intermediate positions               |
| `max_duration`       | int   | `200`   | Maximum animation duration (ms)                |
| `duration_per_pixel` | float | `0.1`   | Ms per pixel for adaptive duration calculation |

### How It Works

The animation uses **adaptive duration**:

- Distance-based: longer moves get longer animations
- Capped at `max_duration` to prevent slowdowns
- Minimum duration of 10ms ensures visibility
- Steps are automatically reduced when needed so the total animation time stays within the computed duration

The animation runs **asynchronously** in a goroutine, so key presses never block waiting for cursor movement.

### Example

```toml
[smooth_cursor]
move_mouse_enabled = false  # default (instant)

# Smoother (more intermediate positions)
steps = 20

# Less smooth (fewer intermediate positions)
steps = 5

# Adaptive duration (long moves take longer)
max_duration = 200        # max 200ms
duration_per_pixel = 0.1  # 100ms per 1000 pixels
```

---

## Systray

System tray (menu bar) icon configuration.

### When to Use

- **Quick access** to menu options
- **Status indicator** - see if Neru is running
- **Manual control** - start/stop from menu

### Configuration

| Option    | Type | Default | Description           |
| --------- | ---- | ------- | --------------------- |
| `enabled` | bool | `true`  | Show icon in menu bar |

```toml
[systray]
enabled = true     # Show icon (default)
# enabled = false  # Headless mode
```

### Headless Mode

When disabled:

- No menu bar icon
- Control via hotkeys and CLI only
- Useful for minimal setups or custom status bars

> [!NOTE]
> Changing this requires restart (`neru config reload` won't work).

---

## Font Configuration

Customize fonts used in overlays.

### Default Behavior

When `font_family = ""` (empty string):

- **Hints**: Bold system font
- **Grid/Scroll**: Regular system font

### Font Resolution

Neru tries to find fonts in this order:

1. **PostScript name** (exact match)
    - `"SFMono-Bold"`, `"Menlo-Regular"`, `"CourierNew-Bold"`

2. **Family name** (looser matching)
    - `"SF Mono"`, `"JetBrains Mono"`, `"Fira Code"`

3. **Fallback** to system font

### Common Fonts

```toml
[hints]

[hints.ui]
# System default
font_family = ""

# Monospace fonts
font_family = "SF Mono"
font_family = "Menlo"
font_family = "Monaco"
font_family = "JetBrains Mono"
font_family = "Fira Code"
font_family = "Source Code Pro"

# With variants (PostScript names)
font_family = "SFMono-Bold"
font_family = "Menlo-Bold"
```

### Finding Available Fonts

```bash
# List all font families
system_profiler SPFontsDataType | grep "Family:"

# Or use a font manager app like Font Book
open -a "Font Book"
```

> **Tip:** Family names (e.g., "SF Mono") are more portable than PostScript names.

---

## Color Format

Neru colors use hex notation with optional alpha transparency.

### Default Behavior

Neru comes with built-in **theme-aware defaults** for all color configurations. The hex values shown in the tables above represent these defaults.

- **Light Mode active:** Neru uses the light variant defaults.
- **Dark Mode active:** Neru uses the dark variant defaults.

If you omit a color option from your config file, or explicitly set it to `""` (empty string), Neru will automatically use these built-in defaults. This ensures that the application always looks correct when you switch system themes in macOS System Settings.

### Specifying Colors

If you want to customize colors, you can set them explicitly in your config file. When a value is provided, it is always used for that specific theme variant, overriding the default behavior.

```toml
# Customizing only light mode background
[hints]

[hints.ui]
background_color_light = "#FF0000AA"  # Custom red for light mode
# background_color_dark is left empty, so it uses the dark mode default
```

### Color Options Suffixes

All color options use `_light` and `_dark` suffixes to support macOS Dark and Light Mode:

- **`*_light`** — used when macOS is in Light Mode
- **`*_dark`** — used when macOS is in Dark Mode

### Supported Formats

| Format      | Example     | Alpha | Description                       |
| ----------- | ----------- | ----- | --------------------------------- |
| `#AARRGGBB` | `#FF000000` | Yes   | 8-char: Alpha + RGB (recommended) |
| `#RRGGBB`   | `#FF0000`   | No    | 6-char: RGB only (fully opaque)   |
| `#RGB`      | `#F00`      | No    | 3-char shorthand                  |

### Format Breakdown: `#AARRGGBB`

| Position | Component | Range | Description          |
| -------- | --------- | ----- | -------------------- |
| 1-2      | AA        | 00-FF | Alpha (transparency) |
| 3-4      | RR        | 00-FF | Red                  |
| 5-6      | GG        | 00-FF | Green                |
| 7-8      | BB        | 00-FF | Blue                 |

### Alpha Channel Reference

| Opacity | Alpha | Use Case                            |
| ------- | ----- | ----------------------------------- |
| 100%    | `FF`  | Solid colors, high contrast         |
| 95%     | `F2`  | Slight transparency                 |
| 90%     | `E6`  | Moderate transparency               |
| 80%     | `CC`  | Noticeable transparency             |
| 70%     | `B3`  | Semi-transparent (default for grid) |
| 60%     | `99`  | More transparent                    |
| 50%     | `80`  | Half transparent                    |
| 40%     | `66`  | Low visibility                      |
| 30%     | `4D`  | Subtle (default highlight)          |
| 20%     | `33`  | Very subtle                         |
| 10%     | `1A`  | Barely visible                      |
| 0%      | `00`  | Invisible                           |

### Formula

To calculate alpha from opacity:

```bash
alpha_hex = round(opacity * 255)

# Examples:
# 70% = 0.7 * 255 = 178.5 → round to 179 → 0xB3
# 95% = 0.95 * 255 = 242.25 → round to 242 → 0xF2
```

### Common Examples

The following hex values represent the built-in defaults used by Neru.

```toml
# Hints (cyan / deep teal)
background_color_light = "#F200CFCF"    # Cyan, 95% opacity
background_color_dark  = "#F2007A9E"    # Deep teal, 95% opacity

# Grid background
background_color_light = "#9900B4D8"    # Sky blue, 60% opacity
background_color_dark  = "#99003554"    # Deep navy, 60% opacity

# Grid matched cell highlight
matched_background_color_light = "#B300CFCF"  # Cyan, 70% opacity
matched_background_color_dark  = "#B300B4D8"  # Sky blue, 70% opacity

# Recursive grid lines and highlights
line_color_light      = "#FF007A9E"     # Deep teal, fully opaque
line_color_dark       = "#FF00CFCF"     # Bright cyan, fully opaque
highlight_color_light = "#4D007A9E"     # Deep teal, 30% opacity
highlight_color_dark  = "#4D00CFCF"     # Bright cyan, 30% opacity

# Mode indicator
background_color_light = "#F200CFCF"    # Cyan, 95% opacity (same in both modes)
background_color_dark  = "#F200CFCF"    # Cyan, 95% opacity
```

---

## Logging

Control how Neru logs information for debugging and monitoring.

### Configuration

| Option                 | Type   | Default  | Description          |
| ---------------------- | ------ | -------- | -------------------- |
| `log_level`            | string | `"info"` | Minimum level to log |
| `log_file`             | string | `""`     | Custom log path      |
| `structured_logging`   | bool   | `true`   | JSON format          |
| `disable_file_logging` | bool   | `true`   | Console only         |
| `max_file_size`        | int    | `10`     | MB before rotation   |
| `max_backups`          | int    | `5`      | Old files to keep    |
| `max_age`              | int    | `30`     | Days to keep         |

### Log Levels

| Level   | What Gets Logged     |
| ------- | -------------------- |
| `debug` | Everything (verbose) |
| `info`  | Normal operations    |
| `warn`  | Warnings and errors  |
| `error` | Errors only          |

### Common Configurations

**Default (recommended):**

```toml
[logging]
log_level = "info"
structured_logging = true
```

**Debug mode (troubleshooting):**

```toml
[logging]
log_level = "debug"
```

**Console only (no file):**

```toml
[logging]
disable_file_logging = true
```

**Custom location:**

```toml
[logging]
log_file = "/var/log/neru.log"
```

### Log Location

Default: `~/Library/Logs/neru/app.log`

### Viewing Logs

```bash
# Real-time monitoring
tail -f ~/Library/Logs/neru/app.log

# Last 50 lines
tail -50 ~/Library/Logs/neru/app.log

# Search for errors
grep ERROR ~/Library/Logs/neru/app.log

# Search for specific app
grep "com.apple.Safari" ~/Library/Logs/neru/app.log
```

### Log Rotation

Logs automatically rotate based on settings:

- **max_file_size**: Triggers rotation when file exceeds this
- **max_backups**: Number of old logs to keep
- **max_age**: Days before old logs are deleted

Example: With 10MB max size, 5 backups, 30-day max age

- Logs rotate when current file hits 10MB
- Keep last 5 rotated files
- Delete files older than 30 days
