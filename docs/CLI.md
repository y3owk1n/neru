# CLI Usage

Neru provides a comprehensive command-line interface for controlling the daemon and automating workflows.

## Overview

The Neru CLI communicates with the daemon via IPC (Inter-Process Communication) using Unix domain sockets in the OS temporary directory. This enables:

- Fast, reliable communication
- Multiple concurrent CLI commands
- Proper error handling
- Scriptable workflows

---

## Table of Contents

- [Quick Reference](#quick-reference)
- [Daemon Control](#daemon-control)
- [Screen Sharing](#screen-sharing)
- [Service Management](#service-management)
- [Navigation Commands](#navigation-commands)
  - [Basic Navigation](#basic-navigation)
  - [Hints Mode](#hints-mode)
  - [Grid Mode](#grid-mode)
  - [Recursive-Grid Mode](#recursive-grid-mode)
  - [Scroll Mode](#scroll-mode)
- [Action Commands](#action-commands)
- [Configuration Management](#configuration-management)
- [Status & Info](#status--info)
- [Shell Integration](#shell-integration)
- [Scripting](#scripting)
- [Technical Details](#technical-details)
- [Troubleshooting](#troubleshooting)

---

## Quick Reference

**Common Commands:**

```bash
neru launch              # Start daemon
neru status              # Check status
neru services install    # Install launchd service
neru services status     # Check service status
neru hints               # Start hint mode
neru grid                # Start grid mode
neru recursive_grid      # Start recursive-grid mode
neru scroll              # Start scroll mode
neru action left_click   # Click at cursor (immediate)
neru config init         # Create default config file
neru config validate     # Validate config file
neru config reload       # Reload config
neru doctor              # Run system diagnostics
```

**Screen Sharing:**

- `neru toggle-screen-share` - Toggle overlay visibility in screen sharing

**Navigation:**

- `neru hints` - Show clickable hints
- `neru grid` - Show coordinate grid
- `neru recursive_grid` - Recursive grid selection
- `neru scroll` - Vim-style scrolling

**Actions:**

- `neru action left_click` - Immediate left click at cursor
- `neru action right_click` - Immediate right click at cursor
- `neru hints --action right_click` - Right-click via hints
- `neru grid --action left_click` - Left-click via grid

**Configure hotkeys:** See [CONFIGURATION.md](CONFIGURATION.md#hotkeys)

---

## Daemon Control

```bash
neru launch                                # Start daemon
neru launch --config /path/to/config.toml  # Start with custom config file
# or shorter:
neru launch -c ./configs/author-config.toml
neru start                                 # Resume paused daemon
neru stop                                  # Pause daemon (keep running)
neru idle                                  # Cancel active mode
```

**Options:**

- `--config, -c` - Path to custom config file
- `--timeout` - IPC timeout in seconds (default: 5)

**Use `stop`** for temporary disable, **`idle`** to cancel modes.

---

## Screen Sharing

Control overlay visibility during screen sharing (Zoom, Google Meet, OBS, etc.):

```bash
neru toggle-screen-share     # Toggle overlay visibility in screen sharing
```

**Behavior:**

- When toggled to **hidden**, the overlay will not appear in shared screens but remains visible locally
- When toggled to **visible**, the overlay appears normally in screen sharing
- The state resets to **visible** on Neru restart
- Also accessible via system tray menu: "Screen Share: Visible/Hidden"

> [!NOTE]
> This feature uses macOS `NSWindow.sharingType` API (deprecated and no better workaround now yet). Effectiveness varies by screen sharing application:

- Works reliably on macOS 14 and earlier with all applications
- Limited effectiveness on macOS 15.4+ with modern screen capture (ScreenCaptureKit)
- Always test with your specific video conferencing software

---

## Service Management

Manage Neru as a system service for automatic startup:

### macOS (`launchd`)

```bash
neru services install     # Install and load launchd service (~/Library/LaunchAgents)
neru services status      # Check service status
```

### Linux (`systemd`)

(Planned) Neru will support `systemctl` for managing user services.

### Windows (`Task Scheduler`)

(Planned) Neru will support Windows Task Scheduler for auto-start on login.

**Installation:** Sets up Neru to start automatically on login.

> [!NOTE]
> If you have Neru installed via other methods (Nix, Homebrew), use their respective service managers (e.g., `home-manager`).

---

## Action Commands

Perform actions at current cursor position (subcommands only):

```bash
neru action left_click          # Left click
neru action right_click         # Right click
neru action middle_click        # Middle click
neru action mouse_down          # Hold mouse button
neru action mouse_up            # Release mouse button

# With modifier keys
neru action left_click --modifier cmd          # Cmd+click (e.g. open in new tab)
neru action left_click --modifier shift        # Shift+click (e.g. extend selection)
neru action left_click --modifier cmd,shift    # Cmd+Shift+click
neru action right_click --modifier alt         # Alt+right-click

# Mouse movement (absolute coordinates)
neru action move_mouse --x 500 --y 300

# Mouse movement to screen center
neru action move_mouse --center

# Mouse movement to screen center with offset (both axes)
neru action move_mouse --center --x 50 --y -30

# Mouse movement to screen center with single-axis offset (omitted axis defaults to 0)
neru action move_mouse --center --x 100

# Mouse movement to center of a specific monitor (by display name)
neru action move_mouse --center --monitor "DELL U2720Q"

# Mouse movement to center of a specific monitor with offset
neru action move_mouse --center --monitor "Built-in Retina Display" --x 50 --y -30

# Mouse movement (relative from current position)
neru action move_mouse_relative --dx 10 --dy -5
```

**Modifier option (click/mouse actions only):**

- `--modifier <keys>` - Comma-separated modifier keys to hold during the action
- Valid modifiers: `cmd` (or `command`), `shift`, `alt` (or `option`), `ctrl` (or `control`)
- Example: `--modifier cmd,shift`

**Mouse movement options:**

- `move_mouse --x <pixels> --y <pixels>` - Move to absolute screen coordinates
- `move_mouse --center` - Move to center of active screen
- `move_mouse --center --x <pixels> --y <pixels>` - Move to center with offset (each is optional, defaults to 0)
- `move_mouse --center --monitor <name>` - Move to center of named monitor (case-insensitive)
- `move_mouse --center --monitor <name> --x <pixels> --y <pixels>` - Move to center of named monitor with offset
- `move_mouse_relative --dx <pixels> --dy <pixels>` - Move by delta from current position

> [!TIP]
> Monitor names are the localized display names reported by macOS (e.g. "Built-in Retina Display", "DELL U2720Q"). You can find your display names in **System Settings → Displays**.
> If you use the wrong name, the error message will list all available monitor names so you can copy the correct one.

---

## Navigation Commands

Neru provides four navigation modes: hints, grid, recursive-grid, and scroll. Each mode can be activated via CLI or hotkey.

### Basic Navigation

```bash
neru hints              # Show clickable hints on UI elements
neru grid               # Show coordinate grid for screen
neru recursive_grid     # Recursive cell-based navigation
neru scroll             # Vim-style scrolling
```

**Using the `--action` flag:**

All navigation modes support an `--action` or `-a` flag to perform an action immediately upon selection:

```bash
neru hints --action left_click           # Left-click via hints
neru hints --action right_click          # Right-click via hints
neru hints --action middle_click         # Middle-click via hints
neru grid --action left_click            # Left-click via grid
neru grid --action right_click           # Right-click via grid
neru recursive_grid --action left_click  # Left-click via recursive-grid
```

**Behavior:** When `--action` is specified, the action executes automatically when a location is selected, then the mode exits.

#### How `--action` flag works in detail in different modes

This flag was introduced to imitate workflow of `vimium` style. When you are in the hint mode with `--action`, once you satifies the label (e.g. AA), it will perform the action directly.

This flag is exceptionally useful for `hints` mode, but not for other modes. Right now grid mode with `--action` is a little bit silly.

- Grid: it will perform the click after the last selection of sublayer (3x3)
- Recursive Grid: it will perform the click after the last selection of the last depth

Right now, i don't think anyone should use this flag other than `hints` mode, use the `auto_exit_actions` in config file might be a better choice.

### Hints Mode

Hint mode uses macOS Accessibility APIs to identify clickable UI elements and overlays hint labels on them.

**Quick start:**

```bash
neru hints
# Type the hint label to select an element
# The action (click by default) executes automatically
```

See [Configuration Guide](CONFIGURATION.md#hint-mode) for customization options.

### Grid Mode

Grid mode divides the screen into a coordinate-based grid for accessibility-independent navigation.

**Quick start:**

```bash
neru grid
# Type row+column labels (e.g., "ab") to select position
# The action executes automatically
```

See [Configuration Guide](CONFIGURATION.md#grid-mode) for customization options.

### Recursive-Grid Mode

Recursive grid provides recursive cell-based navigation that works anywhere on screen. The screen is divided into a grid (default 2x2), and each selection narrows the active area.

**Default keys:**

| Key                    | Action                         |
| ---------------------- | ------------------------------ |
| `u`                    | Upper-left cell                |
| `i`                    | Upper-right cell               |
| `j`                    | Lower-left cell                |
| `k`                    | Lower-right cell               |
| `Backspace` / `Delete` | Move up one depth and recenter |
| `Space` (default)      | Reset to initial center        |
| `Esc`                  | Exit mode                      |

**Quick start:**

```bash
neru recursive_grid
# Press u/i/j/k to narrow selection
# Press backspace to move up a level
# Press space to reset to initial center
```

See [Configuration Guide](CONFIGURATION.md#recursive-grid-mode) for customization options.

---

## Scroll Mode

Activate vim-style scrolling at the current cursor position. Keys are configurable in your config file.

**Default scroll keys:**

- `j` / `k` - Scroll down/up
- `h` / `l` - Scroll left/right
- `d` / `u` - Half-page down/up
- `gg` - Jump to top
- `Shift+G` - Jump to bottom
- `Esc` - Exit scroll mode

**Customization:** See `[scroll.key_bindings]` in your config file. Each action can have multiple keys, including modifier combinations (e.g., `Cmd+Up`) and multi-key sequences (e.g., `gg`).

**Workflow example:**

```bash
# Start scroll mode
neru scroll
# Scrolls at current cursor position
# Use j/k/gg/G/d/u to scroll (or your custom bindings)
# Press Esc to exit
```

---

## Configuration Management

Manage the Neru configuration file without starting the daemon (except `dump` and `reload`, which require a running instance).

### `config init`

Create a default configuration file with all options documented:

```bash
neru config init        # Create at $XDG_CONFIG_HOME/neru/config.toml or ~/.config/neru/config.toml
neru config init -f     # Overwrite an existing config file
neru config init -c /path/to/config.toml  # Create at a custom path
```

**Options:**

- `--force, -f` — Overwrite an existing config file
- `--config, -c` — Write to a custom path instead of the default location
  The generated file is a fully-commented copy of the built-in defaults, ready to customize. See [CONFIGURATION.md](CONFIGURATION.md) for the full reference.

### `config validate`

Check the configuration file for syntax errors, invalid values, and conflicts — without starting the daemon:

```bash
neru config validate                        # Validate config in standard locations
neru config validate -c /path/to/config.toml  # Validate a specific file
```

If no config file is found, the command exits successfully with a note that defaults will be used.

### `config dump`

Print the currently active configuration as JSON (requires a running daemon):

```bash
neru config dump
```

### `config reload`

Reload the configuration from disk without restarting (requires a running daemon):

```bash
neru config reload
```

> [!NOTE]
> Some settings (e.g., `systray.enabled`) require a full restart. See [CONFIGURATION.md](CONFIGURATION.md) for details.

---

## Status & Info

```bash
neru status                               # Daemon status and mode
neru doctor                               # Full system diagnostics
neru --version                            # Version info
```

**Status values:** `running`, `disabled`
**Mode values:** `idle`, `hints`, `grid`, `recursive_grid`, `scroll`

> [!TIP]
> Use neru doctor as your first debugging step — unlike neru status, it works even when the daemon isn't running and checks config validity, socket health, and all internal components.

---

## Shell Integration

### Health Check

```bash
neru doctor    # Run comprehensive system diagnostics
```

### Completions

Generate shell completions:

```bash
# Bash
neru completion bash > /usr/local/etc/bash_completion.d/neru

# Zsh
neru completion zsh > "${fpath[1]}/_neru"

# Fish
neru completion fish > ~/.config/fish/completions/neru.fish
```

---

## Scripting

### Toggle Script

```bash
#!/bin/bash
STATUS=$(neru status | grep "Status:" | awk '{print $2}')
if [ "$STATUS" = "running" ]; then
    neru stop
else
    neru start
fi
```

### External Hotkeys (skhd)

```bash
# ~/.config/skhd/skhdrc
ctrl - f : neru hints
ctrl - g : neru grid
ctrl - r : neru hints --action right_click
```

### Status Check

```bash
neru status &>/dev/null && echo "Running" || echo "Not running"
```

---

## Technical Details

### IPC Communication

CLI and daemon communicate via Unix socket (typically `/var/folders/.../T/neru.sock`) using JSON messages.

**Request:**

```json
{ "action": "hints", "params": {}, "args": [] }
```

**Response:**

```json
{ "success": true, "message": "OK", "code": "OK" }
```

### Error Codes

Structured error codes for scripting:

- `ERR_MODE_DISABLED` - Mode disabled in config
- `ERR_UNKNOWN_COMMAND` - Invalid command
- Connection errors when daemon not running

### Concurrency

Multiple CLI commands run concurrently, processed sequentially by daemon.

### Log Monitoring

```bash
tail -f ~/Library/Logs/neru/app.log
grep ERROR ~/Library/Logs/neru/app.log
```

---

## Troubleshooting

### Command hangs

If a command hangs, the daemon may be stuck:

```bash
# Force quit daemon
pkill -9 neru

# Restart
neru launch
```

### Socket permission errors

If you get permission errors on the IPC socket:

```bash
# Remove stale socket (path is printed in logs; typically under /var/folders/.../T)
rm -f /var/folders/*/*/T/neru.sock

# Restart daemon
neru launch
```

### Commands not working

Verify daemon is running:

```bash
neru status
```

If not running:

```bash
neru launch
```
