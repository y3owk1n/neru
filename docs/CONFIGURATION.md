# Configuration Guide

Neru uses TOML for configuration. This guide covers all available options with examples.

---

## Table of Contents

- [Configuration Overview](#configuration-overview)
- [Hotkeys](#hotkeys)
- [General Settings](#general-settings)
- [Keyboard Layout Requirements](#keyboard-layout-requirements)
- [Hint Mode](#hint-mode)
- [Grid Mode](#grid-mode)
- [Quad-Grid Mode](#quad-grid-mode)
- [Scroll Mode](#scroll-mode)
- [Mouse Movement Actions](#mouse-movement-actions)
- [Smooth Cursor](#smooth-cursor)
- [Metrics](#metrics)
- [Systray](#systray)
- [Logging](#logging)
- [Complete Example](#complete-example)

---

## Configuration Overview

Neru uses TOML configuration files. Configuration is loaded from:

1. `~/.config/neru/config.toml` (recommended)
2. `~/Library/Application Support/neru/config.toml`
3. Custom path via `--config` flag

**No config file?** Neru uses sensible defaults. Copy from [default-config.toml](../configs/default-config.toml) to get started.

**Reload config:** Use `neru config reload` or restart the app.

---

## Hotkeys

Bind global hotkeys to Neru actions. Remove or comment out to disable.

> **⚠️ Important:** Providing any custom hotkeys **replaces all defaults**. If you define even one custom hotkey, you must explicitly define all hotkeys you want to use. The defaults (Cmd+Shift+Space, Cmd+Shift+G, Cmd+Shift+S) will be disabled.

```toml
[hotkeys]
# Hint modes
"Cmd+Shift+Space" = "hints"

# Grid mode
"Cmd+Shift+G" = "grid"

# Quad-Grid mode
"Cmd+Shift+C" = "quadgrid"

# Scroll
"Cmd+Shift+S" = "scroll"

# Execute shell commands
# "Cmd+Alt+T" = "exec open -a Terminal"
# "Cmd+Alt+N" = "exec osascript -e 'display notification \"Hello!\" with title \"Neru\"'"
```

### Hotkey Syntax

**Modifiers:** `Cmd`, `Ctrl`, `Alt`/`Option`, `Shift`
**Format:** `"Modifier1+Modifier2+Key" = "action"`

An `action` can be one of the following:

- A Neru command (essentially any Neru CLI command without the `neru` prefix)
  - Say in CLI you can run `neru hints` and `neru grid`
  - To map it into a hotkey, just omit the `neru` prefix: `"Cmd+Shift+Space" = "hints"`
- A shell command (using `exec <command>`)
  - Say in CLI you can run `open -a Terminal` to launch Terminal
  - To map it into a hotkey, use the full command: `"Cmd+Alt+T" = "exec open -a Terminal"`

**Shell Commands:**

```toml
"Cmd+Alt+T" = "exec open -a Terminal"
"Cmd+Alt+C" = "exec open -a 'Visual Studio Code'"
"Cmd+Alt+S" = "exec ~/scripts/screenshot.sh"
```

**Alternative:** Use [skhd](https://github.com/koekeishiya/skhd) for hotkey management:

```bash
# ~/.config/skhd/skhdrc
ctrl - f : neru hints
ctrl - g : neru grid
```

---

## General Settings

```toml
[general]
# Exclude apps by bundle ID
excluded_apps = [
    "com.apple.Terminal",
    "com.googlecode.iterm2",
    "com.microsoft.rdc.macos",
]
accessibility_check_on_start = true
restore_cursor_position = true
mode_exit_keys = ["escape"]
```

**Cursor restoration:**

- When `restore_cursor_position = true`, the cursor returns to its original coordinates after exiting any mode.
- Handles screen resolution changes by restoring proportionally within the current active screen.
- Default is `false` if omitted.

**Finding Bundle IDs:**

```bash
osascript -e 'id of app "Safari"'  # com.apple.Safari
osascript -e 'id of app "VS Code"' # com.microsoft.VSCode
```

**Mode Exit Keys:**

Use `general.mode_exit_keys` to specify which keys should exit any active mode (hints/grid/scroll).

Examples:

```toml
[general]
mode_exit_keys = ["escape"]                # default: Escape key
# mode_exit_keys = ["Ctrl+C"]              # Ctrl+C to exit
# mode_exit_keys = ["escape", "Ctrl+C"]    # accept either Escape or Ctrl+C
```

Format notes:

- Use plain text key names: `escape`, `return`, `tab`, `space`, `backspace`, `delete`, `home`, `end`, `pageup`, `pagedown`
- Modifiers: `Cmd`, `Ctrl`, `Alt`/`Option`, `Shift`
- Modifier combos: `Modifier+Key` (e.g. `Ctrl+R`, `Cmd+Shift+Space`)

---

## Keyboard Layout Requirements

Neru uses direct keycode-to-character mapping for keyboard input. This approach has specific requirements and limitations:

**Supported Layout:** US QWERTY only

Neru is designed to work with the standard US QWERTY physical keyboard layout. This includes:

- All letter keys (a-z)
- Number row and symbols (`1` through `0`)
- Punctuation keys (`-`, `=`, `[`, `]`, `\`, `;`, `'`, `,`, `.`, `/`)
- Space bar and modifiers (Cmd, Option/Alt, Control, Shift)

### Input Method Independence

Neru intentionally bypasses macOS input methods (such as Chinese, Japanese, and Korean IME systems). This ensures:

- Direct character input without input method interference
- Consistent behavior across all applications
- No unexpected character conversion

### Unsupported Layouts

The following keyboard layouts are **not supported**:

- AZERTY (French)
- QWERTZ (German, Swiss)
- Dvorak
- Colemak
- Any other non-US layouts

Users with these physical keyboard layouts will experience incorrect character output.

---

## Hint Mode

Hint mode overlays clickable labels on UI elements using macOS accessibility APIs.

### Basic Configuration

```toml
[hints]
enabled = true
hint_characters = "asdfghjkl"  # Home row keys recommended

# Visual styling
font_size = 12
font_family = ""               # System default
border_radius = 4
padding = 4
opacity = 0.95

background_color = "#FFD700"
text_color = "#000000"
matched_text_color = "#737373"
border_color = "#000000"
```

### Visibility Options

```toml
[hints]
# Show hints in system areas
include_menubar_hints = false # Menubar
include_dock_hints = false # Dock
include_nc_hints = false # Notification Center
include_stage_manager_hints = false # Stage Manager

# Target specific menubar apps
additional_menubar_hints_targets = [
     "com.apple.TextInputMenuAgent",
     "com.apple.controlcenter",
]
```

### Clickable Elements

Define which UI elements are clickable:

```toml
[hints]
# Clickable roles that should generate hints
clickable_roles = [
     "AXButton", "AXLink", "AXTextField", "AXCheckBox",
     "AXComboBox", "AXRadioButton", "AXPopUpButton",
     "AXSlider", "AXTabButton", "AXSwitch"
]

# We have a heuristic to detect if an element is clickable to avoid noises
# If you do not want to use this heuristic, set ignore_clickable_check = true
ignore_clickable_check = false
```

### Refresh Hints with delay after mouse action

This option is useful for applications like browsers that needs to wait for a bit to get the latest content.

```toml
[hints]
# Delay (in milliseconds) before refreshing hints after mouse click actions.
# Set to 0 for immediate refresh (default). Higher values reduce rapid refreshes
# when performing multiple clicks quickly. Maximum: 10000 (10 seconds).
# You can also override this for specific apps in the [app_configs] section below.
mouse_action_refresh_delay = 0

```

### Per-App Overrides

```toml
[[hints.app_configs]]
bundle_id = "com.google.Chrome"
additional_clickable_roles = ["AXTabGroup"]

[[hints.app_configs]]
bundle_id = "com.adobe.illustrator"
additional_clickable_roles = ["AXStaticText", "AXImage"]
ignore_clickable_check = true

# Override mouse action refresh delay for specific apps.
# Omit mouse_action_refresh_delay to use the global hints.mouse_action_refresh_delay.
# Set to 0 for immediate refresh, or a positive value for delay in milliseconds.
[[hints.app_configs]]
bundle_id = "com.apple.Safari"
mouse_action_refresh_delay = 1000 # refresh hints after 1000ms / 1 second
```

### Enhanced Browser Support

Enable improved accessibility for Electron/Chromium/Firefox apps:

```toml
[hints.additional_ax_support]
enable = false

# Auto-detected: VS Code, Chrome, Firefox, Slack, etc.
additional_electron_bundles = ["com.example.app"]
additional_chromium_bundles = ["com.example.browser"]
additional_firefox_bundles = ["com.example.firefox"]
```

---

## Grid Mode

Grid mode provides accessibility-independent navigation using coordinate-based selection.

### Basic Configuration

```toml
[grid]
enabled = true

# Characters to use for grid labels
# Requirements:
# - Must contain at least 2 characters
# - Only ASCII characters allowed (no Unicode)
# - Cannot contain ',' (reserved for reset)
# - No duplicate characters (case-insensitive)
characters = "abcdefghijklmnpqrstuvwxyz"

# Characters to use for subgrid labels (fallback to grid.characters)
# Same requirements as characters above
sublayer_keys = "abcdefghijklmnpqrstuvwxyz"

reset_key = "," # hotkey to reset/clear grid input

# Optional custom labels for rows and columns
# If not provided, labels will be inferred from characters
# Requirements (when specified):
# - Must contain at least 2 characters
# - Only ASCII characters allowed
# - Cannot contain ',' (reserved for reset)
# - No duplicate characters (case-insensitive)
# - Can include symbols for advanced keyboard layouts
# row_labels = "123456789"      # Numbers for rows
# col_labels = "abcdefghij"     # Letters for columns
# row_labels = "'.pyfgcrl/"     # Symbols for rows
# col_labels = "aoeuidhtns"     # Dvorak-style for columns

# Visual styling
font_size = 12
opacity = 0.7

background_color = "#abe9b3"
text_color = "#000000"
matched_text_color = "#f8bd96"
border_color = "#abe9b3"
```

### Grid Behavior

```toml
[grid]
live_match_update = true  # Highlight matches as you type
hide_unmatched = true     # Hide non-matching cells
prewarm_enabled = true    # Prewarm grid caches on startup for faster first use, but uses ~1.5MB memory and CPU at startup. Disable to reduce startup overhead.
enable_gc = false         # Enable periodic garbage collection every 5 minutes to reduce peak memory usage, but adds CPU overhead. Enable if memory is a concern.
```

**Workflow:** Press grid hotkey → Type coordinates → Action executes

### Reset Key

Use `grid.reset_key` to set the key that clears current grid input. It can be a single character (`,` by default) or a modifier combo.

Examples:

```toml
[grid]
reset_key = ","        # default behavior: comma resets current grid input
reset_key = "Ctrl+R"   # use Ctrl+R to reset instead
```

Notes:

- Single-character reset keys are reserved and must not appear in `grid.characters` or label sets.
- Modifier combos (e.g. `Ctrl+R`) do not conflict with single-character grid labels.

---

## Quad-Grid Mode

Quad-grid provides recursive quadrant-based navigation that works anywhere. The screen is divided into four quadrants using keys (default: `u`, `i`, `j`, `k`). Each selection narrows the active area. The reset key returns to the initial center, and backspace/delete move up one depth and recenter.

### Basic Configuration

```toml
[quad_grid]
enabled = true

# Quadrant keys (must be exactly 4 unique ASCII characters)
# u = upper-left, i = upper-right, j = lower-left, k = lower-right
keys = "uijk"

# Behavior
min_size = 25        # Minimum quadrant size in pixels
max_depth = 10       # Maximum recursion depth
reset_key = ","      # Reset to initial center (can be modifier combo like "Ctrl+R")

# Visual styling
line_color = "#8EE2FF"
line_width = 1
highlight_color = "#00BFFF"
highlight_opacity = 0.3
label_color = "#FFFFFF"
label_font_size = 12
label_font_family = "SF Mono"
```

### Key Behavior

- Press quadrant key to narrow selection and preview cursor at center
- Press backspace/delete to move up one depth and recenter cursor
- Press reset_key to return to initial center and clear state
- Press exit key (default: escape) to exit mode

---

## Scroll Mode

Vim-style scrolling with fully configurable keybindings:

```toml
[scroll]
scroll_step = 50           # j/k keys
scroll_step_half = 500     # Ctrl+D/U
scroll_step_full = 1000000 # gg/G (top/bottom)

# Scroll indicator styling
font_size = 12
font_family = "SF Mono"
opacity = 0.95
background_color = "#FFD700"
text_color = "#000000"
border_color = "#000000"
border_width = 1
padding = 4
border_radius = 4
indicator_x_offset = 20
indicator_y_offset = 20
```

### Customizable Key Bindings

Configure which keys trigger each scroll action. Each action can have multiple keys:

```toml
[scroll.key_bindings]
# Movement
scroll_up = ["k", "Up"]      # Scroll up by one line
scroll_down = ["j", "Down"]  # Scroll down by one line
scroll_left = ["h", "Left"]  # Scroll left by one line
scroll_right = ["l", "Right"] # Scroll right by one line

# Navigation
go_top = ["gg", "Cmd+Up"]    # Go to top of page
go_bottom = ["G", "Cmd+Down"] # Go to bottom of page

# Page movement
page_up = ["Ctrl+U", "PageUp"]   # Scroll up by half page
page_down = ["Ctrl+D", "PageDown"] # Scroll down by half page
```

**Key Format Options:**

| Format             | Example                                     | Description                            |
| ------------------ | ------------------------------------------- | -------------------------------------- |
| Single key         | `"j"`, `"k"`                                | Direct key press                       |
| Arrow keys         | `"Up"`, `"Down"`                            | Named arrow keys                       |
| Modifier combo     | `"Ctrl+U"`, `"Cmd+Down"`                    | Modifier + key                         |
| Special keys       | `"PageUp"`, `"PageDown"`, `"Home"`, `"End"` | Named special keys                     |
| Multi-key sequence | `"gg"`                                      | Press keys in sequence (500ms timeout) |

**Multi-key Sequences:**

- Must be exactly 2 letters (a-z, A-Z)
- Case-insensitive (both "gg" and "GG" work)
- 500ms timeout between key presses
- Example: `"gg"` for go to top

**Supported Modifiers:**

- `Cmd`/`Command` - Command key
- `Ctrl`/`Control` - Control key
- `Alt`/`Option` - Option/Alt key
- `Shift` - Shift key

**Default Bindings:**

| Action       | Keys                 |
| ------------ | -------------------- |
| scroll_up    | `k`, `Up`            |
| scroll_down  | `j`, `Down`          |
| scroll_left  | `h`, `Left`          |
| scroll_right | `l`, `Right`         |
| go_top       | `gg`, `Cmd+Up`       |
| go_bottom    | `G`, `Cmd+Down`      |
| page_up      | `Ctrl+U`, `PageUp`   |
| page_down    | `Ctrl+D`, `PageDown` |

**Customization Examples:**

```toml
# Vim-style with arrow keys
# [scroll.key_bindings]
# scroll_up = ["k", "Up", "Ctrl+P"]
# scroll_down = ["j", "Down", "Ctrl+N"]
# go_top = ["gg", "Home"]
# go_bottom = ["G", "End"]

# Home/End for navigation
# [scroll.key_bindings]
# scroll_up = ["k"]
# scroll_down = ["j"]
# scroll_left = ["h"]
# scroll_right = ["l"]
# go_top = ["gg", "Home"]
# go_bottom = ["G", "End"]
# page_up = ["PageUp"]
# page_down = ["PageDown"]

# Mac-style with Command arrows for navigation
# [scroll.key_bindings]
# scroll_up = ["k", "Up"]
# scroll_down = ["j", "Down"]
# go_top = ["gg", "Cmd+Up"]
# go_bottom = ["G", "Cmd+Down"]
```

**Exit:** Press `Esc` to exit scroll mode.

## Mouse Movement Actions

Move the cursor using keyboard keys in hints or grid mode. Configure the step size and key bindings:

```toml
[action]
move_mouse_step = 10  # Pixels to move per key press (default: 10)

[action.key_bindings]
left_click = "Shift+L"       # Left click at cursor position
right_click = "Shift+R"      # Right click at cursor position
middle_click = "Shift+M"     # Middle click at cursor position
mouse_down = "Shift+I"       # Press and hold mouse button
mouse_up = "Shift+U"         # Release mouse button
move_mouse_up = "Up"         # Move cursor up by move_mouse_step
move_mouse_down = "Down"     # Move cursor down by move_mouse_step
move_mouse_left = "Left"     # Move cursor left by move_mouse_step
move_mouse_right = "Right"   # Move cursor right by move_mouse_step
```

### Arrow Key Movement

The arrow keys (`Up`, `Down`, `Left`, `Right`) move the cursor by `move_mouse_step` pixels in the corresponding direction. The cursor stays at the new position after exiting the mode (unless `restore_cursor_position` is enabled in `[general]`).

**Customizing step size:**

```toml
[action]
move_mouse_step = 5   # Smaller movements (more precise)
# move_mouse_step = 20  # Larger movements (faster navigation)
```

### Valid Key Formats

| Format              | Example                     | Description                  |
| ------------------- | --------------------------- | ---------------------------- |
| Modifier + alphabet | `Cmd+L`, `Shift+R`          | At least 1 modifier + letter |
| Modifier + special  | `Shift+Return`, `Cmd+Enter` | Modifier + Return/Enter      |
| Single special      | `Return`, `Enter`           | Standalone special key       |

### Supported Modifiers

- `Cmd`/`Command` - Command key
- `Ctrl`/`Control` - Control key
- `Alt`/`Option` - Option/Alt key
- `Shift` - Shift key

**Usage:** Press the configured key in hints or grid mode to perform the action at the current cursor position.

---

## Smooth Cursor

Configure smooth mouse movement, note that this is refering the when a mouse move from a point to another point, enabled
means you can see it moves, where disabled means it will show on another point instantly:

```toml
[smooth_cursor]
move_mouse_enabled = true
steps = 10        # Intermediate positions
delay = 1         # Milliseconds between steps
```

**Parameters:**

- `move_mouse_enabled`: When `true`, mouse movements will be smooth instead of instant
- `steps`: Number of intermediate positions between start and end (higher = smoother but slower)
- `delay`: Time in milliseconds between each step (lower = faster but less smooth)

**Example configurations:**

```toml
# Very smooth, slow movement
[smooth_cursor]
move_mouse_enabled = true
steps = 10
delay = 10

# Fast, less smooth movement
[smooth_cursor]
move_mouse_enabled = true
steps = 5
delay = 2

# Disable smooth movement entirely
[smooth_cursor]
move_mouse_enabled = false
```

---

## Metrics

Configure application metrics collection:

```toml
[metrics]
enabled = false  # Enable metrics collection
```

**When enabled:**

- Metrics are exposed via the `neru metrics` command
- Tracks accessibility element counts and other performance data
- Disabled by default to reduce overhead

---

## Systray

Configure system tray behavior:

```toml
[systray]
enabled = true # Enable system tray icon (set to false for headless mode)
```

**Headless Mode:**

- When `enabled = false`, Neru runs without a menu bar icon.
- You can still control the application via hotkeys and the CLI.
- Useful for minimal setups or when using other status bar tools.

> [!NOTE]
> Changing this setting requires a restart to take effect (`neru config reload` is not sufficient).

---

## Logging

```toml
[logging]
log_level = "info"          # "debug", "info", "warn", "error"
log_file = ""               # Empty = ~/Library/Logs/neru/app.log
structured_logging = true   # JSON format for file logs
disable_file_logging = false # Disable file logging
max_file_size = 10          # Max file size in MB
max_backups = 5             # Max number of backups
max_age = 30                # Max age in days
```

**Default location:** `~/Library/Logs/neru/app.log`

**Debug logging:**

```toml
[logging]
log_level = "debug"
```

Then check logs:

```bash
tail -f ~/Library/Logs/neru/app.log
```

---

## Complete Example

A full configuration example:

```toml
# ~/.config/neru/config.toml

[hotkeys]
"Cmd+Shift+Space" = "hints"
"Cmd+Shift+G" = "grid"
"Cmd+Shift+S" = "scroll"

[general]
excluded_apps = ["com.apple.Terminal", "com.googlecode.iterm2"]
accessibility_check_on_start = true
restore_cursor_position = false

[hints]
enabled = true
hint_characters = "asdfghjkl"
font_size = 14
border_radius = 6
padding = 5
opacity = 0.9
background_color = "#FFD700"
text_color = "#000000"
matched_text_color = "#737373"
border_color = "#000000"
include_menubar_hints = false
include_dock_hints = false
include_nc_hints = false
include_stage_manager_hints = false
clickable_roles = ["AXButton", "AXLink", "AXTextField", "AXCheckBox"]
ignore_clickable_check = false
mouse_action_refresh_delay = 0

[[hints.app_configs]]
bundle_id = "com.google.Chrome"
additional_clickable_roles = ["AXTabGroup"]
mouse_action_refresh_delay = 0

[hints.additional_ax_support]
enable = false

[systray]
enabled = true

[grid]
enabled = true
characters = "abcdefghijklmnpqrstuvwxyz"
sublayer_keys = "abcdefghijklmnpqrstuvwxyz"
font_size = 12
opacity = 0.7
background_color = "#abe9b3"
text_color = "#000000"
matched_text_color = "#f8bd96"
matched_background_color = "#f8bd96"
matched_border_color = "#f8bd96"
border_color = "#abe9b3"
live_match_update = true
hide_unmatched = true

[scroll]
scroll_step = 60
scroll_step_half = 400
scroll_step_full = 1000000
highlight_scroll_area = true
highlight_color = "#00FF00"
highlight_width = 3

[action]
move_mouse_step = 10

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

[smooth_cursor]
move_mouse_enabled = true
steps = 10
delay = 1

[metrics]
enabled = false

[logging]
log_level = "info"
structured_logging = true
disable_file_logging = false
max_file_size = 10
max_backups = 5
max_age = 30
```
