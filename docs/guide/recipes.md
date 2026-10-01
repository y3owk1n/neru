# Recipes

Worked configurations for common goals. Each recipe states the goal, then the
configuration that reaches it. What every option does is in the
[configuration reference](../reference/configuration.md), and every command and
flag is in the [CLI reference](../reference/cli.md). Driving Neru from scripts
and external hotkey daemons is covered in [Scripting](../reference/scripting.md).

The launcher keys below are examples. On Linux no global hotkeys are bound by
default, so bind whichever keys you like. See [Getting started](getting-started.md).

## Clicking

### Vimium-style click-on-select

Hints mode that clicks as soon as you finish typing a label, like Vimium in a
browser. Give each button its own launcher:

```toml
[hotkeys]
"Primary+Shift+Space" = "hints --action left_click"
"Primary+Shift+R"     = "hints --action right_click"   # context menu
```

On macOS and Windows `Primary+Shift+Space` is the default hints launcher, so the
first line replaces the default behaviour rather than adding to it. Bind another
key if you want both.

### Homerow action clicks

Click the selected hint with `Return`, homerow.app style:

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

Click and leave the mode with one key. A binding can hold a list of steps:

```toml
[hints.hotkeys]
"Shift+L" = ["action left_click", "idle"]
"Shift+R" = ["action right_click", "idle"]
```

This works in any mode. By default the mode exits whether or not the click
landed. End the click step with `--bail-on-error` to stop the sequence there
instead, which leaves you in the mode to try again:

```toml
[hints.hotkeys]
"Shift+L" = ["action left_click --bail-on-error", "idle"]
```

The directive is described under
[Failure policy](../reference/cli.md#failure-policy).

### Click, sleep, move

Some apps, such as Discord, register a click only if the pointer stays put for
a moment afterwards. Sleep between the click and the next move:

```toml
[recursive_grid.hotkeys]
# Click, wait, then reset (which moves the cursor back to the centre of the grid)
"Ctrl+J" = ["action left_click", "action sleep 0.05", "action reset"]
```

### Drag with any mouse button

Click actions take `--state down` and `--state up`, so a drag is "press here,
move there, release". This works for the left, right and middle button.
Right-drag creates shortcuts on Windows, and middle-drag pans the canvas in apps
like TouchDesigner, Blender and Photoshop.

```toml
[recursive_grid.hotkeys]
# Press at the current selection, then navigate and release somewhere else
"Shift+I" = "action left_click --state down"
"Shift+U" = "action left_click --state up"

"Shift+O" = "action right_click --state down"
"Shift+P" = "action right_click --state up"

"Shift+K" = "action middle_click --state down"
"Shift+J" = "action middle_click --state up"
```

`Shift+I` and `Shift+U` are the shipped defaults for the left button. The right
and middle bindings are additions.

If two keys per button is too many, `--toggle` presses the button when it is
free and releases it when it is held, so one key covers the whole drag:

```toml
[recursive_grid.hotkeys]
"Shift+I" = "action left_click --toggle"
"Shift+O" = "action right_click --toggle"
"Shift+K" = "action middle_click --toggle"
```

Moving the cursor while a button is held drags with that button, so the usual
grid and hint navigation keys steer the drag. Neru releases any button it is
holding when it returns to idle, so pressing `Escape` mid-drag never leaves a
button stuck down.

To start a drag from a hinted element instead of a grid selection, use the
action name form, since a mode's `--action` takes names, not flags:

```toml
[hotkeys]
"Primary+Shift+D" = "hints --action left_mouse_down"
```

### Bind a shortcut to a specific UI element

Some apps never expose a keyboard shortcut for a UI element you use often, and
some remove one you relied on. Claude for macOS, for example, dropped `Cmd+1`,
`Cmd+2` and `Cmd+3` for switching between its Home, Code and Cowork views. In
some cases you can rebuild one by driving Neru to click a fixed spot or a
specific element in the focused window.

Bind the key inside a root-level `[[app_configs]]` block scoped to the app by its
`bundle_id`. That block overrides `[hotkeys]` only while the app is focused, so
the key drives Neru there and passes straight through everywhere else. See
[Per-App Global Hotkey Overrides](../reference/configuration.md#per-app-global-hotkey-overrides).

Pointing at an element reliably is the hard part, and the three approaches below
break under different conditions:

| Approach                    | Survives window move | Survives resize | Breaks when                      |
| --------------------------- | -------------------- | --------------- | -------------------------------- |
| Absolute coordinates        | No                   | No              | the window ever moves            |
| Window-relative coordinates | Yes                  | No              | the layout is responsive         |
| Filtered hints + feed       | Yes                  | Yes             | the role and text are not unique |

**1. Absolute coordinates.** This is the most direct option. It holds only if
the window never moves or resizes:

```toml
[[app_configs]]
bundle_id = "com.anthropic.claudefordesktop"
hotkeys = { "Cmd+1" = ["action move_mouse --x 113 --y 123", "action left_click"] }
```

**2. Window-relative coordinates.** Recomputes the target from the focused
window each time, so it survives moving the window and breaks only on resize.
Move to the window centre, offset to a corner, then nudge to the target:

```toml
[[app_configs]]
bundle_id = "com.anthropic.claudefordesktop"
hotkeys = { "Cmd+1" = ["action move_mouse --window --x -1000 --y -1000", "action sleep 0.1", "action move_mouse_relative --dx 100 --dy 70", "action sleep 0.1", "action left_click"] }
```

`--window` measures the offset from the window centre. A large negative offset
like `--x -1000 --y -1000` is clamped to the window's top-left corner, which
gives a stable origin, and `move_mouse_relative` then walks to the element.

**3. Filtered hints + feed.** Targets an element by its accessibility role and
text, then feeds the first hint label to click it:

```toml
[[app_configs]]
bundle_id = "com.anthropic.claudefordesktop"
hotkeys = { "Cmd+1" = ["hints --role button --text Home --action left_click", "action feed --mode a"] }
```

`action feed --mode a` types the first hint label into Neru. Which element gets
`a` depends on your hint configuration, such as menu-bar hints and label
direction, so confirm it lands on the element you mean. This method is precise when
the element is unique and fragile when the text is common. A button labelled
"Code" is easy to confuse with every "Copy code" button in the same window. When
a filter is too ambiguous to trust, fall back to the window-relative form for
that key.

Putting it together, a Claude view switcher scopes three keys to the app. Only
the offsets differ between them, so name the sequence once in
[`[macros]`](../reference/configuration.md#macros) and pass the offsets in:

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

Capture your own offsets once, since they depend on the window's layout. Move the
pointer over each button, read its screen coordinates, and subtract the window's
top-left corner to get the `--dx` and `--dy` values.

## Hints

### Hints search (Homerow.app style, sort of...)

Filter hints by typing what the element says, similar to homerow.app. Press `/`
in hints mode and type. `/` is the default binding for `action search_hints`. To open
hints with the search box already showing, bind a launcher with `--search`:

```toml
[hotkeys]
"Primary+Shift+Space" = "hints --search"
```

While searching:

- Typing filters hints by element title, description, or value. `Space` is
  allowed, for multi-word queries.
- `Backspace` removes a character, and `Escape` cancels the search and restores
  every hint.
- `Return` closes the search and selects the first match, running the binding's
  `--action` if it has one. When the binding has an `--action` and more than one
  hint matches, `Return` only closes the search, so you can type the exact label.
- `Tab` (`action cycle_hint`) moves between the filtered results without running
  the action.

Search needs element text, so it works with the `axtree` and `vision`
strategies but not `contour`. See [`[hints]`](../reference/configuration.md#hints).

### Give browser content time to load before refreshing hints

Some browser-like apps need a short delay after a click so the page can finish
updating before Neru draws hints again. Override only that app's hint hotkeys:

```toml
[[hints.app_configs]]
bundle_id = "com.brave.Browser"
hotkeys = {
	"Return" = ["action left_click", "action sleep 0.8", "hints"],
	"Shift+L" = "__disabled__"
}
```

This merges over `[hints.hotkeys]`, so only the keys listed here change for Brave,
and every other key keeps your normal hint bindings. The same pattern works in
`[[grid.app_configs]]`, `[[recursive_grid.app_configs]]` and the other modes'
`app_configs`.

### Checking the accessibility tree on macOS

To see which role and text an element exposes, for `clickable_roles` or a
`--role` filter, inspect it with one of these:

- **UIElementInspector**, a small sample app from Apple that needs no Xcode:
  [UIElementInspector.zip](https://developer.apple.com/library/mac/samplecode/UIElementInspector/UIElementInspector.zip)
- **Accessibility Inspector**, which ships with Xcode: **Xcode > Open Developer
  Tool > Accessibility Inspector**

`neru roles` shows how Neru's role names map to the platform's names.

## Cursor

### Restore cursor position after mode exit

Put the cursor back where it was after hints clicks something. Save it before
the mode opens and restore it after the click:

```toml
[hotkeys]
"Primary+Shift+Space" = ["action save_cursor_pos", "hints"]

[hints.hotkeys]
"Enter" = ["action left_click", "idle", "action restore_cursor_pos"]
```

If the click comes from the mode's own `--action` rather than a key inside hints,
put the restore in `--on-exit`. The flag is repeatable, so the whole sequence
lives on the binding that starts the mode:

```toml
[hotkeys]
"Primary+Shift+Space" = [
    "action save_cursor_pos",
    "hints --action left_click --on-exit 'action restore_cursor_pos'",
]
```

`--on-exit` steps run only after the action is fulfilled, so escaping out of
hints leaves the cursor where you moved it rather than snapping it back. To keep
two saved positions apart, give each its own `--slot`. See
[Cursor slots](../reference/cli.md#cursor-slots).

### Target menus without moving the real cursor

Some menus close as soon as the pointer leaves them. Start grid or recursive
grid with `--cursor-selection-mode hold` so the real pointer stays still while
you refine the selection:

```toml
[hotkeys]
"Primary+Shift+G" = "grid --cursor-selection-mode hold"
"Primary+Shift+C" = "recursive_grid --cursor-selection-mode hold"

[recursive_grid.hotkeys]
"Return" = "action left_click"
```

Click and scroll actions act on the current selection, so `Return` clicks the
cell you picked and moves the pointer only then. Add `--bare` to an action to
act at the real cursor instead. Inside either mode, the default `` ` `` binding,
`toggle-cursor-follow-selection`, switches between `hold` and `follow`.

### Auto-zoom to depth on activation

Open recursive grid already drilled down at the cursor, skipping the first
levels:

```toml
[hotkeys]
"Primary+Shift+2" = "recursive_grid --zoom-to-depth 2"
"Primary+Shift+3" = "recursive_grid --zoom-to-depth 3 --action left_click"
```

A depth beyond the deepest level the grid can reach stops at that level.

## Modes and hotkeys

### Mode toggle (on/off)

Use one key to both enter and leave a mode. With `--toggle`, the first press
opens the mode and the second returns to idle:

```toml
[hotkeys]
"Ctrl+F" = "grid --toggle"
"Ctrl+G" = "recursive_grid --toggle"
"Ctrl+H" = "hints --toggle"
```

### Cycle through modes with one hotkey

Walk through several modes with one key. Press it from idle to open the first
mode, then press it again to advance through hints, recursive grid, grid and
scroll, wrapping back to hints at the end.

This works because inside a mode, that mode's own `[<mode>.hotkeys]` binding
wins over a global binding for the same key. See
[Resolution order](../reference/configuration.md#resolution-order). A global
binding the active mode does not rebind keeps working inside it, so the cycle
needs a per-mode entry for every mode it walks.

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

`Escape` still returns to idle from any point in the cycle. To change the order
or shorten the loop, edit which mode each block points to.

On macOS and Windows you may want to drop the default launchers this cycle
replaces:

```toml
[hotkeys]
"Primary+Shift+Space" = "__disabled__"   # default hints launcher
"Primary+Shift+G"     = "__disabled__"   # default grid launcher
"Primary+Shift+C"     = "__disabled__"   # default recursive_grid launcher
"Primary+Shift+S"     = "__disabled__"   # default scroll launcher
```

### Switch a grid layout from one key

Swap recursive grid between two layouts without editing the file. `config set`
and `config reset` work as binding steps, and `--no-reload` holds the reload
until the last step so the fields change together:

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

`config set` changes persist across restarts in the override file. See
[Runtime config changes](getting-started.md#runtime-config-changes).

### Disabling all built-in hotkeys

Leave every global binding to an external hotkey daemon such as skhd. An empty
`[hotkeys]` table clears all default global hotkeys:

```toml
[hotkeys]
# No bindings, so every default is cleared.
```

Each mode keeps its own `[<mode>.hotkeys]`. Trigger the modes from the daemon
with `neru hints`, `neru grid` and so on, as shown in
[Scripting](../reference/scripting.md).

## Config files

### Running a custom configuration via app bundle

On macOS, start the app bundle with a specific config file:

```bash
open -a neru --args launch -c /absolute/path/to/your/config
```

> [!NOTE]
> `~` is not expanded here, so use the full absolute path.

This is useful for testing a config before committing it to your dotfiles, or for
keeping separate profiles, such as a lighter one for screen sharing.

### Edit config file directly

Open the config file the daemon is using, without looking up its path:

```bash
neru status --json | jq -r .config | xargs nvim
```

To open it in a new terminal window, run the same pipeline through your
terminal's command-line launcher. For Ghostty on macOS:

```bash
open -na Ghostty --args -e bash -c "neru status --json | jq -r .config | xargs nvim"
```

Wrap it in a shell alias, or bind it in your window manager or hotkey daemon.
