# Configuring Neru

How to create, edit and apply your Neru config. Neru reads one TOML file, and
anything the file leaves out keeps its built-in default, so write only what
you want to change. Every option is in the
[configuration reference](../reference/configuration.md).

## Create the file

```bash
neru config init
```

This writes the full default config with every option commented out, to
`~/.config/neru/config.toml`, or `%APPDATA%\neru\config.toml` on Windows. On
Linux its `[hotkeys]` table is commented out too. Pass `--force` to overwrite
an existing file, or `-c PATH` to write somewhere else.

## Apply your changes

> [!IMPORTANT]
> Neru does not watch the file. Edits take effect only after a reload.

After editing it:

```bash
neru config validate   # check the file, no daemon needed
neru config reload     # apply it to the running daemon
```

A reload rebuilds hotkeys, overlays and services. If the file has an error, the
reload fails and Neru keeps the previous config.

Options read only at startup, such as `[logging]` and `[systray] enabled`,
need you to [restart the daemon](troubleshooting.md#restart-the-daemon).

## Change one value without editing the file

`neru config set` changes one value on the running daemon right away, and
`neru config reset` undoes it.

```bash
neru config set hints.hint_characters "qwerty"
neru config set scroll.scroll_step 25
neru config reset scroll.scroll_step
```

Values that only make sense together, such as a grid's size and keys, are set
with `--no-reload` and applied with one reload at the end:

```bash
neru config set --no-reload recursive_grid.grid_cols 3
neru config set --no-reload recursive_grid.grid_rows 3
neru config set --no-reload recursive_grid.keys "gcrhtnmwv"
neru config reload
```

- `neru config dump | jq` shows the dotted path of every setting.
- Only single values can be set: strings, numbers, booleans, colors and string
  lists. Setting a list replaces it rather than adding to it.
- Edit hotkey tables and `app_configs` in the file itself.
- `config set` and `config reset` also work as hotkey steps, so one key can
  switch a setting, see [Recipes](recipes.md#switch-grid-layouts-with-one-key).

## How the layers combine

Each layer overrides the one before it:

1. Built-in defaults for your platform.
2. `config.toml`, the file you edit.
3. The override file, which holds every `neru config set` change. It sits next
   to your config and takes its name, so `config.toml` gets
   `config.override.toml`.

To drop every `neru config set` change, delete the override file and run
`neru config reload`.

## Where Neru looks for the file

At launch Neru uses the first of these that exists:

1. `$XDG_CONFIG_HOME/neru/config.toml` when `XDG_CONFIG_HOME` is set,
   otherwise `%APPDATA%\neru\config.toml` on Windows or
   `~/.config/neru/config.toml` elsewhere
2. `~/.config/neru/config.toml`, so a dotfiles config works on Windows as-is
3. `~/.neru.toml`
4. `neru.toml` in the current directory
5. `config.toml` in the current directory

`neru launch -c /path/to/config.toml` skips the search.
`neru status --json | jq -r .config` shows which file the running daemon read.

## Next steps

- [Recipes](recipes.md) for ready-made configs.
- [How bindings work](../concepts/bindings.md) before writing your own
  hotkey tables.
- [Configuration reference](../reference/configuration.md) for every option.
