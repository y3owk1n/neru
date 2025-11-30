# Configuration Guide

Neru uses TOML for configuration. This guide covers all available options with examples.

## Configuration File Locations

Neru searches for configuration in the following order:

1. `~/.config/neru/config.toml` (XDG standard - **recommended for dotfiles**)
2. `~/Library/Application Support/neru/config.toml` (macOS convention)
3. Custom path: `neru launch --config /path/to/config.toml`

**No config file?** Neru uses sensible defaults. See [default-config.toml](../configs/default-config.toml). Need help troubleshooting? See [TROUBLESHOOTING.md](TROUBLESHOOTING.md).

---

## Table of Contents

- [Configuration Overview](#configuration-overview)
- [Hotkeys](#hotkeys)
- [General Settings](#general-settings)
- [Hint Mode](#hint-mode)
- [Grid Mode](#grid-mode)
- [Scroll Mode](#scroll-mode)
- [Action Mode](#action-mode)
- [Advanced Settings](#advanced-settings)
- [Complete Example](#complete-example)

---

## Configuration Overview

Neru uses TOML configuration files. Configuration is loaded from:

1. `~/.config/neru/config.toml` (recommended)
2. `~/Library/Application Support/neru/config.toml`
3. Custom path via `--config` flag

**No config file?** Neru uses sensible defaults. Copy from `configs/default-config.toml` to get started.

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

# Scroll
"Cmd+Shift+S" = "scroll"

# these keys might not work and conflict with system or apps, change them!
# "Cmd+Shift+L" = "action left_click"
# "Cmd+Shift+R" = "action right_click"
# "Cmd+Shift+M" = "action middle_click"
# "Cmd+Shift+N" = "action mouse_down"
# "Cmd+Shift+P" = "action mouse_up"

# Execute shell commands
# "Cmd+Alt+T" = "exec open -a Terminal"
# "Cmd+Alt+N" = "exec osascript -e 'display notification \"Hello!\" with title \"Neru\"'"
```

### Hotkey Syntax

**Modifiers:** `Cmd`, `Ctrl`, `Alt`/`Option`, `Shift`
**Format:** `"Modifier1+Modifier2+Key" = "action"`

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
include_menubar_hints = false
include_dock_hints = false
include_nc_hints = false

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
clickable_roles = [
     "AXButton", "AXLink", "AXTextField", "AXCheckBox",
     "AXComboBox", "AXRadioButton", "AXPopUpButton",
     "AXSlider", "AXTabButton", "AXSwitch"
]

ignore_clickable_check = false  # Make all elements clickable
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
```

### Enhanced Browser Support

Enable improved accessibility for Electron/Chromium/Firefox apps:

```toml
[hints.additional_ax_support]
enable = false  # ⚠️ May conflict with tiling WMs

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
characters = "abcdefghijklmnpqrstuvwxyz"
sublayer_keys = "abcdefghijklmnpqrstuvwxyz"

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

---

## Scroll Mode

Vim-style scrolling for standalone navigation:

```toml
[scroll]
scroll_step = 50           # j/k keys
scroll_step_half = 500     # Ctrl+D/U
scroll_step_full = 1000000 # gg/G (top/bottom)

highlight_scroll_area = true
highlight_color = "#FF0000"
highlight_width = 2
```

**Keys:** `j/k` (up/down), `h/l` (left/right), `Ctrl+d/u` (half-page), `gg/G` (top/bottom), `Esc` (exit)

## Action Mode

Interactive action mode for mouse operations (enter with `neru action` or `Tab` in hint/grid mode):

```toml
[action]
highlight_color = "#00FF00"
highlight_width = 3

# Key mappings
left_click_key = "l"
right_click_key = "r"
middle_click_key = "m"
mouse_down_key = "i"
mouse_up_key = "u"
```

## Advanced Settings

### Smooth Cursor

Configure smooth mouse movement:

```toml
[smooth_cursor]
move_mouse_enabled = true
steps = 10        # Intermediate positions
delay = 1         # Milliseconds between steps
```

### Metrics

Enable performance metrics collection:

```toml
[metrics]
enabled = false   # Access via `neru metrics` command
```

### Logging

Configure logging behavior:

```toml
[logging]
log_level = "info"          # debug, info, warn, error
log_file = ""               # Default: ~/Library/Logs/neru/app.log
structured_logging = true   # JSON format
max_file_size = 10          # MB
max_backups = 5
max_age = 30                # Days
```

**Debug logging:** Set `log_level = "debug"` and run `tail -f ~/Library/Logs/neru/app.log`

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
"Cmd+Shift+L" = "action left_click"
"Cmd+Shift+R" = "action right_click"
"Cmd+Shift+M" = "action middle_click"
"Cmd+Shift+N" = "action mouse_down"
"Cmd+Shift+P" = "action mouse_up"
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
clickable_roles = ["AXButton", "AXLink", "AXTextField", "AXCheckBox"]
ignore_clickable_check = false

[[hints.app_configs]]
bundle_id = "com.google.Chrome"
additional_clickable_roles = ["AXTabGroup"]

[hints.additional_ax_support]
enable = false

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
highlight_color = "#00FF00"
highlight_width = 3

left_click_key = "l"
right_click_key = "r"
middle_click_key = "m"
mouse_down_key = "i"
mouse_up_key = "u"

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

---

## Tips

**Version control your config:**

```bash
cd ~/.config/neru
git init
git add config.toml
git commit -m "Initial Neru configuration"
```

**Share with others:**

```bash
# Export
cp ~/.config/neru/config.toml ~/Downloads/neru-config.toml

# Import
cp ~/Downloads/neru-config.toml ~/.config/neru/config.toml
neru launch  # Restart to apply
```

**Test changes:**

1. Edit `~/.config/neru/config.toml`
2. Reload: `neru config reload` (or use "Reload Config" from systray menu)
3. Test your changes
