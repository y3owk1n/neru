# CLI Reference

Neru provides a comprehensive command-line interface for controlling the daemon, triggering navigation modes, and building keyboard-driven workflows. Commands communicate with a running daemon over a Unix socket.

---

## Quick Reference

```bash
neru launch              # Start daemon
neru hints               # Click UI elements via labels
neru grid                # Navigate by coordinate grid
neru recursive_grid      # Recursive cell-based navigation
neru scroll              # Vim-style scrolling
neru status              # Check daemon status
neru doctor              # Full system diagnostics
neru config init         # Create default config file
```

---

## Daemon Control

```bash
neru launch [-c /path/to/config.toml]    # Start daemon with optional custom config
neru start                                # Resume a paused daemon
neru stop                                 # Pause daemon (keeps running)
neru idle                                 # Cancel active navigation mode
```

### Global Flags

| Flag        | Shorthand | Type   | Default | Description            |
| :---------- | :-------- | :----- | :------ | :--------------------- |
| `--config`  | `-c`      | string | `""`    | Path to config file    |
| `--timeout` |           | int    | `5`     | IPC timeout in seconds |

---

## Navigation Modes

### Common Flags

| Flag                      | Type   | Description                                                                                                                               |
| :------------------------ | :----- | :---------------------------------------------------------------------------------------------------------------------------------------- |
| `--action, -a`            | string | Action on selection: `left_click`, `right_click`, `middle_click`, `mouse_down`, `mouse_up`, `move_mouse`, `move_mouse_relative`, `scroll` |
| `--repeat, -r`            | bool   | Re-activate mode after action (requires `--action`)                                                                                       |
| `--toggle, -t`            | bool   | Toggle mode on/off — exit to idle if already active                                                                                       |
| `--cursor-selection-mode` | string | `follow` (cursor follows) or `hold` (cursor stays)                                                                                        |

Not allowed as `--action`: `reset`, `backspace`, `search_hints`, `cycle_hint`, `sleep`, `wait_for_mode_exit`, `save_cursor_pos`, `restore_cursor_pos`, and scroll sub-actions.

### Hints Mode

Labels clickable UI elements with short overlay labels. Uses macOS Accessibility API (`axtree`) or Vision Framework (`vision`).

```bash
neru hints
neru hints --search                            # Start with search input active
neru hints --action left_click --repeat        # Click multiple elements
neru hints --strategy vision                   # Use Vision Framework
neru hints --label-direction reverse           # Alternative label algorithm
neru hints --role AXButton --text submit       # Filter by role and text
neru hints --debug                                # Print detected elements, no overlay
```

| Flag                | Type   | Description                                                              |
| :------------------ | :----- | :----------------------------------------------------------------------- |
| `--search, -s`      | bool   | Start with search input active                                           |
| `--role`            | string | Filter by AX role (comma-separated for multiple)                         |
| `--text`            | string | Filter by text content (case-insensitive, substring)                     |
| `--strategy`        | string | `axtree` (default) or `vision`                                           |
| `--label-direction` | string | `normal` (default) or `reverse`                                          |
| `--debug, -d`       | bool   | Probe focused window and print detected elements without showing overlay |

### Grid Mode

Divides the screen into a labelled coordinate grid. Type row+column labels to jump.

```bash
neru grid
neru grid --action left_click --repeat
neru grid --cursor-selection-mode hold
```

### Recursive Grid Mode

Divides screen into cells; each keypress narrows the active area recursively.

```bash
neru recursive_grid
neru recursive_grid --action middle_click
```

```
┌───────┬───────┐
│   u   │   i   │   u = upper-left
├───────┼───────┤   i = upper-right
│   j   │   k   │   j = lower-left
└───────┴───────┘   k = lower-right
```

### Scroll Mode

Vim-style scrolling at the current cursor position.

```bash
neru scroll
neru scroll --toggle    # Toggle on/off
```

| Key       | Action            |
| :-------- | :---------------- |
| `j`/`k`   | Scroll down/up    |
| `h`/`l`   | Scroll left/right |
| `d`/`u`   | Half-page down/up |
| `gg`      | Jump to top       |
| `Shift+G` | Jump to bottom    |
| `Esc`     | Exit              |

### Monitor Select Mode

Opens per-display overlay panels showing labelled selection badges.

```bash
neru monitor_select
neru monitor_select --toggle
```

| Key     | Action                  |
| :------ | :---------------------- |
| `1`–`9` | Select monitor by label |
| `Esc`   | Cancel                  |

---

## Toggle Commands

```bash
neru toggle-screen-share                # Toggle overlay visibility in screen sharing
neru toggle-cursor-follow-selection     # Toggle cursor-follow in active mode
neru toggle-scroll-invert               # Toggle scroll direction inversion
```

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

| Flag          | Description                                                    |
| :------------ | :------------------------------------------------------------- |
| `--modifier`  | Hold modifier: `cmd`, `shift`, `alt`, `ctrl` (comma-separated) |
| `--selection` | Target mode selection                                          |
| `--bare`      | Target cursor position instead of mode selection               |

```bash
neru action left_click --bare                  # Click at cursor position
neru action left_click --modifier cmd          # Cmd+click (open in new tab)
neru action left_click --modifier cmd,shift    # Cmd+Shift+click
```

### Mouse Movement

```bash
neru action move_mouse --x 500 --y 300        # Absolute coordinates
neru action move_mouse --center                # Screen center
neru action move_mouse --window                # Focused window center
neru action move_mouse_relative --dx 10 --dy -5
```

| Flag           | Description                                                |
| :------------- | :--------------------------------------------------------- |
| `--x`, `--y`   | Absolute coordinates, or offset with `--center`/`--window` |
| `--center`     | Active screen center                                       |
| `--window`     | Focused window center                                      |
| `--dx`, `--dy` | Relative delta in pixels                                   |

### Scrolling

```bash
neru action scroll_down                       # Default step
neru action scroll_down --steps 200           # 200px
neru action scroll_left --steps 100           # 100px left
neru action page_up                           # Half-page up
neru action go_top                            # Jump to top
```

| Flag          | Description                   |
| :------------ | :---------------------------- |
| `--steps`     | Override scroll step (pixels) |
| `--selection` | Target mode selection         |
| `--bare`      | Target cursor position        |

### Mode Commands

```bash
neru action reset                             # Reset state in current mode
neru action backspace                         # Mode-aware backspace
neru action wait_for_mode_exit                # Block until mode exits
neru action wait_for_mode_exit --bail         # Abort chain if cancelled
neru action save_cursor_pos                   # Save cursor position
neru action restore_cursor_pos                # Restore saved cursor
```

### Feed Keys

Posts keystrokes through IPC:

```bash
neru action feed o
neru action feed ctrl+c
neru action feed h e l l o return
```

Each space-separated item is one key press or chord. Chords use `+` (e.g. `ctrl+c`).

**Supported key names:** `a`–`z`, `0`–`9`, symbols, `space`, `return`, `enter`, `escape`, `tab`, `delete`, `backspace`, navigation keys, `f1`–`f20`, chord modifiers (`cmd`, `ctrl`, `shift`, `alt`).

> Linux and Windows: returns not-supported error.

### Cycling Hints

```bash
neru action cycle_hint                        # Next hint
neru action cycle_hint --backward             # Previous hint
```

Bind to hotkeys:

```toml
[hints.hotkeys]
"Tab" = "action cycle_hint"
"Shift+Tab" = "action cycle_hint --backward"
```

### Monitor Movement

```bash
neru action move_monitor                      # Next monitor
neru action move_monitor --previous           # Previous monitor
neru action move_monitor --name "DELL U2720Q" # Named monitor
```

### Delay

```bash
neru action sleep 0.5      # 0.5 seconds
neru action sleep 500ms    # 500 milliseconds
neru action sleep 1s       # 1 second
```

---

## Configuration Management

```bash
neru config init [-f] [-c /path/to/config]    # Create starter config
neru config validate [-c /path/to/config]     # Check syntax (no daemon needed)
neru config dump                              # Print active config as JSON (daemon required)
neru config reload                            # Apply changes without restart
```

---

## Status & Diagnostics

```bash
neru status           # Daemon status and current mode
neru doctor           # Full system diagnostics (works without daemon)
neru --version        # Version info
```

**Status:** `running`, `disabled`
**Mode:** `idle`, `hints`, `grid`, `recursive_grid`, `scroll`

---

## Service Management

macOS only. Manages launchd service for automatic startup.

```bash
neru services install     # Install and load launchd service
neru services uninstall   # Unload and remove
neru services start       # Start service
neru services stop        # Stop service
neru services restart     # Restart
neru services status      # Check status
```

---

## Documentation

```bash
neru docs config          # Open configuration reference in browser
neru docs cli             # Open CLI reference in browser
```

URLs point to the Git tag matching your installed version. Dev builds fall back to `main`. macOS only.

---

## Scripting

```bash
# Toggle daemon
STATUS=$(neru status | grep "Status:" | awk '{print $2}')
if [ "$STATUS" = "running" ]; then neru stop; else neru start; fi

# External hotkey manager (skhd)
# ~/.config/skhd/skhdrc
ctrl - f : neru hints
ctrl - g : neru grid

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

### Error Codes

| Code                  | Meaning                             |
| :-------------------- | :---------------------------------- |
| `ERR_MODE_DISABLED`   | Mode disabled in config             |
| `ERR_UNKNOWN_COMMAND` | Invalid command                     |
| `ERR_CHAIN_BAIL`      | Chain aborted (e.g. user cancelled) |

### Log Monitoring

```bash
tail -f ~/Library/Logs/neru/app.log    # Real-time log stream
grep ERROR ~/Library/Logs/neru/app.log # Errors only
```

### Troubleshooting

| Problem            | Solution                                            |
| :----------------- | :-------------------------------------------------- |
| Command hangs      | `pkill -9 neru && neru launch`                      |
| Socket errors      | `rm -f /var/folders/*/*/T/neru.sock && neru launch` |
| Daemon not running | `neru status && neru launch`                        |
