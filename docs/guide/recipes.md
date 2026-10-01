# Recipes

Options are in the [configuration reference](../reference/configuration.md),
commands and flags in the [CLI reference](../reference/cli.md), and external
hotkey daemons in [Scripting](../reference/scripting.md). The launcher keys
below are examples. Linux binds no global hotkeys by default, see
[Getting started](getting-started.md).

## Clicking

### Vimium-style click-on-select

Click as soon as you finish typing a hint label:

```toml
[hotkeys]
"Primary+Shift+Space" = "hints --action left_click"
"Primary+Shift+R"     = "hints --action right_click"   # context menu
```

On macOS and Windows the first line replaces the default hints launcher. Bind
another key to keep both.

### Homerow action clicks

Click the selected hint with `Return`:

```toml
[hints.hotkeys]
"Enter"         = "action left_click"
"Shift+Enter"   = "action right_click"
"Primary+Enter" = "action middle_click"
"Ctrl+Enter"    = "action left_click,left_click"   # double-click
```

A comma chain clicks several times at one point, so
`"action left_click,left_click,left_click"` triple-clicks. See
[`neru action left_click`](../reference/cli.md#neru-action-left_click-right_click-middle_click).

### Auto-exit after click

Click and leave the mode with one key, in any mode:

```toml
[hints.hotkeys]
"Shift+L" = ["action left_click", "idle"]
"Shift+R" = ["action right_click", "idle"]
```

The mode exits even if the click fails. To stay in the mode on failure, write
`"action left_click --bail-on-error"`, see
[Failure policy](../reference/cli.md#failure-policy).

### Click, sleep, move

Hold the pointer still after a click, for apps such as Discord that need it:

```toml
[recursive_grid.hotkeys]
# Click, wait, then reset, which moves the cursor back to the grid centre
"Ctrl+J" = ["action left_click", "action sleep 0.05", "action reset"]
```

### Drag with any mouse button

Press a button, navigate, and release it elsewhere. Middle-drag pans the
canvas in apps such as Blender, and right-drag creates shortcuts on Windows.

```toml
[recursive_grid.hotkeys]
"Shift+I" = "action left_click --state down"    # shipped default
"Shift+U" = "action left_click --state up"      # shipped default
"Shift+O" = "action right_click --state down"
"Shift+P" = "action right_click --state up"
"Shift+K" = "action middle_click --state down"
"Shift+J" = "action middle_click --state up"
```

`--toggle` presses a free button and releases a held one, so one key covers the
whole drag, as in `"Shift+I" = "action left_click --toggle"`. Navigation keys
steer the drag. Neru releases any held button when it returns to idle, so
`Escape` mid-drag never leaves a button stuck.

To start a drag from a hint, use the action name, since `--action` takes names:

```toml
[hotkeys]
"Primary+Shift+D" = "hints --action left_mouse_down"
```

### Bind a shortcut to a specific UI element

Add a shortcut an app lacks, such as `Cmd+1` to `Cmd+3` for the Home, Code and
Cowork views in Claude for macOS. A root-level `[[app_configs]]` block
overrides `[hotkeys]` only while that app is focused, see
[Per-App Global Hotkey Overrides](../reference/configuration.md#per-app-global-hotkey-overrides).

| Approach                    | Survives window move | Survives resize | Breaks when                      |
| --------------------------- | -------------------- | --------------- | -------------------------------- |
| Absolute coordinates        | No                   | No              | the window ever moves            |
| Window-relative coordinates | Yes                  | No              | the layout is responsive         |
| Filtered hints + feed       | Yes                  | Yes             | the role and text are not unique |

**1. Absolute coordinates.**

```toml
[[app_configs]]
bundle_id = "com.anthropic.claudefordesktop"
hotkeys = { "Cmd+1" = ["action move_mouse --x 113 --y 123", "action left_click"] }
```

**2. Window-relative coordinates.** `--window` offsets from the window centre,
and a large negative offset clamps to the top-left corner. `move_mouse_relative`
then walks to the element.

```toml
[[app_configs]]
bundle_id = "com.anthropic.claudefordesktop"
hotkeys = { "Cmd+1" = ["action move_mouse --window --x -1000 --y -1000", "action sleep 0.1", "action move_mouse_relative --dx 100 --dy 70", "action sleep 0.1", "action left_click"] }
```

**3. Filtered hints + feed.** Filter hints by role and text, then feed the
first label. Which element gets `a` depends on your hint settings, so confirm
it. A common label such as "Code" also matches every "Copy code" button, so
use the window-relative form there.

```toml
[[app_configs]]
bundle_id = "com.anthropic.claudefordesktop"
hotkeys = { "Cmd+1" = ["hints --role button --text Home --action left_click", "action feed --mode a"] }
```

To bind several keys that differ only in offsets, name the sequence once in
[`[macros]`](../reference/configuration.md#macros):

```toml
[macros]
window_click = [
    "action move_mouse --window --x -1000 --y -1000",
    "action sleep 0.1",
    "action move_mouse_relative --dx $1 --dy $2",
    "action sleep 0.1",
    "action left_click",
]

[[app_configs]]
bundle_id = "com.anthropic.claudefordesktop"
hotkeys = {
    "Cmd+1" = "macro window_click 100 70",
    "Cmd+2" = "macro window_click 250 70",
    "Cmd+3" = "macro window_click 400 70"
}
```

Get each offset by subtracting the window's top-left corner from the button's
screen coordinates.

## Hints

### Hints search (Homerow.app style, sort of...)

Filter hints by element text. Press `/` (`action search_hints`) in hints mode,
or open hints with search showing:

```toml
[hotkeys]
"Primary+Shift+Space" = "hints --search"
```

- Typing filters by element title, description or value. `Space` is allowed.
- `Backspace` deletes a character. `Escape` cancels and restores every hint.
- `Return` selects the first match and runs the binding's `--action`. With an
  `--action` and several matches, `Return` only closes the search so you can
  type the label.
- `Tab` (`action cycle_hint`) moves between matches without running the action.

Search needs element text, so it works with the `axtree` and `vision`
strategies but not `contour`. See [`[hints]`](../reference/configuration.md#hints).

### Give browser content time to load before refreshing hints

Wait for the page to update after a click before hints redraw, in one app only:

```toml
[[hints.app_configs]]
bundle_id = "com.brave.Browser"
hotkeys = {
	"Return" = ["action left_click", "action sleep 0.8", "hints"],
	"Shift+L" = "__disabled__"
}
```

These keys merge over `[hints.hotkeys]` for Brave only. Other modes'
`app_configs` work the same way.

### Checking the accessibility tree on macOS

Find an element's role and text for `clickable_roles` or `--role` with one of:

- **UIElementInspector**, an Apple sample app that needs no Xcode:
  [UIElementInspector.zip](https://developer.apple.com/library/mac/samplecode/UIElementInspector/UIElementInspector.zip)
- **Accessibility Inspector**: **Xcode > Open Developer Tool > Accessibility
  Inspector**

`neru roles` maps Neru's role names to the platform's.

## Cursor

### Restore cursor position after mode exit

Return the cursor to where it was after hints clicks:

```toml
[hotkeys]
"Primary+Shift+Space" = ["action save_cursor_pos", "hints"]

[hints.hotkeys]
"Enter" = ["action left_click", "idle", "action restore_cursor_pos"]
```

When the click comes from the mode's `--action`, use the repeatable
`--on-exit` flag:

```toml
[hotkeys]
"Primary+Shift+Space" = [
    "action save_cursor_pos",
    "hints --action left_click --on-exit 'action restore_cursor_pos'",
]
```

`--on-exit` runs only after the action, so `Escape` leaves the cursor where you
moved it. Use `--slot` to keep two saved positions apart, see
[Cursor slots](../reference/cli.md#cursor-slots).

### Target menus without moving the real cursor

Keep the pointer still while you refine a selection, for menus that close when
the pointer leaves:

```toml
[hotkeys]
"Primary+Shift+G" = "grid --cursor-selection-mode hold"
"Primary+Shift+C" = "recursive_grid --cursor-selection-mode hold"

[recursive_grid.hotkeys]
"Return" = "action left_click"
```

Click and scroll actions act on the selection, and move the pointer only then.
Add `--bare` to act at the real cursor. The default `` ` `` binding,
`toggle-cursor-follow-selection`, switches between `hold` and `follow`.

### Auto-zoom to depth on activation

Open recursive grid already drilled down at the cursor:

```toml
[hotkeys]
"Primary+Shift+2" = "recursive_grid --zoom-to-depth 2"
"Primary+Shift+3" = "recursive_grid --zoom-to-depth 3 --action left_click"
```

A depth beyond the deepest level stops at that level.

## Modes and hotkeys

### Mode toggle (on/off)

Enter and leave a mode with one key:

```toml
[hotkeys]
"Ctrl+F" = "grid --toggle"
"Ctrl+G" = "recursive_grid --toggle"
"Ctrl+H" = "hints --toggle"
```

### Cycle through modes with one hotkey

Step through hints, recursive grid, grid and scroll with one key. Inside a
mode, its `[<mode>.hotkeys]` binding wins over the global one, see
[Resolution order](../reference/configuration.md#resolution-order), so every
mode in the cycle needs an entry.

```toml
[hotkeys]
"Primary+Ctrl+F" = "hints"            # from idle

[hints.hotkeys]
"Primary+Ctrl+F" = "recursive_grid"

[recursive_grid.hotkeys]
"Primary+Ctrl+F" = "grid"

[grid.hotkeys]
"Primary+Ctrl+F" = "scroll"

[scroll.hotkeys]
"Primary+Ctrl+F" = "hints"            # wrap back to the start
```

`Escape` still returns to idle. On macOS and Windows, you can set the default
launchers `Primary+Shift+Space`, `Primary+Shift+G`, `Primary+Shift+C` and
`Primary+Shift+S` to `"__disabled__"`.

### Switch a grid layout from one key

Swap recursive grid layouts. `--no-reload` defers the reload to the last step:

```toml
[hotkeys]
"Cmd+8" = [
    "config set recursive_grid.grid_cols 3 --no-reload",
    "config set recursive_grid.grid_rows 3 --no-reload",
    "config set recursive_grid.keys gcrhtnmwv",
]
"Cmd+9" = [
    "config reset recursive_grid.grid_cols --no-reload",
    "config reset recursive_grid.grid_rows --no-reload",
    "config reset recursive_grid.keys",
]
```

`config set` values persist in the override file, see
[Runtime config changes](getting-started.md#runtime-config-changes).

### Disabling all built-in hotkeys

Leave global bindings to an external daemon such as skhd. An empty `[hotkeys]`
table clears every default global hotkey:

```toml
[hotkeys]
```

Each `[<mode>.hotkeys]` stays. Trigger modes with `neru hints`, `neru grid` and
so on, as in [Scripting](../reference/scripting.md).

## Config files

### Running a custom configuration via app bundle

Start the macOS app with a specific config, such as a screen-sharing profile.
`~` is not expanded, so use an absolute path:

```bash
open -a neru --args launch -c /absolute/path/to/your/config
```

### Edit config file directly

Open the config the daemon uses:

```bash
neru status --json | jq -r .config | xargs nvim
```

For a new window, pass the pipeline to your terminal, such as
`open -na Ghostty --args -e bash -c "..."` on macOS.
