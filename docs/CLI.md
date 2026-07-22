# CLI Usage

Neru provides a comprehensive command-line interface for controlling the daemon, triggering navigation modes, and building keyboard-driven workflows. Commands communicate with a running daemon over a Unix socket.

> "The daemon" refers to the background process started with `neru launch`. Commands are also documented as manpages (`man neru` after install).

---

## Table of Contents

- [1. neru launch](#1-neru-launch)
- [2. neru start](#2-neru-start)
- [3. neru stop](#3-neru-stop)
- [4. neru idle](#4-neru-idle)
- [5. neru status](#5-neru-status)
- [6. neru doctor](#6-neru-doctor)
- [7. neru hints](#7-neru-hints)
- [8. neru grid](#8-neru-grid)
- [9. neru recursive_grid](#9-neru-recursive_grid)
- [10. neru scroll](#10-neru-scroll)
- [11. neru monitor_select](#11-neru-monitor_select)
- [12. neru action](#12-neru-action)
- [13. neru config](#13-neru-config)
- [14. neru toggle-scroll-invert](#14-neru-toggle-scroll-invert)
- [15. neru toggle-cursor-follow-selection](#15-neru-toggle-cursor-follow-selection)
- [16. neru toggle-screen-share](#16-neru-toggle-screen-share)
- [17. neru services](#17-neru-services)
- [18. neru docs](#18-neru-docs)
- [Scripting](#scripting)
- [IPC Communication](#ipc-communication)

---

### Global Flags

| Flag        | Shorthand | Type   | Default | Description            |
| ----------- | --------- | ------ | ------- | ---------------------- |
| `--config`  | `-c`      | string | `""`    | Path to config file    |
| `--timeout` |           | int    | `5`     | IPC timeout in seconds |

---

## 1. neru launch

`neru launch [-h|--help] [-c|--config <path>] [--timeout <seconds>]`

Start the Neru daemon.

Does not require a running daemon (this starts it). The daemon runs as a background process and listens for IPC commands on a Unix socket.

**OPTIONS**

`-h`, `--help` -- Print help

`-c`, `--config <path>` -- Path to config file. Overrides default config search paths. See [CONFIGURATION.md](CONFIGURATION.md#config-file-location).

`--timeout <seconds>` -- IPC timeout in seconds (default: `5`).

---

## 2. neru start

`neru start [-h|--help]`

Resume Neru after it has been stopped. Requires a running daemon.

---

## 3. neru stop

`neru stop [-h|--help]`

Pause Neru. The daemon stays running but mode switching and overlay rendering are disabled. Use `neru start` to resume.

---

## 4. neru idle

`neru idle [-h|--help]`

Cancel the currently active navigation mode and return to idle. If no mode is active, this is a no-op.

---

## 5. neru status

`neru status [-h|--help]`

Show the Neru daemon status and current mode.

**OUTPUT**

`Status`: `running`, `disabled`
`Mode`: `idle`, `hints`, `grid`, `recursive_grid`, `scroll`

---

## 6. neru doctor

`neru doctor [-h|--help]`

Run comprehensive system diagnostics. Works even when the daemon is not running. Checks config validity, socket health, platform compatibility, and internal components.

---

## 7. neru hints

`neru hints [-h|--help] [-a|--action <action>] [-t|--toggle] [-r|--repeat] [--modifier <mod>] [--cursor-selection-mode <mode>] [-s|--search] [--hide-on-empty-search] [--role <role>] [--text <text>] [--strategy <strategy>] [-d|--debug] [--label-direction <dir>] [--split-word]`

Labels clickable UI elements with short overlay labels. Type a hint label to interact with the element.

Uses the macOS Accessibility API (`axtree`) or Vision Framework (`vision`) to discover elements. Default strategy is `axtree`.

**OPTIONS**

`-h`, `--help` -- Print help

`-a`, `--action <action>` -- Action on selection. Commas chain multiple actions (e.g. `left_click,left_click` for double-click). Valid: `left_click`, `right_click`, `middle_click`, `mouse_down`, `mouse_up`.

`-t`, `--toggle` -- Toggle mode on/off.

`-r`, `--repeat` -- Re-activate mode after performing the action (requires `--action`).

`--modifier <mod>` -- Comma-separated modifier keys to hold: `cmd`, `super`, `meta`, `shift`, `alt`, `option`, `ctrl` (requires `--action`).

`--cursor-selection-mode <mode>` -- `follow` (default, cursor jumps to selection) or `hold` (cursor stays).

`-s`, `--search` -- Start with search input active.

`--hide-on-empty-search` -- Hide all hints when search is empty (requires `--search`).

`--role <role>` -- Filter by AX role. Comma-separated (e.g. `AXButton,AXLink`).

`--text <text>` -- Filter by text content. Case-insensitive substring match. Comma-separated for OR.

`--strategy <strategy>` -- Detection strategy: `axtree` (default) or `vision`. Overrides config.

`-d`, `--debug` -- Probe the focused window and print detected elements without overlay.

`--label-direction <dir>` -- Label algorithm: `normal` (default) or `reverse`. Overrides config.

`--split-word` -- Split detected text into word-level regions (requires `vision`).

**EXAMPLES**

```bash
neru hints
neru hints --action left_click
neru hints --action left_click --modifier shift
neru hints --action left_click --repeat
neru hints --search
neru hints --role AXButton --text submit
neru hints --strategy vision --split-word
```

---

## 8. neru grid

`neru grid [-h|--help] [-a|--action <action>] [-t|--toggle] [-r|--repeat] [--modifier <mod>] [--cursor-selection-mode <mode>]`

Divide the screen into a labelled coordinate grid. Type row+column labels to jump to a position.

**OPTIONS**

`-h`, `--help` -- Print help

`-a`, `--action <action>` -- Action on selection. Valid: `left_click`, `right_click`, `middle_click`, `mouse_down`, `mouse_up`.

`-t`, `--toggle` -- Toggle mode on/off.

`-r`, `--repeat` -- Re-activate after action (requires `--action`).

`--modifier <mod>` -- Modifier keys to hold during action: `cmd`, `super`, `meta`, `shift`, `alt`, `option`, `ctrl` (requires `--action`).

`--cursor-selection-mode <mode>` -- `follow` (default) or `hold`.

**EXAMPLES**

```bash
neru grid
neru grid --action left_click --repeat
neru grid --cursor-selection-mode hold
```

---

## 9. neru recursive_grid

`neru recursive_grid [-h|--help] [-a|--action <action>] [-t|--toggle] [-r|--repeat] [--modifier <mod>] [--cursor-selection-mode <mode>] [--zoom-to-depth <depth>]`

Divide the screen into cells. Each keypress narrows the active area recursively.

**OPTIONS**

`-h`, `--help` -- Print help

`-a`, `--action <action>` -- Action on selection. Valid: `left_click`, `right_click`, `middle_click`, `mouse_down`, `mouse_up`.

`-t`, `--toggle` -- Toggle mode on/off.

`-r`, `--repeat` -- Re-activate after action (requires `--action`).

`--modifier <mod>` -- Modifier keys to hold during action (requires `--action`).

`--cursor-selection-mode <mode>` -- `follow` (default) or `hold`.

`--zoom-to-depth <depth>` -- Auto-drill to the specified depth at the current cursor position. If the grid cannot divide further (min size or max depth), zooming stops early. Negative values are rejected.

**EXAMPLES**

```bash
neru recursive_grid
neru recursive_grid --action middle_click
neru recursive_grid --zoom-to-depth 2
neru recursive_grid --zoom-to-depth 3 --action left_click
```

---

## 10. neru scroll

`neru scroll [-h|--help] [-t|--toggle]`

Vim-style scrolling at the current cursor position. Scroll speed and step sizes are configured in [CONFIGURATION.md](CONFIGURATION.md#scroll).

**OPTIONS**

`-h`, `--help` -- Print help

`-t`, `--toggle` -- Toggle scroll mode on/off.

**KEY BINDINGS**

| Key       | Action              |
| --------- | ------------------- |
| `j` / `k` | Scroll down / up    |
| `h` / `l` | Scroll left / right |
| `d` / `u` | Half-page down / up |
| `gg`      | Jump to top         |
| `Shift+G` | Jump to bottom      |
| `Esc`     | Exit                |

**EXAMPLES**

```bash
neru scroll
neru scroll --toggle
```

---

## 11. neru monitor_select

`neru monitor_select [-h|--help] [-t|--toggle]`

Open per-display overlay panels showing labelled selection badges. Type a label to move the cursor to that monitor. The current monitor is excluded from selection.

**OPTIONS**

`-h`, `--help` -- Print help

`-t`, `--toggle` -- Toggle monitor_select mode on/off.

**KEY BINDINGS**

| Key     | Action                       |
| ------- | ---------------------------- |
| `1`–`9` | Type label to select monitor |
| `Esc`   | Cancel and return to idle    |

**EXAMPLES**

```bash
neru monitor_select
neru monitor_select --toggle
```

---

## 12. neru action

One-shot commands that operate independently of active modes. All require a running daemon.

### 12a. left_click, right_click, middle_click, mouse_down, mouse_up

`neru action <click-type> [-h|--help] [--modifier <mod>] [--selection] [--bare]`

Perform a mouse click or button press.

**OPTIONS**

`--modifier <mod>` -- Hold modifier: `cmd`, `shift`, `alt`, `ctrl` (comma-separated).

`--selection` -- Target the active mode selection.

`--bare` -- Use the cursor position even when a mode selection exists.

**EXAMPLES**

```bash
neru action left_click
neru action left_click --modifier cmd
neru action left_click --modifier cmd,shift
neru action right_click --modifier alt
```

Commas chain multiple click actions directly:

```bash
neru action left_click,left_click             # Double-click
neru action left_click,left_click,left_click  # Triple-click
neru hints --action left_click,left_click     # Same, via mode --action
```

### 12b. move_mouse

`neru action move_mouse [-h|--help] [--x <px>] [--y <px>] [--center] [--window] [--selection] [--bare]`

Move the cursor to an absolute position.

**OPTIONS**

`--x <px>` -- X coordinate (pixels). With `--center` or `--window`, acts as horizontal offset.

`--y <px>` -- Y coordinate (pixels). With `--center` or `--window`, acts as vertical offset.

`--center` -- Move to the center of the active screen.

`--window` -- Move to the center of the focused window.

`--selection` -- Use the active mode selection.

`--bare` -- Use the current cursor position when no other target is specified.

**EXAMPLES**

```bash
neru action move_mouse --x 500 --y 300
neru action move_mouse --center
neru action move_mouse --center --x 50 --y -30
neru action move_mouse --window
neru action move_mouse --window --x -50
```

### 12c. move_mouse_relative

`neru action move_mouse_relative [-h|--help] --dx <px> --dy <px>`

Move the cursor by a relative delta.

**OPTIONS**

`--dx <px>` (required) -- Delta X. Positive = right, negative = left.

`--dy <px>` (required) -- Delta Y. Positive = down, negative = up.

**EXAMPLES**

```bash
neru action move_mouse_relative --dx 10 --dy -5
```

### 12d. scroll_up, scroll_down, scroll_left, scroll_right

`neru action scroll_<dir> [-h|--help] [--steps <px>] [--selection] [--bare]`

Scroll in the specified direction.

**OPTIONS**

`--steps <px>` -- Override scroll step (pixels). Uses configured default when omitted.

`--selection` -- Target the active mode selection.

`--bare` -- Target the cursor position.

**EXAMPLES**

```bash
neru action scroll_down
neru action scroll_down --steps 200
neru action scroll_left --steps 100
```

### 12e. page_up, page_down, go_top, go_bottom

`neru action page_up [-h|--help] [--selection] [--bare]`

Same as scroll actions without `--steps`.

**EXAMPLES**

```bash
neru action page_up
neru action page_down
neru action go_top
neru action go_bottom
```

### 12f. feed

`neru action feed [-h|--help] [--mode] <key> [<key>...]`

Post keystrokes to the system or to Neru's mode system.

Chords use `+` (e.g. `ctrl+c`, `Cmd+Shift+P`). `space` for a literal space key.

**OPTIONS**

`--mode` -- Route keys through Neru's active mode instead of posting to the OS.

**ARGUMENTS**

`<key>` -- One or more keys or chords. Supported names: letters `a`–`z`, numbers `0`–`9`, symbols (`=`, `-`, `[`, `]`, etc.), named keys (`space`, `return`, `escape`, `tab`, `delete`), navigation (`left`, `right`, `up`, `down`, `pageup`, `home`, `end`), function (`f1`–`f20`), chord modifiers (`cmd`, `shift`, `alt`, `ctrl`, `LeftCmd`, `RightShift`).

**EXAMPLES**

```bash
neru action feed o
neru action feed ctrl+c
neru action feed Cmd+Shift+P
neru action feed h e l l o return
neru action feed --mode o
neru action feed --mode Escape
```

### 12g. cycle_hint

`neru action cycle_hint [-h|--help] [--backward]`

In hints mode, cycle through visible hints without executing an action.

**OPTIONS**

`--backward` -- Cycle to previous hint instead of next.

### 12h. reset, backspace

`neru action reset [-h|--help]` -- Reset state in the current mode.

`neru action backspace [-h|--help]` -- Mode-aware backspace.

### 12i. wait_for_mode_exit

`neru action wait_for_mode_exit [-h|--help] [--bail]`

Block the action chain until the current mode exits to idle.

**OPTIONS**

`--bail` -- Abort the chain if the mode exits without a selection.

### 12j. save_cursor_pos, restore_cursor_pos, hide_cursor, show_cursor

`neru action <cmd> [-h|--help]`

Save/restore the cursor position, or hide/show the system cursor.

### 12k. sleep

`neru action sleep [-h|--help] <duration>`

Pause execution. Useful in hotkey arrays to sequence actions.

**ARGUMENTS**

`<duration>` -- Plain numbers are seconds (`0.2`, `1`). Explicit units: `ms`, `s`.

**EXAMPLES**

```bash
neru action sleep 0.5
neru action sleep 500ms
neru action sleep 1s
neru action sleep 1
```

### 12l. move_monitor

`neru action move_monitor [-h|--help] [--name <name>] [--previous]`

Move the cursor to another monitor. When a mode overlay is active, it follows.

**OPTIONS**

`--name <name>` -- Target monitor by display name (e.g. `"Built-in Retina Display"`).

`--previous` -- Cycle to the previous monitor.

**EXAMPLES**

```bash
neru action move_monitor
neru action move_monitor --previous
neru action move_monitor --name "DELL U2720Q"
```

---

## 13. neru config

### 13a. init

`neru config init [-h|--help] [-f|--force] [-c|--config <path>]`

Create a default configuration file. Does not require a running daemon.

**OPTIONS**

`-f`, `--force` -- Overwrite existing file.

`-c`, `--config <path>` -- Write to custom path.

**EXAMPLES**

```bash
neru config init
neru config init --force
neru config init -c /path/to/config.toml
```

### 13b. validate

`neru config validate [-h|--help] [-c|--config <path>]`

Check config for syntax errors and invalid values. Does not require a running daemon. Exits successfully if no config is found (Neru uses built-in defaults).

### 13c. set

`neru config set <key> <value>`

Set a configuration value on the running daemon without restarting. Changes take effect immediately and are persisted to `config.override.toml` so they survive restarts. Requires a running daemon.

The key uses dotted TOML path notation matching your config file (e.g. `hints.hint_characters`, `general.passthrough_unbounded_keys`).

**SUPPORTED TYPES**

| Type    | Example value                          |
| ------- | -------------------------------------- |
| string  | `"asdfghjkl"`                          |
| integer | `14`                                   |
| boolean | `true`                                 |
| float   | `0.5`                                  |
| color   | `"#FF0000AA"` or `{"light":"#000","dark":"#FFF"}` |
| array   | `"AXButton,AXLink"` or `'["AXButton","AXLink"]'` |

**EXAMPLES**

```bash
neru config set hints.hint_characters "asdfghjkl"
neru config set hints.ui.font_size 14
neru config set general.passthrough_unbounded_keys true
neru config set hints.clickable_roles "AXButton,AXLink"
neru config set scroll.scroll_step 50
```

**REVERTING CHANGES**

Changes are stored in an override file derived from your config filename (e.g. `config.toml` → `config.override.toml`, `my-neru.toml` → `my-neru.override.toml`). To revert:

```bash
# Revert a single field:
# Edit the override file and remove the line, then:
neru config reload

# Revert all overrides:
rm ~/.config/neru/config.override.toml
neru config reload
```

Use `neru config dump | jq` to explore all available keys and their current values.

### 13d. dump

`neru config dump [-h|--help]`

Print active config as JSON. Requires a running daemon.

### 13e. reload

`neru config reload [-h|--help]`

Reload config from disk without restarting. Requires a running daemon. Some settings (e.g. `systray.enabled`) require a full restart.

---

## 14. neru toggle-scroll-invert

`neru toggle-scroll-invert [-h|--help]`

Toggle scroll direction inversion at runtime. State resets to configured `invert_scroll` on daemon restart. Also accessible via systray menu.

---

## 15. neru toggle-cursor-follow-selection

`neru toggle-cursor-follow-selection [-h|--help]`

Toggle cursor-follow-selection in the active hints, grid, or recursive_grid session.

---

## 16. neru toggle-screen-share

`neru toggle-screen-share [-h|--help]`

Toggle overlay visibility during screen sharing. Hidden overlays are invisible on shared screens but remain visible locally. State resets to visible on restart.

| macOS Version | Effectiveness                         |
| ------------- | ------------------------------------- |
| ≤ 14          | Works reliably                        |
| 15.0 – 15.3   | Partially effective                   |
| 15.4+         | Limited (ScreenCaptureKit-based apps) |

> Uses a deprecated `NSWindow.sharingType` API. Test with your setup.

---

## 17. neru services

Manage Neru as a system service for automatic startup on login. macOS only; other platforms return not-supported.

| Subcommand                | Description                                |
| ------------------------- | ------------------------------------------ |
| `neru services install`   | Install and load the launchd service       |
| `neru services uninstall` | Unload and remove the service              |
| `neru services start`     | Start the service                          |
| `neru services stop`      | Stop the service                           |
| `neru services restart`   | Restart the service                        |
| `neru services status`    | Check if the service is loaded and running |

> If installed via Nix, Homebrew, or another package manager, use that tool's service manager instead.

---

## 18. neru docs

`neru docs config [-h|--help]` -- Open configuration reference in browser.

`neru docs cli [-h|--help]` -- Open CLI reference in browser.

URLs point to the exact Git tag matching the installed version. Dev builds fall back to `main`. macOS only.

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

## IPC Communication

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

**Error Codes:**

| Code                  | Meaning                       |
| --------------------- | ----------------------------- |
| `ERR_MODE_DISABLED`   | Mode is disabled in config    |
| `ERR_UNKNOWN_COMMAND` | Invalid command name          |
| `ERR_CHAIN_BAIL`      | Chain aborted (e.g. `--bail`) |
| _(connection error)_  | Daemon is not running         |

### Log Monitoring

```bash
tail -f ~/Library/Logs/neru/app.log
grep ERROR ~/Library/Logs/neru/app.log
```

### Troubleshooting

**Command hangs:**

```bash
pkill -9 neru
neru launch
```

**Socket permission errors:**

```bash
SOCK=$(grep "socket path" ~/Library/Logs/neru/app.log | tail -1 | awk '{print $NF}')
rm -f "$SOCK"
neru launch
```

**Daemon not running:**

```bash
neru status
neru launch
neru doctor
```
