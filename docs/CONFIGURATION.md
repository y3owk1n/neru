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
- [Smooth Cursor](#smooth-cursor)
- [Systray](#systray)
- [Font Configuration](#font-configuration)
- [Color Format](#color-format)
- [Logging](#logging)

---

## Configuration Overview

Neru uses TOML configuration files. Configuration is loaded from (in order of priority):

1. **`~/.config/neru/config.toml`** (recommended)
    - User-level configuration directory
    - Create if it doesn't exist

2. **`~/Library/Application Support/neru/config.toml`**
    - macOS Application Support directory
    - Used as fallback if first location doesn't exist

3. **`--config` or `-c` flag**
    - Specify a custom config file path when launching
    - Example: `neru launch -c ~/my-neru-config.toml`

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

**Modifiers:** `Cmd`, `Ctrl`, `Alt`/`Option`, `Shift` (case-insensitive)
**Format:** `"Modifier1+Modifier2+Key" = "action"`

**Actions can be:**

- **Neru commands**: `"Cmd+Shift+Space" = "hints"`
- **Commands with actions**: `"Cmd+Shift+G" = "grid left_click"`
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
"Cmd+Shift+Space" = "hints left_click"
"Cmd+Shift+G" = "grid left_click"
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

Neru automatically detects your keyboard layout via macOS APIs - no configuration needed.

### Supported Layouts

| Layout  | Regions               | Works Out of Box |
| ------- | --------------------- | ---------------- |
| QWERTY  | US, UK, International | Yes              |
| AZERTY  | French                | Yes              |
| QWERTZ  | German, Swiss         | Yes              |
| Dvorak  | English (Dvorak)      | Yes              |
| Colemak | English (Colemak)     | Yes              |

### How It Works

Neru reads your macOS input source settings and translates keycodes accordingly. This means:

- Your physical key positions are used for hint characters
- Layout switching in macOS is respected

### Input Methods (CJK)

Neru supports Chinese, Japanese, and Korean input methods:

- Hints display based on your **physical keyboard layout**
- Key presses are translated through your layout before being sent to the IME
- The IME receives the expected key events

**No additional configuration required.**

---

## General Settings

Core behavior settings that affect all Neru functionality.

### Option Reference

| Option                         | Type  | Default      | Description                                |
| ------------------------------ | ----- | ------------ | ------------------------------------------ |
| `excluded_apps`                | array | `[]`         | Bundle IDs where Neru won't activate       |
| `accessibility_check_on_start` | bool  | `true`       | Verify accessibility permissions on launch |
| `restore_cursor_position`      | bool  | `false`      | Return cursor to pre-mode position on exit |
| `mode_exit_keys`               | array | `["escape"]` | Keys that exit any active mode             |
| `hide_overlay_in_screen_share` | bool  | `false`      | Hide overlay in screen sharing apps        |

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

### mode_exit_keys

Keys that exit any active mode (hints, grid, scroll, recursive_grid).

```toml
[general]
mode_exit_keys = ["escape"]           # default
# mode_exit_keys = ["Ctrl+C"]         # Vim-style
# mode_exit_keys = ["escape", "q"]    # Multiple keys
```

**Valid keys:**

- Plain: `escape`, `return`, `tab`, `space`, `backspace`, `delete`
- Navigation: `home`, `end`, `pageup`, `pagedown`
- With modifiers: `Ctrl+C`, `Cmd+Q`, `Alt+X`

### hide_overlay_in_screen_share

Hide Neru overlays during screen sharing.

```toml
[general]
hide_overlay_in_screen_share = false  # default
```

- `true`: Overlay hidden in shared screens (visible locally)
- `false`: Overlay always visible

> **Note:** Uses macOS `NSWindow.sharingType` API. Reliability varies:
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

| Option               | Type   | Default       | Description                |
| -------------------- | ------ | ------------- | -------------------------- |
| `enabled`            | bool   | `true`        | Enable/disable hints mode  |
| `hint_characters`    | string | `"asdfghjkl"` | Characters used for labels |
| `font_size`          | int    | `10`          | Label font size            |
| `font_family`        | string | `""`          | Font name (empty = system) |
| `border_radius`      | int    | `4`           | Corner radius in pixels    |
| `border_width`       | int    | `1`           | Border width in pixels     |
| `padding`            | int    | `4`           | Internal padding in pixels |
| `background_color`   | string | `"#F2FFD700"` | Label background (gold)    |
| `text_color`         | string | `"#FF000000"` | Label text (black)         |
| `matched_text_color` | string | `"#FF737373"` | Typed text color (gray)    |
| `border_color`       | string | `"#FF000000"` | Border color (black)       |

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
- No commas (reserved for grid reset)

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

| Option                       | Type | Default | Description                           |
| ---------------------------- | ---- | ------- | ------------------------------------- |
| `mouse_action_refresh_delay` | int  | `0`     | Delay after click before refresh (ms) |
| `max_depth`                  | int  | `50`    | Max accessibility tree depth          |
| `parallel_threshold`         | int  | `20`    | Min children for parallel processing  |

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

| Option          | Type   | Default              | Description              |
| --------------- | ------ | -------------------- | ------------------------ |
| `enabled`       | bool   | `true`               | Enable/disable grid mode |
| `characters`    | string | (see below)          | Primary grid labels      |
| `sublayer_keys` | string | (same as characters) | Subgrid labels           |
| `reset_key`     | string | `","`                | Key to clear input       |
| `font_size`     | int    | `10`                 | Label font size          |
| `font_family`   | string | `""`                 | Font name                |
| `border_width`  | int    | `1`                  | Cell border width        |

**Default characters:** `abcdefghijklmnpqrstuvwxyz`

### Character Requirements

- At least 2 unique ASCII characters
- No duplicates (case-insensitive)
- Cannot contain the reset key (`,` by default)

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
reset_key = ","         # default
# reset_key = "."       # period
# reset_key = "Ctrl+R"  # modifier combo
```

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

| Option            | Type   | Default  | Description              |
| ----------------- | ------ | -------- | ------------------------ |
| `enabled`         | bool   | `true`   | Enable/disable mode      |
| `grid_cols`       | int    | `2`      | Number of columns        |
| `grid_rows`       | int    | `2`      | Number of rows           |
| `keys`            | string | `"uijk"` | Cell selection keys      |
| `min_size_width`  | int    | `25`     | Min cell width (pixels)  |
| `min_size_height` | int    | `25`     | Min cell height (pixels) |
| `max_depth`       | int    | `10`     | Maximum recursion levels |
| `reset_key`       | string | `","`    | Key to reset to start    |

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

### Key Behavior

| Key                                    | Action                   |
| -------------------------------------- | ------------------------ |
| Cell keys (`u`,`i`,`j`,`k` by default) | Narrow to that cell      |
| `Backspace` or `Delete`                | Go up one level          |
| Reset key (`,` by default)             | Return to initial center |
| `Esc`                                  | Exit mode                |

### Cell Key Mapping

Default 2x2 layout:

```
u   →   i          u = Upper-left
                   i = Upper-right
j   →   k          j = Lower-left
                   k = Lower-right
```

### Visual Options

| Option            | Type   | Default       | Description             |
| ----------------- | ------ | ------------- | ----------------------- |
| `line_color`      | string | `"#FF8EE2FF"` | Grid line color         |
| `line_width`      | int    | `1`           | Line thickness          |
| `highlight_color` | string | `"#4D00BFFF"` | Selected cell highlight |
| `label_color`     | string | `"#FFFFFFFF"` | Cell label text         |
| `label_font_size` | int    | `10`          | Label size              |

---

## Scroll Mode

Vim-style scrolling at the cursor position.

### When to Use

- **Scrolling documents** without reaching for mouse
- **Code review** in terminal or editors
- **Long web pages** in browsers

### Basic Configuration

| Option             | Type | Default   | Description                  |
| ------------------ | ---- | --------- | ---------------------------- |
| `scroll_step`      | int  | `50`      | Pixels per j/k press         |
| `scroll_step_half` | int  | `500`     | Pixels for Ctrl+D/U          |
| `scroll_step_full` | int  | `1000000` | Pixels for gg/G (top/bottom) |

```toml
[scroll]
scroll_step = 50            # j/k: 50 pixels
scroll_step_half = 500      # Ctrl+D/U: half page
scroll_step_full = 1000000  # gg/G: jump to top/bottom
```

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

| Format         | Example                       | Description              |
| -------------- | ----------------------------- | ------------------------ |
| Single key     | `"j"`, `"k"`, `"g"`           | Direct press             |
| Arrow keys     | `"Up"`, `"Down"`              | Named keys               |
| Modifier combo | `"Ctrl+U"`, `"Cmd+Down"`      | Modifier + key           |
| Special keys   | `"PageUp"`, `"Home"`, `"End"` | Named specials           |
| Multi-key      | `"gg"`                        | Sequence (500ms timeout) |

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

| Option            | Type | Default | Description                |
| ----------------- | ---- | ------- | -------------------------- |
| `move_mouse_step` | int  | `10`    | Pixels per arrow key press |

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

### Configuration

| Option                   | Type | Default | Description                 |
| ------------------------ | ---- | ------- | --------------------------- |
| `scroll_enabled`         | bool | `true`  | Show in scroll mode         |
| `hints_enabled`          | bool | `false` | Show in hints mode          |
| `grid_enabled`           | bool | `false` | Show in grid mode           |
| `recursive_grid_enabled` | bool | `false` | Show in recursive_grid mode |

```toml
[mode_indicator]
# Enable per-mode
scroll_enabled = true
hints_enabled = false
grid_enabled = false
recursive_grid_enabled = false
```

### Appearance

| Option               | Type   | Default       | Description            |
| -------------------- | ------ | ------------- | ---------------------- |
| `font_size`          | int    | `10`          | Text size              |
| `font_family`        | string | `""`          | Font (empty = system)  |
| `background_color`   | string | `"#F2FFD700"` | Background (gold, 95%) |
| `text_color`         | string | `"#FF000000"` | Text (black)           |
| `border_color`       | string | `"#FF000000"` | Border (black)         |
| `border_width`       | int    | `1`           | Border width           |
| `padding`            | int    | `4`           | Internal padding       |
| `border_radius`      | int    | `4`           | Corner radius          |
| `indicator_x_offset` | int    | `20`          | X offset from cursor   |
| `indicator_y_offset` | int    | `20`          | Y offset from cursor   |

---

## Smooth Cursor

Animate cursor movement between positions.

### When to Use

- **Visual tracking** - see where cursor is going
- **Aesthetic preference** - smooth motion looks nicer
- **Large jumps** - helps track movement

### Configuration

| Option               | Type | Default | Description                      |
| -------------------- | ---- | ------- | -------------------------------- |
| `move_mouse_enabled` | bool | `false` | Enable smooth movement           |
| `steps`              | int  | `10`    | Number of intermediate positions |
| `delay`              | int  | `1`     | Milliseconds between steps       |

```toml
[smooth_cursor]
move_mouse_enabled = false  # default (instant)

# When enabled:
move_mouse_enabled = true

# Smooth but slower
steps = 10
delay = 10

# Faster, less smooth
steps = 5
delay = 2
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

> **Note:** Changing this requires restart (`neru config reload` won't work).

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

```toml
# Gold (classic hint color)
background_color = "#F2FFD700"          # FF = opaque, F2 = 95%
background_color = "#FFD700"            # 6-char, fully opaque

# Grid colors
background_color = "#B3ABE9B3"          # Light purple, 70%
matched_background_color = "#B3F8BD96"  # Orange, 70%

# Blue highlight
highlight_color = "#4D00BFFF"           # Deep sky blue, 30%

# White/black
text_color = "#FF000000"                # Black, opaque
label_color = "#FFFFFFFF"               # White, opaque
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
| `disable_file_logging` | bool   | `false`  | Console only         |
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
