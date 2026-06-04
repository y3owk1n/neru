# CLI Usage

Neru provides a comprehensive command-line interface for controlling the daemon, triggering navigation modes, and building keyboard-driven workflows. Commands communicate with a running daemon over a Unix socket.

> "The daemon" refers to the background process started with `neru launch`.

---

## Table of Contents

- [Quick Start](#quick-start)
- [Daemon Control](#daemon-control)
- [Navigation Modes](#navigation-modes)
- [Toggle Commands](#toggle-commands)
- [Action Commands](#action-commands)
- [Configuration Management](#configuration-management)
- [Status & Diagnostics](#status--diagnostics)
- [Service Management](#service-management)
- [Documentation](#documentation)
- [Shell Completions](#shell-completions)
- [Scripting](#scripting)
- [Technical Details](#technical-details)

---

## Quick Start

```bash
# First-time setup
neru config init        # Create config file
neru services install   # Auto-start on login
neru launch             # Start daemon

# Daily use
neru hints             # Click UI elements via labels
neru grid              # Navigate by coordinate grid
neru recursive_grid    # Recursive cell-based navigation
neru scroll            # Vim-style scrolling
neru status            # Check daemon status
```

### Global Flags

| Flag        | Shorthand | Type   | Default | Description            |
| ----------- | --------- | ------ | ------- | ---------------------- |
| `--config`  | `-c`      | string | `""`    | Path to config file    |
| `--timeout` |           | int    | `5`     | IPC timeout in seconds |

---

## Daemon Control

```bash
neru launch                                # Start daemon
neru launch -c /path/to/config.toml        # Start with custom config (see [CONFIGURATION.md](CONFIGURATION.md#config-file-location))
neru start                                 # Resume a paused daemon
neru stop                                  # Pause daemon (keeps running)
neru idle                                  # Cancel active navigation mode
```

`stop` pauses Neru without quitting. `idle` cancels whichever mode is currently active.

---

## Navigation Modes

### Common Flags

| Flag                      | Type   | Description                                                                                                                               |
| ------------------------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `--action, -a`            | string | Action on selection: `left_click`, `right_click`, `middle_click`, `mouse_down`, `mouse_up`, `move_mouse`, `move_mouse_relative`, `scroll` |
| `--repeat, -r`            | bool   | Re-activate mode after action (requires `--action`)                                                                                       |
| `--cursor-selection-mode` | string | `follow` (cursor follows selection) or `hold` (cursor stays)                                                                              |

Not allowed as `--action`: `reset`, `backspace`, `search_hints`, `cycle_hint`, `focus_window`, `space`, `move_window_to_space`, `sleep`, `wait_for_mode_exit`, `save_cursor_pos`, `restore_cursor_pos`, and scroll sub-actions (`scroll_up`, `page_down`, `go_top`, etc.).

> The `--action` flag is most useful in hints mode (Vimium-style). In grid/recursive-grid, prefer composing behavior in per-mode hotkeys: `["action left_click", "idle"]`.

```bash
# Examples
neru hints --action left_click                     # Click via hints
neru hints --action left_click --repeat            # Click and re-enter hints
neru hints --search                                # Start with search input visible
neru grid --cursor-selection-mode hold             # Grid with stationary cursor
neru recursive_grid --action middle_click          # Middle-click via recursive grid
```

### Hints Mode

Labels clickable UI elements with short overlay labels. Uses either the macOS Accessibility API (`axtree`) or Vision Framework (`vision`) to discover elements. Default is `axtree`.

```bash
neru hints
# Type hint label (e.g. "as") to select an element

neru hints --search                                # Start with search input active
neru hints --action left_click --repeat            # Click multiple elements in succession
neru hints --strategy vision                       # Use Vision Framework for element detection

# Filtering
neru hints --role AXButton --text submit           # Show only buttons containing "submit"
neru hints --role AXButton,AXLink --text save,cancel  # Multiple roles/texts (comma-separated)
neru hints --text next --action left_click --repeat   # Filter persists across repeats
```

| Flag                      | Type   | Description                                                                                                                  |
| ------------------------- | ------ | ---------------------------------------------------------------------------------------------------------------------------- |
| `--action, -a`            | string | Action on selection (same values as [Common Flags](#common-flags))                                                           |
| `--repeat, -r`            | bool   | Re-activate hints after action (requires `--action`)                                                                         |
| `--cursor-selection-mode` | string | `follow` (default) or `hold` — whether cursor jumps to selection                                                             |
| `--search, -s`            | bool   | Start with search input active.                                                                                              |
| `--role`                  | string | Filter by AX role. Comma-separated for multiple (e.g. `--role AXButton,AXLink`).                                             |
| `--text`                  | string | Filter elements by text content (title, description, value). Case-insensitive substring match. Comma-separated for OR match. |
| `--strategy`              | string | Element detection strategy: `axtree` (macOS AX API, default) or `vision` (Vision Framework). Overrides config for this invocation. |

The filter is preserved across repeat activations.

### Grid Mode

Divides the screen into a labelled coordinate grid. Type row+column labels to jump to a position.

```bash
neru grid
neru grid --action left_click --repeat          # Click and re-enter grid
neru grid --cursor-selection-mode hold          # Grid with stationary cursor
```

| Flag                      | Type   | Description                                                        |
| ------------------------- | ------ | ------------------------------------------------------------------ |
| `--action, -a`            | string | Action on selection (same values as [Common Flags](#common-flags)) |
| `--repeat, -r`            | bool   | Re-activate grid after action (requires `--action`)                |
| `--cursor-selection-mode` | string | `follow` (default) or `hold`                                       |

### Recursive Grid Mode

Divides the screen into cells. Each keypress narrows the active area recursively.

```
┌───────┬───────┐
│   u   │   i   │   u = upper-left
├───────┼───────┤   i = upper-right
│   j   │   k   │   j = lower-left
└───────┴───────┘   k = lower-right
```

```bash
neru recursive_grid
neru recursive_grid --action middle_click       # Middle-click via recursive grid
```

| Flag                      | Type   | Description                                                        |
| ------------------------- | ------ | ------------------------------------------------------------------ |
| `--action, -a`            | string | Action on selection (same values as [Common Flags](#common-flags)) |
| `--repeat, -r`            | bool   | Re-activate recursive grid after action (requires `--action`)      |
| `--cursor-selection-mode` | string | `follow` (default) or `hold`                                       |

### Scroll Mode

Vim-style scrolling at the current cursor position. Scroll speed and step sizes are configured in [CONFIGURATION.md](CONFIGURATION.md#scroll).

| Key       | Action              |
| --------- | ------------------- |
| `j` / `k` | Scroll down / up    |
| `h` / `l` | Scroll left / right |
| `d` / `u` | Half-page down / up |
| `gg`      | Jump to top         |
| `Shift+G` | Jump to bottom      |
| `Esc`     | Exit                |

```bash
neru scroll
# Use j/k to scroll, gg/G to jump, Esc to exit
```

---

## Toggle Commands

```bash
neru toggle-screen-share                  # Toggle overlay visibility during screen sharing
neru toggle-cursor-follow-selection       # Toggle cursor-follow-selection in active hints/grid/recursive-grid session
neru toggle-scroll-invert                 # Toggle scroll direction inversion
```

### Scroll Invert

Toggles whether vertical and horizontal scroll deltas are inverted at runtime.
Useful when using tools like [Mos](https://github.com/Caldis/Mos) that reverse
synthetic scroll events. Also configurable via `invert_scroll` in
[CONFIGURATION.md](CONFIGURATION.md#scroll) and accessible via systray menu.

- State resets to the configured `invert_scroll` value on daemon restart

### Screen Sharing

Controls overlay visibility in screen sharing (Zoom, Google Meet, OBS, etc.). Also configurable via `hide_overlay_in_screen_share` in [CONFIGURATION.md](CONFIGURATION.md#general):

- Hidden: overlay not visible on shared screens, but visible locally
- State resets to visible on daemon restart
- Also accessible via systray menu

| macOS Version | Effectiveness                         |
| ------------- | ------------------------------------- |
| ≤ 14          | Works reliably in most apps           |
| 15.0 – 15.3   | Partially effective                   |
| 15.4+         | Limited (ScreenCaptureKit-based apps) |

> Uses a deprecated macOS `NSWindow.sharingType` API. Test with your setup.

---

## Action Commands

One-shot commands that operate independently of active modes.

### Clicks

```bash
neru action left_click                    # Left click
neru action right_click                   # Right click
neru action middle_click                  # Middle click
neru action mouse_down                    # Hold mouse button
neru action mouse_up                      # Release mouse button
```

| Flag          | Description                                                                 |
| ------------- | --------------------------------------------------------------------------- |
| `--modifier`  | Hold modifier: `cmd`, `shift`, `alt`, `ctrl` (comma-separated: `cmd,shift`) |
| `--selection` | Explicitly target mode selection                                            |
| `--bare`      | Target cursor position instead of mode selection                            |

```bash
neru action left_click                           # Click mode selection (when available)
neru action left_click --bare                    # Click at cursor position
neru action left_click --modifier cmd            # Cmd+click (open in new tab)
neru action left_click --modifier cmd,shift      # Cmd+Shift+click
neru action right_click --modifier alt           # Alt+right-click
```

### Mouse Movement

**Absolute:**

```bash
neru action move_mouse --x 500 --y 300           # Move to coordinates
neru action move_mouse                            # Move to mode selection
neru action move_mouse --center                   # Move to screen center
neru action move_mouse --center --x 50 --y -30    # Screen center with offset
neru action move_mouse --window                   # Move to focused window center
neru action move_mouse --window --x -50           # Window center with X offset
```

| Flag          | Description                                                |
| ------------- | ---------------------------------------------------------- |
| `--x`, `--y`  | Absolute coordinates, or offset with `--center`/`--window` |
| `--center`    | Active screen center                                       |
| `--window`    | Focused window center                                      |
| `--selection` | Explicitly use mode selection                              |
| `--bare`      | Force cursor-position targeting                            |

**Relative:**

```bash
neru action move_mouse_relative --dx 10 --dy -5
```

| Flag   | Type | Required | Description                                     |
| ------ | ---- | -------- | ----------------------------------------------- |
| `--dx` | int  | yes      | Delta X (pixels, positive=right, negative=left) |
| `--dy` | int  | yes      | Delta Y (pixels, positive=down, negative=up)    |

### Scrolling

```bash
neru action scroll_down                       # Scroll down (configured step)
neru action scroll_down --steps 200           # Scroll down 200px
neru action scroll_left --steps 100           # Scroll left 100px
neru action scroll_up                         # Scroll up
neru action scroll_right                      # Scroll right
neru action page_up                           # Half-page up
neru action page_down                         # Half-page down
neru action go_top                            # Jump to top
neru action go_bottom                         # Jump to bottom
```

| Flag          | Description                                                           |
| ------------- | --------------------------------------------------------------------- |
| `--steps`     | Override scroll step (pixels); `scroll_up`/`down`/`left`/`right` only (see [CONFIGURATION.md](CONFIGURATION.md#scroll)) |
| `--selection` | Target mode selection                                                 |
| `--bare`      | Target cursor position instead of mode selection                      |

### Mode Commands

```bash
neru action reset                             # Reset state in current mode
neru action backspace                         # Mode-aware backspace
neru action wait_for_mode_exit                # Block until mode exits to idle
neru action save_cursor_pos                   # Save current cursor position
neru action restore_cursor_pos                # Restore saved cursor position
```

### Feed Keys

Posts keystrokes to the system through IPC. Works from CLI and config [hotkey arrays](CONFIGURATION.md#hotkeys).

```bash
neru action feed o
neru action feed ctrl+c
neru action feed Cmd+Shift+P
neru action feed h e l l o return
```

**Syntax:** `neru action feed <key-or-chord> [key-or-chord...]`

Each space-separated item is one key press or chord. Chords use `+` (e.g. `ctrl+c`, `Cmd+Shift+P`). Use `space` for a literal space key.

**Supported key names:**

- Letters: `a`–`z`
- Numbers: `0`–`9`
- Symbols: `=`, `-`, `[`, `]`, `'`, `;`, `\`, `,`, `/`, `.`, `` ` ``
- Named: `space`, `return`, `enter`, `escape`, `esc`, `tab`, `delete`, `backspace`
- Navigation: `left`, `right`, `up`, `down`, `pageup`, `pagedown`, `home`, `end`
- Function: `f1`–`f20`
- Chord modifiers: `cmd`, `command`, `super`, `meta`, `shift`, `alt`, `option`, `ctrl`, `control`, and left/right forms (`LeftCmd`, `RightShift`)

> Linux and Windows: returns not-supported error.

### Cycling Hints

In hints mode, cycles through visible hints without requiring label input.

```bash
neru action cycle_hint                        # Next hint
neru action cycle_hint --backward             # Previous hint
```

| Flag         | Description                            |
| ------------ | -------------------------------------- |
| `--backward` | Cycle to previous hint instead of next |

Cycling respects any active input filter. Bind to hotkeys:

```toml
[hints.hotkeys]
"Tab" = "action cycle_hint"
"Shift+Tab" = "action cycle_hint --backward"
```

### Moving Monitors

Multi-monitor cursor movement. When a mode overlay is active, it follows the cursor to the new monitor.

```bash
neru action move_monitor                      # Cycle to next monitor
neru action move_monitor --previous           # Cycle to previous monitor
neru action move_monitor --name "DELL U2720Q" # Move to named monitor
```

| Flag         | Description                    |
| ------------ | ------------------------------ |
| `--name`     | Target monitor by display name |
| `--previous` | Cycle to previous monitor      |

Find monitor names in **System Settings → Displays**.

### Window Focus

Cycles keyboard focus through all focusable windows on the current Space. Filters out minimized, hidden, and off-space windows.

```bash
neru action focus_window                        # Focus next window
neru action focus_window --backward             # Focus previous window
```

| Flag         | Description                              |
| ------------ | ---------------------------------------- |
| `--backward` | Cycle to the previous window instead of next |

Windows are enumerated across all running applications. Only windows with role `AXWindow` that are visible, not minimized, and on the active space are included.

Bind to hotkeys for quick window switching:

```toml
[global.hotkeys]
"Primary+Tab"       = "action focus_window"
"Primary+Shift+Tab" = "action focus_window --backward"
```

### Switching Spaces

Focuses a Mission Control space by its 1-based index. macOS exposes no public API to activate a space, so Neru synthesizes a high-velocity horizontal dock swipe gesture to fast-forward to the destination space without the standard swipe animation. When the destination sits on a different display, the cursor is warped to its center first so the gesture is attributed to the correct screen.

```bash
neru action space 1     # Focus the first Mission Control space
neru action space 3     # Focus the third
```

The index is 1-based and counted in Mission Control ordering across all connected displays. Index `1` is typically the leftmost space on the primary display. Returns a clear error if Mission Control is already active (swipe gestures are ignored there).

> **Note:** This action is macOS only.

### Moving Windows to Spaces

Moves the current focused window to a Mission Control space by its 1-based index. This action is implemented on macOS without requiring scripting additions (no need to disable SIP).

```bash
neru action move_window_to_space 1     # Move focused window to the first Mission Control space
neru action move_window_to_space 3     # Move focused window to the third
```

The index is 1-based and counted in Mission Control ordering across all connected displays. Returns an error if no active window is found or if the space index is out of range.

> **Note:** This action is macOS only.

### Delay

Pauses execution for a specified duration. Useful in hotkey arrays to sequence actions:

```bash
neru action sleep 0.5      # 0.5 seconds
neru action sleep 500ms    # 500 milliseconds
neru action sleep 1s       # 1 second
neru action sleep 1        # 1 second (plain number = seconds)
```

**Duration format:** plain numbers are seconds (`0.2`, `1`). Explicit units: `ms` (milliseconds), `s` (seconds).

---

## Configuration Management

### `neru config init`

Create a default config file with all options documented. No daemon required.

All available options are documented in [CONFIGURATION.md](CONFIGURATION.md).

```bash
neru config init                              # Create at ~/.config/neru/config.toml
neru config init --force                      # Overwrite existing
neru config init -c /path/to/config.toml      # Custom path
```

| Flag       | Shorthand | Description             |
| ---------- | --------- | ----------------------- |
| `--force`  | `-f`      | Overwrite existing file |
| `--config` | `-c`      | Write to custom path    |

### `neru config validate`

Check config for syntax errors, invalid values, and conflicts. No daemon required.

```bash
neru config validate                          # Standard locations
neru config validate -c /path/to/config.toml  # Specific file
```

If no config is found, exits successfully (Neru uses built-in defaults).

### `neru config dump` / `reload`

Require a running daemon.

```bash
neru config dump                              # Print active config as JSON
neru config reload                            # Reload config without restart
```

> Some settings (e.g. `systray.enabled`) require a full daemon restart.

---

## Status & Diagnostics

```bash
neru status                                   # Daemon status and current mode
neru doctor                                   # Full system diagnostics
neru --version                                # Version info
```

**Status:** `running`, `disabled`
**Mode:** `idle`, `hints`, `grid`, `recursive_grid`, `scroll`

> `neru doctor` works even when the daemon isn't running — checks config validity, socket health, and internal components.

---

## Service Management

Manage Neru as a system service for automatic startup on login. macOS only (macOS `launchd`); other platforms return not-supported.

```bash
neru services install                         # Install and load launchd service
neru services uninstall                       # Unload and remove service
neru services start                           # Start the service
neru services stop                            # Stop the service
neru services restart                         # Restart the service
neru services status                          # Check service status
```

> If installed via Nix, Homebrew, or another package manager, use that tool's service manager instead.

---

## Documentation

Open version-aware documentation in default browser. No daemon required. macOS only.

```bash
neru docs config                              # Open configuration reference
neru docs cli                                 # Open CLI reference
```

URLs point to the exact Git tag matching your installed version. Dev builds fall back to `main`.

---

## Shell Completions

```bash
# Bash
neru completion bash > /usr/local/etc/bash_completion.d/neru

# Zsh
neru completion zsh > "${fpath[1]}/_neru"
exec zsh

# Fish
neru completion fish > ~/.config/fish/completions/neru.fish

# PowerShell
neru completion powershell > neru.ps1
```

---

## Scripting

```bash
# Toggle daemon on/off
STATUS=$(neru status | grep "Status:" | awk '{print $2}')
if [ "$STATUS" = "running" ]; then
    neru stop
else
    neru start
fi

# External hotkey manager (skhd)
# ~/.config/skhd/skhdrc
ctrl - f : neru hints
ctrl - g : neru grid
ctrl - r : neru hints --action right_click
ctrl - t : neru hints --action left_click --repeat

# Status check
neru status &>/dev/null && echo "Running" || echo "Not running"
```

---

## Technical Details

### IPC Communication

CLI and daemon communicate via a Unix domain socket using JSON messages.

**Request:**

```json
{ "action": "hints", "params": {}, "args": [] }
```

**Response:**

```json
{ "success": true, "message": "OK", "code": "OK" }
```

Commands are queued by the daemon, so concurrent calls from scripts work safely.

### Error Codes

| Code                  | Meaning                              |
| --------------------- | ------------------------------------ |
| `ERR_MODE_DISABLED`   | Requested mode is disabled in config |
| `ERR_UNKNOWN_COMMAND` | Invalid command name                 |
| _(connection error)_  | Daemon is not running                |

### Log Monitoring

```bash
tail -f ~/Library/Logs/neru/app.log        # Real-time log stream
grep ERROR ~/Library/Logs/neru/app.log     # Errors only
```

### Troubleshooting

**Command hangs:**

```bash
pkill -9 neru    # Force quit
neru launch      # Restart
```

**Socket permission errors:**

```bash
SOCK=$(grep "socket path" ~/Library/Logs/neru/app.log | tail -1 | awk '{print $NF}')
rm -f "$SOCK"
neru launch
```

**Daemon not running:**

```bash
neru status        # Verify
neru launch        # Start
neru doctor        # Comprehensive diagnosis
```
