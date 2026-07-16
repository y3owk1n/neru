# Tips & Tricks

---

## Table of Contents

- [Vimium-Style Click-on-Select](#vimium-style-click-on-select)
- [Homerow Action Clicks](#homerow-action-clicks)
- [Hints Search (Homerow.app Style, sort of...)](#hints-search-homerowapp-style-sort-of)
- [Auto-Exit After Click](#auto-exit-after-click)
- [Restore Cursor Position After Mode Exit](#restore-cursor-position-after-mode-exit)
- [Custom Mouse Movement Step Size](#custom-mouse-movement-step-size)
- [Click, sleep, move](#click-sleep-move)
- [Target Menus Without Moving the Real Cursor](#target-menus-without-moving-the-real-cursor)
- [Mode Toggle (On/Off)](#mode-toggle-onoff)
- [Cycle Through Modes with One Hotkey](#cycle-through-modes-with-one-hotkey)
- [Bind a Shortcut to a Specific UI Element](#bind-a-shortcut-to-a-specific-ui-element)
- [Disabling All Built-In Hotkeys](#disabling-all-built-in-hotkeys)
- [Give Browser Content Time To Load Before Refreshing Hints](#give-browser-content-time-to-load-before-refreshing-hints)
- [Checking the Accessibility Tree on macOS](#checking-the-accessibility-tree-on-macos)
- [Running a Custom Configuration via App Bundle](#running-a-custom-configuration-via-app-bundle)
- [Edit Config File Directly](#edit-config-file-directly)
- [Triggering Neru Actions from External Tools](#triggering-neru-actions-from-external-tools)
- [Combining Hints with Other Actions](#combining-hints-with-other-actions)
- [Further Reading](#further-reading)

---

## Vimium-Style Click-on-Select

Hints mode that clicks automatically when you finish typing a label — similar to Vimium in a browser.

> [!NOTE]
> The default hotkey for Hints mode is `Primary+Shift+Space`. The snippet below rebinds that same key to auto-click on select, so you are _replacing_ the default hints behaviour, not adding a new one. Bind it to a separate key if you want both.

```toml
[hotkeys]
"Primary+Shift+Space" = "hints --action left_click"
```

## Homerow Action Clicks

Homerow-style `Return` click actions via mode `hotkeys`:

```toml
[hints.hotkeys]
"Enter" = "action left_click" # press twice quickly for double-click, three times for triple-click
"Shift+Enter" = "action right_click"
"Primary+Enter" = "action middle_click"
```

## Hints Search (Homerow.app Style, sort of...)

Neru supports text search in hints mode, similar to homerow.app. Press `/` to enter search mode, type to filter hints, and press `Enter` to auto-select when 1 or more hints remain.

```toml
[hints.hotkeys]
"/" = "action search_hints"
```

If you want to have the search input shown automatically when activating hints mode, use the `--search` flag in your binding:

```toml
[hotkeys]
"Primary+Shift+Space" = "hints --search"
```

**Features:**

- Type to filter hints by element title, description, or value
- `Space` is supported for multi-word searches (e.g., "search for issue")
- `Backspace` removes characters
- `Escape` cancels and restores all hints
- `Enter` with 1 result: executes the pending action (if any) and exits
- `Enter` with multiple results: closes search only, letting you type the exact hint label to select
- `Tab` / `cycle_hint`: navigates between filtered results without executing the action

## Auto-Exit After Click

The old `auto_exit_actions` config field was removed. Use a `hotkeys` array to click and exit in one key:

```toml
[hints.hotkeys]
"Shift+L" = ["action left_click", "idle"]
"Shift+R" = ["action right_click", "idle"]
```

This works in any mode — hints, grid, recursive_grid, or scroll.

## Restore Cursor Position After Mode Exit

The old `restore_cursor_position` config field was removed. Compose the same behavior with action primitives:

```toml
[hotkeys]
"Primary+Shift+Space" = ["action save_cursor_pos", "hints"] # add the save cursor pos action before launch hints

[hints.hotkeys]
"Enter" = ["action left_click", "idle", "action restore_cursor_pos"]
```

This saves the cursor position, clicks, exits hints, waits for the mode to fully exit, then moves the cursor back.

## Custom Mouse Movement Step Size

The old `action.move_mouse_step` config field was removed. Control step size directly via `--dx`/`--dy` flags in `hotkeys`:

```toml
[hints.hotkeys]
# Default 10px step
"Up"    = "action move_mouse_relative --dx=0 --dy=-10"
"Down"  = "action move_mouse_relative --dx=0 --dy=10"
"Left"  = "action move_mouse_relative --dx=-10 --dy=0"
"Right" = "action move_mouse_relative --dx=10 --dy=0"
```

To use a larger step (e.g. 20px), just change the values:

```toml
[hints.hotkeys]
"Up"    = "action move_mouse_relative --dx=0 --dy=-20"
"Down"  = "action move_mouse_relative --dx=0 --dy=20"
```

## Click, sleep, move

On some apps (e.g. Discord), it requires you to wait for a bit after clicking before moving to consider as a success click. Try this snippet:

```toml
[recursive_grid.hotkeys]
# Click, sleep for a bit, and then only reset (that moves the cursor to center in recursive grid mode)
"Ctrl+J" = ["action left_click", "action sleep 0.05", "action reset"]
```

## Target Menus Without Moving the Real Cursor

Some menus disappear as soon as the pointer leaves them. For grid and recursive-grid workflows, start the mode in `hold` mode so you can refine the selection first and only move on commit:

```toml
[hotkeys]
"Primary+Shift+G" = "grid --cursor-selection-mode hold"
"Primary+Shift+C" = "recursive_grid --cursor-selection-mode hold"
```

Grid and recursive-grid now include this toggle in the default config, bound to backtick:

```toml
[grid.hotkeys]
"`" = "toggle-cursor-follow-selection"

[recursive_grid.hotkeys]
"`" = "toggle-cursor-follow-selection"
"Shift+D" = "action move_mouse"
"Return" = "action left_click"
```

This keeps the real pointer still while you navigate. Point-targeted actions now prefer the current selection by default, so `"Return" = "action left_click"` and scroll actions will commit against the selection unless you opt out with `--bare`.

## Mode Toggle (On/Off)

Use `--toggle` to turn a single hotkey into a mode toggle — pressing it once activates the mode, pressing it again returns to idle:

```toml
[hotkeys]
"Ctrl+F" = "grid --toggle"
"Ctrl+G" = "recursive_grid --toggle"
"Ctrl+H" = "hints --toggle"
```

This is especially useful when you want a single key to both enter and exit a mode, avoiding the need for a separate `Escape` press or a dedicated exit keybinding.

## Cycle Through Modes with One Hotkey

Instead of a separate launcher key for every mode, one key can walk through all of them. Press it from idle to open the first mode, then press the same key again to advance through hints, recursive grid, grid, and scroll, wrapping back to hints at the end.

This works because a per-mode hotkey overrides a global hotkey bound to the same key (requires Neru 1.47.0 and later). The global `[hotkeys]` binding fires only from idle. Once you are inside a mode, that mode's own `[<mode>.hotkeys]` binding for the same key wins, and it points at the next mode in the cycle.

```toml
# From idle, this opens hints.
[hotkeys]
"Primary+Ctrl+F" = "hints"

# Inside each mode, the same key advances to the next mode.
[hints.hotkeys]
"Primary+Ctrl+F" = "recursive_grid"

[recursive_grid.hotkeys]
"Primary+Ctrl+F" = "grid"

[grid.hotkeys]
"Primary+Ctrl+F" = "scroll"

[scroll.hotkeys]
"Primary+Ctrl+F" = "hints"   # wrap back to the start
```

`Enter` and `Escape` still return you to idle from any point in the cycle. To change the order, drop a mode, or shorten the loop, edit which mode each block points to.

If you keep the default per-mode launcher keys, disable the ones this cycle replaces so a leftover default doesn't compete with the shared key:

```toml
[hotkeys]
"Primary+Shift+Space" = "__disabled__"   # default hints launcher
"Primary+Shift+G" = "__disabled__"       # default grid launcher
"Primary+Shift+C" = "__disabled__"       # default recursive_grid launcher
"Primary+Shift+S" = "__disabled__"       # default scroll launcher
```

## Bind a Shortcut to a Specific UI Element

Some apps never expose a keyboard shortcut for a UI element you use often, and some remove one you relied on. Claude for macOS, for example, dropped `Cmd+1` / `Cmd+2` / `Cmd+3` for switching between its Home, Code, and Cowork views, leaving no shortcut for those buttons at all. In some limited cases you can rebuild one by driving Neru to click a fixed spot or a specific element in the focused window.

Bind the key inside a root-level `[[app_configs]]` block scoped to the target app by its `bundle_id` (requires Neru 1.47.0 and later). That block overrides `[hotkeys]` only while that app is focused, so the key drives Neru there and passes straight through to every other app. See [Per-App Global Hotkey Overrides](CONFIGURATION.md#per-app-global-hotkey-overrides) for the full syntax.

Inside the block, each hotkey value is an action sequence that clicks the element. Pointing at a UI element reliably is the hard part, and the three approaches below break under different conditions:

| Approach                     | Survives window move | Survives resize | Breaks when                          |
| ---------------------------- | -------------------- | --------------- | ------------------------------------ |
| Absolute coordinates         | No                   | No              | the window ever moves                |
| Window-relative coordinates  | Yes                  | No              | the layout is responsive             |
| Filtered hints + feed        | Yes                  | Yes             | the role and text are not unique     |

**1. Absolute coordinates.** The most direct option, but it only holds if the window never moves or resizes:

```toml
[[app_configs]]
bundle_id = "com.anthropic.claudefordesktop"
hotkeys = { "Cmd+1" = ["action move_mouse --x 113 --y 123", "action left_click"] }
```

**2. Window-relative coordinates.** Recomputes the target from the focused window each time, so it survives moving the window and breaks only on resize. Move to the window center, offset to a corner, then nudge to the target:

```toml
[[app_configs]]
bundle_id = "com.anthropic.claudefordesktop"
hotkeys = { "Cmd+1" = ["action move_mouse --window --x -1000 --y -1000", "action sleep 0.1", "action move_mouse_relative --dx 100 --dy 70", "action sleep 0.1", "action left_click"] }
```

`--window` measures the offset from the window center. A large negative offset like `--x -1000 --y -1000` is clamped to the window's top-left corner, giving you a stable origin to measure from, and `move_mouse_relative` then walks to the element.

**3. Filtered hints + feed.** Targets an element by its accessibility role and text, then feeds the first hint label to click it:

```toml
[[app_configs]]
bundle_id = "com.anthropic.claudefordesktop"
hotkeys = { "Cmd+1" = ["hints --role AXButton --text Home --action left_click", "action feed --mode a"] }
```

`action feed --mode a` presses the first hint label. Which element gets `a` depends on your hint configuration (menu-bar hints, label direction, and so on), so confirm it lands on the element you mean. This method is precise when the element is unique, and fragile when the text is common. A button labelled "Code" is easy to confuse with every "Copy code" button in the same window. When a view's hint filter is too ambiguous to trust, fall back to the window-relative form for that key.

Putting it together, a Claude view switcher scopes three keys to the app. `Cmd+1`, `Cmd+2`, and `Cmd+3` click the Home, Code, and Cowork buttons, each nudged to a different offset from the window's top-left corner:

```toml
[[app_configs]]
bundle_id = "com.anthropic.claudefordesktop"
hotkeys = {
    "Cmd+1" = ["action move_mouse --window --x -1000 --y -1000", "action sleep 0.1", "action move_mouse_relative --dx 100 --dy 70", "action sleep 0.1", "action left_click"],
    "Cmd+2" = ["action move_mouse --window --x -1000 --y -1000", "action sleep 0.1", "action move_mouse_relative --dx 250 --dy 70", "action sleep 0.1", "action left_click"],
    "Cmd+3" = ["action move_mouse --window --x -1000 --y -1000", "action sleep 0.1", "action move_mouse_relative --dx 400 --dy 70", "action sleep 0.1", "action left_click"]
}
```

Capture your own offsets once, since they depend on the window's layout. Move the pointer over each button, read its screen coordinates, and subtract the window's top-left corner to get the `--dx` / `--dy` values.

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

## Give Browser Content Time To Load Before Refreshing Hints

Some browser-like apps need a short delay after a click so the page content can finish updating before Neru refreshes hints. Override just that app's hint hotkeys:

```toml
[[hints.app_configs]]
bundle_id = "com.brave.Browser"
hotkeys = {
	"Return" = ["action left_click", "action sleep 0.8", "hints"],
	"Shift+L" = "__disabled__"
}
```

This merges on top of `[hints.hotkeys]`, so only the keys listed here change for Brave Browser. Everything else keeps using your normal hint bindings.

You can use the same pattern for grid and recursive_grid modes:

```toml
[[grid.app_configs]]
bundle_id = "com.brave.Browser"
hotkeys = { "Return" = "action left_click" }

[[recursive_grid.app_configs]]
bundle_id = "com.brave.Browser"
hotkeys = { "u" = "action left_click" }
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

> [!NOTE]
> `~` expansion does not work here — use the full absolute path.

This is useful for testing a config before committing it to your dotfiles, or for keeping separate profiles (e.g. a lighter config when presenting or screen-sharing).

## Edit Config File Directly

Quickly open your config in an editor without hunting for the file path:

```bash
neru status 2>&1 | grep Config | awk '{print $2}' | xargs nvim
```

Or if you would like to open a new window and edit the config:

```bash
# both works the same except one uses ghostty cli and another uses macos open command
ghostty -e bash -c "neru status 2>&1 | grep Config | awk '{print \$2}' | xargs nvim" # check if your terminal has equivalent cli support
open -na Ghostty --args -e bash -c "neru status 2>&1 | grep Config | awk '{print \$2}' | xargs nvim" # this should generally work
```

You can wrap this in a shell alias or bind it to a key in your window manager / hotkey daemon.

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
"Primary+Shift+Space" = "hints --action left_click"   # click
"Primary+Shift+R"     = "hints --action right_click"  # context menu
```

Useful for apps where you frequently need a right-click menu (e.g. Finder, VS Code file tree) without moving your hands to the mouse.

---

## Further Reading

- [CONFIGURATION.md](CONFIGURATION.md) — every TOML option explained
- [CLI.md](CLI.md) — all commands and flags
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) — common issues and fixes
- [CONFIG_SHOWCASES.md](CONFIG_SHOWCASES.md) — see how others configure Neru
