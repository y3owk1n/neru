# Getting started

From a fresh install to your own config and hotkeys. If Neru is not installed
yet, start with [Installation](installation.md).

## Quick start

```bash
neru launch          # or open Neru.app on macOS, or the Start Menu entry on Windows
neru config init     # write a commented starter config
neru doctor          # check permissions and backends
```

Then press `Primary+Shift+Space` for hints. `Primary` is `Cmd` on macOS and
`Ctrl` on Linux and Windows. On Linux, bind a key first. See
[Binding your first hotkeys](#binding-your-first-hotkeys).

If you installed the login service, the daemon is already running and
`neru launch` is not needed.

## First launch

With no config file, Neru runs on built-in defaults. What it tells you about
that differs by platform:

- **macOS and Windows** show a welcome dialog that offers to write the starter
  config, to run on defaults without one, or to quit.
- **Linux** starts on defaults. It sends a notification or prints to the
  terminal, telling you to run `neru config init`.

`neru status` reports whether the daemon is running. `neru doctor` works with
or without a daemon and lists each permission and backend with its state.

## Permissions

### macOS

- **Accessibility** is required. Neru asks for it at launch when it is missing.
  Grant it in System Settings, under Privacy & Security, then Accessibility.
- **Screen Recording** is needed only for the `vision` (OCR) and `contour` hint
  strategies. Neru asks the first time one runs, and macOS applies the grant
  only after Neru restarts.

If a grant does not seem to take, see
[Troubleshooting](troubleshooting.md#installation--setup).

### Linux

- **X11** needs nothing beyond the runtime libraries.
- **Wayland** needs your user in the `input` group, to read `/dev/input`, and
  a udev rule that makes `/dev/uinput` writable. The install script offers the
  group. Log out and back in after joining it.

Details, the udev rule and what degrades without each one are in
[Linux setup](linux.md#install-time-environment-adjustments). Some desktops
need more. See [Linux desktops](linux-desktops.md).

### Windows

No permissions to grant. Windows does not let a normal process drive a window
running as administrator, so Neru cannot reach elevated apps unless it runs
elevated too.

## Config file location

`neru config init` writes `~/.config/neru/config.toml`, or
`%APPDATA%\neru\config.toml` on Windows. Set `XDG_CONFIG_HOME` to move it, on
any platform.

At launch Neru uses the first file it finds:

1. `$XDG_CONFIG_HOME/neru/config.toml` when `XDG_CONFIG_HOME` is set,
   otherwise `%APPDATA%\neru\config.toml` on Windows or
   `~/.config/neru/config.toml` elsewhere
2. `~/.config/neru/config.toml`, so a config from a cross-platform dotfiles
   repo works on Windows as-is
3. `~/.neru.toml`
4. `neru.toml` in the current directory
5. `config.toml` in the current directory

`neru launch -c /path/to/config.toml` skips the search.

The starter file is the full default config with every option commented. On
Linux its `[hotkeys]` table is commented out, matching the built-in defaults.

```bash
neru config init                          # write the starter file
neru config init --force                  # overwrite an existing one
neru config init -c /path/to/config.toml  # write somewhere else
```

## Config layering

Each layer overrides the one before it:

```
built-in defaults -> config.toml -> config.override.toml
```

- **Built-in defaults** for your platform. Anything your file leaves out keeps
  its default.
- **config.toml** is the file you edit.
- **The override file** holds changes made with `neru config set`. It sits
  next to your config and is named after it: `config.toml` gets
  `config.override.toml`, `my-neru.toml` gets `my-neru.override.toml`.

So `neru config set` changes survive restarts without touching your
`config.toml`. To drop them all, delete the override file and run
`neru config reload`.

## Managing your config

Neru does not watch the file. After editing it, apply the change:

```bash
neru config validate   # check the file, no daemon needed
neru config reload     # apply it to the running daemon
neru config dump       # print the config the daemon is using, as JSON
```

A reload re-reads the config and override file and rebuilds hotkeys, overlays
and services. Options read only at startup, such as `[logging]` and
`[systray] enabled`, need a restart. Restart with `neru services restart` if
the login service runs Neru, or quit it from the tray menu and run
`neru launch` again.

Every `neru config` subcommand and flag is in the
[CLI reference](../reference/cli.md#configuration-commands).

## Binding your first hotkeys

Global hotkeys live in `[hotkeys]`. Each key maps to a command, the same one
you would type after `neru` in a shell:

```toml
[hotkeys]
"Primary+Shift+Space" = "hints --action left_click"   # click on select
"Primary+Shift+C" = "recursive_grid"
"Primary+Shift+S" = "scroll"
```

macOS and Windows ship with default bindings for hints, grid, recursive grid,
bisect and scroll. Writing a `[hotkeys]` table adds to them or replaces the
same keys. An empty `[hotkeys]` table turns them all off.

**Linux has no default global hotkeys**, because `Ctrl+Shift+C` and similar
shortcuts belong to your terminal. Either write a `[hotkeys]` table, or bind
`neru hints`, `neru grid` and the rest in your compositor. On Wayland, a
`[hotkeys]` table needs the `input` group from [Permissions](#linux).
Compositor examples are in [Global hotkeys on Wayland](linux-desktops.md#global-hotkeys-on-wayland).

Keys inside a mode, per-app overrides and the full binding syntax are in the
[configuration reference](../reference/configuration.md#hotkeys).

## Runtime config changes

`neru config set` changes one value on the running daemon and saves it to the
override file. `neru config reset` removes it again.

```bash
neru config set hints.hint_characters "qwerty"
neru config set scroll.scroll_step 25
neru config reset scroll.scroll_step
```

Use `--no-reload` when changing fields that only make sense together, then
reload once:

```bash
neru config set --no-reload recursive_grid.grid_cols 3
neru config set --no-reload recursive_grid.grid_rows 3
neru config set --no-reload recursive_grid.keys "gcrhtnmwv"
neru config reload
```

`neru config dump | jq` shows the dotted path of every setting.

Limits:

- Only single values can be set: strings, numbers, booleans, colors and string
  lists. Hotkey tables and `app_configs` are edited in `config.toml`.
- A list is replaced, not appended to.
- Values Neru computes from other settings, such as theme colors and grid
  labels, are recomputed after each change.

`config set` and `config reset` also work as hotkey commands, which is how a
key can toggle a setting.

## Next steps

- [Recipes](recipes.md): worked configs for click on select, restoring the
  cursor, drag, external hotkey daemons and more.
- [Configuration reference](../reference/configuration.md): every option, its
  default and the platforms it affects.
- [CLI reference](../reference/cli.md): every command and flag.
- [Troubleshooting](troubleshooting.md): symptoms and fixes.
