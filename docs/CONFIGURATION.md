# Configuration Guide

Neru uses TOML for configuration. This guide covers all available options with examples.

## Configuration File Locations

Neru searches for configuration in the following order:

1. `~/.config/neru/config.toml` (XDG standard - **recommended for dotfiles**)
2. `~/Library/Application Support/neru/config.toml` (macOS convention)
3. Custom path: `neru launch --config /path/to/config.toml`

**No config file?** Neru uses sensible defaults. See [../configs/default-config.toml](../configs/default-config.toml).

---

## Table of Contents

- [Hotkeys](#hotkeys)
- [General Settings](#general-settings)
- [Hint Mode](#hint-mode)
- [Grid Mode](#grid-mode)
- [Scroll Configuration](#scroll-configuration)
- [Smooth Cursor](#smooth-cursor)
- [Logging](#logging)
- [Complete Example](#complete-example)

---

## Hotkeys

Bind global hotkeys to Neru actions. Remove or comment out to disable.

```toml
[hotkeys]
# Hint modes
"Cmd+Shift+Space" = "hints"

# Grid mode
"Cmd+Shift+G" = "grid"

# Scroll
"Cmd+Shift+S" = "action scroll"

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
hint_characters = "asdfghjkl"  # At least 2 distinct characters

# Visual styling
font_size = 12                 # Range: 6-72
font_family = ""               # Empty = system default
border_radius = 4
padding = 4
border_width = 1
opacity = 0.95                 # Range: 0.0-1.0

background_color = "#FFD700"
text_color = "#000000"
matched_text_color = "#737373" # Matched text color - color for characters that have been typed
border_color = "#000000"
```

**Choosing hint characters:**

- Use home row keys for comfort: `"asdfghjkl"`
- Left hand only: `"asdfqwertzxcv"`
- Custom: `"fjdksla"`

### Hint Visibility Options

```toml
[hints]
# Show hints in menubar
include_menubar_hints = false

# Target specific menubar apps (requires include_menubar_hints = true)
additional_menubar_hints_targets = [
    "com.apple.TextInputMenuAgent",
    "com.apple.controlcenter",
    "com.apple.systemuiserver",
]

# Show hints in Dock and Mission Control
include_dock_hints = false

# Show hints in notification popups
include_nc_hints = false
```

### Accessibility Configuration

Define which UI elements are clickable:

```toml
[hints]
# Global clickable roles
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

# ⚠️ Make all elements clickable (use with caution)
ignore_clickable_check = false
```

### Per-App Overrides

Customize accessibility for specific apps:

```toml
[[hints.app_configs]]
bundle_id = "com.google.Chrome"
additional_clickable_roles = ["AXTabGroup"]
ignore_clickable_check = false

[[hints.app_configs]]
bundle_id = "com.adobe.illustrator"
additional_clickable_roles = ["AXStaticText", "AXImage"]
ignore_clickable_check = true
```

**How it works:**

- App-specific `additional_clickable_roles` are **merged** with global roles
- App-specific `ignore_clickable_check` overrides the global setting

### Enhanced Browser Support

Enable improved accessibility for Electron, Chromium, and Firefox apps:

```toml
[hints.additional_ax_support]
enable = false  # Off by default

# Automatically supported (no need to add):
# Electron: VS Code, Windsurf, Cursor, Slack, Spotify, Obsidian
# Chromium: Chrome, Brave, Arc, Helium
# Firefox: Firefox, Zen

# Add custom apps
additional_electron_bundles = ["com.example.electronapp"]
additional_chromium_bundles = ["com.example.custombrowser"]
additional_firefox_bundles = ["com.example.firefoxfork"]
```

**⚠️ Tiling Window Manager Warning:**

Enabling accessibility support for Chromium/Firefox can interfere with tiling window managers (yabai, Amethyst, Aerospace). Symptoms include:

- Windows resist tiling
- Layout glitches
- Windows snap to wrong positions

**Recommendation:** If you use a tiling WM, keep `enable = false` and use grid mode instead.

---

## Grid Mode

Grid mode provides a universal, accessibility-independent way to click anywhere on screen using coordinate-based selection.

### Basic Configuration

```toml
[grid]
enabled = true
characters = "abcdefghijklmnpqrstuvwxyz"

# Visual styling
font_size = 12
font_family = ""
opacity = 0.7
border_width = 1

background_color = "#abe9b3"
text_color = "#000000"
matched_text_color = "#f8bd96"
matched_background_color = "#f8bd96"
matched_border_color = "#f8bd96"
border_color = "#abe9b3"
```

**Cell sizing:** Automatically optimized based on screen resolution. Uses 2-4 character labels with square cells.

### Grid Behavior

```toml
[grid]
live_match_update = true    # Highlight matches as you type
hide_unmatched = true       # Hide non-matching cells while typing

# Subgrid keys (requires at least 9 characters)
sublayer_keys = "abcdefghijklmnpqrstuvwxyz"
```

**Workflow:**

1. Press grid hotkey (e.g., `Cmd+Shift+G`)
2. Type main grid coordinate (2-4 characters)
3. If subgrid enabled, type subgrid position (1 character, a-i)
4. Action executes at selected location

---

## Scroll Configuration

Configure Vim-style scrolling behavior for both standalone scrolling and hint/grid-based scrolling.

```toml
[scroll]
# Scroll amounts
scroll_step = 50              # j/k keys
scroll_step_half = 500        # Ctrl+D/U
scroll_step_full = 1000000    # gg/G (top/bottom)

# Visual feedback
highlight_scroll_area = true
highlight_color = "#FF0000"
highlight_width = 2
```

### Scroll Keys

- `j` / `k` - Scroll down/up
- `h` / `l` - Scroll left/right
- `Ctrl+d` / `Ctrl+u` - Half-page down/up
- `gg` - Jump to top (press `g` twice)
- `G` - Jump to bottom
- `Esc` - Exit scroll mode

### Scroll Modes

**Standalone scroll** (`neru action scroll`):

- Scrolls at current cursor position
- No location selection required
- Only `Esc` to exit

**Hint/Grid scroll** (`neru hints scroll` or `neru grid scroll`):

- Select location first, then scroll
- Press `Esc` to exit

---

## Action

Action mode is a special mode where you can toggle using <tab> when in hints or grid mode.

### Basic Configuration

```toml
[action]
highlight_color = "#00FF00"
highlight_width = 3

# Action key mappings
left_click_key = "l"
right_click_key = "r"
middle_click_key = "m"
mouse_down_key = "i"
mouse_up_key = "u"
```

**Action key mappings:**

- `left_click_key` - Left click at cursor position
- `right_click_key` - Right click at cursor position
- `middle_click_key` - Middle click at cursor position
- `mouse_down_key` - Hold mouse button at cursor position
- `mouse_up_key` - Release mouse button at cursor position

### Action Behavior

```toml
[action]
# Action mode highlight appearance
highlight_color = "#00FF00"
highlight_width = 3
```

**Action mode highlight appearance:**

- Color and width can be customized
- Color can be any hex color (e.g., `#FF0000`)
- Width can be any number (e.g., `2`)

---

## Smooth Cursor

Configure smooth cursor movement for mouse operations:

```toml
[smooth_cursor]
move_mouse_enabled = true  # Enable smooth mouse movement
steps = 10                 # Number of steps for smooth movement
delay = 1                   # Delay between steps in milliseconds
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
"Cmd+Shift+S" = "action scroll"

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
font_size = 12
opacity = 0.7
background_color = "#abe9b3"
text_color = "#000000"
matched_text_color = "#f8bd96"
matched_background_color = "#f8bd96"
matched_border_color = "#f8bd96"
border_color = "#abe9b3"
live_match_update = true
subgrid_enabled = true
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

[logging]
log_level = "info"
structured_logging = true
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

---

## Next Steps

- See [CLI.md](CLI.md) for command-line usage
- Check [TROUBLESHOOTING.md](TROUBLESHOOTING.md) if configs aren't working
- Review [default-config.toml](../configs/default-config.toml) for all options
