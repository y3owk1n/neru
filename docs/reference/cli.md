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

Every command exits 0 on success and 1 on any failure. When the daemon
refuses a request, the error line on stderr ends with its
[response code](ipc.md#response-codes), as in `(code: ERR_NOT_RUNNING)`.
Scripts read that code, since the exit status is the same for every failure.

## Global flags

| Flag        | Shorthand | Type   | Default | Description                                                                        |
| ----------- | --------- | ------ | ------- | ---------------------------------------------------------------------------------- |
| `--config`  | `-c`      | string | `""`    | Config file path. Overrides the default search paths. See [Config file location](../guide/configuring.md#where-neru-looks-for-the-file). |
| `--timeout` |           | int    | `10`    | IPC timeout in seconds. |

`neru --version`, or `neru -v`, prints the version, git commit and build date.
It works on `neru` itself, not after a subcommand.

## Daemon lifecycle

### neru launch

```
neru launch [-c <path>] [--timeout <seconds>]
```

Start the daemon that owns the event tap, overlays and IPC server. Takes only
the [global flags](#global-flags). With a daemon already running, it prints
`Neru is already running` and exits 0.

### neru start

```
neru start
```

Resume mode switching and overlay rendering after `neru stop`.

Errors: `ERR_ALREADY_RUNNING` when Neru is not stopped.

### neru stop

```
neru stop
```

Disable mode switching and overlay rendering, and exit the active mode. The
daemon keeps running and keeps its socket open. Until `neru start`, the mode
commands, `action`, `run` and `macro` fail with `ERR_NOT_RUNNING`. The
[IPC Commands table](ipc.md#commands) lists what still answers.

Errors: `ERR_NOT_RUNNING` when Neru is already stopped.

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

The human form prints `Status` (`running`, or `stopped` after `neru stop`) and
`Mode` (`idle`, `hints`, `grid`, `recursive_grid`, `bisect`, `scroll`,
`monitor_select`, or the name of the open [declared mode](configuration.md#modes)).
It then prints `Config`, `Platform`, the adapter profile, and the runtime
toggles as `yes` or `no`. `Cursor follows selection` appears only while it has
a value. With `--json`, the object goes to stdout alone, and errors go to
stderr with a non-zero exit status.

| Key                                                     | Type   | Description                                              |
| ------------------------------------------------------- | ------ | -------------------------------------------------------- |
| `enabled`                                               | bool   | `false` after `neru stop`, `true` after `neru start`.     |
| `mode`                                                  | string | The active mode, same values as `Mode`.                   |
| `config`                                                | string | Path of the config file in use.                           |
| `hints_enabled`, `grid_enabled`, `recursive_grid_enabled`, `bisect_enabled` | bool | Whether each mode is enabled in the config. |
| `scroll_inverted`                                       | bool   | Set by [`toggle-scroll-invert`](#neru-toggle-scroll-invert). |
| `hidden_for_screen_share`                               | bool   | Set by [`toggle-screen-share`](#neru-toggle-screen-share). `true` means hidden. Always `false` on Linux and Windows, where nothing hides the overlay. |
| `cursor_follow_selection`                               | bool or null | Set by [`toggle-cursor-follow-selection`](#neru-toggle-cursor-follow-selection). `null` unless hints, grid, recursive_grid or bisect is active, so test for `null` before reading it as a boolean. |
| `saved_cursor_slots`                                    | object | The occupied [cursor slots](#cursor-slots), each `{"x": …, "y": …}`. Empty when none are saved. |
| `sticky_modifiers`                                      | string | The sticky modifiers held, such as `cmd,shift`. Empty when none is. |
| `capabilities`                                          | object | Per-subsystem support on this platform, as `neru doctor` reports it. |
| `profile`                                               | object | Which adapter serves each subsystem, and whether it needs CGO. |

Every key except `capabilities` and `profile` is stable. Those two gain
entries as subsystems are added, so read only the keys you need.

### neru watch

```
neru watch [--event <name>[,<name>...]]
```

Print the status, then one JSON object per line for each event the daemon
publishes, as it happens. The events are the ones [`[hooks]`](configuration.md#hooks)
runs on, named without the `on_` prefix. The hook `on_mode_enter` runs on the
event `mode_enter`.

| Flag      | Type   | Default | Description |
| --------- | ------ | ------- | ----------- |
| `--event` | string | none    | Print only these events, comma-separated, such as `mode_enter,mode_exit`. Repeat to add more. Snapshots always print. An unknown name fails with `ERR_INVALID_INPUT`. |

```json
{"seq":0,"event":"snapshot","status":{"enabled":true,"mode":"idle"}}
{"seq":1,"event":"mode_enter","mode":"hints"}
{"seq":2,"event":"mode_exit","mode":"hints","reason":"completed"}
{"seq":3,"event":"config_reload","ok":false}
```

The `snapshot` line comes first, with `status` holding the object
[`neru status --json`](#neru-status) prints (shortened above). Every line after
it is an event, except when the reader falls behind. Then the events it missed
are replaced by a fresh `snapshot` with a `dropped` count, so take each
snapshot as the whole state and each event as a change to it.

| Key         | Present on                     | Value                                                                 |
| ----------- | ------------------------------ | --------------------------------------------------------------------- |
| `seq`       | every line                     | Rises by one with each event. On a snapshot, the `seq` of the last event it includes. With `--event`, it skips the events left out. |
| `event`     | every line                     | `snapshot`, or the event, such as `mode_enter`                        |
| `status`    | `snapshot`                     | The `neru status --json` object                                       |
| `mode`      | `mode_enter`, `mode_exit`, `select` | The mode, named as `neru status` names it                        |
| `reason`    | `mode_exit`                    | `completed`, `switched` or `canceled`, as `NERU_REASON` has them. A `mode_exit` with `switched` is always followed by the next mode's `mode_enter`. A switch whose next mode fails to open, such as hints finding nothing to label, reports `canceled` |
| `action`    | `select`, and `mode_exit` when `completed` | The action the selection ran, such as `left_click`. Absent when it ran none, such as for a binding with no `--action` |
| `bundle_id` | `app_focus`                    | The application, as [`bundle_id`](configuration.md#app-identity-across-platforms-bundle_id) names it |
| `ok`        | `config_reload`                | `true` or `false`                                                     |
| `on`        | `scroll_invert`, `screen_share_hide` | The state switched to, as `scroll_inverted` and `hidden_for_screen_share` report it |
| `slot`      | `cursor_save`, `cursor_restore` | The [cursor slot](#cursor-slots)                                     |
| `x`, `y`    | `cursor_save`, `select`        | The position saved, or the point selected                             |
| `modifiers` | `sticky_modifiers`             | The set held now, as `sticky_modifiers` reports it                    |
| `monitor`   | `monitor_move`                 | The display moved to, as `move_monitor --name` takes it               |
| `dropped`   | a snapshot after missed events | How many events this snapshot replaces. With `--event`, only the events named count, and the rest can never fill the reader's buffer. |

The daemon disconnects a reader once a line has
waited 5 seconds to send. At most 16 watches can be open at once, and the next
one fails with `ERR_BUSY`.

The command runs while Neru is stopped too, and exits 0 when the daemon exits.
`--timeout` bounds connecting only, never the stream.

Errors: `ERR_INVALID_INPUT` for an unknown event name, `ERR_BUSY` when 16
watches are open. Every key is stable.

### neru query

```
neru query displays [--json]
neru query cursor [--json]
neru query window [--json]
neru query app [--json]
neru query hints [--role ...] [--text ...] [--strategy ...] [--capture-scope ...] [--label-direction ...] [--split-word] [--json]
```

Print what the daemon sees right now. Positions and sizes use the coordinates
[`move_mouse`](#neru-action-move_mouse) takes, with a global top-left origin
and Y growing down. Queries answer while Neru is stopped, as `neru status`
does, except `query hints`.

| Flag     | Type | Default | Description |
| -------- | ---- | ------- | ----------- |
| `--json` | bool | `false` | Print the answer as one JSON object on stdout. Errors still go to stderr with a non-zero exit status. |

`query hints` also takes six `hints` flags, listed with it below.

Errors: `ERR_NOT_SUPPORTED` when the platform cannot answer the query, and
`ERR_ACTION_FAILED` when it tried and failed. Every key is stable.

`neru query displays` lists every connected display, in the order the
platform enumerates them.

```json
{"displays":[{"name":"Built-in Retina Display","x":0,"y":0,"width":1512,"height":982,"scale":1}]}
```

| Key                         | Type   | Description                                                      |
| --------------------------- | ------ | ---------------------------------------------------------------- |
| `name`                      | string | The name [`move_monitor --name`](#neru-action-move_monitor) takes. Two identical monitors can share one. |
| `x`, `y`, `width`, `height` | int    | The display's bounds.                                            |
| `scale`                     | number | Physical pixels per apparent unit. The display's DPI scale on Windows and X11, and `1` on macOS and Wayland, whose bounds are already logical. |

`neru query cursor` prints the cursor position and the display holding it.
Where Neru caches the position, as on Wayland, it refreshes it first, and
fails with `ERR_ACTION_FAILED` rather than report a position it could not
refresh.

```json
{"x":812,"y":440,"display":"Built-in Retina Display","display_index":0}
```

| Key             | Type        | Description                                                       |
| --------------- | ----------- | ----------------------------------------------------------------- |
| `x`, `y`        | int         | The cursor position.                                              |
| `display`       | string or null | The display's name, as `query displays` lists it. `null` when no display holds the cursor. |
| `display_index` | int or null | The display's position in the `query displays` list, which tells apart two displays that share a name. |

`neru query window` prints the focused window's bounds. When no window has
focus, such as when the desktop does, it prints `No window has focus`, and
with `--json` it prints `null`. A Wayland compositor that exposes no window
geometry fails it with `ERR_NOT_SUPPORTED`. GNOME needs the
[Neru GNOME Shell extension](../guide/linux-desktops.md#gnome-wayland).

```json
{"x":0,"y":25,"width":1440,"height":875}
```

| Key                         | Type | Description             |
| --------------------------- | ---- | ----------------------- |
| `x`, `y`, `width`, `height` | int  | The window's bounds.    |

`neru query app` prints the focused application. Run from a terminal, it
reports the terminal, so give yourself time to switch:

```bash
sleep 3; neru query app
```

```json
{"bundle_id":"com.apple.Safari"}
```

| Key         | Type   | Description |
| ----------- | ------ | ----------- |
| `bundle_id` | string | The application, read from the same source per-app config matches against. It is the exact string to write in [`[[app_configs]]`](configuration.md#app-identity-across-platforms-bundle_id). |

GNOME needs the Neru GNOME Shell extension here too.

`neru query hints` lists the elements hints mode would label in the focused
window, without drawing the overlay or entering the mode. Like `query app`, it
reports the terminal when run from one.

It takes the `hints` flags that decide which elements it collects, and
`--label-direction`, which decides their labels. Each works as in the
[mode flag reference](#mode-flag-reference).

| Flag                | Type                | Default                 | Description |
| ------------------- | ------------------- | ----------------------- | ----------- |
| `--role`            | string, repeatable  | none                    | Only these roles. |
| `--text`            | string, repeatable  | none                    | Only elements whose text contains one of these. |
| `--strategy`        | string              | `hints.strategy`        | How to find elements. |
| `--capture-scope`   | string              | `hints.capture_scope`   | Region `vision`, `contour` and `segment` scan. |
| `--split-word`      | bool                | `false`                 | One element per word of detected text. Needs `vision`. |
| `--label-direction` | string              | `hints.label_direction` | Label order. |

Unlike the other queries, it does not answer while Neru is stopped, because
the `vision`, `contour` and `segment` strategies capture the screen. It fails with
`ERR_NOT_RUNNING` then. When one of those scans fails, the query fails with
its error, where hints mode would open on what else it found.

```json
{"hints":[{"label":"A","role":"ax:AXButton","title":"","description":"Back","value":"","source":"axtree","identifier":"back-button","focused":false,"x":100,"y":38,"width":44,"height":52}]}
```

| Key                         | Type   | Description                                                     |
| --------------------------- | ------ | --------------------------------------------------------------- |
| `label`                     | string | The label `neru hints` with the same flags would draw on the element now. |
| `role`                      | string | The native role, spelled as `--role` takes it, such as `ax:AXButton`. |
| `title`, `description`, `value` | string | The element's text. `--text` matches any of the three. Empty when the element has none. |
| `source`                    | string | What found the element: `axtree`, `vision`, `contour` or `segment`, as `--strategy` names them. Actions on an `axtree` element can use its accessibility handle. On the others they act at its coordinates. |
| `identifier`                | string | The ID the app's developer gave the element, such as an `AXIdentifier`, UI Automation `AutomationId` or AT-SPI `AccessibleId`. It does not change with the element's text or language, so it is the steadiest way to find one element. Empty when the app set none. |
| `focused`                   | bool   | Whether the element has keyboard focus. |
| `x`, `y`, `width`, `height` | int    | The element's bounds.                                           |

`identifier` and `focused` come from the accessibility tree, so they mean
something only when `source` is `axtree`. An element found any other way
reports `""` and `false`.

Neru assigns labels afresh on every scan, after the flags filter the
elements. A label stays valid only while the screen does not change, and only
for `neru hints` given the same `--role`, `--text`, `--strategy`,
`--capture-scope`, `--split-word` and `--label-direction`. To select an
element by its label:

1. Run `neru query hints` with the flags you need, and read the label.
2. Start hints mode with exactly the same flags, plus `--action`.
3. Type the label into the mode with `neru action feed --mode`, one key per
   character, before the screen changes.

```sh
label=$(neru query hints --text Save --json | jq -r '.hints[0].label')
neru hints --text Save --action left_click
neru action feed --mode $(echo "$label" | fold -w1)
```

### neru doctor

```
neru doctor [--json]
```

Report config validity, socket health, platform capabilities from the
[capability matrix](platform-support.md#capability-matrix), and component
state.

| Flag     | Type | Default | Description                         |
| -------- | ---- | ------- | ----------------------------------- |
| `--json` | bool | `false` | Print every check as one JSON object. |

- With the daemon running, the `Overlay backend` line names the renderer in
  use. On Windows it tells DirectComposition + Direct2D from the GDI fallback
  and says why the fallback was taken.
- The `platform_support` row lists the options, actions and mode flags your
  config writes that do nothing on this platform, with the reason for each.
  These never fail the check. The full set is in
  [Platform support per word](platform-support.md#platform-support-per-word).
- The check fails when the config fails to load or selects no role that
  applies here, when the daemon reports an unhealthy component, or when no
  daemon is running. A daemon paused by `neru stop` still answers and does not
  fail it. On Windows a daemon that is not running does not fail it, and the
  command prints its client-side checks instead.

With `--json`, the object goes to stdout alone, and the exit status is the same
as without it. The object leaves out the checks Windows prints while no daemon
is running.

| Key                | Type           | Description |
| ------------------ | -------------- | ----------- |
| `healthy`          | bool           | `false` exactly when the command exits non-zero. |
| `config`           | object         | `path` is the config file checked, empty for the built-in defaults. `error` says why it failed to load, and is empty when it loaded. |
| `clickable_roles`  | object         | How `hints.clickable_roles` resolves here. `entries` counts the entries written, `native` the native roles they select, and `ignored` the entries for other platforms. `usable` is `false` when no role applies. |
| `platform_support` | object or null | `inert` lists `{"written": …, "note": …}` for each setting that does nothing here. `null` when the config failed to load, or when Neru declares no platform support for this OS. |
| `ipc_endpoint`     | object         | `path` is where the daemon listens, empty when Neru cannot determine it for this user. `reachable` is whether the daemon answers there. |
| `daemon`           | object or null | The [`health`](ipc.md#commands) reply's data. `null` when the daemon does not answer. |

Every key is stable except the contents of `daemon`, which follow the `health`
reply.

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

**Cycling lists.** A `--strategy` or `--capture-scope` list follows these rules:

- The value in use may come from the config.
- The list wraps, and falls back to its first entry if it does not name the
  value in use.
- With `--toggle`, a press while the mode is on the last entry closes it
  instead of wrapping. `hints --strategy axtree,contour --toggle` opens on
  `axtree`, steps to `contour`, then closes.
- Each value may appear once.
- Only one flag per command may take a list.
- `--split-word` refuses a `--strategy` list.

<!-- BEGIN GENERATED MODE FLAGS: edit internal/domain/modecmd, then run `just genflagref` -->

#### `--action`

Shorthand `-a`. Takes a value. Modes: `hints` · `grid` · `recursive_grid`.

Action to run on the selection: left_click, right_click, middle_click, left_mouse_down, left_mouse_up, right_mouse_down, right_mouse_up, middle_mouse_down, middle_mouse_up, left_mouse_toggle, right_mouse_toggle, middle_mouse_toggle, move_mouse. move_mouse only leaves the cursor there. Chain with commas, as in left_click,left_click.

#### `--modifier`

Takes a value. Modes: `hints` · `grid` · `recursive_grid`.

Modifiers to hold during the action, comma-separated: cmd, super, meta, shift, alt, option, ctrl. Needs --action.

#### `--on-exit`

Takes a value, and can be repeated. Modes: `hints` · `grid` · `recursive_grid`.

Step to run after the action, written as in a hotkey binding. Repeat for more steps. Needs --action and refuses --repeat. Does not run if the mode is canceled. Exec steps see the selection as NERU_X, NERU_Y, NERU_MODE and NERU_ACTION.

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

#### `--confirm-on-label`

Takes no value. Modes: `hints`.

Confirm the search as soon as a label typed after the search text leaves one match, without Return.

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

How hints find elements: axtree (accessibility tree), vision (OCR), contour (shape detection) or segment (controls and labels from pixels). A comma-separated list cycles, moving to the next value each time the command runs while the mode is open, as in --strategy=axtree,vision. With --toggle, running the command on the last value leaves the mode instead of wrapping.

#### `--capture-scope`

Takes a value. Modes: `hints` · `grid` · `recursive_grid` · `bisect`.

Region to work in: window (the focused window) or screen (the whole screen). A comma-separated list cycles, moving to the next value each time the command runs while the mode is open, as in --capture-scope=window,screen. With --toggle, running the command on the last value leaves the mode instead of wrapping.

#### `--label-direction`

Takes a value. Modes: `hints`.

Label order: normal (shorter labels first) or reverse (spread across the alphabet).

#### `--targets`

Takes a value. Modes: `hints`.

Label these targets instead of scanning, as JSON in the shape 'neru query hints --json' prints. Refuses the flags that decide what a scan collects, and --repeat.

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

### What a mode command answers

A mode command succeeds once the daemon accepts the activation, which does not
mean the mode opened. `--toggle` on the open mode leaves it, and
`monitor_select` with one display opens nothing, and both still succeed. To
confirm a mode opened, read `mode` from [`neru status --json`](#neru-status),
or wait for the `mode_enter` line of `neru watch --event mode_enter`.

Errors: `ERR_MODE_DISABLED` when the config sets `enabled = false` for the
mode, which hints, grid, recursive_grid, bisect and monitor_select can do.
`ERR_NOT_RUNNING` after `neru stop`. A flag the mode refuses fails too. The CLI
refuses it before sending, and the daemon answers `ERR_INVALID_INPUT`.

### neru hints

```
neru hints [flags]
```

Label the interactive elements of the focused window, and select one by
typing its label. Takes the `hints` flags in the
[mode flag reference](#mode-flag-reference). To list what it would label
without the overlay, use [`neru query hints`](#neru-query).

Element discovery uses the full accessibility tree on macOS, an AT-SPI walk
on Linux whose coverage depends on the application, and a cached UI Automation
walk on Windows. The `vision` strategy is the fallback where the tree is thin.
It works on every platform and needs an OCR language pack on Windows. See
[Accessibility and hints](platform-support.md#notes).

```bash
neru hints --action left_click --role button --text submit
```

#### Supplying your own targets

Where no strategy finds what you need to click, another tool can supply the
targets instead. `--targets` takes JSON in the shape
[`neru query hints --json`](#neru-query) prints, and hints mode labels those
targets in place of a scan. Each target needs `x`, `y`, `width` and `height`
in the coordinates `move_mouse` takes. `role`, `title`, `description` and
`value` are optional, and other keys, such as a query's `label`, are ignored.
From the command line, `--targets -` reads the JSON from stdin.

```bash
neru query hints --json | jq '.hints |= map(select(.width > 20))' \
  | neru hints --targets - --action left_click
```

- Targets whose center is off the active display are dropped, as scanned
  elements are.
- A target has no accessibility handle, so every action works at its
  coordinates, as it does on what the `vision` strategy finds.
- `--targets` refuses `--role`, `--text`, `--strategy`, `--capture-scope`,
  `--split-word` and `--repeat`. Filter the targets before sending them.
- A screen change or a monitor move exits hints mode, since nothing can scan
  the targets again.
- The [request size cap](ipc.md#request) holds several hundred targets.

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

The CLI also accepts `neru recursive-grid`. Over IPC and in bindings the name
is `recursive_grid` only.

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
whose label you type. The mode is off by default, so set
`monitor_select.enabled = true` first, or it fails with `ERR_MODE_DISABLED`.
It needs more than one display. With one, the command succeeds and opens
nothing. Labels come from
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
| Scroll   | `scroll` ³, `scroll_up`, `scroll_down`, `scroll_left`, `scroll_right`, `page_up`, `page_down`, `go_top`, `go_bottom` |
| Mode     | `reset`, `backspace`, `move_cell`, `bisect`, `cycle_hint`, `search_hints` ³, `wait_for_mode_exit`   |
| Cursor   | `save_cursor_pos`, `restore_cursor_pos`, `hide_cursor`, `show_cursor`                               |
| Keys     | `feed`                                                                                              |
| Timing   | `sleep`, in [hotkey bindings only](#action-sleep-hotkey-bindings-only)                                |

³ No `neru action` subcommand. `search_hints` opens the hint search field in
hints mode. Bind it as `"action search_hints"` in a mode's `[hotkeys]`, or run
it through `neru run` or `neru macro`. `search_hints --confirm-on-label` turns
on the hints flag of the same name for that one search. `scroll` takes no
flags. For one-shot scrolling use the directional names below.

A mode `--action` accepts only the click, press, release and toggle names,
`move_mouse`, and the deprecated `mouse_down` and `mouse_up`. `move_mouse`
leaves the cursor on the selection and presses nothing, so a mode refuses
`--modifier` when every action in the chain is `move_mouse`. A chain with a
button keeps it, as in `move_mouse,left_click --modifier cmd`. A mode refuses
every other name with `ERR_INVALID_INPUT`.

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
- A comma chain takes `--modifier`, `--selection` and `--bare` when every
  action in it does. Neither a chain nor a mode `--action` takes `--state` or
  `--toggle`, so use the action name instead. `<button>_click --state down`
  is `<button>_mouse_down`, `--state up` is `<button>_mouse_up`, and
  `--toggle` is `<button>_mouse_toggle`.

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

Move the cursor by a delta.

| Flag   | Type | Default | Description                                  |
| ------ | ---- | ------- | -------------------------------------------- |
| `--dx` | int  |         | Required. Pixels to move. Positive moves right. |
| `--dy` | int  |         | Required. Pixels to move. Positive moves down.  |

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

| Flag         | Type | Default | Description                     |
| ------------ | ---- | ------- | ------------------------------- |
| `--backward` | bool | `false` | Cycle to the previous hint.     |

### neru action wait_for_mode_exit

```
neru action wait_for_mode_exit [--bail]
```

Block an action chain until the current mode exits.

| Flag     | Type | Default | Description |
| -------- | ---- | ------- | ----------- |
| `--bail` | bool | `false` | Abort the chain with `ERR_CHAIN_BAIL` if the mode exits with no selection. |

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

- The CLI refuses a blank step. Over IPC, the daemon drops blank steps, and
  refuses only a request with no steps left, with `ERR_INVALID_INPUT`.
- `action wait_for_mode_exit --bail` after a canceled mode ends the sequence
  with `ERR_CHAIN_BAIL`.
- Neru reports a failing step and still runs the rest. The command fails with
  `ERR_ACTION_FAILED` naming the first failure.
- A sequence may start another sequence, up to five levels deep. Neru refuses
  one nested deeper with `ERR_INVALID_INPUT` before any of its steps run, and
  the sequence that started it counts that as one failed step.
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

Response codes match [`neru run`](#neru-run), plus `ERR_INVALID_INPUT` for an
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
`<key>` is a dotted TOML path, such as `hints.strategy`. `neru config dump`
prints the whole configuration as nested JSON, where each path is a chain of
keys.

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

The CLI checks the key and the value before it sends them, so a bad one fails
with the field's type and no response code.

Errors: `ERR_ACTION_FAILED` when the daemon cannot apply or save the change.

### neru config reset

```
neru config reset [--no-reload] <key>
```

Remove a key from the override file, and reload so it reverts to the base
config or the built-in default. To remove every override at once, see
[How the layers combine](../guide/configuring.md#how-the-layers-combine).

| Flag          | Type | Default | Description |
| ------------- | ---- | ------- | ----------- |
| `--no-reload` | bool | `false` | Skip the reload. Run `neru config reload` after the last of several resets. |

Errors: `ERR_INVALID_INPUT` for an unknown key, `ERR_ACTION_FAILED` when the
override file cannot be changed or the reload fails.

### neru config dump

```
neru config dump
```

Print the configuration the daemon runs on as JSON, under the key names
`config.toml` uses. It is your file, its overrides and the defaults, merged.
The output matches the [config schema](configuration.md#editor-support).

### neru config reload

```
neru config reload
```

Reload the config from disk. Some settings, such as `systray.enabled`, need a
daemon restart.

## Runtime toggles

Each toggle lasts until the daemon restarts, and answers while Neru is
stopped. All three take one flag.

| Flag      | Type   | Default  | Description |
| --------- | ------ | -------- | ----------- |
| `--state` | string | `toggle` | `on` or `off` sets the state whatever it was, and `toggle` flips it. |

Scripts should use `on` or `off`, and read the result from
`neru status --json`. The CLI prints the reply as a line of text. Over IPC the
reply also carries the new state in `data`, whose keys are in the
[IPC Commands table](ipc.md#commands).

Errors: a `--state` value other than these three fails. The CLI refuses it
before sending, and the daemon answers `ERR_INVALID_INPUT`.

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
grid, recursive_grid or bisect session. In any other mode, idle included, it
fails with `ERR_INVALID_INPUT`, even with `--state`. Reported as
`cursor_follow_selection`.

### neru toggle-screen-share

```
neru toggle-screen-share [--state on|off|toggle]
```

Hide overlays from screen sharing while keeping them visible locally.
`--state on` hides, matching `hidden_for_screen_share`. Only macOS can hide the
overlay from a screen share. On Linux and Windows the command fails with
`ERR_NOT_SUPPORTED` and changes nothing. Neru uses the deprecated
`NSWindow.sharingType` API:

| macOS version  | Behavior                              |
| -------------- | -------------------------------------- |
| 14 and older   | Reliable                               |
| 15.0 to 15.3   | Partially effective                    |
| 15.4 and newer | Limited to ScreenCaptureKit-based apps |

## Utilities

### neru roles

```
neru roles [--explain] [--json] [-c <path>]
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
| `--json`    | bool | `false` | Print the answer as one JSON object.                                               |

With `--json`, the object goes to stdout alone, and errors go to stderr with a
non-zero exit status. Every key is stable.

| Key          | Type   | Description |
| ------------ | ------ | ----------- |
| `platform`   | string | The OS the roles resolve on: `darwin`, `linux` or `windows`. |
| `vocabulary` | string | The native vocabulary here: `ax`, `atspi` or `uia`. Empty with no accessibility backend. |
| `roles`      | array  | One object per semantic role. `semantic` is its name, and `native` lists its native roles here. `subrole` is `true` when macOS reports those names as an element's subrole. |

With `--explain --json`, the keys are:

| Key        | Type   | Description |
| ---------- | ------ | ----------- |
| `platform` | string | As above. |
| `config`   | string | Path of the config file resolved, empty for the built-in defaults. |
| `entries`  | array  | One object per `hints.clickable_roles` entry, in order. `entry` is the entry as written, and `vocabulary` its prefix, empty for a semantic name. `native` lists the native roles it adds here. `ignored` says why it adds none, and is empty when it applies. |
| `native`   | array  | Every native role the entries select, once each. |

### neru services

```
neru services install|uninstall|start|stop|restart
neru services status [--json]
```

Manage Neru as a login service. If a package manager such as Nix, Homebrew or
home-manager manages the service, use that tool instead.

`install` writes the service definition and enables it for login, and
`uninstall` disables and removes it. `stop` stops the service and disables it,
so it stays stopped across logins until `start`, which enables it again and
starts it. The definition stays installed. `restart` restarts a running service
and refuses a stopped one, saying to run `start`. `status` reports whether it is
installed and running.

| Platform | Definition | Notes |
| -------- | ---------- | ----- |
| macOS    | launchd plist in `~/Library/LaunchAgents` | `install` refuses if a plist exists, so run `uninstall` first. `stop` runs `launchctl disable` and `bootout`, and `restart` runs `launchctl kickstart -k`. `status` reports a stopped service as stopped. For where its output goes, see [Log file locations](../guide/troubleshooting.md#log-file-locations). |
| Linux    | systemd user unit | See [Linux setup](../guide/linux.md#systemd-user-service). |
| Windows  | Task Scheduler task `\Neru`, logon trigger | Runs `neru launch` as you with an interactive token, restarts on failure, no time limit, no admin rights. `status` reads the task state (running, ready, queued, disabled). `stop` works like `schtasks /Change /DISABLE` followed by `schtasks /End`. |

On Linux and Windows, every subcommand but `status` refuses to touch a unit or
task Neru did not write. `status` on a machine without the service reports that
rather than failing.

| Flag     | Type | Default | Description |
| -------- | ---- | ------- | ----------- |
| `--json` | bool | `false` | `status` only. Print the answer as one JSON object. |

`neru services status --json` exits 0 whatever the service is doing. Every
key is stable.

| Key               | Type   | Description |
| ----------------- | ------ | ----------- |
| `supported`       | bool   | Whether this machine has the service manager Neru drives. `false` on Linux without systemd. |
| `installed`       | bool   | Whether the service is installed. |
| `running`         | bool   | Whether the daemon the service starts is running. |
| `starts_at_login` | bool   | Whether the service starts Neru at login. `neru services stop` turns it off. |
| `message`         | string | The line `neru services status` prints without `--json`. |
| `error`           | string | Why the state could not be read, empty when it was. When set, the booleans say nothing. |

### neru docs

```
neru docs config|cli
```

Open the configuration reference (`config`, or its alias `configuration`) or
this page (`cli`) with the desktop's default handler. URLs point at the tag of the installed version, or
`main` for development builds.
