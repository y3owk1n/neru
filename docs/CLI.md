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

- [Daemon Management](#daemon-management)
- [Action Commands](#action-commands)
- [Hint Commands](#hint-commands)
- [Status and Info](#status-and-info)
- [Health Check](#health-check)
- [Shell Completions](#shell-completions)
- [Scripting Examples](#scripting-examples)
- [IPC Details](#ipc-details)

---

## Daemon Management

### Launch

Start the Neru daemon:

```bash
# Use default config location
neru launch

# Use custom config
neru launch --config /path/to/config.toml
```

**Flags:**

- `--config` - Path to config file

### Start

Resume a paused daemon (does not launch if not running):

```bash
neru start
```

### Stop

Pause the daemon (disables functionality but doesn't quit):

```bash
neru stop
```

**Use cases:**

- Temporarily disable Neru without quitting
- Prevent accidental hint activation during presentations
- Quick toggle via script

### Idle

Return to idle state (cancel any active mode):

```bash
neru idle
```

**Use cases:**

- Cancel hint / grid mode programmatically
- Reset state in scripts
- Force exit from stuck modes

---

## Action Commands

Perform actions directly at the current cursor position without selecting a hint or grid location.

### General Syntax

```bash
neru action <subcommand>
```

**Available actions:**

- `left_click` - Left click at cursor position (press 2 times to double click and so on)
- `right_click` - Right click at cursor position
- `middle_click` - Middle click at cursor position
- `mouse_down` - Hold mouse button at cursor position
- `scroll` - Scroll at cursor position (Vim-style)

### Examples

```bash
# Click at current cursor position
neru action left_click

# Scroll at current cursor position
neru action scroll
# Use j/k/gg/G/Ctrl+D/U to scroll, Esc to exit
```

**Use cases:**

- Quick actions without hint selection
- Scripting mouse actions at specific coordinates (after moving cursor)
- Scrolling without selecting a location first

---

## Hint / Grid Commands

### Basic Usage

Move the mouse cursor to a selection location:

```bash
neru hints  # activate hint mode
neru grid   # activate grid mode
```

After selecting a location, you can perform actions later using the action mode (press `Tab` to toggle).

### Direct Action Execution

Execute an action immediately upon selection without using action mode:

```bash
# Hints with action
neru hints --action left_click
neru hints --action right_click
neru hints -a middle_click  # short form

# Grid with action
neru grid --action left_click
neru grid --action right_click
neru grid -a middle_click  # short form
```

**Available actions:**

- `left_click` - Left click at selected position
- `right_click` - Right click at selected position
- `middle_click` - Middle click at selected position
- `mouse_up` - Mouse up at selected position
- `mouse_down` - Mouse down at selected position

**Behavior with `--action` flag:**

- Tab key is disabled (no action mode toggle)
- Action executes automatically when you select a hint/grid location
- Mode exits immediately after action execution
- For grid mode, action executes after final subgrid selection

**Workflow example:**

```bash
# Right-click workflow
neru hints --action right_click
# 1. Hints overlay appears
# 2. Type hint label (e.g., "aa")
# 3. Mouse moves to position
# 4. Right click executes automatically
# 5. Mode exits

# Grid workflow
neru grid --action left_click
# 1. Grid overlay appears
# 2. Select main grid cell
# 3. Select subgrid position
# 4. Left click executes automatically
# 5. Mode exits
```

---

## Scroll Actions

It will scroll with the movement keys based on the current cursor position.

2. Use Vim-style keys to scroll:
    - `j` / `k` - Scroll down/up
    - `h` / `l` - Scroll left/right
    - `Ctrl+d` / `Ctrl+u` - Half-page down/up
    - `gg` - Jump to top
    - `G` - Jump to bottom
    - `Esc` - Exit scroll mode

**Workflow example:**

```bash
# Start scroll mode
neru action scroll
# Scrolls at current cursor position
# Use j/k/gg/G/Ctrl+D/U to scroll
# Press Esc to exit
```

---

## Status and Info

### Status

Check daemon status and current mode:

```bash
neru status
```

**Example output:**

```
Neru Status:
  Status: running
  Mode: idle
  Config: /Users/you/.config/neru/config.toml
```

**Possible statuses:**

- `running` - Daemon active and responsive
- `disabled` - Daemon paused via `neru stop`

**Possible modes:**

- `idle` - No active hint/grid mode
- `hints` - Hint mode active
- `grid` - Grid mode active

### Config

Print the effective configuration currently loaded by the daemon:

```bash
neru config dump
```

Reload configuration from file without restarting:

```bash
neru config reload
```

This dumps the full config as pretty JSON. Use this to verify what the daemon is using without opening files.

### Version

```bash
neru --version
```

### Metrics

Show application metrics (if enabled):

```bash
neru metrics
```

**Example output:**

```json
{
 "success": true,
 "data": [
  {
   "name": "accessibility_clickable_elements_count",
   "type": 2,
   "value": 42,
   "labels": null,
   "timestamp": "2023-10-27T10:00:00Z"
  }
 ],
 "code": "OK"
}
```

If metrics are disabled in config, this command will return an error.

### Help

```bash
# General help
neru --help

# Command-specific help
neru hints --help
neru grid --help
neru action --help
neru launch --help
```

---

## Health Check

### Doctor

Check the health of Neru components:

```bash
neru doctor
```

**Example output:**

```
✅ All systems operational
```

Or if there are issues:

```
⚠️  Some components are unhealthy:
  ❌ Accessibility: Permission denied
  ✅ IPC: OK
```

---

## Shell Completions

Generate shell completions for your shell:

### Bash

```bash
# Generate completion
neru completion bash > /usr/local/etc/bash_completion.d/neru

# Add to ~/.bashrc
source /usr/local/etc/bash_completion.d/neru
```

### Zsh

```bash
# Generate completion
neru completion zsh > "${fpath[1]}/_neru"

# Reload completions
rm -f ~/.zcompdump
compinit
```

### Fish

```bash
# Generate completion
neru completion fish > ~/.config/fish/completions/neru.fish
```

---

## Scripting Examples

### Toggle Neru

```bash
#!/bin/bash
# toggle-neru.sh - Toggle Neru on/off

STATUS=$(neru status | grep "Status:" | awk '{print $2}')

if [ "$STATUS" = "running" ]; then
    echo "Pausing Neru..."
    neru stop
else
    echo "Resuming Neru..."
    neru start
fi
```

### Hotkey Integration (skhd)

Instead of Neru's built-in hotkeys, use external hotkey managers:

```bash
# ~/.config/skhd/skhdrc

# Neru hotkeys - basic
ctrl - f : neru hints
ctrl - g : neru grid
ctrl - s : neru action scroll

# Neru hotkeys - with actions
ctrl - r : neru hints --action right_click
ctrl - m : neru hints --action middle_click
ctrl + shift - r : neru grid --action right_click

# Toggle Neru
ctrl + alt - n : ~/scripts/toggle-neru.sh
```

### Check if Running

```bash
#!/bin/bash
# check-neru.sh

if neru status &>/dev/null; then
    echo "Neru is running"
    exit 0
else
    echo "Neru is not running"
    exit 1
fi
```

### Alfred Workflow

Create an Alfred workflow for quick access:

```bash
# Alfred Script Filter
neru hints
```

Trigger: `nerul` (Neru)
Action: Run script above

---

## IPC Details

### Socket Location

Unix domain socket in the OS temporary directory (typically `/var/folders/.../T/neru.sock` on macOS). The exact path is printed in logs when the daemon starts, for example:

```
IPC server created {"socket":"/var/folders/xx/xxxxxxxxx/T/neru.sock"}
```

### Communication Protocol

The CLI and daemon communicate via JSON messages over the Unix socket.

**Request format:**

```json
{
 "action": "hints",
 "params": { "key": "value" },
 "args": ["optional", "args"]
}
```

**Response format:**

```json
{
 "success": true,
 "message": "Hints activated",
 "code": "OK",
 "data": { "optional": "payload" }
}
```

### Error Handling

CLI errors include structured `code` values to aid scripting. Examples:

- Daemon not running:

```bash
$ neru hints
failed to send command: failed to connect to neru (is it running?): ...
```

- Mode disabled by configuration:

```bash
$ neru hints
hints mode is disabled by config (code: ERR_MODE_DISABLED)
```

- Unknown command:

```bash
$ neru foobar
unknown command: foobar (code: ERR_UNKNOWN_COMMAND)
```

### Rate Limiting

There's no artificial rate limiting on IPC commands. You can send commands as fast as your scripts need.

### Concurrency

Multiple CLI commands can run concurrently. The daemon handles them sequentially in the order received.

### Log Monitoring

```bash
# Real-time log monitoring
tail -f ~/Library/Logs/neru/app.log

# Search for errors
grep ERROR ~/Library/Logs/neru/app.log

# Watch for specific events
tail -f ~/Library/Logs/neru/app.log | grep "hints"
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
