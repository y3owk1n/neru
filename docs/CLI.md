# CLI Usage

Neru provides a comprehensive command-line interface for controlling the daemon, triggering navigation modes, and building keyboard-driven workflows — all from scripts or hotkey managers. Commands talk to a running daemon over a Unix socket, so they're fast and scriptable.

---

## Table of Contents

- [Getting Started](#getting-started)
- [Quick Reference](#quick-reference)
- [Daemon Control](#daemon-control)
- [Screen Sharing](#screen-sharing)
- [Service Management](#service-management)
- [Navigation Commands](#navigation-commands)
  - [When to Use Which Mode](#when-to-use-which-mode)
  - [Hints Mode](#hints-mode)
  - [Grid Mode](#grid-mode)
  - [Recursive-Grid Mode](#recursive-grid-mode)
  - [Scroll Mode](#scroll-mode)
- [Action Commands](#action-commands)
- [Configuration Management](#configuration-management)
- [Status & Info](#status--info)
- [Documentation](#documentation)
- [Shell Completions](#shell-completions)
- [Scripting](#scripting)
- [Technical Details](#technical-details)
- [Troubleshooting](#troubleshooting)

---

## Getting Started

New to Neru? Follow these steps in order:

1. **Create a config** (optional, but recommended):

    ```
    neru config init
    ```

2. **Start the daemon:**

    ```
    neru launch
    ```

3. **Verify it's running:**

    ```
    neru status
    ```

4. **Try hint mode** (press the default hotkey `Cmd+Shift+Space`, or run):

    ```
    neru hints
    ```

5. **Install as a system service** so Neru starts automatically on login:

    ```
    neru services install
    ```

---

## Quick Reference

### First-time setup

```
neru config init         # Create config file
neru services install    # Auto-start on login
neru launch              # Start daemon
```

### Daily use

```
neru hints               # Click UI elements via labels
neru grid                # Navigate by coordinate grid
neru recursive_grid      # Recursive cell-based navigation
neru scroll              # Vim-style scrolling
neru status              # Check daemon status
```

### Configuration

```
neru config validate     # Check config for errors (no daemon needed)
neru config reload       # Apply changes to a running daemon
neru config dump         # Print active config as JSON (daemon required)
neru doctor              # Run system diagnostics
```

**Configure hotkeys:** See [CONFIGURATION.md](CONFIGURATION.md)

---

## Daemon Control

```
neru launch                                # Start daemon
neru launch --config /path/to/config.toml  # Start with custom config file
neru launch -c ./configs/my-config.toml   # Shorter form
neru start                                 # Resume a paused daemon
neru stop                                  # Pause daemon (keeps it running)
neru idle                                  # Cancel the active navigation mode
```

**Options:**

| Flag           | Description                         |
| -------------- | ----------------------------------- |
| `--config, -c` | Path to a custom config file        |
| `--timeout`    | IPC timeout in seconds (default: 5) |

**`stop` vs `idle`:** Use `stop` to temporarily disable Neru without quitting it. Use `idle` to cancel whichever navigation mode is currently active.

---

## Screen Sharing

Control overlay visibility during screen sharing (Zoom, Google Meet, OBS, etc.):

```
neru toggle-screen-share     # Toggle overlay visibility
```

**Behaviour:**

- When toggled to **hidden**, the overlay does not appear in shared screens but remains visible locally.
- When toggled to **visible**, the overlay appears normally in screen sharing.
- The state resets to **visible** on daemon restart.
- Also accessible via the system tray menu: "Screen Share: Visible/Hidden".

> [!WARNING]
> This feature uses a deprecated macOS `NSWindow.sharingType` API. Effectiveness varies by macOS version and screen sharing application. Always test with your specific setup.

| macOS version | Effectiveness                         |
| ------------- | ------------------------------------- |
| ≤ 14          | Works reliably in most apps           |
| 15.0 – 15.3   | Partially effective                   |
| 15.4+         | Limited (ScreenCaptureKit-based apps) |

> [!NOTE]
> To set the default visibility state on launch, use `hide_overlay_in_screen_share` in your config file. See [CONFIGURATION.md](CONFIGURATION.md).

---

## Service Management

Manage Neru as a system service for automatic startup on login.

### macOS (`launchd`)

```
neru services install     # Install and load launchd service (~/Library/LaunchAgents)
neru services status      # Check service status
```

### Linux (`systemd`)

_(Planned)_ Neru will support `systemctl` for managing user services.

### Windows (Task Scheduler)

_(Planned)_ Neru will support Windows Task Scheduler for auto-start on login.

> [!NOTE]
> If you installed Neru via Nix, Homebrew, or another package manager, use that tool's service manager instead (e.g. `home-manager` for Nix).

---

## Navigation Commands

Neru provides four navigation modes. Each can be activated via CLI or a configured hotkey.

### When to Use Which Mode

| Mode               | Best for                                                          |
| ------------------ | ----------------------------------------------------------------- |
| **Hints**          | Clicking buttons, links, and menus in standard macOS apps         |
| **Grid**           | Apps with no accessibility support, or anywhere hints can't reach |
| **Recursive Grid** | Precise positioning on large or high-resolution screens           |
| **Scroll**         | Scrolling documents and pages without touching the mouse          |

### Using the `--action` flag

All navigation modes accept an `--action` (or `-a`) flag to perform an action automatically when a position is selected:

```
neru hints --action left_click           # Left-click via hints
neru hints --action right_click          # Right-click via hints
neru hints --action middle_click         # Middle-click via hints
neru grid --action left_click            # Left-click via grid
neru recursive_grid --action left_click  # Left-click via recursive-grid
```

Supported `--action` values: `left_click`, `right_click`, `middle_click`, `mouse_down`, `mouse_up`, `move_mouse`, `move_mouse_relative`, `scroll`.

Not allowed for mode `--action`: `reset`, `backspace`, `wait_for_mode_exit`, `save_cursor_pos`, `restore_cursor_pos`, and scroll sub-actions (for example `scroll_up`, `page_down`, `go_top`).

All selection modes also accept `--cursor-selection-mode follow|hold`:

```sh
neru hints --cursor-selection-mode hold
neru grid --cursor-selection-mode hold
neru recursive_grid --cursor-selection-mode follow
```

`follow` keeps the real cursor synced with the current selection. `hold` keeps the real cursor in place until you explicitly move or act on the selection.

> [!TIP]
> The `--action` flag is most useful in hints mode, where it mirrors a Vimium-style workflow: select a label and the action fires immediately. In grid and recursive-grid modes, the action triggers only after the final cell selection, which is less ergonomic. For those modes, prefer composing behavior in per-mode `hotkeys` (for example: `["action left_click", "idle"]`).

### Using the `--repeat` flag

Add `--repeat` (or `-r`) to stay in the mode after the action is performed. The mode re-activates so you can immediately select another target without re-entering the mode:

```
neru hints --action left_click --repeat           # Click, then show hints again
neru recursive_grid --action left_click --repeat  # Click, then restart recursive-grid
neru grid --action left_click --repeat            # Click, then restart grid
```

> [!TIP]
> `--repeat` requires `--action`. It is especially useful for workflows that involve clicking multiple elements in succession — you stay in the mode until you press the exit key (default: Escape).

---

### Hints Mode

Hint mode uses macOS Accessibility APIs to identify clickable UI elements and overlay short labels on them. Type a label to move the cursor and perform the configured action.

```
neru hints
# Type the hint label (e.g. "as") to select an element
```

See [CONFIGURATION.md](CONFIGURATION.md) for customisation options.

---

### Grid Mode

Grid mode divides the screen into a labelled coordinate grid. Type a row+column combination to jump to that position.

```
neru grid
# Type row+column labels (e.g. "ab") to select a position
```

See [CONFIGURATION.md](CONFIGURATION.md) for customisation options.

---

### Recursive-Grid Mode

Recursive grid divides the screen into cells. Each keypress narrows the active area recursively until the cell is small enough to click precisely.

**Default key layout:**

```
┌───────┬───────┐
│   u   │   i   │   u = upper-left
├───────┼───────┤   i = upper-right
│   j   │   k   │   j = lower-left
└───────┴───────┘   k = lower-right
```

**All default keys:**

| Key         | Action           |
| ----------- | ---------------- |
| `u`         | Upper-left cell  |
| `i`         | Upper-right cell |
| `j`         | Lower-left cell  |
| `k`         | Lower-right cell |
| `Space`     | Reset to center  |
| `Backspace` | Go up one level  |
| `Esc`       | Exit mode        |

```
neru recursive_grid
# Press u/i/j/k to narrow the selection
# Press Backspace to go up one level
```

See [CONFIGURATION.md](CONFIGURATION.md) for customisation options.

---

### Scroll Mode

Vim-style scrolling at the current cursor position. Keys are fully configurable through `scroll.hotkeys`.

**Default scroll keys:**

| Key       | Action              |
| --------- | ------------------- |
| `j` / `k` | Scroll down / up    |
| `h` / `l` | Scroll left / right |
| `d` / `u` | Half-page down / up |
| `gg`      | Jump to top         |
| `Shift+G` | Jump to bottom      |
| `Esc`     | Exit scroll mode    |

Action hotkeys (for example `Shift+L` for click and arrow-key mouse movement) can be defined in `scroll.hotkeys`, just like other modes.

```
neru scroll
# Use j/k to scroll, gg/G to jump, Esc to exit
# Use arrow keys to nudge cursor, Shift+L to click
```

See [CONFIGURATION.md](CONFIGURATION.md) for configuring step sizes and mode custom hotkeys.

---

## Action Commands

### Stateless Actions

These actions do not depend on any active mode and can be used as one-shot commands.

```
neru action left_click          # Left click
neru action right_click         # Right click
neru action middle_click        # Middle click
neru action mouse_down          # Hold mouse button
neru action mouse_up            # Release mouse button
neru action save_cursor_pos     # Save current cursor position
neru action restore_cursor_pos     # Restore saved cursor position
neru action scroll_up           # Scroll up at cursor
neru action scroll_down         # Scroll down at cursor
neru action scroll_left         # Scroll left at cursor
neru action scroll_right        # Scroll right at cursor
neru action page_up             # Half-page up at cursor
neru action page_down           # Half-page down at cursor
neru action go_top              # Jump to top at cursor
neru action go_bottom           # Jump to bottom at cursor
```

### Mode-Aware Actions

These actions depend on the current mode and are primarily useful inside `hotkeys` arrays.

```
neru action reset               # Reset state in current mode
neru action backspace           # Mode-aware backspace
neru action wait_for_mode_exit  # Block until mode exits to idle
neru toggle-cursor-follow-selection         # Toggle cursor-follow-selection in active hints/grid/recursive-grid session
```

### Modifier keys

Hold a modifier during a click using `--modifier`:

```
neru action left_click --modifier cmd          # Cmd+click (open in new tab)
neru action left_click                         # Click the active mode selection when available
neru action left_click --bare                  # Force current-cursor targeting
neru action left_click --modifier shift        # Shift+click (extend selection)
neru action left_click --modifier cmd,shift    # Cmd+Shift+click
neru action right_click --modifier alt         # Alt+right-click
```

**Valid modifiers:** `cmd` (or `command`), `shift`, `alt` (or `option`), `ctrl` (or `control`). Combine with commas: `--modifier cmd,shift`.

### Mouse movement

**Absolute position:**

```
neru action move_mouse --x 500 --y 300
neru action move_mouse                         # Move to the active mode selection
neru action move_mouse --bare                  # Use current-cursor targeting explicitly
```

**Screen center:**

```
neru action move_mouse --center
neru action move_mouse --center --x 50 --y -30    # Center with offset
neru action move_mouse --center --x 100            # Single-axis offset (y defaults to 0)
```

**Named monitor:**

```
neru action move_mouse --center --monitor "DELL U2720Q"
neru action move_mouse --center --monitor "Built-in Retina Display" --x 50 --y -30
```

**Relative movement:**

```
neru action move_mouse_relative --dx 10 --dy -5
```

**`move_mouse` flag reference:**

| Flag               | Description                                                  |
| ------------------ | ------------------------------------------------------------ |
| `--x <px>`         | Absolute X coordinate, or X offset when used with `--center` |
| `--y <px>`         | Absolute Y coordinate, or Y offset when used with `--center` |
| `--center`         | Move to the center of the active screen                      |
| `--monitor <name>` | Target a named display (requires `--center`)                 |
| `--selection`      | Explicitly use the active mode selection                     |
| `--bare`           | Force current-cursor targeting even when a selection exists  |

> [!TIP]
> Monitor names are the display names reported by macOS (e.g. "Built-in Retina Display", "DELL U2720Q"). Find yours in **System Settings → Displays**. If you use an incorrect name, the error message will list all available names.

> [!TIP]
> Point-targeted actions prefer the active mode selection by default. Use `--bare` when you want `left_click`, `right_click`, `middle_click`, `mouse_down`, `mouse_up`, `move_mouse`, or scroll actions to ignore the selection and use the current cursor position instead.

---

## Configuration Management

These commands manage the Neru config file. Note which ones require a running daemon:

### `config init` — no daemon needed

Create a default config file with all options documented:

```
neru config init                           # Create at ~/.config/neru/config.toml
neru config init -f                        # Overwrite an existing file
neru config init -c /path/to/config.toml  # Create at a custom path
```

| Flag           | Description                       |
| -------------- | --------------------------------- |
| `--force, -f`  | Overwrite an existing config file |
| `--config, -c` | Write to a custom path            |

The generated file is a fully-commented copy of the built-in defaults, ready to customise. See [CONFIGURATION.md](CONFIGURATION.md) for the full reference.

### `config validate` — no daemon needed

Check the config file for syntax errors, invalid values, and conflicts:

```
neru config validate                          # Validate config in standard locations
neru config validate -c /path/to/config.toml  # Validate a specific file
```

If no config file is found, the command exits successfully — Neru will use its built-in defaults.

### `config dump` — requires running daemon

Print the currently active configuration as JSON:

```
neru config dump
```

### `config reload` — requires running daemon

Reload the configuration from disk without restarting:

```
neru config reload
```

> [!NOTE]
> Some settings (e.g. `systray.enabled`) require a full daemon restart and cannot be applied with `reload`. See [CONFIGURATION.md](CONFIGURATION.md) for details on which settings this affects.

---

## Status & Info

```
neru status        # Daemon status and current mode
neru doctor        # Full system diagnostics
neru --version     # Version info
```

**Status values:** `running`, `disabled`

**Mode values:** `idle`, `hints`, `grid`, `recursive_grid`, `scroll`

> [!TIP]
> Use `neru doctor` as your first debugging step. Unlike `neru status`, it works even when the daemon isn't running and checks config validity, socket health, and all internal components.

---

## Documentation

Open version-aware documentation pages in the default browser. No running daemon required.

```
neru docs config    # Open configuration reference
neru docs cli       # Open CLI reference
```

The URL points to the exact Git tag matching your installed version (e.g. `v1.29.0`). Unmatched or development builds fall back to the `main` branch.

---

## Shell Completions

Add tab-completion for all Neru commands and flags.

**Bash:**

```bash
neru completion bash > /usr/local/etc/bash_completion.d/neru
```

**Zsh:**

```zsh
neru completion zsh > "${fpath[1]}/_neru"
exec zsh  # Reload shell to activate
```

**Fish:**

```fish
neru completion fish > ~/.config/fish/completions/neru.fish
```

---

## Scripting

### Toggle daemon on/off

```bash
#!/bin/bash
STATUS=$(neru status | grep "Status:" | awk '{print $2}')
if [ "$STATUS" = "running" ]; then
    neru stop
else
    neru start
fi
```

### External hotkey manager (skhd)

```bash
# ~/.config/skhd/skhdrc
ctrl - f : neru hints
ctrl - g : neru grid
ctrl - r : neru hints --action right_click
ctrl - t : neru hints --action left_click --repeat   # Multi-click workflow
```

### Status check in scripts

```bash
neru status &>/dev/null && echo "Running" || echo "Not running"
```

---

## Technical Details

### IPC Communication

The CLI and daemon communicate via a Unix domain socket (typically `/var/folders/.../T/neru.sock`) using JSON messages.

**Request format:**

```json
{ "action": "hints", "params": {}, "args": [] }
```

**Response format:**

```json
{ "success": true, "message": "OK", "code": "OK" }
```

Commands are queued by the daemon, so concurrent calls from scripts work safely.

### Error Codes

| Code                  | Meaning                                  |
| --------------------- | ---------------------------------------- |
| `ERR_MODE_DISABLED`   | The requested mode is disabled in config |
| `ERR_UNKNOWN_COMMAND` | Invalid command name                     |
| _(connection error)_  | Daemon is not running                    |

### Log Monitoring

```bash
tail -f ~/Library/Logs/neru/app.log      # Real-time log stream
grep ERROR ~/Library/Logs/neru/app.log   # Errors only
```

---

## Troubleshooting

### Command hangs

If a command hangs, the daemon may be stuck:

```bash
pkill -9 neru    # Force quit
neru launch      # Restart
```

### Socket permission errors

```bash
# Find the socket path from logs and remove it:
SOCK=$(grep "socket path" ~/Library/Logs/neru/app.log | tail -1 | awk '{print $NF}')
rm -f "$SOCK"

# Or use the glob (works on most macOS versions):
rm -f /var/folders/*/*/T/neru.sock

# Then restart:
neru launch
```

### Commands not working

Verify the daemon is running:

```bash
neru status
```

If not running:

```bash
neru launch
```

For a thorough diagnosis of any issue:

```bash
neru doctor
```
