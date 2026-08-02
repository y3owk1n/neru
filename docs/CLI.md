# CLI Reference

Complete reference for every `neru` command, flag, and argument.

Neru runs as a background daemon. Most commands are thin clients that send a
request to that daemon over a Unix socket (a named pipe on Windows) and print
the reply. "The daemon" below means the process started by `neru launch`.

The same content is available as manpages (`man neru`) after installation.

**Related:** [Configuration Reference](CONFIGURATION.md) ·
[Installation](INSTALLATION.md) · [Troubleshooting](TROUBLESHOOTING.md)

---

## Table of Contents

- [How to read this reference](#how-to-read-this-reference)
- [Global flags](#global-flags)
- [Command index](#command-index)
- [Daemon lifecycle](#daemon-lifecycle) — `launch` · `start` · `stop` · `idle` · `status` · `doctor`
- [Navigation modes](#navigation-modes) — `hints` · `grid` · `recursive_grid` · `scroll` · `monitor_select`
- [Actions](#actions) — `action` and its subcommands
- [Sequences](#sequences) — `run`
- [Configuration commands](#configuration-commands) — `config`
- [Runtime toggles](#runtime-toggles)
- [Utilities](#utilities) — `roles` · `services` · `docs`
- [Scripting](#scripting)
- [IPC protocol](#ipc-protocol)

---

## How to read this reference

Every command is documented in the same shape: a one-line purpose, a synopsis,
a description, a flag table, and examples.

**Synopsis notation**

| Notation      | Meaning                                  |
| ------------- | ---------------------------------------- |
| `<value>`     | Required placeholder you replace         |
| `[--flag]`    | Optional flag                            |
| `a\|b`        | Choose one                               |
| `[<key>...]`  | Repeatable argument                      |

**Daemon requirement** is stated per command. Commands that do not need the
daemon are `launch`, `doctor`, `roles`, `config init`, and `config validate`;
every other command requires a running daemon.

**Platform support** is listed for every command and flag whose behaviour is not
identical on all three platforms, as a `Platforms:` line or a `Platforms`
column. Anything without such a note works the same on macOS, Linux, and
Windows. Commands that are unavailable return `ERR_NOT_SUPPORTED`.

On Linux, "supported" means an X11 session or a Wayland session on wlroots or
KWin. GNOME Wayland is not supported at all — the daemon exits at startup. See
[CROSS_PLATFORM.md](CROSS_PLATFORM.md#platform-status).

`-h`, `--help` is accepted by every command and is omitted from the flag tables
below.

---

## Global flags

Accepted by every command.

| Flag        | Shorthand | Type   | Default | Description                                                                        |
| ----------- | --------- | ------ | ------- | ---------------------------------------------------------------------------------- |
| `--config`  | `-c`      | string | `""`    | Path to the config file. Overrides the default search paths. See [Config file location](CONFIGURATION.md#config-file-location). |
| `--timeout` |           | int    | `10`    | IPC timeout in seconds.                                                             |

---

## Command index

| Command                             | Purpose                                        | Needs daemon | Platforms |
| ----------------------------------- | ---------------------------------------------- | :----------: | --------- |
| [`launch`](#neru-launch)                                     | Start the daemon                | No  | All |
| [`start`](#neru-start)                                       | Resume after `stop`             | Yes | All |
| [`stop`](#neru-stop)                                         | Pause without exiting           | Yes | All |
| [`idle`](#neru-idle)                                         | Exit the active mode            | Yes | All |
| [`status`](#neru-status)                                     | Print daemon state              | Yes | All |
| [`doctor`](#neru-doctor)                                     | Run diagnostics                 | No  | All |
| [`hints`](#neru-hints)                                       | Label and click UI elements     | Yes | All ¹ |
| [`grid`](#neru-grid)                                         | Coordinate grid navigation      | Yes | All |
| [`recursive_grid`](#neru-recursive_grid)                     | Recursive cell navigation       | Yes | All |
| [`scroll`](#neru-scroll)                                     | Vim-style scrolling             | Yes | All |
| [`monitor_select`](#neru-monitor_select)                     | Jump the cursor to a display    | Yes | macOS · Linux |
| [`action`](#actions)                                         | One-shot mouse/scroll/key input | Yes | All ² |
| [`run`](#neru-run)                                           | Run several actions in order    | Yes | All |
| [`config`](#configuration-commands)                          | Inspect and change config       | Mixed | All |
| [`toggle-scroll-invert`](#neru-toggle-scroll-invert)         | Invert scroll direction         | Yes | All |
| [`toggle-cursor-follow-selection`](#neru-toggle-cursor-follow-selection) | Toggle cursor follow | Yes | All |
| [`toggle-screen-share`](#neru-toggle-screen-share)           | Hide overlays while sharing     | Yes | macOS |
| [`roles`](#neru-roles)                                       | List the role vocabulary        | No  | All |
| [`services`](#neru-services)                                 | Manage the system service       | No  | macOS |
| [`docs`](#neru-docs)                                         | Open documentation in a browser | No  | macOS |

¹ Element discovery quality differs by platform: a full accessibility tree on
macOS, an AT-SPI walk on Linux whose coverage depends on the application, and an
initial shallow UI Automation walk on Windows. The `vision` strategy is macOS
only. See [Accessibility and hints](CROSS_PLATFORM.md#accessibility-and-hints).

² Two action subcommands are limited: `hide_cursor` and `show_cursor` are macOS
only, and `scroll_left` / `scroll_right` have no effect on Windows. See
[Action platform support](#action-platform-support).

---

# Daemon lifecycle

## neru launch

Start the Neru daemon.

```
neru launch [-c <path>] [--timeout <seconds>]
```

Runs the background process that owns the event tap, overlays, and IPC server.
Does not require a running daemon; this is what starts one. Takes only the
[global flags](#global-flags).

---

## neru start

Resume Neru after `neru stop`.

```
neru start
```

Requires a running daemon. Re-enables mode switching and overlay rendering.

---

## neru stop

Pause Neru without exiting the daemon.

```
neru stop
```

Requires a running daemon. The process keeps running and keeps its socket open,
but mode switching and overlay rendering are disabled. Resume with
[`neru start`](#neru-start).

---

## neru idle

Exit the active navigation mode.

```
neru idle
```

Requires a running daemon. Returns to idle. No-op when no mode is active.

---

## neru status

Print the daemon state and current mode.

```
neru status
```

Requires a running daemon.

**Output fields**

| Field    | Values                                                  |
| -------- | ------------------------------------------------------- |
| `Status` | `running`, `disabled`                                   |
| `Mode`   | `idle`, `hints`, `grid`, `recursive_grid`, `scroll`, `monitor_select` |

---

## neru doctor

Run system diagnostics.

```
neru doctor
```

Does not require a running daemon. Reports config validity, socket health,
platform capabilities, and internal component state. Platform capabilities come
from the capability matrix described in
[CROSS_PLATFORM.md](CROSS_PLATFORM.md#capability-matrix).

---

# Navigation modes

Modes take over the keyboard until you select a target or exit. All five
require a running daemon.

## Flags shared by hints, grid, and recursive_grid

These three modes accept the same selection flags. Mode-specific flags are
listed under each command.

| Flag                       | Shorthand | Type   | Default  | Description                                                                                                       |
| -------------------------- | --------- | ------ | -------- | ----------------------------------------------------------------------------------------------------------------- |
| `--action`                 | `-a`      | string |          | Mouse-button action to perform on selection — see [action names](#action-names) for the accepted set. Commas chain multiple actions (`left_click,left_click` is a double-click). |
| `--toggle`                 | `-t`      | bool   | `false`  | Exit to idle if this mode is already active, otherwise enter it.                                                    |
| `--repeat`                 | `-r`      | bool   | `false`  | Re-enter the mode after the action instead of exiting. Requires `--action`.                                         |
| `--modifier`               |           | string |          | Comma-separated modifiers held during the action: `cmd`, `super`, `meta`, `shift`, `alt`, `option`, `ctrl`. Requires `--action`. |
| `--on-exit`                |           | string |          | Step run after the action completes and the mode exits. Uses hotkey-binding syntax (`'action left_click'`, `'exec notify-send done'`). Repeat the flag to run several steps in order, as one [action sequence](#neru-run). Requires `--action`. Not run when the mode is left manually via escape or `neru idle`. |
| `--cursor-selection-mode`  |           | string | `follow` | `follow` moves the real cursor to the selection; `hold` leaves it in place.                                          |

---

## neru hints

Label clickable elements and act on the one you type.

```
neru hints [-a <action>] [-t] [-r] [--modifier <mods>] [--on-exit <step>]...
           [--cursor-selection-mode follow|hold] [-s] [--hide-on-empty-search]
           [--role <roles>] [--text <text>] [--strategy axtree|vision]
           [--label-direction normal|reverse] [--split-word] [-d]
```

Scans the focused window for interactive elements and overlays a short letter
label on each. Typing a label selects that element.

Element discovery uses the `axtree` strategy by default. The `vision` strategy
is macOS-only and detects on-screen text and rectangles via the Vision
framework. Coverage per platform is documented in
[CROSS_PLATFORM.md](CROSS_PLATFORM.md#accessibility-and-hints).

**Flags** — in addition to the [shared mode flags](#flags-shared-by-hints-grid-and-recursive_grid).

| Flag                     | Shorthand | Type   | Default  | Description                                                                                     |
| ------------------------ | --------- | ------ | -------- | ------------------------------------------------------------------------------------------------- |
| `--search`               | `-s`      | bool   | `false`  | Show the search input when the mode activates.                                                    |
| `--hide-on-empty-search` |           | bool   | `false`  | Hide all hints while the search query is empty. Requires `--search`.                              |
| `--role`                 |           | string |          | Only hint elements whose role matches. Comma-separated. Accepts the vocabulary listed by [`neru roles`](#neru-roles). |
| `--text`                 |           | string |          | Only hint elements whose text matches. Comma-separated (OR), case-insensitive substring match.    |
| `--strategy`             |           | string | `axtree` | Element detection strategy: `axtree` or `vision`. `vision` is macOS only. Overrides `hints.strategy`. |
| `--label-direction`      |           | string | `normal` | Label enumeration: `normal` or `reverse`. Overrides `hints.label_direction`. See [Choosing a label direction](CONFIGURATION.md#choosing-a-label-direction). |
| `--split-word`           |           | bool   | `false`  | Split detected text into word-level regions. Requires `--strategy vision`, so macOS only.          |
| `--debug`                | `-d`      | bool   | `false`  | Print the elements that would be hinted, with a count and a sample, without showing the overlay.  |

**Examples**

```bash
neru hints
neru hints --action left_click
neru hints --action left_click --modifier shift
neru hints --action left_click --repeat
neru hints --search
neru hints --role button --text submit
neru hints --strategy vision --split-word
neru hints --debug
```

---

## neru grid

Divide the screen into a labelled coordinate grid.

```
neru grid [-a <action>] [-t] [-r] [--modifier <mods>] [--on-exit <step>]...
          [--cursor-selection-mode follow|hold]
```

Overlays a grid of labelled cells. Typing a cell label moves the cursor there.
Takes only the [shared mode flags](#flags-shared-by-hints-grid-and-recursive_grid).

Grid size, labels, and appearance are configured under
[`[grid]`](CONFIGURATION.md#grid).

Typing a full label opens a 3x3 subgrid inside that cell. To correct an
off-by-one label without retyping it, bind
[`move_cell`](#neru-action-move_cell) — it moves the open subgrid to a
neighbouring cell.

**Examples**

```bash
neru grid
neru grid --action left_click --repeat
neru grid --cursor-selection-mode hold
neru grid --action left_click --on-exit 'exec notify-send clicked'
```

---

## neru recursive_grid

Narrow the screen recursively, one keypress per level.

```
neru recursive_grid [-a <action>] [-t] [-r] [--modifier <mods>]
                    [--on-exit <step>]... [--cursor-selection-mode follow|hold]
                    [--zoom-to-depth <depth>]
```

Each keypress subdivides the selected cell, so successive presses converge on a
point. Depth limits and per-depth layout are configured under
[`[recursive_grid]`](CONFIGURATION.md#recursive_grid).

Backspace backtracks one level. To correct sideways instead of upwards, bind
[`move_cell`](#neru-action-move_cell) — it slides the selection to a
neighbouring cell without leaving the current depth.

**Flags** — in addition to the [shared mode flags](#flags-shared-by-hints-grid-and-recursive_grid).

| Flag              | Type | Default | Description                                                                                                       |
| ----------------- | ---- | ------- | ------------------------------------------------------------------------------------------------------------------- |
| `--zoom-to-depth` | int  |         | Drill to this depth at the current cursor position on activation. Stops early if the grid cannot subdivide further (minimum cell size or maximum depth). Negative values are rejected. |

**Examples**

```bash
neru recursive_grid
neru recursive_grid --action middle_click
neru recursive_grid --zoom-to-depth 2
neru recursive_grid --zoom-to-depth 3 --action left_click
```

---

## neru scroll

Scroll at the cursor with vim-style keys.

```
neru scroll [-t]
```

| Flag       | Shorthand | Type | Default | Description                                        |
| ---------- | --------- | ---- | ------- | -------------------------------------------------- |
| `--toggle` | `-t`      | bool | `false` | Exit to idle if scroll mode is active, else enter.  |

**Default key bindings**

Every binding below is configurable under [`[scroll.hotkeys]`](CONFIGURATION.md#scroll).

| Key                    | Action                     |
| ---------------------- | -------------------------- |
| `j` / `k`              | Scroll down / up           |
| `h` / `l`              | Scroll left / right        |
| `d` / `PageDown`       | Page down                  |
| `u` / `PageUp`         | Page up                    |
| `gg`                   | Jump to top                |
| `Shift+G`              | Jump to bottom             |
| `Up` / `Down` / `Left` / `Right` | Move the cursor by 10 px |
| `Shift+L` / `Shift+R` / `Shift+M` | Left / right / middle click |
| `Shift+I` / `Shift+U`  | Press / release the left button |
| `Escape`               | Exit to idle               |

Step sizes come from `scroll.scroll_step`, `scroll_step_half`, and
`scroll_step_full`.

**Examples**

```bash
neru scroll
neru scroll --toggle
```

---

## neru monitor_select

Move the cursor to another display.

```
neru monitor_select [-t]
```

**Platforms:** macOS · Linux. Not implemented on Windows, where it returns
`ERR_NOT_SUPPORTED`.

Opens a labelled panel on each display. Typing a label moves the cursor to that
display. The current display is excluded.

| Flag       | Shorthand | Type | Default | Description                                            |
| ---------- | --------- | ---- | ------- | ------------------------------------------------------ |
| `--toggle` | `-t`      | bool | `false` | Exit to idle if the mode is active, else enter it.      |

**Default key bindings**

| Key     | Action                          |
| ------- | ------------------------------- |
| `1`–`9` | Select the display with that label |
| `Escape`| Cancel and return to idle       |

Labels come from `monitor_select.characters` (default `123456789`).

**Examples**

```bash
neru monitor_select
neru monitor_select --toggle
```

---

# Actions

One-shot input that runs without entering a mode. All action subcommands
require a running daemon.

```
neru action <subcommand> [flags]
```

Each subcommand accepts only the flags documented in its own section. Passing
any other flag fails with `ERR_INVALID_INPUT` and a message naming the actions
that do accept it — flags are never accepted and then ignored.

## Targeting

Point-targeted actions resolve their target in this order: the active mode
selection when one exists, otherwise the current cursor position.

| Flag          | Type | Description                                                        |
| ------------- | ---- | ------------------------------------------------------------------ |
| `--selection` | bool | Target the active mode selection.                                   |
| `--bare`      | bool | Target the cursor position even when a mode selection exists.        |

## Action names

Every action has a name usable anywhere a name is expected: a mode `--action`,
a hotkey binding string, or `neru action` directly.

| Category | Names                                                                                             |
| -------- | ------------------------------------------------------------------------------------------------- |
| Click    | `left_click`, `right_click`, `middle_click`                                                        |
| Press    | `left_mouse_down`, `right_mouse_down`, `middle_mouse_down`                                          |
| Release  | `left_mouse_up`, `right_mouse_up`, `middle_mouse_up`                                                |
| Toggle   | `left_mouse_toggle`, `right_mouse_toggle`, `middle_mouse_toggle`                                    |
| Movement | `move_mouse`, `move_mouse_relative`, `move_monitor`                                                 |
| Scroll   | `scroll`, `scroll_up`, `scroll_down`, `scroll_left`, `scroll_right`, `page_up`, `page_down`, `go_top`, `go_bottom` |
| Mode     | `reset`, `backspace`, `move_cell`, `cycle_hint`, `wait_for_mode_exit`                               |
| Cursor   | `save_cursor_pos`, `restore_cursor_pos`, `hide_cursor`, `show_cursor`                               |
| Keys     | `feed`                                                                                              |
| Timing   | `sleep` — [hotkey bindings only](#action-sleep-hotkey-bindings-only)                                |

**Mode `--action` accepts mouse-button names only** — the click, press, release,
and toggle rows above, plus the deprecated `mouse_down` / `mouse_up`. Every
other name, including `move_mouse`, `move_mouse_relative`, and `scroll`, is
rejected with `ERR_INVALID_INPUT` and must be run as `neru action <name>` or
bound as a hotkey action instead.

## Action platform support

Every action not listed here behaves identically on macOS, Linux, and Windows.

| Action                            | macOS | Linux | Windows | Note                                                     |
| --------------------------------- | :---: | :---: | :-----: | -------------------------------------------------------- |
| `hide_cursor`, `show_cursor`      | Yes   | No    | No      | Uses a Quartz API with no cross-platform equivalent; a no-op elsewhere. |
| `scroll_left`, `scroll_right`     | Yes   | Yes   | No      | Windows scroll injection ignores the horizontal delta.    |
| `move_monitor`                    | Yes   | Yes   | Yes     | Requires more than one display.                           |

The injection mechanism differs per platform even where behaviour matches:
`CGEventPost` on macOS, XTest on X11, `zwlr_virtual_pointer` on wlroots, libei
on KDE, and `SendInput` on Windows. This affects nothing user-visible except the
scroll limitation above.

---

## neru action left_click, right_click, middle_click

Press and release a mouse button, or perform one half of that.

```
neru action left_click|right_click|middle_click
            [--modifier <mods>] [--selection] [--bare] [--state down|up] [--toggle]
```

| Flag         | Type   | Description                                                                                          |
| ------------ | ------ | ---------------------------------------------------------------------------------------------------- |
| `--modifier` | string | Modifiers held during the click: `cmd`, `shift`, `alt`, `ctrl`. Comma-separated.                      |
| `--state`    | string | Perform one half only: `down` presses and holds, `up` releases. Without it the button is pressed and released in one action. |
| `--toggle`   | bool   | Release the button if held, press and hold it otherwise.                                              |
| `--selection`| bool   | See [Targeting](#targeting).                                                                          |
| `--bare`     | bool   | See [Targeting](#targeting).                                                                          |

`--state` and `--toggle` cannot be combined, and are accepted only by these
three click subcommands. Held buttons are released automatically when Neru
returns to idle.

**Flag forms and their action names**

`--state` and `--toggle` are flags, so they cannot appear inside a comma chain
or a mode `--action`. Each combination also has an action name, which can:

| Flag form                   | Action name           |
| --------------------------- | --------------------- |
| `left_click --state down`   | `left_mouse_down`     |
| `left_click --state up`     | `left_mouse_up`       |
| `left_click --toggle`       | `left_mouse_toggle`   |
| `right_click --state down`  | `right_mouse_down`    |
| `right_click --state up`    | `right_mouse_up`      |
| `right_click --toggle`      | `right_mouse_toggle`  |
| `middle_click --state down` | `middle_mouse_down`   |
| `middle_click --state up`   | `middle_mouse_up`     |
| `middle_click --toggle`     | `middle_mouse_toggle` |

**Examples**

```bash
neru action left_click
neru action left_click --modifier cmd
neru action left_click --modifier cmd,shift
neru action right_click --modifier alt

# Drags: press at the start, release at the destination
neru action left_click --state down
neru action middle_click --state down
neru action middle_click --state up

# One binding for both halves
neru action left_click --toggle

# Comma chains
neru action left_click,left_click             # Double-click
neru action left_click,left_click,left_click  # Triple-click
neru hints --action left_click,left_click     # Same, via a mode

# Action-name equivalents
neru action right_mouse_down
neru hints --action right_mouse_down
```

### mouse_down, mouse_up (deprecated)

`mouse_down` and `mouse_up` are the original left-button spellings, from before
the right and middle buttons could be pressed and released separately. They
still work everywhere `left_mouse_down` and `left_mouse_up` do, including in
existing configs, and the CLI prints a deprecation warning on stderr naming the
replacement.

Use `neru action left_click --state down` and `neru action left_click --state up`.

---

## neru action move_mouse

Move the cursor to an absolute position.

```
neru action move_mouse [--x <px>] [--y <px>] [--center] [--window] [--selection] [--bare]
```

| Flag          | Type | Default | Description                                                                 |
| ------------- | ---- | ------- | ---------------------------------------------------------------------------- |
| `--x`         | int  | `0`     | X coordinate in pixels. With `--center` or `--window`, a horizontal offset.   |
| `--y`         | int  | `0`     | Y coordinate in pixels. With `--center` or `--window`, a vertical offset.     |
| `--center`    | bool | `false` | Target the center of the active screen.                                       |
| `--window`    | bool | `false` | Target the center of the focused window.                                      |
| `--selection` | bool | `false` | See [Targeting](#targeting).                                                  |
| `--bare`      | bool | `false` | See [Targeting](#targeting).                                                  |

**Examples**

```bash
neru action move_mouse --x 500 --y 300
neru action move_mouse --center
neru action move_mouse --center --x 50 --y -30
neru action move_mouse --window
neru action move_mouse --window --x -50
```

---

## neru action move_mouse_relative

Move the cursor by a delta.

```
neru action move_mouse_relative --dx <px> --dy <px>
```

| Flag   | Type | Required | Description                                  |
| ------ | ---- | :------: | -------------------------------------------- |
| `--dx` | int  | Yes      | Horizontal delta. Positive right, negative left. |
| `--dy` | int  | Yes      | Vertical delta. Positive down, negative up.  |

**Examples**

```bash
neru action move_mouse_relative --dx 10 --dy -5
```

---

## neru action move_monitor

Move the cursor to another display.

```
neru action move_monitor [--name <name>] [--previous]
```

Cycles to the next display by default. An active mode overlay follows the
cursor to the new display.

| Flag         | Type   | Default | Description                                                            |
| ------------ | ------ | ------- | ------------------------------------------------------------------------ |
| `--name`     | string |         | Target a display by name, e.g. `"Built-in Retina Display"`.              |
| `--previous` | bool   | `false` | Cycle to the previous display instead of the next.                       |

**Examples**

```bash
neru action move_monitor
neru action move_monitor --previous
neru action move_monitor --name "DELL U2720Q"
```

---

## neru action move_cell

Slide the active mode's selection to a neighbouring cell on the same layer.

```
neru action move_cell --direction left|right|up|down [--count <n>]
```

In **recursive-grid mode** the highlighted region moves at the current depth.
Movement is spatial rather than confined to the region you drilled into: when
the selection reaches the edge of its parent it crosses into the neighbouring
one, and the depth you can backtrack through follows it. Once the grid has
bottomed out (`max_depth` or `min_size_*`), the final cell moves instead.

In **grid mode** an open subgrid moves to the neighbouring cell. Before a
subgrid is open no cell is selected, so the action does nothing.

Movement stops at the screen edge rather than wrapping. A `--count` that runs
past the edge applies as many steps as fit. Modes with no cell selection —
hints, scroll, idle — ignore the action.

| Flag          | Type   | Default | Description                                    |
| ------------- | ------ | ------- | ---------------------------------------------- |
| `--direction` | string |         | Required. One of `left`, `right`, `up`, `down`. |
| `--count`     | int    | `1`     | Number of cells to move. Must be at least 1.    |

This action is held-key repeatable: with
[`[held_repeat]`](CONFIGURATION.md#held_repeat) enabled (it is off by default),
a hotkey bound to it slides continuously while the key is held.

**Examples**

```bash
neru action move_cell --direction right
neru action move_cell --direction up --count 3
```

Bound as hotkeys, using arrow keys because letters are already taken by cell
selection:

```toml
[recursive_grid.hotkeys]
"Left"  = "action move_cell --direction=left"
"Right" = "action move_cell --direction=right"
"Up"    = "action move_cell --direction=up"
"Down"  = "action move_cell --direction=down"
```

---

## neru action scroll_up, scroll_down, scroll_left, scroll_right

Scroll one step in a direction.

```
neru action scroll_up|scroll_down|scroll_left|scroll_right [--steps <px>] [--selection] [--bare]
```

| Flag          | Type | Description                                                                |
| ------------- | ---- | -------------------------------------------------------------------------- |
| `--steps`     | int  | Scroll amount in pixels. Uses `scroll.scroll_step` when omitted.            |
| `--selection` | bool | See [Targeting](#targeting).                                               |
| `--bare`      | bool | See [Targeting](#targeting).                                               |

**Platforms:** vertical scrolling works everywhere. Horizontal scrolling
(`scroll_left`, `scroll_right`) is not implemented on Windows and has no effect
there.

**Examples**

```bash
neru action scroll_down
neru action scroll_down --steps 200
neru action scroll_left --steps 100
```

---

## neru action page_up, page_down, go_top, go_bottom

Scroll by a page, or to the top or bottom.

```
neru action page_up|page_down|go_top|go_bottom [--selection] [--bare]
```

Page actions use `scroll.scroll_step_half` and `scroll.scroll_step_full`. These
subcommands take no `--steps` flag.

**Examples**

```bash
neru action page_up
neru action page_down
neru action go_top
neru action go_bottom
```

---

## neru action feed

Send keystrokes to the focused application or to Neru's mode system.

```
neru action feed [--mode] <key> [<key>...]
```

Chords use `+`, for example `ctrl+c` or `Cmd+Shift+P`. Use `space` for a
literal space key.

| Flag     | Type | Default | Description                                                              |
| -------- | ---- | ------- | -------------------------------------------------------------------------- |
| `--mode` | bool | `false` | Route the keys through Neru's active mode instead of posting them to the OS. |

**Arguments**

`<key>` — one or more keys or chords. Accepted names:

| Group        | Names                                                             |
| ------------ | ------------------------------------------------------------------- |
| Letters      | `a`–`z`                                                             |
| Numbers      | `0`–`9`                                                             |
| Symbols      | `=`, `-`, `[`, `]`, and similar                                     |
| Named keys   | `space`, `return`, `escape`, `tab`, `delete`                        |
| Navigation   | `left`, `right`, `up`, `down`, `pageup`, `home`, `end`              |
| Function     | `f1`–`f24` (`f21`–`f24` on Linux and Windows only)                  |
| Modifiers    | `cmd`, `shift`, `alt`, `ctrl`, `LeftCmd`, `RightShift`              |

**Examples**

```bash
neru action feed o
neru action feed ctrl+c
neru action feed Cmd+Shift+P
neru action feed h e l l o return
neru action feed --mode o
neru action feed --mode Escape
```

---

## neru action cycle_hint

Move the hint selection without acting on it.

```
neru action cycle_hint [--backward]
```

Valid in hints mode only.

| Flag         | Type | Default | Description                                    |
| ------------ | ---- | ------- | ---------------------------------------------- |
| `--backward` | bool | `false` | Cycle to the previous hint instead of the next. |

---

## neru action wait_for_mode_exit

Block an action chain until the current mode exits.

```
neru action wait_for_mode_exit [--bail]
```

| Flag     | Type | Default | Description                                                            |
| -------- | ---- | ------- | ------------------------------------------------------------------------ |
| `--bail` | bool | `false` | Abort the chain with `ERR_CHAIN_BAIL` if the mode exits with no selection. |

---

## neru action reset, backspace

Mode state control.

```
neru action reset
neru action backspace
```

`reset` clears the current mode's input state. `backspace` applies
mode-specific backspace behaviour: hints and grid input, grid subgrid, or
recursive-grid backtracking.

---

## neru action save_cursor_pos, restore_cursor_pos, hide_cursor, show_cursor

Cursor position and visibility.

```
neru action save_cursor_pos
neru action restore_cursor_pos
neru action hide_cursor
neru action show_cursor
```

**Platforms:** `save_cursor_pos` and `restore_cursor_pos` work everywhere.
`hide_cursor` and `show_cursor` are macOS only and are no-ops on Linux and
Windows.

`save_cursor_pos` records the cursor position; `restore_cursor_pos` returns it
there. `hide_cursor` and `show_cursor` control the visibility of the system
cursor.

---

## action sleep (hotkey bindings only)

Pause between steps of a hotkey action array.

```
"action sleep <duration>"
```

There is no `neru action sleep` subcommand. Running it from a shell fails with
`ERR_INVALID_INPUT`. The name exists only inside a hotkey binding, where the
daemon executes it directly.

**Arguments**

`<duration>` — plain numbers are seconds (`0.2`, `1`). Explicit units are `ms`
and `s`.

`sleep` cannot appear in a comma-separated chain; `action left_click,sleep` is
rejected at config validation. It must be its own entry in an action array.

**Examples**

```toml
[hotkeys]
"Return" = ["action left_click", "action sleep 0.5", "hints"]
"F1" = ["action left_click", "action sleep 500ms", "action right_click"]
```

To pause inside a shell script, use the shell's own `sleep`.

---

# Sequences

## neru run

Run several actions in order, in a single call.

```
neru run <step> [step...]
```

Each argument is one step, written exactly as it would be written in a hotkey
binding: an action (`action left_click`), a mode (`hints --action left_click`),
or a shell command (`exec open -a Safari`). The daemon executes the steps in
order.

This is the same executor that runs a multi-action hotkey binding, so a
sequence behaves identically whether it is written in `[hotkeys]`, passed to
`--on-exit`, or run here. Reach for it from an external driver (skhd,
Hammerspoon, a shell script) that would otherwise spawn one `neru` process per
step and lose the sequencing rules between them.

**Sequencing rules**

- Steps run in order; blank steps are rejected.
- A step that asks the sequence to stop ends it. Today that is
  `action wait_for_mode_exit --bail` after a mode was cancelled — the command
  then exits with `ERR_CHAIN_BAIL`.
- Any other failing step is reported, and the remaining steps still run. The
  command exits with `ERR_ACTION_FAILED` naming the first failure.
- A sequence may start another sequence, up to five levels deep. Deeper
  nesting is refused rather than recursing.

**Timeouts**

The sequence runs while the caller waits, so a sequence containing sleeps or
`wait_for_mode_exit` can outlast the default 10-second IPC timeout. Raise it
with the global `--timeout` flag; the daemon holds the reply until the sequence
finishes, however long that takes.

A sequence that is not worth waiting on at all — one that ends with an
interactive mode, say — is better bound to a hotkey, which is dispatched in the
background with no caller attached.

**Examples**

```bash
# Save the cursor, pick a target, click it, then put the cursor back
neru run "action save_cursor_pos" "hints --action left_click" \
         "action wait_for_mode_exit" "action restore_cursor_pos"

# Click, wait for the app to settle, then re-scan for hints
neru --timeout 30 run "action left_click" "action sleep 0.8" hints

# Stop early when the user escapes out of hints instead of selecting
neru run "hints --action left_click" "action wait_for_mode_exit --bail" \
         "exec notify-send clicked"
```

---

# Configuration commands

Full option reference: [CONFIGURATION.md](CONFIGURATION.md).

## neru config init

Create a default configuration file.

```
neru config init [-f] [-c <path>]
```

Does not require a running daemon.

| Flag      | Shorthand | Type   | Default | Description                       |
| --------- | --------- | ------ | ------- | --------------------------------- |
| `--force` | `-f`      | bool   | `false` | Overwrite an existing file.        |
| `--config`| `-c`      | string | `""`    | Write to a specific path.          |

**Examples**

```bash
neru config init
neru config init --force
neru config init -c /path/to/config.toml
```

---

## neru config validate

Check a config file for syntax errors and invalid values.

```
neru config validate [-c <path>]
```

Does not require a running daemon. Exits successfully when no config file is
found, because Neru runs on built-in defaults in that case.

---

## neru config set

Change a configuration value on the running daemon.

```
neru config set [--no-reload] <key> <value>
```

Requires a running daemon. Changes take effect immediately and are written to
an override file so they survive restarts.

`<key>` is a dotted TOML path matching the config file, for example
`hints.hint_characters` or `general.passthrough_unbounded_keys`. Run
`neru config dump` to list every key and its current value.

| Flag          | Type | Default | Description                                                                                                                            |
| ------------- | ---- | ------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| `--no-reload` | bool | `false` | Skip hotkey re-registration and mode exit. Use when setting interdependent fields in sequence, then run `neru config reload` once at the end. |

**Value types**

| Type    | Example                                             |
| ------- | ----------------------------------------------------- |
| string  | `"asdfghjkl"`                                        |
| integer | `14`                                                  |
| boolean | `true`                                                |
| float   | `0.5`                                                 |
| color   | `"#FF0000AA"` or `{"light":"#000","dark":"#FFF"}`     |
| array   | `"button,link"` or `'["button","link"]'`              |

**Override file**

The override file name is derived from the config file name: `config.toml`
becomes `config.override.toml`, `my-neru.toml` becomes
`my-neru.override.toml`.

**Examples**

```bash
neru config set hints.hint_characters "asdfghjkl"
neru config set hints.ui.font_size 14
neru config set general.passthrough_unbounded_keys true
neru config set hints.clickable_roles "button,link"
neru config set scroll.scroll_step 50

# Interdependent fields, applied together
neru config set --no-reload recursive_grid.grid_cols 3
neru config set --no-reload recursive_grid.keys "abcdefghijkl"
neru config reload
```

---

## neru config reset

Remove a field from the override file.

```
neru config reset [--no-reload] <key>
```

Requires a running daemon. The field reverts to the value in the base config
file, or the built-in default, on the next reload.

| Flag          | Type | Default | Description                                                          |
| ------------- | ---- | ------- | ---------------------------------------------------------------------- |
| `--no-reload` | bool | `false` | Defer reloading. Run `neru config reload` after the last reset.        |

**Examples**

```bash
neru config reset recursive_grid.grid_cols

neru config reset --no-reload recursive_grid.grid_rows
neru config reset --no-reload recursive_grid.keys
neru config reload

# Remove every override at once
rm ~/.config/neru/config.override.toml
neru config reload
```

---

## neru config dump

Print the active configuration as JSON.

```
neru config dump
```

Requires a running daemon. Reflects the merged result of the base config,
overrides, and defaults.

**Examples**

```bash
neru config dump | jq
neru config dump | jq '.hints'
```

---

## neru config reload

Reload the configuration from disk.

```
neru config reload
```

Requires a running daemon. Some settings, such as `systray.enabled`, take
effect only after a full daemon restart.

---

# Runtime toggles

Each toggle changes daemon state for the current session only; the configured
value is restored on restart.

## neru toggle-scroll-invert

Invert the scroll direction.

```
neru toggle-scroll-invert
```

Requires a running daemon. Overrides `scroll.invert_scroll` until restart. Also
available from the systray menu.

---

## neru toggle-cursor-follow-selection

Toggle whether the real cursor follows the selection.

```
neru toggle-cursor-follow-selection
```

Requires a running daemon. Applies to the active hints, grid, or
recursive_grid session.

---

## neru toggle-screen-share

Hide or show overlays on shared screens.

```
neru toggle-screen-share
```

**Platforms:** macOS only.

Requires a running daemon. Hidden overlays remain visible locally.

Implemented with the deprecated `NSWindow.sharingType` API, so effectiveness
depends on the macOS version and the screen-sharing application:

| macOS version | Behaviour                             |
| ------------- | ------------------------------------- |
| 14 and older  | Reliable                              |
| 15.0 – 15.3   | Partially effective                   |
| 15.4 and newer| Limited to ScreenCaptureKit-based apps |

---

# Utilities

## neru roles

List the accessibility role vocabulary.

```
neru roles [--explain] [-c <path>]
```

Does not require a running daemon. Lists the semantic roles accepted by
`hints.clickable_roles` and `neru hints --role`, and shows how each resolves on
the current platform.

A role is written either as a semantic name, such as `button` or `text_field`,
which resolves to the native roles of the current platform, or as a native role
carrying a vocabulary prefix:

| Prefix   | Platform              | Example                    |
| -------- | --------------------- | -------------------------- |
| `ax:`    | macOS Accessibility   | `ax:AXDisclosureTriangle`  |
| `atspi:` | Linux AT-SPI          | `atspi:page tab list`      |
| `uia:`   | Windows UI Automation | `uia:Custom`               |

Prefixed entries belonging to other platforms are ignored rather than rejected,
so one config file can serve several machines.

| Flag        | Type | Default | Description                                                                                       |
| ----------- | ---- | ------- | --------------------------------------------------------------------------------------------------- |
| `--explain` | bool | `false` | Resolve the loaded config entry by entry, showing which native roles each contributes and which entries do not apply here. |

**Examples**

```bash
neru roles
neru roles --explain
```

---

## neru services

Manage Neru as a system service that starts on login.

```
neru services install|uninstall|start|stop|restart|status
```

**Platforms:** macOS only, using launchd. Other platforms return
`ERR_NOT_SUPPORTED`. When
Neru was installed through Nix, Homebrew, or another package manager, use that
tool's service manager instead.

| Subcommand  | Description                                |
| ----------- | ------------------------------------------ |
| `install`   | Install and load the launchd service        |
| `uninstall` | Unload and remove the service               |
| `start`     | Start the service                           |
| `stop`      | Stop the service                            |
| `restart`   | Restart the service                         |
| `status`    | Report whether the service is loaded and running |

---

## neru docs

Open documentation in a browser.

```
neru docs config|cli
```

**Platforms:** macOS only; other platforms return `ERR_NOT_SUPPORTED`.

URLs point at the Git tag matching the installed version. Development builds
fall back to `main`.

| Subcommand | Opens                       |
| ---------- | --------------------------- |
| `config`   | The configuration reference |
| `cli`      | This CLI reference          |

---

# Scripting

Neru commands are ordinary processes with conventional exit statuses, so they
compose with shell scripts and external hotkey daemons.

**Toggle the daemon**

```bash
STATUS=$(neru status | grep "Status:" | awk '{print $2}')
if [ "$STATUS" = "running" ]; then
    neru stop
else
    neru start
fi
```

**Check whether the daemon is reachable**

```bash
neru status &>/dev/null && echo "Running" || echo "Not running"
```

**Drive Neru from an external hotkey manager**

```
# ~/.config/skhd/skhdrc
ctrl - f : neru hints
ctrl - g : neru grid
ctrl - r : neru hints --action right_click
ctrl - t : neru hints --action left_click --repeat
```

**Run several steps as one unit**

Chaining `neru` invocations with `&&` spawns a process and opens a connection
per step. [`neru run`](#neru-run) sends the whole sequence once and the daemon
executes it in order, under the same rules a hotkey binding gets:

```bash
neru run "action save_cursor_pos" "hints --action left_click" \
         "action wait_for_mode_exit --bail" "action restore_cursor_pos"
```

---

# IPC protocol

The CLI and the daemon exchange JSON over a Unix domain socket, or a named pipe
on Windows. The daemon queues incoming commands, so concurrent calls from
scripts are safe.

**Request**

```json
{ "action": "hints", "params": {}, "args": [] }
```

**Response**

```json
{ "success": true, "message": "OK", "code": "OK" }
```

**Response codes**

| Code                    | Meaning                                                  |
| ----------------------- | -------------------------------------------------------- |
| `OK`                    | Command succeeded                                        |
| `ERR_UNKNOWN_COMMAND`   | No such command                                          |
| `ERR_INVALID_INPUT`     | Malformed arguments or flag values                       |
| `ERR_NOT_RUNNING`       | Neru is paused via `neru stop`                            |
| `ERR_ALREADY_RUNNING`   | Target is already in the requested state                 |
| `ERR_MODE_DISABLED`     | The requested mode is disabled in the configuration      |
| `ERR_ACTION_FAILED`     | The action was dispatched but did not complete           |
| `ERR_CHAIN_BAIL`        | An action chain aborted, for example `--bail`             |
| `ERR_NOT_SUPPORTED`     | Not implemented on this platform                         |
| `ERR_VERSION_MISMATCH`  | Client and daemon builds differ; restart the daemon       |

A connection error rather than a response code means no daemon is running.

**Log file locations** are listed in
[TROUBLESHOOTING.md](TROUBLESHOOTING.md#log-file-locations).
