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
- [Service Management](#service-management)
- [Navigation Commands](#navigation-commands)
- [Action Commands](#action-commands)
- [Status & Info](#status--info)
- [Shell Integration](#shell-integration)
- [Scripting](#scripting)
- [Technical Details](#technical-details)

---

## Quick Reference

**Common Commands:**

```bash
neru launch              # Start daemon
neru status              # Check status
neru services install    # Install launchd service
neru services status     # Check service status
neru profile             # Show profiling setup instructions
neru hints               # Start hint mode
neru grid                # Start grid mode
neru scroll              # Start scroll mode
neru action left_click   # Click at cursor (immediate)
neru config reload       # Reload config
```

**Navigation:**

- `neru hints` - Show clickable hints
- `neru grid` - Show coordinate grid
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
neru launch              # Start daemon
neru launch --config file # Start with custom config
neru start               # Resume paused daemon
neru stop                # Pause daemon (keep running)
neru idle                # Cancel active mode
```

**Use `stop`** for temporary disable, **`idle`** to cancel modes.

---

## Service Management

Manage Neru as a launchd service for automatic startup:

```bash
neru services install     # Install and load launchd service
neru services uninstall   # Unload and remove launchd service
neru services start       # Start the service
neru services stop        # Stop the service
neru services restart     # Restart the service
neru services status      # Check service status
```

**Installation:** Sets up Neru to start automatically on login with `KeepAlive = true`.

**Uninstallation:** Removes the service and stops automatic startup.

**Status values:** `Service loaded` or `Service not loaded`

**Notes:** If you have Neru installed via other methods (nix-darwin, home-manager, etc.), `install` will detect conflicts and refuse to overwrite. Uninstall the existing service first.

**Workflow example:**

```bash
# Install for auto-startup
neru services install

# Check status
neru services status  # Should show "Service loaded"

# Restart service
neru services restart

# Remove auto-startup
neru services uninstall
```

---

## Action Commands

Perform actions at current cursor position (subcommands only):

```bash
neru action left_click     # Left click
neru action right_click    # Right click
neru action middle_click   # Middle click
neru action mouse_down     # Hold mouse button
```

---

## Navigation Commands

### Basic Navigation

```bash
neru hints    # Show clickable hints
neru grid     # Show coordinate grid
neru scroll   # Vim-style scrolling
```

### Direct Actions in Hints/Grid

Execute action immediately upon selection:

```bash
neru hints --action left_click     # Left-click via hints
neru hints --action right_click    # Right-click via hints
neru grid --action middle_click    # Middle-click via grid
```

**Available actions:** `left_click`, `right_click`, `middle_click`, `mouse_up`, `mouse_down`

**Behavior:** Action executes automatically when location is selected, then mode exits.

---

## Scroll Mode

Activate vim-style scrolling at the current cursor position. Keys are configurable in your config file.

**Default scroll keys:**

- `j` / `k` - Scroll down/up
- `h` / `l` - Scroll left/right
- `Ctrl+d` / `Ctrl+u` - Half-page down/up
- `gg` - Jump to top
- `G` - Jump to bottom
- `Esc` - Exit scroll mode

**Customization:** See `[scroll.key_bindings]` in your config file. Each action can have multiple keys, including modifier combinations (e.g., `Cmd+Up`) and multi-key sequences (e.g., `gg`).

**Workflow example:**

```bash
# Start scroll mode
neru scroll
# Scrolls at current cursor position
# Use j/k/gg/G/Ctrl+D/U to scroll (or your custom bindings)
# Press Esc to exit
```

---

## Status & Info

```bash
neru status           # Daemon status and mode
neru profile          # Profiling setup instructions
neru config dump      # Show loaded configuration
neru config reload    # Reload config without restart
neru metrics          # Show metrics (if enabled)
neru --version        # Version info
neru --help           # General help
neru command --help   # Command-specific help
```

**Status values:** `running`, `disabled`
**Mode values:** `idle`, `hints`, `grid`, `scroll`

### Profiling

```bash
neru profile    # Show profiling setup instructions
```

For performance analysis, enable Go's pprof HTTP server via environment variable:

```bash
export NERU_PPROF=:6060
neru launch
# Then visit http://localhost:6060/debug/pprof/ in browser
# Or use: go tool pprof http://localhost:6060/debug/pprof/heap
```

The `neru profile` command prints these instructions for easy reference.

---

## Shell Integration

### Health Check

```bash
neru doctor    # Check component health
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
