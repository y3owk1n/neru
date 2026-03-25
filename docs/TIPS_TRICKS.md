# Some Tips & Tricks

---

## Table of Contents

- [Vimium-Style Click-on-Select](#vimium-style-click-on-select)
- [Homerow Action Clicks](#homerow-action-clicks)
- [Auto-Exit After Click](#auto-exit-after-click)
- [Restore Cursor Position After Mode Exit](#restore-cursor-position-after-mode-exit)
- [Custom Mouse Movement Step Size](#custom-mouse-movement-step-size)
- [Disabling All Built-In Hotkeys](#disabling-all-built-in-hotkeys)
- [Checking the Accessibility Tree on macOS](#checking-the-accessibility-tree-on-macos)
- [Running a Custom Configuration via App Bundle](#running-a-custom-configuration-via-app-bundle)
- [Cycling Between Different Monitors](#cycling-between-different-monitors)
- [Triggering Neru Actions from External Tools](#triggering-neru-actions-from-external-tools)
- [Combining Hints with Other Actions](#combining-hints-with-other-actions)

---

## Vimium-Style Click-on-Select

Hints mode that clicks automatically when you finish typing a label — similar to Vimium in a browser.

> **Note:** The default hotkey for Hints mode is `Cmd+Shift+Space`. The snippet below rebinds that same key to auto-click on select, so you are _replacing_ the default hints behaviour, not adding a new one. Bind it to a separate key if you want both.

```toml
[hotkeys]
"Cmd+Shift+Space" = "hints --action left_click"
```

## Homerow Action Clicks

Homerow-style `Return` click actions via mode `custom_hotkeys`:

```toml
[hints.custom_hotkeys]
"Enter" = "action left_click" # press twice quickly for double-click, three times for triple-click
"Shift+Enter" = "action right_click"
"Cmd+Enter" = "action middle_click"
```

## Auto-Exit After Click

The old `auto_exit_actions` config field was removed. Use a `custom_hotkeys` array to click and exit in one key:

```toml
[hints.custom_hotkeys]
"Shift+L" = ["action left_click", "idle"]
"Shift+R" = ["action right_click", "idle"]
```

This works in any mode — hints, grid, recursive_grid, or scroll.

## Restore Cursor Position After Mode Exit

The old `restore_cursor_position` config field was removed. Compose the same behavior with action primitives:

```toml
[hotkeys]
"Cmd+Shift+Space" = ["action save_cursor_pos", "hints"] # add the save cursor pos action before launch hints

[hints.custom_hotkeys]
"Enter" = ["action left_click", "idle", "action restore_cursor_pos"]
```

This saves the cursor position, clicks, exits hints, waits for the mode to fully exit, then moves the cursor back.

## Custom Mouse Movement Step Size

The old `action.move_mouse_step` config field was removed. Control step size directly via `--dx`/`--dy` flags in `custom_hotkeys`:

```toml
[hints.custom_hotkeys]
# Default 10px step
"Up"    = "action move_mouse_relative --dx=0 --dy=-10"
"Down"  = "action move_mouse_relative --dx=0 --dy=10"
"Left"  = "action move_mouse_relative --dx=-10 --dy=0"
"Right" = "action move_mouse_relative --dx=10 --dy=0"
```

To use a larger step (e.g. 20px), just change the values:

```toml
[hints.custom_hotkeys]
"Up"    = "action move_mouse_relative --dx=0 --dy=-20"
"Down"  = "action move_mouse_relative --dx=0 --dy=20"
```

## Click, sleep, move

On some apps (e.g. Discord), it requires you to wait for a bit after clicking before moving to consider as a success click. Try this snippet:

```toml
[recursive_grid.custom_hotkeys]
# Click, sleep for a bit, and then only reset (that moves the cursor to center in recursive grid mode)
"Ctrl+J" = ["action left_click", "exec sleep 0.05", "action reset"]
```

## Disabling All Built-In Hotkeys

To disable all built-in hotkeys (e.g. when using an external hotkey daemon like skhd), provide an empty `[hotkeys]` section:

```toml
[hotkeys]
# No bindings — all defaults are cleared.
# Trigger modes via CLI: neru hints, neru grid, etc.
```

### Using skhd or other external hotkey managers

```bash
# ~/.config/skhd/skhdrc
ctrl - f : neru hints
ctrl - g : neru grid
ctrl - r : neru hints --action right_click
```

## Checking the Accessibility Tree on macOS

Neru doesn't ship its own accessibility inspector. You have two options:

**Option 1 — UIElementInspector (lightweight, no Xcode needed)**

Download the sample app directly from Apple:
[UIElementInspector.zip](https://developer.apple.com/library/mac/samplecode/UIElementInspector/UIElementInspector.zip)

**Option 2 — Accessibility Inspector (ships with Xcode)**

1. Open Xcode
2. Go to **Xcode → Open Developer Tool → Accessibility Inspector**

Both tools let you inspect element roles, labels, and positions. UIElementInspector is quicker to grab if you don't already have Xcode installed.

## Running a Custom Configuration via App Bundle

```bash
open -a neru --args launch -c /absolute/path/to/your/config
```

> **Note:** `~` expansion does not work here — use the full absolute path.

This is useful for testing a config before committing it to your dotfiles, or for keeping separate profiles (e.g. a lighter config when presenting or screen-sharing).

## Cycling Between Different Monitors

- Neru shows the mode overlay based on the current cursor position
- To show the overlay on another monitor, move your cursor there first
- Neru provides a base action command for this: `neru action move_mouse --center --monitor <monitor-name>`

### Example: Cycle Between Monitors

1. Create a bash script, e.g. `/path/cycle-monitor.sh`:

```bash
#!/usr/bin/env bash
STATE_FILE="${HOME}/.neru_monitor_cycle"

# Auto-detect monitors from system
mapfile -t MONITORS < <(system_profiler SPDisplaysDataType | grep -E "^\s{8}[A-Za-z].*:$" | sed 's/://g' | sed 's/^[[:space:]]*//')

if [[ ${#MONITORS[@]} -eq 0 ]]; then
    echo "No monitors detected." >&2
    exit 1
fi

# Read current index, default to 0
current=0
if [[ -f "$STATE_FILE" ]]; then
    current=$(cat "$STATE_FILE")
fi

# Cycle to next
next=$(((current + 1) % ${#MONITORS[@]}))

# Move mouse to center of next monitor
neru action move_mouse --center --monitor "${MONITORS[$next]}"

echo "$next" > "$STATE_FILE"
echo "Moved to: ${MONITORS[$next]}"
```

1. Make it executable, then bind it to a hotkey:

```bash
chmod +x /path/cycle-monitor.sh
```

```toml
[hotkeys]
"Alt+Z" = "exec bash /path/cycle-monitor.sh"
```

## Triggering Neru Actions from External Tools

Because Neru exposes an IPC-based CLI, you can drive it from anything — Hammerspoon, Raycast scripts, shell aliases, or other hotkey daemons — without going through Neru's own hotkey system.

```bash
# Move mouse to an absolute position
neru action move_mouse --x 500 --y 300

# Trigger a left click at the current cursor position
neru action left_click

# Enter hints mode programmatically
neru hints
```

This is handy when a Neru hotkey conflicts with an app's own shortcut and you'd rather let an external tool handle the trigger.

## Combining Hints with Other Actions

The `--action` flag on hints mode is not limited to `left_click`. You can pass other actions to change what happens when a hint label is completed:

```toml
[hotkeys]
"Cmd+Shift+Space" = "hints --action left_click"   # click
"Cmd+Shift+R"     = "hints --action right_click"  # context menu
```

Useful for apps where you frequently need a right-click menu (e.g. Finder, VS Code file tree) without moving your hands to the mouse.

---

## Further Reading

- [Configuration Reference](CONFIGURATION.md) — every TOML option explained
- [CLI Usage](CLI.md) — all commands and flags
- [Troubleshooting](TROUBLESHOOTING.md) — common issues and fixes
- [Community configs](CONFIG_SHOWCASES.md) — see how others configure Neru
