# Getting started

## Quick Start

```toml
[hints.hotkeys]
"Shift+L" = ["action left_click", "idle"]

[scroll]
scroll_step = 50
```

Generate a fully-commented starter file:

```bash
neru config init                          # Creates ~/.config/neru/config.toml
neru config init --force                  # Overwrite existing
neru config init -c /path/to/config.toml  # Custom path
```

---

## Config File Location

> **Recommended:** `~/.config/neru/config.toml`

Loaded in priority order (highest first):

1. `$XDG_CONFIG_HOME/neru/config.toml`
2. `%APPDATA%\neru\config.toml` (Windows only)
3. `~/.config/neru/config.toml`
4. `~/.neru.toml` (legacy)
5. `neru.toml` (current directory)
6. `config.toml` (current directory)

Override at launch: `neru launch -c /path/to/config.toml`

On Windows, `neru config init` writes to `%APPDATA%\neru\config.toml` — the
platform convention — but `~/.config/neru/config.toml` is still read, so a config
kept in a cross-platform dotfiles repo works as-is. Set `XDG_CONFIG_HOME` if you
want `neru config init` and `neru config set` to write there too.

### Config Layering

Neru loads configuration in layers. Each layer overrides the previous one:

```
Defaults → config.toml → config.override.toml → Runtime (in-memory)
```

- **Defaults**: Built-in sensible defaults for your platform.
- **config.toml**: Your hand-crafted configuration file.
- **Override file**: Runtime changes from `neru config set` (persistent). Named after your config file (e.g. `config.override.toml` for `config.toml`).
- **Runtime**: In-memory changes that are lost on restart.

This means `neru config set` changes survive restarts without modifying your `config.toml`. To revert, edit or remove the override file.

---

## Managing Your Config

```bash
neru config validate    # Check syntax (no daemon needed)
neru config reload      # Apply changes to running daemon
neru config dump        # Print loaded config as JSON (daemon required)
neru config init        # Create default config file
neru config set <key> <value>   # Change a single value at runtime (see below)
neru config reset <key>         # Remove a single override (reverts to base config)
```

See [CLI.md](../reference/cli.md#configuration-commands) for full flag documentation.

---

## Runtime Config Changes

Neru supports changing individual configuration values at runtime without restarting the daemon or re-reading the config file from disk.

```bash
neru config set hints.hint_characters "qwerty"
neru config set scroll.scroll_step 25
neru config set general.passthrough_unbounded_keys true
```

### How it works

1. The CLI validates the path and value locally before sending to the daemon.
2. The daemon deep-copies the current in-memory config, applies the change, and validates the result.
3. Services, overlays, and hotkeys are reconfigured automatically — the same internal path used by `neru config reload`.
4. The change is automatically persisted to `config.override.toml` alongside your config file. This file is loaded on every start, so changes survive restarts.

### Batch changes with `--no-reload`

Use `--no-reload` when setting or resetting multiple interdependent fields (e.g. `recursive_grid.grid_cols` + `recursive_grid.keys`). Each change persists to the override file without disrupting active hotkeys or exiting the current mode. Run `neru config reload` once after all changes to apply them.

```bash
neru config set --no-reload recursive_grid.grid_cols 3
neru config set --no-reload recursive_grid.grid_rows 3
neru config set --no-reload recursive_grid.keys "gcrhtnmwv"
neru config reload
```

### Resetting overrides with `config reset`

To revert a single field to its base config value, use `neru config reset <key>`. Like `config set`, it supports `--no-reload` for batch operations.

```bash
neru config reset recursive_grid.grid_cols
neru config reset --no-reload recursive_grid.grid_rows
neru config reload
```

### Config override file

Runtime changes via `neru config set` and `neru config reset` are written to an override file alongside your main config file. The override filename is derived from your config file's name: `config.toml` produces `config.override.toml`, `my-neru.toml` produces `my-neru.override.toml`, etc.

The file uses the same TOML format and follows the same layering:

```
Defaults → config.toml → config.override.toml → Runtime (in-memory)
```

To revert all overrides at once, delete the override file and run `neru config reload`.

### Supported field types

| Type    | Example                                   |
| ------- | ----------------------------------------- |
| string  | `neru config set hints.hint_characters qwerty` |
| integer | `neru config set hints.ui.font_size 14`        |
| boolean | `neru config set scroll.invert_scroll true`    |
| float   | `neru config set hints.vision.minimum_confidence 0.3` |
| color   | `neru config set hints.ui.background_color "#FF0000AA"` |
| array   | `neru config set hints.clickable_roles "button,link"` |

> **Tip:** Use `neru config dump | jq` to explore the full config structure and find the dotted path for any setting.

### Limitations

- **Hotkeys**: Cannot be set via `neru config set` (edit `config.toml` directly instead). However, `config set` and `config reset` can be used *as hotkey actions* to change other config fields at runtime.
- **Struct fields**: Sections like `[theme]` must be set via their leaf sub-paths, not as an object.
- **`app_configs`**: Per-app overrides can't be set via `config set` (edit `config.toml` directly).
- **Override file hotkeys**: If you manually edit the override file to add a `[hotkeys]` section, those bindings won't be loaded. The override file is intended for typed field overrides from `config set` — hotkey changes belong in `config.toml`.
- **Array replacement**: Array fields are replaced wholesale, not appended.
- **Derived values**: Settings computed from other settings — the theme colors filled in from `[theme]`, and `grid.row_labels` / `grid.col_labels` / `grid.sublayer_keys` inferred from `grid.characters` — are recomputed by `config set`, including when you set the setting they are computed *from*. `neru config set grid.characters "qwerty"` relabels the grid immediately, and `neru config set theme.light.surface "#1E1E2E"` recolors immediately. Setting a derived value directly still wins: labels and keys you wrote are kept, and only the ones you left empty are inferred. Note `sublayer_keys` ships with a value, so it is only inferred once you blank it — `neru config set grid.sublayer_keys ""` makes the subgrid follow `grid.characters` from then on.
- **`config reload`**: Re-reading from disk re-applies the override file, but any in-memory-only changes (e.g. before this feature existed) are lost.
