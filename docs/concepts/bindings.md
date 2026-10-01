# How bindings work

A binding maps a key to one or more steps. This page explains what a step can
be, which binding answers a key, and what happens when a step fails. Key names,
modifiers and the default tables are in the
[configuration reference](../reference/configuration.md#hotkeys).

## Where bindings live

| Table                                       | Answers keys                                                                     |
| ------------------------------------------- | -------------------------------------------------------------------------------- |
| `[hotkeys]`                                 | While idle. Inside a mode, for Ctrl/Alt/Cmd chords the mode does not bind itself |
| `[<mode>.hotkeys]`                          | While that mode is open                                                          |
| `[[app_configs]]`, `[[<mode>.app_configs]]` | While the named app is focused, merged over the table they override              |

Mode tables also accept multi-key sequences such as `gg`, with 500 ms allowed
between keys.

## What a step can be

A binding's value is one step, or an array of steps run in order:

```toml
[hotkeys]
"Primary+Shift+D" = ["hints", "exec echo 'hints activated'"]
"PageUp"          = ["action go_top", "action page_down"]
```

Each step is one of:

- a mode command, such as `hints` or `grid --toggle`
- `idle`, which leaves the current mode
- `action <name>`, from the [action names](../reference/cli.md#action-names)
- `macro <name> [args...]`, from
  [`[macros]`](../reference/configuration.md#macros)
- `mode <name>`, for a mode declared under
  [`[modes]`](../reference/configuration.md#modes)
- a `config` command such as `config set`, as on the
  [command line](../reference/cli.md#configuration-commands)
- a runtime toggle: `toggle-cursor-follow-selection`, `toggle-scroll-invert`
  or `toggle-screen-share`
- `run <step>...`, a nested sequence
- `exec <command>`, which runs a shell command through
  [`general.exec_shell`](../reference/configuration.md#general), such as
  `"Primary+T" = "exec open -a Terminal"`

Apart from `exec`, each step is the command you would type after `neru` in a
shell, with the same flags, so you can try a binding from a terminal first.

## Sequences

An array of steps is a sequence. A hotkey binding, a
[macro](../reference/configuration.md#macros), a mode's `--on-exit` steps and
[`neru run`](../reference/cli.md#neru-run) all run sequences the same way.

- Neru logs a failed step and runs the next one.
- End a step with `--bail-on-error`, as its last word, to stop the sequence
  there if that step fails. See
  [Failure policy](../reference/cli.md#failure-policy).
- `action wait_for_mode_exit` pauses until the current mode exits. With
  `--bail`, the sequence stops if the mode was canceled, such as with
  `Escape`.
- A sequence can start another, through `run` or a macro. The nested one
  counts as one step to its caller. The depth limit is under
  [`neru run`](../reference/cli.md#neru-run).

## Which binding wins

While idle, `[hotkeys]` answers, merged with the focused app's
`[[app_configs]]` entry.

Inside a mode, Neru checks in this order:

1. A sticky modifier tap.
2. The mode's table, `[<mode>.hotkeys]` merged with `[[<mode>.app_configs]]`.
3. The mode's own keys, such as hint labels or grid keys.
4. For a Ctrl, Alt or Cmd chord, `[hotkeys]`.

So a mode binding beats a global binding for the same chord. For example,
`"Cmd+Shift+F" = "recursive_grid"` in `[hints.hotkeys]` replaces a global
`"Cmd+Shift+F" = "hints"` while hints are open.

The fallback to `[hotkeys]` is what lets a global
`"Super+;" = "recursive_grid --toggle"` also close the mode from inside it.
Bare keys and Shift-only chords never fall back, because they are labels and
grid keys. In a mode table, `"__disabled__"` hands the chord back to the
global binding rather than silencing it.

Neru rebuilds the merged tables when a mode opens, when the focused app
changes, and when the config is reloaded. Switching apps while a mode is open
applies the new app's overrides from your next key.

## Per-app overrides

An `app_configs` entry changes bindings while one app has focus.
`[[app_configs]]` overrides `[hotkeys]`, and `[[hints.app_configs]]` overrides
`[hints.hotkeys]`, and so on for each mode. The syntax is in
[Per-app hotkey overrides](../reference/configuration.md#per-app-hotkey-overrides).

The entry merges over the base table under the same rules as a config table
merges over the defaults. `bundle_id` means a different thing on each
platform, see
[App identity](../reference/configuration.md#app-identity-across-platforms-bundle_id).

## Mistakes are caught at load

Neru checks every binding and macro when the config loads, so a typo fails
`neru config validate` instead of doing nothing when you press the key.

| In a binding                                                                            | Result                                             |
| --------------------------------------------------------------------------------------- | -------------------------------------------------- |
| A flag no mode has (`hints --serach`), or a value no flag takes (`--strategy=nonsense`) | The config fails to load and Neru runs on defaults |
| A flag the named mode does not accept (`grid --search`)                                 | Loads, and `neru config validate` warns            |
| A flag whose partner is missing (`hints --repeat` with no `--action`)                   | Loads, and `neru config validate` warns            |

A key whose binding loaded with a warning refuses that step when pressed, with
the same message the CLI gives. Steps nested in a `run` or `--on-exit`, and
macro steps with a `$1` placeholder, have their command checked at load and
their flags checked when they run.
