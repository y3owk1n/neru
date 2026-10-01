# Getting started

Install Neru first, see [Installation](installation.md).

## Quick start

1. Start the daemon with `neru launch`, or open `Neru.app` on macOS or the
   Start Menu entry on Windows. Skip this if the login service runs Neru.
2. Grant [permissions](#permissions). `neru doctor` lists each permission and
   backend with its state, and `neru status` reports whether the daemon runs.
3. Write a starter config with `neru config init`. Without one, Neru runs on
   built-in defaults. macOS and Windows show a welcome dialog that offers to
   write it, and Linux sends a notification or prints to the terminal.
4. On Linux, [bind a hotkey](#binding-your-first-hotkeys).
5. Press `Primary+Shift+Space` for hints. `Primary` is `Cmd` on macOS and
   `Ctrl` on Linux and Windows.

## Permissions

### macOS

- **Accessibility** is required. Neru asks for it at launch. Grant it in
  System Settings, under Privacy & Security, then Accessibility.
- **Screen Recording** is needed only for the `vision` (OCR) and `contour`
  hint strategies. Neru asks the first time one runs. macOS applies the grant
  after Neru restarts.

If a grant does not take, see
[Troubleshooting](troubleshooting.md#installation--setup).

### Linux

- **X11** needs only the runtime libraries.
- **Wayland** needs your user in the `input` group, to read `/dev/input`, and
  a udev rule that makes `/dev/uinput` writable. The install script offers the
  group. Log out and back in after joining it.

The udev rule and what degrades without each permission are in
[Linux setup](linux.md#install-time-environment-adjustments). Some desktops
need more, see [Linux desktops](linux-desktops.md).

### Windows

No grants needed. Neru cannot drive windows running as administrator unless
Neru also runs elevated.

## Config file location

`neru config init` writes `~/.config/neru/config.toml`, or
`%APPDATA%\neru\config.toml` on Windows. Set `XDG_CONFIG_HOME` to move it on
any platform. At launch Neru uses the first file it finds:

1. `$XDG_CONFIG_HOME/neru/config.toml` when `XDG_CONFIG_HOME` is set,
   otherwise `%APPDATA%\neru\config.toml` on Windows or
   `~/.config/neru/config.toml` elsewhere
2. `~/.config/neru/config.toml`, so a dotfiles config works on Windows as-is
3. `~/.neru.toml`
4. `neru.toml` in the current directory
5. `config.toml` in the current directory

`neru launch -c /path/to/config.toml` skips the search. The starter file
from `neru config init` is the full default config with every option
commented, and on Linux its `[hotkeys]` table is commented out. Pass `--force`
to overwrite an existing file, or `-c PATH` to write somewhere else.

## Config layering

Each layer overrides the one before it:

1. Built-in defaults for your platform. Anything your file leaves out keeps
   its default.
2. `config.toml`, the file you edit.
3. The override file, which holds changes made with `neru config set`. It sits
   next to your config and takes its name: `config.toml` gets
   `config.override.toml`, `my-neru.toml` gets `my-neru.override.toml`.

To drop every `neru config set` change, delete the override file and run
`neru config reload`.

## Managing your config

Neru does not watch the file. Apply edits yourself:

```bash
neru config validate   # check the file, no daemon needed
neru config reload     # apply it to the running daemon
neru config dump       # print the daemon's config as JSON
```

A reload rebuilds hotkeys, overlays and services from both files. Options read
only at startup, such as `[logging]` and `[systray] enabled`, need a restart.
Run `neru services restart` under the login service, or quit from the tray menu
and run `neru launch`. Every subcommand is in the
[CLI reference](../reference/cli.md#configuration-commands).

## Binding your first hotkeys

Global hotkeys live in `[hotkeys]`. Each value is the command you would type
after `neru` in a shell:

```toml
[hotkeys]
"Primary+Shift+Space" = "hints --action left_click"   # click on select
"Primary+Shift+C" = "recursive_grid"
"Primary+Shift+S" = "scroll"
```

macOS and Windows ship default bindings for hints, grid, recursive grid,
bisect and scroll. A `[hotkeys]` table adds keys or replaces the same keys,
and an empty table turns them all off.

**Linux has no default global hotkeys**, so none collide with terminal
shortcuts such as `Ctrl+Shift+C`. Write a `[hotkeys]` table, or bind `neru hints`,
`neru grid` and the rest in your compositor. On Wayland a `[hotkeys]` table
needs the `input` group, see [Permissions](#linux). Compositor examples are in
[Global hotkeys on Wayland](linux-desktops.md#global-hotkeys-on-wayland).

Mode keys, per-app overrides and the binding syntax are in the
[configuration reference](../reference/configuration.md#hotkeys).

## Runtime config changes

`neru config set` changes one value on the running daemon and saves it to the
override file. `neru config reset` removes it.

```bash
neru config set hints.hint_characters "qwerty"
neru config set scroll.scroll_step 25
neru config reset scroll.scroll_step
```

For fields that only make sense together, pass `--no-reload` and reload once:

```bash
neru config set --no-reload recursive_grid.grid_cols 3
neru config set --no-reload recursive_grid.grid_rows 3
neru config set --no-reload recursive_grid.keys "gcrhtnmwv"
neru config reload
```

- `neru config dump | jq` shows the dotted path of every setting.
- Only single values can be set: strings, numbers, booleans, colors and string
  lists. Edit hotkey tables and `app_configs` in `config.toml`.
- A list is replaced, not appended to.
- Neru recomputes derived values, such as theme colors and grid labels, after
  each change.
- `config set` and `config reset` work as hotkey commands, so a key can toggle
  a setting.

Next, see [Recipes](recipes.md) for worked configs and the
[configuration reference](../reference/configuration.md) for every option.
