# Configuration

Neru configuration is TOML-based.

## Core Principle

In-mode bindings are defined only through per-mode `custom_hotkeys`.

Removed legacy fields:

- `general.mode_exit_keys`
- `<mode>.auto_exit_actions`
- `<mode>.mode_exit_keys`
- `grid.reset_key`
- `recursive_grid.reset_key`
- `hints.backspace_key`
- `grid.backspace_key`
- `recursive_grid.backspace_key`
- `[scroll.key_bindings]`
- `[action]` and `[action.key_bindings]`

## Priority Order

When a mode is active:

1. modifier toggle
2. mode `custom_hotkeys`
3. mode-specific input keys (hint/grid/recursive-grid character input)

## Actions

`custom_hotkeys` actions use the same syntax as `[hotkeys]` values.

Examples:

- `"idle"`
- `"hints"`
- `"grid"`
- `"recursive_grid"`
- `"scroll"`
- `"action left_click"`
- `"action move_mouse_relative --dx=0 --dy=-10"`
- `"exec /usr/bin/say hello"`

## Multi-action Bindings

Bindings can be string or array:

```toml
"Shift+L" = "action left_click"
"Enter" = ["action left_click", "idle"]
```

## Multi-key Sequences

Two-letter alphabetic sequences are supported in `custom_hotkeys`:

```toml
"gg" = "action go_top"
```

Sequence timeout is `500ms`.

## Starter Config

```toml
[hotkeys]
"Cmd+Shift+Space" = "hints"
"Cmd+Shift+G" = "grid"
"Cmd+Shift+C" = "recursive_grid"
"Cmd+Shift+S" = "scroll"

[hints.custom_hotkeys]
"Escape" = "idle"
"Backspace" = "action backspace"
"Shift+L" = "action left_click"
"Shift+R" = "action right_click"
"Shift+M" = "action middle_click"
"Shift+I" = "action mouse_down"
"Shift+U" = "action mouse_up"
"Up" = "action move_mouse_relative --dx=0 --dy=-10"
"Down" = "action move_mouse_relative --dx=0 --dy=10"
"Left" = "action move_mouse_relative --dx=-10 --dy=0"
"Right" = "action move_mouse_relative --dx=10 --dy=0"

[grid.custom_hotkeys]
"Escape" = "idle"
"Backspace" = "action backspace"
"Shift+L" = "action left_click"
"Shift+R" = "action right_click"
"Shift+M" = "action middle_click"
"Shift+I" = "action mouse_down"
"Shift+U" = "action mouse_up"
"Up" = "action move_mouse_relative --dx=0 --dy=-10"
"Down" = "action move_mouse_relative --dx=0 --dy=10"
"Left" = "action move_mouse_relative --dx=-10 --dy=0"
"Right" = "action move_mouse_relative --dx=10 --dy=0"

[recursive_grid.custom_hotkeys]
"Escape" = "idle"
"Backspace" = "action backspace"
"Shift+L" = "action left_click"
"Shift+R" = "action right_click"
"Shift+M" = "action middle_click"
"Shift+I" = "action mouse_down"
"Shift+U" = "action mouse_up"
"Up" = "action move_mouse_relative --dx=0 --dy=-10"
"Down" = "action move_mouse_relative --dx=0 --dy=10"
"Left" = "action move_mouse_relative --dx=-10 --dy=0"
"Right" = "action move_mouse_relative --dx=10 --dy=0"

[scroll.custom_hotkeys]
"Escape" = "idle"
"k" = "action scroll_up"
"j" = "action scroll_down"
"h" = "action scroll_left"
"l" = "action scroll_right"
"gg" = "action go_top"
"Shift+G" = "action go_bottom"
"u" = "action page_up"
"PageUp" = "action page_up"
"d" = "action page_down"
"PageDown" = "action page_down"
"Shift+L" = "action left_click"
"Shift+R" = "action right_click"
"Shift+M" = "action middle_click"
"Shift+I" = "action mouse_down"
"Shift+U" = "action mouse_up"
"Up" = "action move_mouse_relative --dx=0 --dy=-10"
"Down" = "action move_mouse_relative --dx=0 --dy=10"
"Left" = "action move_mouse_relative --dx=-10 --dy=0"
"Right" = "action move_mouse_relative --dx=10 --dy=0"
```
