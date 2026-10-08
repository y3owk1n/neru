# CLI reference

Every `neru` command and flag. The same content ships as man pages
(`man neru`). To drive Neru from scripts, see [Scripting](../guide/scripting.md).

`neru launch` starts the daemon. Every other command sends one request to it
and prints the reply, except `doctor`, `roles`, `services`, `docs`,
`config init` and `config validate`, which run without a daemon.

Synopses use `<value>` for a required value, `[--flag]` for an optional flag,
`a|b` for a choice, and `[<key>...]` for a repeatable argument. Every command
also takes `-h`/`--help`. A command that does nothing on your platform says so
and exits non-zero, and
[Platform support per word](platform-support.md#platform-support-per-word)
lists the flags and actions this applies to.

## Global flags

| Flag        | Shorthand | Type   | Default | Description                                                                        |
| ----------- | --------- | ------ | ------- | ---------------------------------------------------------------------------------- |
| `--config`  | `-c`      | string | `""`    | Config file path. Overrides the default search paths. See [Config file location](../guide/configuring.md#where-neru-looks-for-the-file). |
| `--timeout` |           | int    | `10`    | IPC timeout in seconds.                                                             |

## Daemon lifecycle

### neru launch

```
neru launch [-c <path>] [--timeout <seconds>]
```

Start the daemon that owns the event tap, overlays and IPC server. Takes only
the [global flags](#global-flags).

### neru start

```
neru start
```

Resume mode switching and overlay rendering after `neru stop`.

### neru stop

```
neru stop
```

Disable mode switching and overlay rendering. The daemon keeps running and
keeps its socket open.

### neru idle

```
neru idle
```

Exit the active mode. Does nothing when no mode is active. Takes no flags or
arguments, and refuses anything written after it.

### neru status

```
neru status [--json]
```

Print the daemon state and the current mode.

| Flag     | Type | Default | Description                          |
| -------- | ---- | ------- | ------------------------------------ |
| `--json` | bool | `false` | Print the state as one JSON object.  |

The human form prints `Status` (`running` or `disabled`) and `Mode` (`idle`,
`hints`, `grid`, `recursive_grid`, `bisect`, `scroll`, `monitor_select`, or
the name of the open [declared mode](configuration.md#modes)). With `--json`,
the object goes to stdout alone, and errors go to stderr with a non-zero exit
status.

| Key                                                     | Type   | Description                                              |
| ------------------------------------------------------- | ------ | -------------------------------------------------------- |
| `enabled`                                               | bool   | `false` after `neru stop`, `true` after `neru start`.     |
| `mode`                                                  | string | The active mode, same values as `Mode`.                   |
| `config`                                                | string | Path of the config file in use.                           |
| `hints_enabled`, `grid_enabled`, `recursive_grid_enabled`, `bisect_enabled` | bool | Whether each mode is enabled in the config. |
| `scroll_inverted`                                       | bool   | Set by [`toggle-scroll-invert`](#neru-toggle-scroll-invert). |
| `hidden_for_screen_share`                               | bool   | Set by [`toggle-screen-share`](#neru-toggle-screen-share). `true` means hidden. |
| `cursor_follow_selection`                               | bool or null | Set by [`toggle-cursor-follow-selection`](#neru-toggle-cursor-follow-selection). `null` when no mode is running, so test for `null` before reading it as a boolean. |
| `saved_cursor_slots`                                    | object | The occupied [cursor slots](#cursor-slots), each `{"x": …, "y": …}`. Empty when none are saved. |
| `capabilities`                                          | object | Per-subsystem support on this platform, as `neru doctor` reports it. |
| `profile`                                               | object | Which adapter serves each subsystem, and whether it needs CGO. |

Every key except `capabilities` and `profile` is stable. Those two gain
entries as subsystems are added, so read only the keys you need.

### neru doctor

```
neru doctor
```

Report config validity, socket health, platform capabilities from the
[capability matrix](platform-support.md#capability-matrix), and component
state.

- With the daemon running, the `Overlay backend` line names the renderer in
  use. On Windows it tells DirectComposition + Direct2D from the GDI fallback
  and says why the fallback was taken.
- The `platform_support` row lists the options, actions and mode flags your
  config writes that do nothing on this platform, with the reason for each.
  These never fail the check. The full set is in
  [Platform support per word](platform-support.md#platform-support-per-word).

## Navigation modes

### Mode flag reference

Flags work the same typed after `neru`, in a
[hotkey binding](configuration.md#hotkeys), or over the
[IPC socket](ipc.md). A mode refuses a flag not listed for it. A repeated flag
replaces its earlier value unless its entry says it can be repeated.
`--action` may be positional: `hints left_click`.

- `--action` takes the mouse-button [action names](#action-names) or
  `move_mouse`, and `--on-exit` steps form one [action sequence](#neru-run).
  `hints --action move_mouse --on-exit scroll` opens scroll mode on the hint
  you pick.
- `--role` takes the [`neru roles`](#neru-roles) vocabulary. `--label-direction`
  is explained in [Choosing a label direction](configuration.md#choosing-a-label-direction).
- A flag left out takes its config value: `--strategy` from
  [`hints.strategy`](configuration.md#hints), `--label-direction` from
  `hints.label_direction`. `--cursor-selection-mode` defaults to `follow`.

**Cycling lists.** In a `--strategy` or `--capture-scope` list, the value in
use may come from the config. The list wraps, and falls back to its first
entry if it does not name the value in use. Each value may appear once, and one
flag per command may take a list. `--toggle` refuses a list, and
`--split-word` refuses a `--strategy` list.

<!-- BEGIN GENERATED MODE FLAGS: edit internal/domain/modecmd, then run `just genflagref` -->

#### `--action`

Shorthand `-a`. Takes a value. Modes: `hints` · `grid` · `recursive_grid`.

Action to run on the selection: left_click, right_click, middle_click, left_mouse_down, left_mouse_up, right_mouse_down, right_mouse_up, middle_mouse_down, middle_mouse_up, left_mouse_toggle, right_mouse_toggle, middle_mouse_toggle, move_mouse. move_mouse only leaves the cursor there. Chain with commas, as in left_click,left_click.

#### `--modifier`

Takes a value. Modes: `hints` · `grid` · `recursive_grid`.

Modifiers to hold during the action, comma-separated: cmd, super, meta, shift, alt, option, ctrl. Needs --action.

#### `--on-exit`

Takes a value, and can be repeated. Modes: `hints` · `grid` · `recursive_grid`.

Step to run after the action, written as in a hotkey binding. Repeat for more steps. Needs --action, and does not run if the mode is canceled.

#### `--repeat`

Shorthand `-r`. Takes no value. Modes: `hints` · `grid` · `recursive_grid`.

Reopen the mode after the action runs. Needs --action.

#### `--toggle`

Shorthand `-t`. Takes no value. Modes: `hints` · `grid` · `recursive_grid` · `bisect` · `scroll` · `monitor_select` · `mode`.

Open the mode, or leave it if it is already open.

#### `--search`

Shorthand `-s`. Takes no value. Modes: `hints`.

Open with the hint search field showing.

#### `--hide-on-empty-search`

Takes no value. Modes: `hints`.

Hide every hint until the search has text. Needs --search.

#### `--exit-on-unmatched`

Takes no value. Modes: `hints`.

Exit when a key matches no hint.

#### `--role`

Takes a value, and can be repeated. Modes: `hints`.

Only hint these roles, comma-separated, such as button,link (see 'neru roles'). Repeat to add more.

#### `--text`

Takes a value, and can be repeated. Modes: `hints`.

Only hint elements whose text contains one of these, comma-separated and case-insensitive. Repeat to add more.

#### `--strategy`

Takes a value. Modes: `hints`.

How hints find elements: axtree (accessibility tree), vision (OCR) or contour (shape detection). A comma-separated list cycles, moving to the next value each time the command runs while the mode is open, as in --strategy=axtree,vision.

#### `--capture-scope`

Takes a value. Modes: `hints` · `grid` · `recursive_grid` · `bisect`.

Region to work in: window (the focused window) or screen (the whole screen). A comma-separated list cycles, moving to the next value each time the command runs while the mode is open, as in --capture-scope=window,screen.

#### `--label-direction`

Takes a value. Modes: `hints`.

Label order: normal (shorter labels first) or reverse (spread across the alphabet).

#### `--split-word`

Takes no value. Modes: `hints`.

Hint each word of detected text separately. Needs the vision strategy.

#### `--zoom-to-depth`

Takes a value. Modes: `recursive_grid`.

Open recursive grid already zoomed to this depth at the cursor.

#### `--cursor-selection-mode`

Takes a value. Modes: `hints` · `grid` · `recursive_grid` · `bisect`.

Whether the real cursor follows the selection (follow) or stays put (hold).

<!-- END GENERATED MODE FLAGS -->

### neru hints

```
neru hints [flags]
```

Label the interactive elements of the focused window, and select one by
typing its label. Takes the `hints` flags in the
[mode flag reference](#mode-flag-reference), plus `--debug`.

Element discovery uses the full accessibility tree on macOS, an AT-SPI walk
on Linux whose coverage depends on the application, and a cached UI Automation
walk on Windows. The `vision` strategy is the fallback where the tree is thin.
It works on every platform, is text-only on Linux and Windows, and needs an
OCR language pack on Windows. See
[Accessibility and hints](platform-support.md#notes).

| Flag      | Shorthand | Type | Default | Description                                                                 |
| --------- | --------- | ---- | ------- | --------------------------------------------------------------------------- |
| `--debug` | `-d`      | bool | `false` | Print a count and a sample of the elements that would be hinted, without the overlay. |

`--debug` is not a mode flag, so bindings refuse it. It combines only with
`--role`, `--text`, `--strategy` and `--split-word`.

```bash
neru hints --action left_click --role button --text submit
```

### neru grid

```
neru grid [flags]
```

Overlay a grid of labeled cells over the screen, or the focused window with
`--capture-scope window`. Typing a label opens a 3x3 subgrid in that cell.
[`move_cell`](#neru-action-move_cell) moves the open subgrid to a neighbouring
cell. Takes the `grid` flags in the [mode flag reference](#mode-flag-reference).
Size, labels and appearance are under [`[grid]`](configuration.md#grid).

```bash
neru grid --action left_click --on-exit 'exec notify-send clicked'
```

### neru recursive_grid

```
neru recursive_grid [flags]
```

Subdivide the selected cell on each keypress until you reach the point.
Backspace backtracks one level, and [`move_cell`](#neru-action-move_cell)
moves sideways at the current depth. Takes the `recursive_grid` flags in the
[mode flag reference](#mode-flag-reference). Depth and layout are under
[`[recursive_grid]`](configuration.md#recursive_grid).

`--zoom-to-depth` drills at the cursor as the mode opens, and stops early at
the minimum cell size or maximum depth.

```bash
neru recursive_grid --zoom-to-depth 3 --action left_click
```

### neru bisect

```
neru bisect [flags]
```

Narrow a region by halves or quadrants until the cursor is on the target.
The region starts as the screen, or the focused window with
`--capture-scope window`, and shows the quadrant keys in its four cells. Each
press keeps one half or quadrant and moves the cursor to its center, or a
pointer stand-in with `--cursor-selection-mode hold`. Backspace undoes the
last cut, and Space starts over.

Takes the `bisect` flags in the [mode flag reference](#mode-flag-reference).
Keys and the default scope are under [`[bisect]`](configuration.md#bisect),
appearance under `[bisect.ui]` and `[bisect.animation]`.

### neru scroll

```
neru scroll [flags]
```

Scroll at the cursor with vim-style keys. Takes the `scroll` flags in the
[mode flag reference](#mode-flag-reference). Step sizes are under
[`[scroll]`](configuration.md#scroll).

The keys are the [`[scroll.hotkeys]` defaults](configuration.md#scroll) plus
the [keys every mode shares](configuration.md#per-mode-hotkeys).

### neru monitor_select

```
neru monitor_select [flags]
```

Show a labeled panel on every other display, and move the cursor to the one
whose label you type. Requires more than one display. Labels come from
`monitor_select.characters`, default `123456789`, and `Escape` cancels. Takes
the `monitor_select` flags in the [mode flag reference](#mode-flag-reference).

### neru mode

```
neru mode <name> [flags]
```

Enter a mode declared under [`[modes.<name>]`](configuration.md#modes). It
captures the keyboard, shows its indicator, and answers keys from its own
`[modes.<name>.hotkeys]` table. `Escape` returns to idle unless the table
rebinds it. The only flag is `--toggle`. An undeclared name is refused with
`ERR_INVALID_INPUT`. In a binding the step is `"mode <name>"`.

## Actions

```
neru action <subcommand> [flags]
```

One-shot input that runs without entering a mode. A subcommand refuses any
flag not listed in its own section with `ERR_INVALID_INPUT`, and the message
names the actions that accept it.

### Targeting

Point-targeted actions use the active mode selection when one exists, and the
cursor position otherwise.

| Flag          | Type | Default | Description                                                  |
| ------------- | ---- | ------- | ------------------------------------------------------------ |
| `--selection` | bool | `false` | Target the active mode selection.                            |
| `--bare`      | bool | `false` | Target the cursor position even when a mode selection exists. |

### Action names

These names work as a mode `--action`, in a hotkey binding, or after
`neru action`.

| Category | Names                                                                                             |
| -------- | ------------------------------------------------------------------------------------------------- |
| Click    | `left_click`, `right_click`, `middle_click`                                                        |
| Press    | `left_mouse_down`, `right_mouse_down`, `middle_mouse_down`                                          |
| Release  | `left_mouse_up`, `right_mouse_up`, `middle_mouse_up`                                                |
| Toggle   | `left_mouse_toggle`, `right_mouse_toggle`, `middle_mouse_toggle`                                    |
| Movement | `move_mouse`, `move_mouse_relative`, `move_monitor`                                                 |
| Scroll   | `scroll`, `scroll_up`, `scroll_down`, `scroll_left`, `scroll_right`, `page_up`, `page_down`, `go_top`, `go_bottom` |
| Mode     | `reset`, `backspace`, `move_cell`, `bisect`, `cycle_hint`, `search_hints` ³, `wait_for_mode_exit`   |
| Cursor   | `save_cursor_pos`, `restore_cursor_pos`, `hide_cursor`, `show_cursor`                               |
| Keys     | `feed`                                                                                              |
| Timing   | `sleep`, in [hotkey bindings only](#action-sleep-hotkey-bindings-only)                                |

³ `search_hints` opens the hint search field in hints mode. It has no
`neru action` subcommand. Bind it as `"action search_hints"` in a mode's
`[hotkeys]`, or run it through `neru run` or `neru macro`.

A mode `--action` accepts only the click, press, release and toggle names,
`move_mouse`, and the deprecated `mouse_down` and `mouse_up`. `move_mouse`
leaves the cursor on the selection and presses nothing, so a mode refuses
`--modifier` alongside it. A mode refuses every other name with
`ERR_INVALID_INPUT`.

### neru action left_click, right_click, middle_click

```
neru action left_click|right_click|middle_click
            [--modifier <mods>] [--selection] [--bare] [--state down|up] [--toggle]
```

Press and release a mouse button, or one half of that.

| Flag         | Type   | Default | Description                                                    |
| ------------ | ------ | ------- | -------------------------------------------------------------- |
| `--modifier` | string |         | Comma-separated modifiers held during the click: `cmd`, `shift`, `alt`, `ctrl`. |
| `--state`    | string |         | `down` presses and holds, `up` releases. Without it, a full click. |
| `--toggle`   | bool   | `false` | Release the button if held, press and hold it otherwise.       |
| `--selection`, `--bare` | bool | `false` | See [Targeting](#targeting).                    |

- `--state` and `--toggle` cannot be combined, and only these three
  subcommands accept them.
- Neru releases held buttons when it returns to idle.
- A comma chain or a mode `--action` cannot carry flags, so use the action
  name instead: `<button>_click --state down` is `<button>_mouse_down`,
  `--state up` is `<button>_mouse_up`, and `--toggle` is
  `<button>_mouse_toggle`.

```bash
neru action left_click,left_click   # double-click
```

#### mouse_down, mouse_up (deprecated)

`mouse_down` and `mouse_up` are older spellings of `left_mouse_down` and
`left_mouse_up`. They still work, and the CLI prints a deprecation warning on
stderr naming the replacement.

### neru action move_mouse

```
neru action move_mouse [--x <px>] [--y <px>] [--center] [--window] [--selection] [--bare]
```

Move the cursor to an absolute position.

| Flag          | Type | Default | Description                                                     |
| ------------- | ---- | ------- | --------------------------------------------------------------- |
| `--x`, `--y`  | int  | `0`     | Coordinates in pixels. With `--center` or `--window`, an offset. |
| `--center`    | bool | `false` | Target the center of the active screen.                         |
| `--window`    | bool | `false` | Target the center of the focused window.                        |
| `--selection`, `--bare` | bool | `false` | See [Targeting](#targeting).                       |

```bash
neru action move_mouse --center --x 50 --y -30
```

### neru action move_mouse_relative

```
neru action move_mouse_relative --dx <px> --dy <px>
```

Move the cursor by a delta. Both flags are required. Positive `--dx` moves
right and positive `--dy` moves down.

### neru action move_monitor

```
neru action move_monitor [--name <name>] [--previous]
```

Move the cursor to the next display. An active mode overlay follows it. With
one display connected, the action fails with `ERR_INVALID_INPUT`.

| Flag         | Type   | Default | Description                                                            |
| ------------ | ------ | ------- | ---------------------------------------------------------------------- |
| `--name`     | string |         | Target a display by name, case-insensitive, such as `"DP-1"`. An unknown name fails with the list of available names. |
| `--previous` | bool   | `false` | Cycle to the previous display instead.                                 |

On Windows, displays that share a driver name get their device name as a
suffix, such as `Generic PnP Monitor (DISPLAY5)`.

### neru action move_cell

```
neru action move_cell --direction left|right|up|down [--count <n>]
```

Slide the active mode's selection to a neighbouring cell on the same layer.

| Flag          | Type   | Default | Description                                     |
| ------------- | ------ | ------- | ----------------------------------------------- |
| `--direction` | string |         | Required. `left`, `right`, `up` or `down`.      |
| `--count`     | int    | `1`     | Cells to move, at least 1.                      |

- In recursive_grid, the region moves at the current depth and crosses into
  the neighbouring parent at its edge. Once the grid has bottomed out at
  `max_depth` or `min_size_*`, the final cell moves.
- In grid, an open subgrid moves. Before a subgrid opens, nothing happens.
- Movement stops at the screen edge, applying as many steps as fit. Hints,
  scroll and idle ignore the action.
- With [`[held_repeat]`](configuration.md#held_repeat) enabled, a key bound to
  it slides while held. `[held_repeat]` is off by default.

### neru action scroll_up, scroll_down, scroll_left, scroll_right

```
neru action scroll_up|scroll_down|scroll_left|scroll_right
            [--modifier <mods>] [--steps <px>] [--selection] [--bare]
```

Scroll one step in a direction.

| Flag          | Type   | Default              | Description                                    |
| ------------- | ------ | -------------------- | ---------------------------------------------- |
| `--modifier`  | string |                      | Comma-separated modifiers held during the scroll: `cmd`, `shift`, `alt`, `ctrl`. |
| `--steps`     | int    | `scroll.scroll_step` | Scroll amount in pixels.                       |
| `--selection`, `--bare` | bool | `false`    | See [Targeting](#targeting).                   |

Most applications read a modified scroll as zoom. The bound key's modifiers
are not added, so `"Ctrl+K" = "action scroll_up"` scrolls plain. Backend
support for `--modifier` is in the [capability matrix](platform-support.md#capability-matrix).

```bash
neru action scroll_up --modifier ctrl   # zoom in
```

### neru action page_up, page_down, go_top, go_bottom

```
neru action page_up|page_down|go_top|go_bottom [--modifier <mods>] [--selection] [--bare]
```

Scroll by a page, or to the top or bottom. Flags are as for `scroll_up`,
without `--steps`. Page actions use `scroll.scroll_step_half` and
`scroll.scroll_step_full`. `go_top` and `go_bottom` scroll
`scroll.scroll_step_full` pixels, a million by default, so `--modifier ctrl`
on them sends a million pixels of zoom.

### neru action feed

```
neru action feed [--mode] <key> [<key>...]
```

Send keystrokes to the focused application. Chords use `+`, such as
`Cmd+Shift+P`.

| Flag     | Type | Default | Description                                                     |
| -------- | ---- | ------- | --------------------------------------------------------------- |
| `--mode` | bool | `false` | Route the keys through Neru's active mode instead of the OS.    |

Key names: `a` to `z`, `0` to `9`, symbols such as `=`, `-`, `[`, `]`,
`space`, `return`, `escape`, `tab`, `delete`, `left`, `right`, `up`, `down`,
`pageup`, `home`, `end`, `f1` to `f24` (`f21` to `f24` on Linux and Windows
only), and modifiers such as `cmd`, `shift`, `alt`, `ctrl`, `LeftCmd`,
`RightShift`.

### neru action bisect

```
neru action bisect --direction left|right|up|down|up_left|up_right|down_left|down_right [--count <n>]
```

Keep a half or a quadrant of the region in [bisect mode](#neru-bisect), and
move the cursor to its center. Outside bisect mode the action does nothing.

| Flag          | Type   | Default | Description                                     |
| ------------- | ------ | ------- | ----------------------------------------------- |
| `--direction` | string |         | Required. One of the eight cuts. A hyphen may replace the underscore. |
| `--count`     | int    | `1`     | Repeat the cut as one press, at least 1.        |

- A cut on an axis already two pixels wide is refused. A count that would go
  below two pixels applies only the cuts that fit.
- One backspace undoes a whole counted press.
- A repeated cut reaches the outer slices only. `left --count 2` keeps the left
  quarter, and the quarter from 25% to 50% is `left` then `right`.

```bash
neru action bisect --direction up_left --count 2
```

### neru action cycle_hint

```
neru action cycle_hint [--backward]
```

Move the hint selection without acting on it. Hints mode only.
`--backward` (default `false`) cycles to the previous hint.

### neru action wait_for_mode_exit

```
neru action wait_for_mode_exit [--bail]
```

Block an action chain until the current mode exits. `--bail` (default
`false`) aborts the chain with `ERR_CHAIN_BAIL` if the mode exits with no
selection.

### neru action reset, backspace

```
neru action reset
neru action backspace
```

`reset` clears the mode's input. `backspace` deletes hints or grid input,
closes a grid subgrid, or backtracks recursive_grid.

### neru action save_cursor_pos, restore_cursor_pos, hide_cursor, show_cursor

```
neru action save_cursor_pos [--slot <name>]
neru action restore_cursor_pos [--slot <name>]
neru action hide_cursor
neru action show_cursor
```

Save, restore, hide or show the cursor. `hide_cursor` and `show_cursor` are
macOS only, and Linux and Windows refuse them with `ERR_NOT_SUPPORTED`.

| Flag     | Type   | Default   | Description                              |
| -------- | ------ | --------- | ---------------------------------------- |
| `--slot` | string | `default` | Named slot to save into or restore from. |

#### Cursor slots

- A slot name starts with a letter, then letters, digits, underscores and
  dashes.
- Restoring consumes the slot. A second restore succeeds and moves nothing.
- Give a macro its own slot so it does not overwrite the caller's saved
  position.
- `neru status --json` reports occupied slots under `saved_cursor_slots`.

```toml
[macros]
peek = ["action save_cursor_pos --slot peek", "action move_mouse --x 0 --y 0",
        "action left_click", "action restore_cursor_pos --slot peek"]
```

### action sleep (hotkey bindings only)

```
"action sleep <duration>"
```

Pause between steps of a hotkey action array. `<duration>` is seconds as a
plain number (`0.2`), or takes a `ms` or `s` unit. There is no
`neru action sleep`, and the shell refuses it with `ERR_INVALID_INPUT`.
`sleep` must be its own array entry, and config validation refuses it in a
comma chain such as `action left_click,sleep`.

```toml
[hotkeys]
"Return" = ["action left_click", "action sleep 500ms", "hints"]
```

## Sequences

### neru run

```
neru run [--stop-on-error] <step> [step...]
```

Run several steps in order in one call, with the executor that runs hotkey
arrays and `--on-exit`. Each step is written as in a hotkey binding: an
action, a mode, `exec <command>`, or `macro <name> [arg...]` from
[`[macros]`](configuration.md#macros).

| Flag              | Type | Default | Description                                                              |
| ----------------- | ---- | ------- | ------------------------------------------------------------------------ |
| `--stop-on-error` | bool | `false` | End at the first failing step, as if every step carried `--bail-on-error`. |

- Blank steps are refused.
- `action wait_for_mode_exit --bail` after a canceled mode ends the sequence
  with `ERR_CHAIN_BAIL`.
- A failing step is reported and the rest still run. The command exits with
  `ERR_ACTION_FAILED` naming the first failure.
- A sequence may start another sequence, up to five levels deep.
- A sequence with sleeps or `wait_for_mode_exit` can outlast the 10-second
  IPC timeout. Raise it with `--timeout`, or bind the sequence to a hotkey,
  which runs with no caller waiting.

```bash
neru run "action save_cursor_pos" "hints --action left_click" \
         "action wait_for_mode_exit --bail" "action restore_cursor_pos"
```

#### Failure policy

End a step with `--bail-on-error` to stop the sequence if that step fails. It
must be the last word of the step, and the daemon removes it before running
the step. It works in hotkey arrays, `--on-exit` and `neru run`.

```toml
[hints.hotkeys]
"Shift+L" = ["action left_click --bail-on-error", "idle"]
```

- Text that only looks like the directive, as in
  `exec sh -c "echo --bail-on-error"`, passes through to the shell.
- A nested `run` or `macro` keeps its own policy inside, and its failure
  counts as one failed step to the caller.
- The error names the step that stopped the sequence, and says whether later
  steps ran.

### neru macro

```
neru macro <name> [arg...]
```

Run a named sequence from [`[macros]`](configuration.md#macros), as a binding's
`macro <name>` step would. Arguments fill `$1`, `$2` and so on, and their
count must match the highest placeholder. Each shell argument passes through
unchanged, so `"hello there"` needs no extra quoting.

Exit codes match [`neru run`](#neru-run), plus `ERR_INVALID_INPUT` for an
unknown macro or a wrong argument count. Raise `--timeout` for long macros.

```bash
neru macro say_it "hello there"
```

## Configuration commands

Every option is in [Configuration](configuration.md).

### neru config init

```
neru config init [-f] [-c <path>]
```

Write a default config file.

| Flag       | Shorthand | Type   | Default | Description                 |
| ---------- | --------- | ------ | ------- | --------------------------- |
| `--force`  | `-f`      | bool   | `false` | Overwrite an existing file. |
| `--config` | `-c`      | string | `""`    | Write to this path.         |

### neru config validate

```
neru config validate [-c <path>]
```

Check a config file for syntax errors and invalid values, including the mode
flags in your bindings. Exits successfully when no config file exists.

A setting that loads but will not take effect is a warning. So is a
deprecated setting, which works until it is removed. The command still
succeeds. Examples are `grid --search`, a clickable role this platform has no
name for, or `hints.on_mission_control_activated`. [Global hotkeys](configuration.md#global-hotkeys)
lists which mistakes warn and which refuse the file.

```
Configuration is valid, with warnings:

  hotkeys.Primary+Shift+G: grid does not accept --search

These parts of the configuration load. Each one will not take effect or is deprecated, so change it as its line says.
```

### neru config set

```
neru config set [--no-reload] <key> <value>
```

Change a value on the running daemon. The change applies at once and is saved
to the [override file](../guide/configuring.md#how-the-layers-combine).
`<key>` is a dotted TOML path, and `neru config dump` lists them all.

| Flag          | Type | Default | Description                                                         |
| ------------- | ---- | ------- | ------------------------------------------------------------------- |
| `--no-reload` | bool | `false` | Skip hotkey re-registration and mode exit. Run `neru config reload` after the last of several dependent changes. |

Values: a string `"asdfghjkl"`, integer `14`, boolean `true`, float `0.5`,
color `"#FF0000AA"` or `{"light":"#000","dark":"#FFF"}`, or array
`"button,link"` or `'["button","link"]'`.

A list of steps, such as a hook, is never split at commas, because a step can
contain one. Give one step as is, `'exec echo a,b'`, or several as a TOML
array, `'["exec echo a,b", "idle"]'`.

Setting several dependent values with `--no-reload` is shown in
[Configuring Neru](../guide/configuring.md#change-one-value-without-editing-the-file).

### neru config reset

```
neru config reset [--no-reload] <key>
```

Remove a key from the override file. It reverts to the base config or the
built-in default on the next reload. `--no-reload` (default `false`) defers
the reload. To remove every override at once, see
[How the layers combine](../guide/configuring.md#how-the-layers-combine).

### neru config dump

```
neru config dump
```

Print the merged base config, overrides and defaults as JSON.

### neru config reload

```
neru config reload
```

Reload the config from disk. Some settings, such as `systray.enabled`, need a
daemon restart.

## Runtime toggles

Each toggle lasts until the daemon restarts. All three accept
`--state on|off|toggle`. `on` and `off` set the state whatever it was, and
`toggle`, the default, flips it. Scripts should use `on` or `off`, and read
the result from `neru status --json`.

### neru toggle-scroll-invert

```
neru toggle-scroll-invert [--state on|off|toggle]
```

Invert the scroll direction, overriding `scroll.invert_scroll`. Also in the
systray menu. Reported as `scroll_inverted`.

### neru toggle-cursor-follow-selection

```
neru toggle-cursor-follow-selection [--state on|off|toggle]
```

Toggle whether the real cursor follows the selection in the active hints,
grid or recursive_grid session. Fails when no mode is running, even with
`--state`. Reported as `cursor_follow_selection`.

### neru toggle-screen-share

```
neru toggle-screen-share [--state on|off|toggle]
```

Hide overlays from screen sharing while keeping them visible locally. macOS
only. `--state on` hides, matching `hidden_for_screen_share`. Neru uses the
deprecated `NSWindow.sharingType` API:

| macOS version  | Behavior                              |
| -------------- | -------------------------------------- |
| 14 and older   | Reliable                               |
| 15.0 to 15.3   | Partially effective                    |
| 15.4 and newer | Limited to ScreenCaptureKit-based apps |

## Utilities

### neru roles

```
neru roles [--explain] [-c <path>]
```

List the roles accepted by `hints.clickable_roles` and `neru hints --role`,
and how each resolves on this platform. A role is a semantic name such as
`button`, or a native role with a prefix. Entries prefixed for another
platform are ignored, so one config serves several machines.

| Prefix   | Platform              | Example                    |
| -------- | --------------------- | -------------------------- |
| `ax:`    | macOS Accessibility   | `ax:AXDisclosureTriangle`  |
| `atspi:` | Linux AT-SPI          | `atspi:page tab list`      |
| `uia:`   | Windows UI Automation | `uia:Custom`               |

| Flag        | Type | Default | Description                                                                         |
| ----------- | ---- | ------- | ----------------------------------------------------------------------------------- |
| `--explain` | bool | `false` | Resolve the loaded config entry by entry, showing each entry's native roles and which do not apply here. |

### neru services

```
neru services install|uninstall|start|stop|restart|status
```

Manage Neru as a login service. If a package manager such as Nix, Homebrew or
home-manager manages the service, use that tool instead.

`install` writes the service definition and enables it for login, and
`uninstall` disables and removes it. `start`, `stop` and `restart` act on the
service, and `status` reports whether it is installed and running.

| Platform | Definition | Notes |
| -------- | ---------- | ----- |
| macOS    | launchd plist in `~/Library/LaunchAgents` | `install` refuses if a plist exists, so run `uninstall` first. Stderr goes to `~/Library/Logs/neru/daemon.err.log`. |
| Linux    | systemd user unit | See [Linux setup](../guide/linux.md#systemd-user-service). |
| Windows  | Task Scheduler task `\Neru`, logon trigger | Runs `neru launch` as you with an interactive token, restarts on failure, no time limit, no admin rights. `status` reads the task state (running, ready, queued, disabled). `stop` works like `schtasks /End`. |

On Linux and Windows, `install` and `uninstall` refuse to touch a unit or task
Neru did not write. `status` on a machine without the service reports that
rather than failing.

### neru docs

```
neru docs config|cli
```

Open the configuration reference (`config`) or this page (`cli`) with the
desktop's default handler. URLs point at the tag of the installed version, or
`main` for development builds.
