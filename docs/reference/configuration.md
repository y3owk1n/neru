# Configuration reference

Every option in `config.toml`, with its type and default. Set only what you
want to change, since anything left out keeps its default.

This page lists facts. To create the file and apply changes, see
[Configuring Neru](../guide/configuring.md). For how bindings combine, see
[How bindings work](../concepts/bindings.md). For ready-made configs, see
[Recipes](../guide/recipes.md).

> [!NOTE]
> A few options do nothing on some platforms, see
> [Platform support per word](platform-support.md#platform-support-per-word).

## Hotkeys

The syntax and defaults of every hotkey table. When each table applies, and
which one answers a key, is in [How bindings work](../concepts/bindings.md).

### Global hotkeys

```toml
[hotkeys]
"Primary+Shift+Space" = "hints"
"Primary+Shift+W"     = ["action save_cursor_pos", "hints --action left_click"]
```

The key is a chord, `"Mod1+Mod2+Key"`. The value is one
[step](../concepts/bindings.md#what-a-step-can-be) or an array of steps.

**Defaults.** macOS and Windows bind `Primary+Shift+Space` (hints),
`Primary+Shift+G` (grid), `Primary+Shift+C` (recursive grid),
`Primary+Shift+B` (bisect) and `Primary+Shift+S` (scroll). Linux binds none,
see [Getting started](../guide/getting-started.md#bind-your-first-hotkey).

| Modifier  | Aliases                                 |
| --------- | --------------------------------------- |
| `Cmd`     | `Command`, `Super`, `Meta`              |
| `Ctrl`    | `Control`                               |
| `Alt`     | `Option`                                |
| `Shift`   |                                         |
| `Primary` | `Cmd` on macOS, `Ctrl` on Linux/Windows |

| Category   | Keys                                                                                                  |
| ---------- | ----------------------------------------------------------------------------------------------------- |
| Letters    | `a` to `z`, `A` to `Z`                                                                                |
| Numbers    | `0` to `9`                                                                                            |
| Symbols    | `` ` ``, `-`, `=`, `[`, `]`, `\`, `;`, `'`, `,`, `.`, `/`                                             |
| Named      | `Space`, `Return`, `Enter`, `Escape`, `Tab`, `Delete`, `Backspace`                                    |
| Navigation | `Up`, `Down`, `Left`, `Right`, `Home`, `End`, `PageUp`, `PageDown`, `Insert` (Linux and Windows only) |
| Function   | `F1` to `F24` (`F21` to `F24` on Linux and Windows only)                                              |

- **Shift with a symbol** is written as the character Shift produces. On
  Linux, `Shift` plus the `;` key is `"Shift+:"`, not `"Shift+;"`. Letters are
  unaffected.
- `Delete` and `Backspace` both name the key that erases to the left. The
  forward-delete key has no hotkey name.
- The full key list with platform behavior is under
  [`neru action feed`](cli.md#neru-action-feed).

### Merging behavior

| Config                 | Result                    |
| ---------------------- | ------------------------- |
| Section absent         | All defaults used         |
| Section present, empty | All hotkeys disabled      |
| Section has entries    | Merged on top of defaults |

Use `__disabled__` to remove one default:

```toml
[hotkeys]
"Primary+Shift+S" = "__disabled__"   # removes default scroll binding
"Ctrl+Space"      = "hints"          # adds binding, other defaults unchanged
```

Disabling a mode (`enabled = false`) also removes its default launcher hotkey,
and `neru <mode>` then fails with [`ERR_MODE_DISABLED`](ipc.md#response-codes).

### Per-mode hotkeys

`[<mode>.hotkeys]` follows the same [merging rules](#merging-behavior), and
also accepts multi-key sequences such as `gg`. Every built-in mode except
`monitor_select` ships these, plus the defaults listed in its own section:

```toml
"Escape"    = "idle"
"Shift+L"   = "action left_click"
"Shift+R"   = "action right_click"
"Shift+M"   = "action middle_click"
"Shift+I"   = "action left_click --state down"
"Shift+U"   = "action left_click --state up"
"Up"        = "action move_mouse_relative --dx=0 --dy=-10"
"Down"      = "action move_mouse_relative --dx=0 --dy=10"
"Left"      = "action move_mouse_relative --dx=-10 --dy=0"
"Right"     = "action move_mouse_relative --dx=10 --dy=0"
```

### Per-app hotkey overrides

`[[<mode>.app_configs]]` overrides `[<mode>.hotkeys]` for `hints`, `grid`,
`recursive_grid`, `bisect`, `scroll` and declared modes
(`[[modes.<name>.app_configs]]`), under the same
[merging rules](#merging-behavior).

```toml
[[hints.app_configs]]
bundle_id = "com.brave.Browser"
hotkeys = { "Return" = "action left_click", "Shift+L" = "__disabled__" }
```

#### Per-app global hotkey overrides

A root-level `[[app_configs]]` entry overrides `[hotkeys]` for one app:

```toml
[[app_configs]]
bundle_id = "com.apple.Terminal"
hotkeys = { "Cmd+Space" = "hints", "Cmd+Shift+Space" = "__disabled__" }
```

#### App identity across platforms (`bundle_id`)

`bundle_id` selects the app for `[[app_configs]]`, every
`[[<mode>.app_configs]]` and `excluded_apps`. To find it on any platform, run
`sleep 3; neru query app` and switch to the app before the three seconds are
up. It prints the exact string Neru matches. What that string is depends on
the platform:

| Platform | Identity Neru matches | How to find it |
| --- | --- | --- |
| macOS | Bundle ID, reverse-DNS (e.g. `com.apple.Safari`) | `osascript -e 'id of app "Safari"'` |
| Linux · X11 | Window `WM_CLASS`, the *class* field | `xprop WM_CLASS`, then click the window |
| Linux · Wayland | Toplevel `app_id` | `swaymsg -t get_tree` (Sway), `hyprctl activewindow` (Hyprland), `niri msg windows` (niri), or your compositor's window inspector |
| Windows | Full path of the focused window's executable (e.g. `C:\Program Files\Google\Chrome\Application\chrome.exe`) | Task Manager, Details tab, right-click the process, **Open file location**, or `(Get-Process chrome).Path` in PowerShell |

Matching is case-insensitive and exact, with no globbing or partial matches.

- Linux identity strings vary by toolkit (e.g. `Google-chrome`, `code`,
  `org.kde.konsole`). Confirm with the commands above.
- On Windows, write the path as a TOML literal string (`'C:\...'`) or double
  every backslash. A copy installed elsewhere, such as under `%LOCALAPPDATA%`,
  needs its own entry. Microsoft Store apps all share the identity
  `ApplicationFrameHost.exe` and cannot be told apart.
- On GNOME under Wayland, no entry matches until the Neru GNOME Shell extension
  loads after your first re-login. See
  [GNOME (Wayland)](../guide/linux-desktops.md#gnome-wayland).

## [macros]

Named action sequences, invoked from any binding with `macro <name> [args...]`.
A macro is written, not recorded.

```toml
[macros]
click_and_exit = ["action left_click --bail-on-error", "idle"]
say = ["exec say \"$1\""]

[hints.hotkeys]
"Enter" = "macro click_and_exit"
"Shift+S" = "macro say hello"
```

A worked example with arguments is in
[Recipes](../guide/recipes.md#add-a-shortcut-for-a-button-an-app-lacks).

- **Names** use letters, digits, `_` and `-`, and start with a letter.
- **Arguments** are positional: `$1`, `$2`, and so on, with `$$` for a literal
  dollar sign. Substitution is textual and happens before the step is split, so
  quote a placeholder that may hold spaces, e.g. `exec say "$1"`.
- **Arity is checked at load** wherever an action can be written, including
  [hooks](#hooks) and nested `run` or `--on-exit` steps. An
  unknown name or wrong argument count fails `neru config validate`.
- A placeholder cannot be the command word. `"$1 --action left_click"` is
  rejected at load.
- A macro runs as a nested [sequence](../concepts/bindings.md#sequences), so
  it can call other macros, and its failure counts as one failed step to the
  caller.
- A mode's `--action` does not take a macro. Use `--on-exit`.
- Run a macro from outside with
  [`neru macro <name> [args...]`](cli.md#neru-macro).

## [modes]

Modes you declare yourself. A declared mode is a name, an indicator label and a
hotkey table. While it is open Neru captures the keyboard and answers every key
from that table, so it works as a layer of bare-letter bindings.

```toml
[modes.window]
indicator = "Window"

[modes.window.hotkeys]
"h" = "exec yabai -m window --focus west"
"f" = ["exec yabai -m window --toggle zoom-fullscreen", "idle"]
"s" = "scroll"

[hotkeys]
"Primary+Shift+W" = "mode window"
```

- **Entering.** Use the step `mode <name>` from any binding, macro or `run`, or
  [`neru mode <name>`](cli.md#neru-mode). It accepts only `--toggle`. A step
  naming an undeclared mode is refused at load.
- **Leaving.** `Escape` is bound to `idle` by default (`"Escape" = "__disabled__"`
  removes it). Any binding ending in `idle` or another mode also leaves. Unbound
  Ctrl/Alt/Cmd chords fall back to `[hotkeys]`, see
  [Which binding wins](../concepts/bindings.md#which-binding-wins), and
  unbound bare keys are swallowed.

### Options

| Option      | Type   | Default                 | Description                                                                    |
| ----------- | ------ | ----------------------- | ------------------------------------------------------------------------------ |
| `indicator` | string | `""`                    | [Mode indicator](#mode_indicator) text while the mode is open, empty hides it  |
| `hotkeys`   | map    | `{ "Escape" = "idle" }` | The mode's [hotkeys](#per-mode-hotkeys), merged over the default               |

The name is the table key: letters, digits, `_` and `-`, starting with a
letter. Built-in mode names (`hints`, `grid`, `recursive_grid`, `scroll`,
`monitor_select`, `idle`) and `mode` are refused. The indicator style comes
from [`[mode_indicator.ui]`](#mode_indicator).

### Per-app config

`[[modes.<name>.app_configs]]` takes `bundle_id` and
[`hotkeys`](#per-app-hotkey-overrides).

## [hooks]

Steps Neru runs when something happens, written like any
[binding](../concepts/bindings.md#what-a-step-can-be) and checked at load the
same way.

```toml
[hooks]
on_mode_enter    = "exec sketchybar --trigger neru_mode MODE=\"$NERU_MODE\""
on_mode_exit     = "exec sketchybar --trigger neru_mode MODE=idle"
on_config_reload = "exec [ \"$NERU_OK\" = true ] || say 'Neru config did not load'"
```

| Option                           | Type         | Default | Runs when                                                                 |
| -------------------------------- | ------------ | ------- | ------------------------------------------------------------------------- |
| `on_mode_enter`                  | string/array | none    | A mode opens                                                              |
| `on_mode_exit`                   | string/array | none    | A mode closes, including on the way into another one                      |
| `on_select`                      | string/array | none    | A step you take moves a mode's target, such as a hint label, a grid cell or a recursive grid level |
| `on_app_focus`                   | string/array | none    | Another application comes to the front                                    |
| `on_enable`                      | string/array | none    | Neru resumes, such as on `neru start`                                     |
| `on_disable`                     | string/array | none    | Neru pauses, such as on `neru stop`                                       |
| `on_config_reload`               | string/array | none    | A config reload finishes, whether or not the new file loaded              |
| `on_mission_control_activated`   | string/array | none    | Mission Control opens. macOS only, needs `hints.detect_mission_control`   |
| `on_mission_control_deactivated` | string/array | none    | Mission Control closes. macOS only, needs `hints.detect_mission_control`  |
| `on_scroll_invert`               | string/array | none    | Scroll inversion switches on or off                                       |
| `on_screen_share_hide`           | string/array | none    | `toggle-screen-share` switches on or off. Only macOS hides the overlay    |
| `on_cursor_save`                 | string/array | none    | `save_cursor_pos` saves a position                                        |
| `on_cursor_restore`              | string/array | none    | `restore_cursor_pos` takes a saved position, which empties its slot       |
| `on_sticky_modifiers`            | string/array | none    | A sticky modifier is armed or released. Needs `sticky_modifiers.enabled`  |
| `on_monitor_move`                | string/array | none    | `move_monitor` or `monitor_select` moves the cursor to another display    |
| `on_screen_change`               | string/array | none    | The displays change, such as on a dock, an undock or a wake               |
| `on_ready`                       | string/array | none    | The daemon has started and takes hotkeys                                  |
| `on_quit`                        | string/array | none    | The daemon starts to shut down. Runs `exec` steps only                    |

A reload takes effect from the next event.

`on_select` runs once per step, so zooming a recursive grid three levels runs
it three times, and a `--repeat` mode runs it for each selection. A click
from a mode key such as `Shift+L` is an action, not a step that moves the
target, so it does not run `on_select`.

A hook runs for every binding. To run steps after one binding's selection
only, give that binding [`--on-exit`](cli.md#mode-flag-reference). It runs
once, after the action, and never on a cancel. `on_mode_exit` runs on every
close. `--on-exit` and `on_select` both start after the selection, in no set
order, so do not give both a step that opens a mode.

`hints.on_mission_control_activated` and `hints.on_mission_control_deactivated`
are deprecated and are removed in v2. Move their steps to the two keys here,
which take the same values. Until then the `[hints]` keys still run, alongside
these, and `neru config validate` warns about each one set.

### What a hook's steps see

`exec` steps get the event as environment variables. Each is set only for the
events that carry it.

| Variable         | Set for                         | Value                                                            |
| ---------------- | ------------------------------- | ---------------------------------------------------------------- |
| `NERU_EVENT`     | every hook                      | The event, such as `mode_enter`                                  |
| `NERU_MODE`      | `on_mode_enter`, `on_mode_exit`, `on_select` | The mode, named as `neru status` names it           |
| `NERU_REASON`    | `on_mode_exit`                  | `completed` after a selection, `switched` on the way into another mode, else `canceled` |
| `NERU_ACTION`    | `on_select`, and `on_mode_exit` when `completed` | The action the selection ran, such as `left_click`, or `left_click,left_click` for a chain. Unset when it ran none, such as for a binding with no `--action` |
| `NERU_BUNDLE_ID` | `on_app_focus`                  | The application, as [`bundle_id`](#app-identity-across-platforms-bundle_id) names it |
| `NERU_OK`        | `on_config_reload`              | `true` or `false`                                                |
| `NERU_ON`        | `on_scroll_invert`, `on_screen_share_hide` | `true` when switched on, else `false`                 |
| `NERU_SLOT`      | `on_cursor_save`, `on_cursor_restore` | The cursor slot, `default` when none was named             |
| `NERU_X`, `NERU_Y` | `on_cursor_save`, `on_select` | The position saved, or the point selected                         |
| `NERU_MODIFIERS` | `on_sticky_modifiers`           | The set held now, such as `cmd,shift`, in the spelling `--modifier` takes. Empty when none is |
| `NERU_MONITOR`   | `on_monitor_move`               | The display, as `move_monitor --name` takes it                   |

Use them in the command as shell variables, in double quotes, such as
`"$NERU_MODE"`. Neru passes them to the shell as environment variables and
never writes their values into the command text. That matters for
`NERU_BUNDLE_ID`, which the application chooses for itself. On X11 any program
can set it, and a crafted value such as `x; rm -rf ~` stays plain text instead
of running as a command.

### When a hook runs

- After the change it reports, on a goroutine apart from key handling, so a
  slow hook never delays a mode.
- One hook runs once at a time. Hooks for different events can run together.
- An event raised while its own hook is still running does not start that hook
  again. This rule stops `on_mode_enter = "grid"` from looping, so it enters
  grid once. The same rule skips a second event of the same kind that arrives
  while the hook is busy. Use a hook to act on an event. It cannot keep another
  program in step with every change.
- Two hooks that keep triggering each other, such as
  `on_mode_exit = "hints"` with `on_mode_enter = "idle"`, are not caught and
  loop until the config changes.
- While Neru is stopped, the only hooks that run are `on_enable`, `on_disable`,
  `on_ready`, `on_quit`, and `on_mode_exit` for the mode the pause closed.
  Their `exec` steps run even though Neru is stopped. Neru refuses their other
  steps, such as opening a mode, until it starts.
- `on_quit` runs before anything shuts down. Neru runs only its `exec` steps
  and skips the rest, such as a step that opens a mode. `neru config validate`
  warns about each step it will skip. Shutdown waits at most one second for
  the hook, then continues without it.

## [general]

Behavior not tied to one mode.

```toml
[general]
excluded_apps = ["com.apple.Terminal"]
passthrough_unbounded_keys = true
```

| Option                                 | Type   | Default       | Description                                                                                 |
| -------------------------------------- | ------ | ------------- | ------------------------------------------------------------------------------------------- |
| `excluded_apps`                        | array  | `[]`          | Apps where Neru won't activate, by [app identity](#app-identity-across-platforms-bundle_id) |
| `kb_layout_to_use`                     | string | `""`          | Keyboard layout keys are named in, as the platform names it (auto if empty). See below      |
| `hide_overlay_in_screen_share`         | bool   | `false`       | Hide overlay in screen sharing apps                                                         |
| `passthrough_unbounded_keys`           | bool   | `false`       | Let unbound Cmd/Ctrl/Alt shortcuts pass through                                             |
| `should_exit_after_passthrough`        | bool   | `false`       | Exit mode after a passthrough shortcut                                                      |
| `passthrough_unbounded_keys_blacklist` | array  | `[]`          | Shortcuts to keep consumed when passthrough is on                                           |
| `exec_shell`                           | string | `"/bin/bash"` | Shell binary used for `exec` hotkey commands                                                |
| `exec_shell_args`                      | array  | `["-lc"]`     | Shell arguments, with the command string appended last                                      |

`kb_layout_to_use` decides which physical key a binding answers. Empty picks a
layout with Latin letters, so bindings stay put while you type in another
language. To force one, use your platform's name for it:

| Platform | Value                                       | Example                      |
| -------- | ------------------------------------------- | ---------------------------- |
| macOS    | Input source ID                             | `com.apple.keylayout.Dvorak` |
| Linux    | XKB layout name, matched regardless of case | `English (Dvorak)`           |
| Windows  | Keyboard layout identifier                  | `00010409`                   |

The `keyboard_layouts` row of `neru doctor` lists the layouts Neru sees, in
this option's form, and the one in use. An unmatched value is reported there and
in the log, and Neru keeps its automatic choice.

`passthrough_unbounded_keys` and `should_exit_after_passthrough` work on macOS,
Windows and Wayland with the evdev keyboard proxy. X11 cannot pass a grabbed
chord through.

## [theme]

Base colors from which all component defaults are derived, set in
`[theme.light]` and `[theme.dark]`. Use solid `#RRGGBB` or `#RGB` (no alpha).
An explicit component color overrides the derived one.

```toml
[theme.light]
accent = "#465FBC"

[theme.dark]
accent = "#6E82D6"
```

| Key             | Role                                                | Light default | Dark default |
| --------------- | --------------------------------------------------- | ------------- | ------------ |
| `surface`       | Translucent fills, badges, indicator backgrounds    | `#EEF2FF`     | `#0A1338`    |
| `accent`        | Borders, lines, primary chrome                      | `#465FBC`     | `#6E82D6`    |
| `accent_alt`    | Active/emphasis states, highlights, virtual pointer | `#0B2377`     | `#8FA2F0`    |
| `on_accent_alt` | Foreground text/icon on `accent_alt` surfaces       | `#F8FAFF`     | `#081022`    |
| `text`          | Foreground text on `surface` backgrounds            | `#17327A`     | `#E8EEFF`    |

### Color format

Every color option takes hex with optional alpha. The alpha byte is
`round(opacity * 255)`, e.g. `F2` for 95% and `B3` for 70%. A color is a string,
or a table with `light` and `dark` keys such as
`{ light = "#FF0000AA", dark = "#00FF00AA" }`. A color you leave out is derived
from `[theme]` and follows the system appearance.

| Format      | Example     | Alpha | Notes              |
| ----------- | ----------- | ----- | ------------------ |
| `#AARRGGBB` | `#FF000000` | Yes   | Recommended format |
| `#RRGGBB`   | `#FF0000`   | No    | Fully opaque       |
| `#RGB`      | `#F00`      | No    | Shorthand          |

### Fonts

Every `font_family` option takes a family name, or one of the generic aliases
`sans`, `serif` and `mono`, which resolve to each platform's own faces. Empty
means sans. On Linux and Windows, a family the system cannot find falls back
to DejaVu Sans or Segoe UI.

## [hints]

Labels clickable elements. The default `axtree` strategy reads the platform
accessibility tree. Two screen-capture strategies cover apps with a thin tree.
Both scan the focused window by default and add the system surfaces the
`include_*` options ask for.

- `vision`: one OCR pass per activation, so hint search (`--search`) and
  `--split-word` work. What it finds on each platform is in
  [Platform support](platform-support.md#notes).
- `contour`: edge analysis from [wl-kbptr](https://github.com/moverest/wl-kbptr),
  a few milliseconds with no dependency. Its hints carry no text, so search and
  word splitting do not apply. Tune it under [`[hints.contour]`](#contour-options).

Press `/` in hints mode to filter hints by text. See the
[hints search recipe](../guide/recipes.md#search-hints-by-text).

```toml
[hints]
hint_characters = "asdfghjkl"
strategy = "vision"

[hints.ui]
font_size = 12
```

### Options

| Option                             | Type         | Default                  | Description                                                                                                                                                |
| ---------------------------------- | ------------ | ------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `enabled`                          | bool         | `true`                   | Turn hints mode on or off                                                                                                                                  |
| `strategy`                         | string       | `"axtree"`               | Element detection: `"axtree"`, `"vision"` or `"contour"`. Overridable per app                                                                              |
| `capture_scope`                    | string       | `"window"`               | Region `vision` and `contour` scan: `"window"` (the screen if nothing is focused) or `"screen"`. Overridable per app and with `neru hints --capture-scope` |
| `hint_characters`                  | string       | `"asdfghjkl"`            | Characters used for labels                                                                                                                                 |
| `label_direction`                  | string       | `"normal"`               | `"normal"` or `"reverse"`, see [Choosing a label direction](#choosing-a-label-direction). Overridable per app and with `neru hints --label-direction`      |
| `max_depth`                        | int          | `50`                     | Deepest accessibility tree level to read, `0` for unlimited                                                                                                               |
| `include_menubar_hints`            | bool         | `false`                  | Show hints on menubar items                                                                                                                                |
| `include_dock_hints`               | bool         | `false`                  | Show hints on Dock items, and on Mission Control's windows and desktops while `detect_mission_control` is on                                              |
| `include_nc_hints`                 | bool         | `false`                  | Show hints in Notification Center                                                                                                                          |
| `include_stage_manager_hints`      | bool         | `false`                  | Show hints in Stage Manager                                                                                                                                |
| `include_pip_hints`                | bool         | `false`                  | Show hints on Picture in Picture controls                                                                                                                  |
| `include_screen_capture_hints`     | bool         | `false`                  | Show hints on Screen Capture controls                                                                                                                      |
| `detect_mission_control`           | bool         | `false`                  | Detect Mission Control, so hints target its windows and desktops instead of the frontmost window                                                          |
| `on_mission_control_activated`     | string/array | none                     | Deprecated, removed in v2. Use [`hooks.on_mission_control_activated`](#hooks)                                                                              |
| `on_mission_control_deactivated`   | string/array | none                     | Deprecated, removed in v2. Use [`hooks.on_mission_control_deactivated`](#hooks)                                                                            |
| `additional_menubar_hints_targets` | array        | macOS-specific defaults  | Extra menubar bundle IDs                                                                                                                                   |
| `clickable_roles`                  | array        | shared semantic defaults | Roles that generate hints. See [Clickable roles](#clickable-roles)                                                                                         |
| `ignore_clickable_check`           | bool         | `false`                  | Skip clickability heuristic                                                                                                                                |
| `visible_check_enabled`            | bool         | `false`                  | Enable visibility hit-test (slower but fewer noisy hints)                                                                                                  |

### Clickable roles

`hints.clickable_roles` decides which accessibility elements get a hint. Write
entries in Neru's semantic vocabulary, and Neru resolves them to the platform's
AX roles, AT-SPI role names or UI Automation control types.

```toml
[hints]
clickable_roles = ["button", "link", "text_field"]
```

`neru roles` shows the vocabulary and how each name resolves on this machine.
`neru roles --explain` shows how your config resolves.

#### Semantic roles

| Semantic       | macOS (`ax:`)          | Linux (`atspi:`)                        | Windows (`uia:`)      |
| -------------- | ---------------------- | --------------------------------------- | --------------------- |
| `button`       | `AXButton`             | `push button`, `button`, `toggle button` | `Button`, `SplitButton` |
| `menu_button`  | `AXMenuButton`         | `push button menu`                      | —                     |
| `popup_button` | `AXPopUpButton`        | `combo box`                             | `ComboBox`            |
| `combo_box`    | `AXComboBox`           | `combo box`                             | `ComboBox`            |
| `link`         | `AXLink`               | `link`                                  | `Hyperlink`           |
| `checkbox`     | `AXCheckBox`           | `check box`, `check menu item`          | `CheckBox`            |
| `radio`        | `AXRadioButton`        | `radio button`, `radio menu item`       | `RadioButton`         |
| `switch`       | `AXSwitch` †           | `switch`, `toggle button`               | —                     |
| `disclosure`   | `AXDisclosureTriangle` | —                                       | —                     |
| `text_field`   | `AXTextField`          | `entry`, `password text`                | `Edit`                |
| `text_area`    | `AXTextArea`           | `entry`                                 | `Edit`                |
| `search_field` | `AXSearchField` †      | `entry`                                 | `Edit`                |
| `slider`       | `AXSlider`             | `slider`                                | `Slider`              |
| `stepper`      | `AXIncrementor`        | `spin button`                           | `Spinner`             |
| `tab`          | `AXTabButton` †        | `page tab`                              | `TabItem`             |
| `menu_item`    | `AXMenuItem`           | `menu item`                             | `MenuItem`            |
| `menubar_item` | `AXMenuBarItem`        | —                                       | —                     |
| `dock_item`    | `AXDockItem`           | —                                       | —                     |
| `cell`         | `AXCell`               | `table cell`                            | `DataItem`            |
| `row`          | `AXRow`                | `table row`                             | `TreeItem`            |
| `list_item`    | `AXRow`                | `list item`                             | `ListItem`            |
| `image`        | `AXImage`              | `image`, `icon`                         | `Image`               |
| `static_text`  | `AXStaticText`         | `static`, `label`, `text`               | `Text`                |
| `heading`      | `AXHeading`            | `heading`                               | —                     |
| `color_well`   | `AXColorWell`          | `color chooser`                         | —                     |
| `toolbar_button` | `AXToolbarButton` †  | —                                       | —                     |

A `—` means the platform has no equivalent and the entry is ignored there.
`neru config validate` warns about such an entry once your `clickable_roles`
differs from the shipped list, and always for `additional_clickable_roles`.
`neru roles --explain` and `neru doctor` always report it.

† A subrole. AppKit reports it in the element's subrole while the role stays
generic, e.g. a search field is an `AXTextField` with subrole `AXSearchField`.
Neru matches names against both role and subrole, so these work as written.

#### Native roles

Address any native role directly with its vocabulary prefix:

```toml
clickable_roles = [
    "button",
    "ax:AXDisclosureTriangle",   # macOS only
    "atspi:page tab list",       # Linux only
    "uia:Custom",                # Windows only
]
```

Prefixed entries for another platform are ignored, not rejected. Many legacy
Win32 and WinForms controls appear only as `uia:Pane`, `uia:Custom` or
`uia:Document` and need naming directly. An unprefixed entry must be a semantic
role, and an unknown one is a config error:

```
hints.clickable_roles: unknown role "AXButton": use "button"
```

### UI

| Option               | Type   | Default    | Description                                                                                                                   |
| -------------------- | ------ | ---------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `font_size`          | int    | `10`       | Font size in points                                                                                                           |
| `font_family`        | string | `""`       | Font family. Accepts [generic aliases](#fonts), and empty means the platform's sans family    |
| `border_radius`      | int    | `-1`       | Corner radius, `-1` for automatic                                                                                                     |
| `padding_x`          | int    | `-1`       | Horizontal padding, `-1` for automatic                                                                                                |
| `padding_y`          | int    | `-1`       | Vertical padding, `-1` for automatic                                                                                                  |
| `border_width`       | int    | `1`        | Border width in pixels                                                                                                        |
| `placement`          | string | `"bottom"` | Label placement relative to the element: `top`, `center`, `bottom`. `top` and `bottom` draw a connector arrow |
| `background_color`   | color  | derived    | Background color                                                                                                              |
| `text_color`         | color  | derived    | Text color                                                                                                                    |
| `matched_text_color` | color  | derived    | Text color for matched characters                                                                                             |
| `border_color`       | color  | derived    | Border color                                                                                                                  |

### Boundary highlight

Element outlines for dense layouts, in `[hints.boundary_highlight]`.

| Option             | Type  | Default | Description                    |
| ------------------ | ----- | ------- | ------------------------------ |
| `enabled`          | bool  | `false` | Draw element boundaries        |
| `border_width`     | int   | `1`     | Stroke width in pixels         |
| `border_radius`    | int   | `-1`    | Corner radius (-1 = auto pill) |
| `background_color` | color | derived | Element fill color             |
| `border_color`     | color | derived | Element stroke color           |

### Search input UI

`[hints.search_input_ui]` also takes the [hints UI](#ui) options except
`placement` and `matched_text_color`.

| Option     | Type   | Default           | Description                                                                                             |
| ---------- | ------ | ----------------- | ------------------------------------------------------------------------------------------------------- |
| `position` | string | `"bottom_center"` | Anchor: `top_left`, `top_center`, `top_right`, `center`, `bottom_left`, `bottom_center`, `bottom_right` |
| `x_offset` | int    | `0`               | Horizontal offset from anchor                                                                           |
| `y_offset` | int    | `24`              | Vertical offset from anchor                                                                             |
| `width`    | int    | `320`             | Width in pixels                                                                                         |

### Vision

`[hints.vision]` is read only when the global or per-app `strategy` is
`"vision"`. The rectangle and confidence options do nothing on some platforms,
see [Platform support per word](platform-support.md#platform-support-per-word).

| Option                             | Type  | Default | Description                                                                   |
| ---------------------------------- | ----- | ------- | ----------------------------------------------------------------------------- |
| `detect_text`                      | bool  | `true`  | Enable text detection. With this off, Linux detects nothing                   |
| `detect_rectangles`                | bool  | `true`  | Enable rectangle detection                                                    |
| `request_timeout_ms`               | int   | `5000`  | Timeout for one analysis request (one OCR pass on Linux), in ms               |
| `minimum_confidence`               | float | `0.0`   | Minimum confidence (0.0 to 1.0) for keeping an observation                    |
| `merge_iou_threshold`              | float | `0.5`   | Intersection-over-Union overlap at which boxes merge                          |
| `rectangle_max_candidates`         | int   | `100`   | Maximum rectangle candidates to evaluate                                      |
| `rectangle_min_size`               | float | `0.01`  | Minimum rectangle size as a fraction of the captured region (`0.01` is 1%)      |
| `rectangle_min_aspect`             | float | `0.3`   | Minimum rectangle aspect ratio (width/height)                                 |
| `rectangle_max_aspect`             | float | `10.0`  | Maximum rectangle aspect ratio (width/height)                                 |
| `button_min_confidence`            | float | `0.3`   | Minimum confidence for classifying a rectangle as a button                    |
| `button_min_aspect`                | float | `0.8`   | Minimum aspect ratio for buttons                                              |
| `button_max_aspect`                | float | `8.0`   | Maximum aspect ratio for buttons                                              |
| `button_icon_max_size`             | int   | `48`    | Maximum width/height in pixels for square buttons or icons                    |
| `link_min_aspect`                  | float | `5.0`   | Minimum aspect ratio for text links                                           |
| `link_max_height`                  | int   | `40`    | Maximum height in pixels for text links                                       |
| `link_min_width`                   | int   | `50`    | Minimum width in pixels for text links                                        |
| `image_min_size`                   | int   | `48`    | Minimum width/height in pixels for images                                     |
| `checkbox_max_size`                | int   | `32`    | Maximum width/height in pixels for checkboxes                                 |
| `generic_clickable_min_confidence` | float | `0.5`   | Minimum confidence for generic clickable elements                             |

### Contour options

`[hints.contour]` works on all three platforms. Defaults are wl-kbptr's. Sizes
are logical pixels (points on Retina). Edge thresholds are Sobel gradient
magnitudes on a 0 to 255 grayscale frame, at most 1530. Lower them for faint
outlines on low-contrast themes, and widen the target bounds to hint
notification cards and toasts.

| Option                | Type  | Default | Description                                                                             |
| --------------------- | ----- | ------- | --------------------------------------------------------------------------------------- |
| `request_timeout_ms`  | int   | `2000`  | Time budget for one pass. A pass over budget returns no targets                         |
| `edge_low_threshold`  | int   | `70`    | Canny low threshold, which extends an edge. Must be at most the high value              |
| `edge_high_threshold` | int   | `220`   | Canny high threshold, which starts an edge. Lower finds fainter outlines                |
| `min_target_width`    | float | `7.0`   | Blobs this wide or narrower are noise                                                   |
| `min_target_height`   | float | `3.0`   | Blobs this tall or shorter are noise                                                    |
| `max_target_width`    | float | `650.0` | Blobs this wide or wider are layout containers                                          |
| `max_target_height`   | float | `160.0` | Blobs this tall or taller are layout containers                                         |
| `flat_line_height`    | float | `6.0`   | Nested strokes no taller than this are dropped (hamburger lines, underlines)            |
| `container_height`    | float | `50.0`  | A blob this tall holding button-sized children is a card and is dropped for them        |
| `same_center_slack`   | float | `8.0`   | A nested blob whose center is this close to its parent's duplicates the parent          |
| `square_icon_size`    | float | `40.0`  | A roughly square parent smaller than this keeps its box and drops its inner detail      |
| `square_icon_slack`   | float | `5.0`   | How far from square (width minus height) that parent may be                             |

### Choosing a label direction

`label_direction` sets how multi-character labels are enumerated once
single-character labels run out. With `asdf` and 5 elements:

| Direction          | Sequence         | Notes                                                                              |
| ------------------ | ---------------- | ---------------------------------------------------------------------------------- |
| `normal` (default) | `A S D FA FS`    | Keeps 3 single-char labels, then expands the 4th alphabet slot into 2-char labels. |
| `reverse`          | `AA SA DA FA AS` | Fills the 2-char tier uniformly from the first alphabet character.                 |

Use `normal` for most workflows and for short alphabets, since it needs fewer
keystrokes. Use `reverse` when many hints cluster in one region or you
regularly need more than `len(hint_characters)` hints, since it spreads first
characters evenly. Set it per app in the [per-app config](#per-app-config-1)
or per activation with [`neru hints --label-direction`](cli.md#neru-hints).

### Default hotkeys

```toml
[hints.hotkeys]  # plus the shared defaults under Per-mode hotkeys
"/"         = "action search_hints"
"Backspace" = "action backspace"
"Tab"       = "action cycle_hint"
"Shift+Tab" = "action cycle_hint --backward"
```

### Per-app config

An empty string falls back to the global value.

| Field                        | Type   | Description                                                                                  |
| ---------------------------- | ------ | -------------------------------------------------------------------------------------------- |
| `bundle_id`                  | string | App bundle ID                                                                                |
| `strategy`                   | string | `"axtree"`, `"vision"` or `"contour"`                                                        |
| `capture_scope`              | string | `"window"` or `"screen"`                                                                     |
| `label_direction`            | string | `"normal"` or `"reverse"`                                                                    |
| `additional_clickable_roles` | array  | Extra roles to treat as clickable, same vocabulary as [`clickable_roles`](#clickable-roles) |
| `ignore_clickable_check`     | bool   | Skip clickability heuristic for this app                                                     |
| `visible_check_enabled`      | bool   | Enable visibility hit-test for this app                                                      |
| `hotkeys`                    | map    | [per-app hotkey overrides](#per-app-hotkey-overrides)                                        |

## [grid]

Settings for grid mode. How the mode behaves is in
[`neru grid`](cli.md#neru-grid).

```toml
[grid]
characters = "asdfghjkl"
capture_scope = "window"
```

### Options

| Option              | Type   | Default                       | Description                                                                                                       |
| ------------------- | ------ | ----------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `enabled`           | bool   | `true`                        | Turn grid mode on or off                                                                                          |
| `capture_scope`     | string | `"screen"`                    | Region the grid covers: `screen` or `window` (the screen if nothing is focused). `--capture-scope` overrides it   |
| `characters`        | string | `"abcdefghijklmnpqrstuvwxyz"` | Primary grid labels. Cannot be empty or contain non-ASCII                                                        |
| `sublayer_keys`     | string | `"abcdefghijklmnpqrstuvwxyz"` | Subgrid labels, first 9 used (3×3). Empty uses the grid's label characters. Cannot contain non-ASCII             |
| `max_label_length`  | int    | `4`                           | Maximum coarse-grid label length (2 to 4). A lower limit enlarges the coarse grid to still cover the screen      |
| `row_labels`        | string | `""`                          | Custom row labels. Empty infers them from `characters`                                                            |
| `col_labels`        | string | `""`                          | Custom column labels. Empty infers them from `characters`                                                         |
| `live_match_update` | bool   | `true`                        | Highlight cells as you type                                                                                       |
| `hide_unmatched`    | bool   | `true`                        | Hide non-matching cells                                                                                           |
| `prewarm_enabled`   | bool   | `true`                        | Pre-compute grid on startup                                                                                       |
| `enable_gc`         | bool   | `false`                       | Periodic memory cleanup                                                                                           |

`neru config validate` warns, without refusing the file, when `characters`,
`row_labels` or `col_labels` has a single character, a repeat (case-folded, so
`aA` repeats), whitespace or a control character, or when `row_labels` or
`col_labels` has non-ASCII.

- A repeat is dropped, in every set including `sublayer_keys`.
- A `characters` with fewer than two distinct characters falls back to `a-z`.
- Short row or column labels cap the grid to the cells they can name.
- An empty label set is checked and reported as `characters`.
- `sublayer_keys` is not checked for these faults. A 9-character set with a
  repeat leaves one subgrid cell unlabeled.

### UI

| Option                     | Type   | Default | Description                                                                                                                |
| -------------------------- | ------ | ------- | -------------------------------------------------------------------------------------------------------------------------- |
| `font_size`                | int    | `10`    | Largest label size in points. Labels shrink, at one size for the whole grid, to fit small cells, and never hide          |
| `font_family`              | string | `""`    | Font family. Accepts [generic aliases](#fonts), and empty means the platform's sans family |
| `border_width`             | int    | `1`     | Border width in pixels                                                                                                     |
| `background_color`         | color  | derived | Cell background                                                                                                            |
| `text_color`               | color  | derived | Label text                                                                                                                 |
| `matched_text_color`       | color  | derived | Matched cell text                                                                                                          |
| `matched_background_color` | color  | derived | Matched cell background                                                                                                    |
| `matched_border_color`     | color  | derived | Matched cell border                                                                                                        |
| `border_color`             | color  | derived | Default cell border                                                                                                        |

### Default hotkeys

```toml
[grid.hotkeys]  # plus the shared defaults under Per-mode hotkeys
"`"         = "toggle-cursor-follow-selection"
"Space"     = "action reset"
"Backspace" = "action backspace"
```

### Per-app config

| Field           | Type   | Description                                           |
| --------------- | ------ | ----------------------------------------------------- |
| `bundle_id`     | string | App bundle ID                                         |
| `capture_scope` | string | `screen` or `window` region for this app              |
| `hotkeys`       | map    | [per-app hotkey overrides](#per-app-hotkey-overrides) |

## [recursive_grid]

Settings for recursive grid mode. How the mode behaves is in
[`neru recursive_grid`](cli.md#neru-recursive_grid).

```toml
[recursive_grid]
grid_cols = 2
grid_rows = 2
keys = "uijk"   # one key per cell
```

### Options

| Option            | Type   | Default       | Description                                                                                                     |
| ----------------- | ------ | ------------- | --------------------------------------------------------------------------------------------------------------- |
| `enabled`         | bool   | `true`        | Turn the mode on or off                                                                                             |
| `capture_scope`   | string | `"screen"`    | Region the first level covers: `screen` or `window` (the screen if nothing is focused). `--capture-scope` overrides it |
| `grid_cols`       | int    | `3`           | Columns (≥ 1, total cells ≥ 2)                                                                                  |
| `grid_rows`       | int    | `3`           | Rows (≥ 1, total cells ≥ 2)                                                                                     |
| `keys`            | string | `"rtyfghvbn"` | Cell selection keys (must be `grid_cols × grid_rows` characters)                                                |
| `min_size_width`  | int    | `1`           | Minimum cell width, in apparent pixels (scaled with the display on Windows and X11)                             |
| `min_size_height` | int    | `1`           | Minimum cell height, in apparent pixels (scaled with the display on Windows and X11)                            |
| `max_depth`       | int    | `10`          | Maximum recursion levels (1 to 20)                                                                              |
| `layers`          | array  | `[]`          | Per-depth layout overrides (see below)                                                                          |

#### Layers

Each entry overrides the grid dimensions and keys at one depth:

| Field       | Type   | Default        | Description                 |
| ----------- | ------ | -------------- | --------------------------- |
| `depth`     | int    | required       | Depth to override (0-based) |
| `grid_cols` | int    | same as parent | Columns at this depth       |
| `grid_rows` | int    | same as parent | Rows at this depth          |
| `keys`      | string | same as parent | Selection keys at this depth |

```toml
[recursive_grid]
layers = [
  { depth = 0, grid_cols = 2, grid_rows = 2, keys = "crtn" },
  { depth = 1, grid_cols = 3, grid_rows = 3, keys = "gcrhtnmwv" },
]
```

### Animation

| Option        | Type | Default | Description                                     |
| ------------- | ---- | ------- | ----------------------------------------------- |
| `enabled`     | bool | `true`  | Native depth transitions on supported platforms |
| `duration_ms` | int  | `50`    | Transition duration, in ms             |

### UI

Labels shrink to fit narrowing cells, keeping each cell at least
`label_autohide_multiplier` times the font size, and hide below
`min_font_size`. All labels in one draw share a size. Set `min_font_size` to
`font_size` to never shrink. The sub-key preview follows the same rule with its
own size and multiplier.

| Option                                | Type   | Default | Description                                                                  |
| ------------------------------------- | ------ | ------- | ---------------------------------------------------------------------------- |
| `font_size`                           | int    | `10`    | Largest label size                                                           |
| `font_family`                         | string | `""`    | Font family. Accepts [generic aliases](#fonts), and empty means the platform's sans family |
| `line_width`                          | int    | `1`     | Grid line width                                                              |
| `line_color`                          | color  | derived | Grid line color                                                              |
| `highlight_color`                     | color  | derived | Selected cell highlight                                                      |
| `text_color`                          | color  | derived | Label text                                                                   |
| `label_background`                    | bool   | `false` | Background behind labels                                                     |
| `label_background_color`              | color  | derived | Label background                                                             |
| `label_background_padding_x`          | int    | `-1`    | Horizontal label padding, `-1` for automatic                                         |
| `label_background_padding_y`          | int    | `-1`    | Vertical label padding, `-1` for automatic                                           |
| `label_background_border_radius`      | int    | `-1`    | Label corner radius, `-1` for automatic                                              |
| `label_background_border_width`       | int    | `1`     | Label border width                                                           |
| `label_char`                          | string | `""`    | Override all cell labels with a single character (e.g. `·`), empty = use key |
| `min_font_size`                       | int    | `6`     | Smallest size a label or the sub-key preview shrinks to before it hides      |
| `label_autohide_multiplier`           | float  | `1.5`   | Keep cell >= fontSize × multiplier by shrinking the label, `0` turns it off      |
| `sub_key_preview`                     | bool   | `false` | Show a mini-grid of the next level's keys inside each cell                   |
| `sub_key_preview_font_size`           | int    | `8`     | Sub-key preview font size                                                    |
| `sub_key_preview_autohide_multiplier` | float  | `1.5`   | The same requirement for the preview, measured against one sub-cell          |
| `sub_key_preview_text_color`          | color  | derived | Sub-key preview text color                                                   |
| `sub_key_preview_label_char`          | string | `""`    | Override sub-key labels with a single character (e.g. `·`), empty = use key  |

### Default hotkeys

The same as [grid](#grid): `` ` `` toggles cursor follow, `Space` resets, and
`Backspace` steps back.

### Per-app config

`[[recursive_grid.app_configs]]` takes `bundle_id`, `capture_scope` and
[`hotkeys`](#per-app-hotkey-overrides), as in [grid](#grid).

## [bisect]

Settings for bisect mode. How the mode behaves is in
[`neru bisect`](cli.md#neru-bisect).

```toml
[bisect]
capture_scope = "window"
```

### Options

| Option          | Type   | Default    | Description                                                                                                        |
| --------------- | ------ | ---------- | ------------------------------------------------------------------------------------------------------------------ |
| `enabled`       | bool   | `true`     | Turn the mode on or off                                                                                                |
| `capture_scope` | string | `"screen"` | Region the session starts from: `screen` or `window` (the screen if nothing is focused). `--capture-scope` overrides it |

### Default hotkeys

The quadrant cells show whichever single-character keys are bound to the
quadrant cuts.

```toml
[bisect.hotkeys]  # plus the shared defaults under Per-mode hotkeys
"`"         = "toggle-cursor-follow-selection"
"h"         = "action bisect --direction=left"
"j"         = "action bisect --direction=down"
"k"         = "action bisect --direction=up"
"l"         = "action bisect --direction=right"
"y"         = "action bisect --direction=up_left"
"u"         = "action bisect --direction=up_right"
"b"         = "action bisect --direction=down_left"
"n"         = "action bisect --direction=down_right"
"Space"     = "action reset"
"Backspace" = "action backspace"
```

### Animation

| Option        | Type | Default | Description                                           |
| ------------- | ---- | ------- | ----------------------------------------------------- |
| `enabled`     | bool | `true`  | Native transition between cuts on supported platforms |
| `duration_ms` | int  | `50`    | Transition duration, in ms                   |

### UI

The same options and defaults as [`[recursive_grid.ui]`](#ui-2). `label_char`
overrides the four quadrant labels, and `label_autohide_multiplier` and
`min_font_size` shrink and then hide them. The `sub_key_preview*` options are
accepted and do nothing.

### Per-app config

`[[bisect.app_configs]]` takes `bundle_id`, `capture_scope` and
[`hotkeys`](#per-app-hotkey-overrides), as in [grid](#grid).

## [scroll]

Keyboard-driven scrolling.

```toml
[scroll]
scroll_step = 80
invert_scroll = true
```

### Options

| Option             | Type | Default   | Description                                                                       |
| ------------------ | ---- | --------- | --------------------------------------------------------------------------------- |
| `scroll_step`      | int  | `50`      | Pixels per line scroll action                                                     |
| `scroll_step_half` | int  | `500`     | Pixels per half-page action                                                       |
| `scroll_step_full` | int  | `1000000` | Pixels for top/bottom jump actions                                                |
| `invert_scroll`    | bool | `false`   | Invert scroll direction (for tools like Mos that reverse synthetic scroll events) |

### Default hotkeys

```toml
[scroll.hotkeys]  # plus the shared defaults under Per-mode hotkeys
"k"       = "action scroll_up"
"j"       = "action scroll_down"
"h"       = "action scroll_left"
"l"       = "action scroll_right"
"gg"      = "action go_top"
"Shift+G" = "action go_bottom"
"u"       = "action page_up"
"PageUp"  = "action page_up"
"d"       = "action page_down"
"PageDown"= "action page_down"
```

### Per-app config

| Field              | Type   | Description                                           |
| ------------------ | ------ | ----------------------------------------------------- |
| `bundle_id`        | string | App bundle ID                                         |
| `scroll_step`      | int    | `scroll_step` for this app                            |
| `scroll_step_half` | int    | `scroll_step_half` for this app                       |
| `scroll_step_full` | int    | `scroll_step_full` for this app                       |
| `hotkeys`          | map    | [per-app hotkey overrides](#per-app-hotkey-overrides) |

## [monitor_select]

Picks a display by typing the label on its badge. Monitors are ordered top to
bottom, then left to right.

```toml
[monitor_select]
enabled = true
characters = "asdf"
```

| Option       | Type   | Default       | Description                        |
| ------------ | ------ | ------------- | ---------------------------------- |
| `enabled`    | bool   | `false`       | Enable interactive monitor picking |
| `characters` | string | `"123456789"` | Characters used for monitor labels |

### UI

| Key                    | Default        | Description                                                                                                         |
| ---------------------- | -------------- | ------------------------------------------------------------------------------------------------------------------- |
| `font_size`            | `96`           | Largest badge label size. It shrinks to fit the badge, which is capped at 80% of the monitor                       |
| `font_family`          | `""` (sans)    | Badge label font family. Accepts [generic aliases](#fonts), and empty means sans     |
| `subtitle_font_size`   | `18`           | Largest monitor name size. It shrinks so a long name fits the badge                                                 |
| `subtitle_font_family` | `""` (label's) | Subtitle font family, defaulting to the label's. Accepts [generic aliases](#fonts)  |
| `border_radius`        | `-1` (auto)    | Badge corner radius                                                                                                 |
| `padding_x`            | `-1` (auto)    | Horizontal padding                                                                                                  |
| `padding_y`            | `-1` (auto)    | Vertical padding                                                                                                    |
| `border_width`         | `1`            | Badge border width                                                                                                  |
| `background_color`     | derived        | Badge fill color                                                                                                    |
| `text_color`           | derived        | Label text color                                                                                                    |
| `matched_text_color`   | derived        | Partially-typed label text color                                                                                    |
| `border_color`         | derived        | Badge border color                                                                                                  |
| `backdrop_color`       | `""` (none)    | Per-monitor overlay backdrop tint                                                                                   |
| `subtitle_text_color`  | derived        | Subtitle text color                                                                                                 |

### Hotkeys

`[monitor_select.hotkeys]` defaults to `"Escape" = "idle"`.

## [virtual_pointer]

Styles the pointer character Neru draws in place of the cursor. The standalone
overlay, drawn when `hide_cursor` hides the system cursor, is macOS-only. The
in-frame indicator in grid and recursive-grid overlays uses the same options on
every platform.

```toml
[virtual_pointer.ui]
char = "+"
font_size = 12
```

### UI

| Option        | Type   | Default | Description                                                                                                                |
| ------------- | ------ | ------- | -------------------------------------------------------------------------------------------------------------------------- |
| `char`        | string | `"●"`   | Character to display                                                                                                       |
| `font_size`   | int    | `8`     | Font size in points                                                                                                        |
| `font_family` | string | `""`    | Font family. Accepts [generic aliases](#fonts), and empty means the platform's sans family |
| `text_color`  | color  | derived | Character color                                                                                                            |

## [mouse_action_indicator]

A transient marker drawn where a mouse action happens. Works on all platforms,
and animation timing may differ slightly between them.

```toml
[mouse_action_indicator]
enabled = true
actions = ["left_click", "right_click"]

[mouse_action_indicator.ui]
shape = "square"
```

| Option    | Type     | Default                                        | Description        |
| --------- | -------- | ---------------------------------------------- | ------------------ |
| `enabled` | bool     | `false`                                        | Enable indicators  |
| `actions` | string[] | every click, press, release, and toggle action | Triggering actions |

`actions` accepts any mouse button action from the
[action names](cli.md#action-names) that a mode `--action` accepts.

### UI

| Option             | Type   | Default    | Description          |
| ------------------ | ------ | ---------- | -------------------- |
| `size`             | int    | `36`       | Diameter in points   |
| `border_width`     | int    | `2`        | Border width         |
| `background_color` | color  | derived    | Fill color           |
| `border_color`     | color  | derived    | Stroke color         |
| `shape`            | string | `"circle"` | `circle` or `square` |

### Animation

| Option          | Type   | Default      | Description                                    |
| --------------- | ------ | ------------ | ---------------------------------------------- |
| `duration_ms`   | int    | `260`        | Animation duration, in ms                       |
| `start_scale`   | float  | `0.55`       | Starting scale                                 |
| `end_scale`     | float  | `1.35`       | Ending scale                                   |
| `start_opacity` | float  | `0.85`       | Starting opacity                               |
| `end_opacity`   | float  | `0.0`        | Ending opacity                                 |
| `easing`        | string | `"ease_out"` | `linear`, `ease_in`, `ease_out`, `ease_in_out` |

## [mode_indicator]

A floating label that follows the cursor and shows the current mode.

```toml
[mode_indicator.hints]
enabled = true
text = "H"
```

### Per-mode

Each mode has a `[mode_indicator.<mode>]` table for `scroll`, `hints`, `grid`,
`recursive_grid`, `bisect` and `monitor_select`. Only `scroll` is shown by
default, and the default `text` is the mode's name in title case, e.g.
`Recursive Grid`.

| Option             | Type   | Default        | Description                       |
| ------------------ | ------ | -------------- | --------------------------------- |
| `enabled`          | bool   | varies by mode | Show the indicator for this mode |
| `text`             | string | varies by mode | Label text                        |
| `background_color` | color  | derived        | Override background color         |
| `text_color`       | color  | derived        | Override text color               |
| `border_color`     | color  | derived        | Override border color             |

### UI

| Option               | Type   | Default | Description                                                                                                                |
| -------------------- | ------ | ------- | -------------------------------------------------------------------------------------------------------------------------- |
| `font_size`          | int    | `10`    | Font size                                                                                                                  |
| `font_family`        | string | `""`    | Font family. Accepts [generic aliases](#fonts), and empty means the platform's sans family |
| `background_color`   | color  | derived | Background with alpha                                                                                                      |
| `text_color`         | color  | derived | Text color                                                                                                                 |
| `border_color`       | color  | derived | Border color                                                                                                               |
| `border_width`       | int    | `1`     | Border width                                                                                                               |
| `padding_x`          | int    | `-1`    | Horizontal padding, `-1` for automatic                                                                                             |
| `padding_y`          | int    | `-1`    | Vertical padding, `-1` for automatic                                                                                               |
| `border_radius`      | int    | `-1`    | Corner radius, `-1` for automatic                                                                                                  |
| `indicator_x_offset` | int    | `20`    | X offset from cursor (positive = right)                                                                                    |
| `indicator_y_offset` | int    | `20`    | Y offset from cursor (positive = down)                                                                                     |

## [sticky_modifiers]

Tap a modifier inside a mode to hold it for the following actions.

```toml
[sticky_modifiers]
tap_max_duration = 200
```

| Option             | Type | Default | Description                                         |
| ------------------ | ---- | ------- | --------------------------------------------------- |
| `enabled`          | bool | `true`  | Enable sticky modifiers                             |
| `tap_max_duration` | int  | `300`   | Longest press that counts as a tap, in ms. `0` always toggles |

### UI

`[sticky_modifiers.ui]` takes the same options and defaults as
[`[mode_indicator.ui]`](#mode_indicator), except `indicator_x_offset` defaults
to `-40` (left of the cursor).

On Linux the indicator draws `❖⇧⌥⌃`. If they show as boxes, set `font_family`
to a font with those glyphs.

## [smooth_cursor]

Animates cursor movement. Off by default.

```toml
[smooth_cursor]
move_mouse_enabled = true
max_duration = 150
```

| Option                       | Type  | Default | Description                                    |
| ---------------------------- | ----- | ------- | ---------------------------------------------- |
| `move_mouse_enabled`         | bool  | `false` | Enable animated mouse movement                 |
| `steps`                      | int   | `10`    | Number of animation steps                      |
| `max_duration`               | int   | `200`   | Longest animation, in ms                   |
| `duration_per_pixel`         | float | `0.1`   | Ms per pixel for jumps, giving constant speed  |
| `relative_movement_duration` | int   | `50`    | Fixed duration per relative move, in ms (>= 10) |

`relative_movement_duration` applies to `move_mouse_relative`, which the
default arrow bindings use. Each relative move takes this fixed time, so cursor
speed scales with the step size. A move arriving mid-animation extends the
current endpoint, so no distance is lost under key repeat or
[`held_repeat` acceleration](#held_repeat).

## [smooth_scroll]

Splits each scroll into chunked ease-out events. How fine a step can be
depends on the platform, see [Platform support](platform-support.md#notes).

```toml
[smooth_scroll]
enabled = true
```

| Option               | Type  | Default | Description                        |
| -------------------- | ----- | ------- | ---------------------------------- |
| `enabled`            | bool  | `false` | Enable smooth scrolling            |
| `steps`              | int   | `20`    | Number of animation steps          |
| `max_duration`       | int   | `180`   | Longest animation, in ms       |
| `duration_per_pixel` | float | `1.0`   | Ms per pixel for adaptive duration |

- On Wayland, enabling it sends a continuous delta instead of wheel notches,
  which some apps scale differently. Trim `scroll.scroll_step` if the distance
  changes.
- Under [`[held_repeat]`](#held_repeat), each repeat folds in what the previous
  animation had not yet sent, so N repeats travel as far as N presses (within a
  wheel notch per repeat on X11).
- A scroll with a different modifier set cancels the animation in flight and
  drops its remaining distance.

## [held_repeat]

Repeats scroll, page and `move_cell` actions while the key is held, and glides
the cursor for a held `move_mouse_relative` (see [Glide](#glide)).

```toml
[held_repeat]
enabled = true
accel_enabled = true
```

| Option                 | Type     | Default                   | Description                                             |
| ---------------------- | -------- | ------------------------- | ------------------------------------------------------- |
| `enabled`              | bool     | `false`                   | Master toggle for held-key repeat and the glide         |
| `initial_delay_ms`     | int      | `50`                      | Delay before the first repeat, in ms                      |
| `interval_ms`          | int      | `50`                      | Interval between repeats, in ms                           |
| `accel_enabled`        | bool     | `false`                   | Ramp the glide's speed up the longer the key stays held |
| `accel_ramp_ms`        | int      | `500`                     | Hold time to reach `accel_max_multiplier`, in ms          |
| `accel_max_multiplier` | float    | `4.0`                     | Speed multiplier at full ramp                           |
| `accel_targets`        | string[] | `["move_mouse_relative"]` | Action names eligible for acceleration                  |

### Glide

A held key bound to a lone `move_mouse_relative` glides the cursor in the
direction of its `--dx`/`--dy` instead of repeating. Speed is the binding's step
per `interval_ms` (10px every 50ms is 200px/s) from key down to release, so
`initial_delay_ms` does not apply. Two held keys move diagonally at the same
speed, opposite keys cancel, and the larger step sets the speed. A short tap
travels about one step. The glide works across monitors, and a click during it
acts at the cursor's live position.

### Acceleration

With `accel_enabled = true`, the glide speed ramps linearly to
`accel_max_multiplier` times the binding's speed over `accel_ramp_ms`. With the
defaults a 10px binding reaches 500px/s at 250ms and 800px/s from 500ms.

- `accel_targets` accepts only `move_mouse_relative`. Any other entry, or an
  empty list while `accel_enabled = true`, is a config error.
- `accel_enabled = true` with `enabled = false` does nothing, and
  `neru config validate` warns.

## [systray]

The system tray icon and its menu.

```toml
[systray]
enabled = false
```

| Option    | Type | Default | Description                |
| --------- | ---- | ------- | -------------------------- |
| `enabled` | bool | `true`  | Show the tray icon |

A change to `enabled` needs a daemon restart, since `neru config reload` keeps
the icon as is. On Windows, notifications need the tray, see
[Platform support](platform-support.md#notes).

## [logging]

Neru always logs to the console. Set `disable_file_logging = false` to also
write a JSON log file. Its path is under
[Log File Locations](../guide/troubleshooting.md#log-file-locations).

```toml
[logging]
log_level = "debug"
disable_file_logging = false
```

| Option                 | Type   | Default  | Description                                                                 |
| ---------------------- | ------ | -------- | --------------------------------------------------------------------------- |
| `log_level`            | string | `"info"` | Level: `debug`, `info`, `warn`, `error`                                     |
| `disable_file_logging` | bool   | `true`   | Log to the console only. Set `false` to also write a JSON log file          |
| `log_file`             | string | `""`     | Log file path when file logging is on. Empty uses the platform default path |
| `max_file_size`        | int    | `10`     | MB before the log file is rotated                                           |
| `max_backups`          | int    | `5`      | Rotated log files to keep                                                   |
| `max_age`              | int    | `30`     | Days to keep rotated log files                                              |

`info` covers daemon start (with the config path) and stop, config reloads,
and mode activation and exit. `warn` marks something Neru worked around, and `error` a
failure it could not recover from. Use `debug` temporarily for key routing,
hint generation, overlays or IPC. No
level logs typed text, fed keys, exec output or config values.
