---
name: neru-setup-config
description: "Set up or change a user's Neru config.toml: find or create the file, bind or disable hotkeys, tune hints, grids, or scroll, add per-app overrides, macros, or declared modes, set up the login service, then validate and apply it the way the running daemon needs. Use when a Neru user asks to configure Neru, bind a key, change hint or grid behaviour, make one app work, or fix a config that does not take effect."
---

# Setting up a Neru config

Neru reads one TOML file, and every key in it has a documented default and a
platform column saying where it does anything. The work is choosing what to
change, then applying it correctly. Most keys reload on demand, a few need a
daemon restart, and a one-value change is faster through `neru config set`
than through the file. The reference is `docs/CONFIGURATION.md` in the repo,
`man neru-config` on any install, and `neru docs config` in a browser.

## Where the reference lives

The user rarely has a checkout. Every install method ships the binary and man
pages only. Resolve the docs in this order:

1. A checkout in the working directory: `docs/CONFIGURATION.md` exists.
2. Man pages on macOS and Linux: `man neru-config-init`,
   `man neru-config-set`, `man neru-config-validate`, `man neru-services`,
   and `man neru-hints` or another mode page for the flags a hotkey may pass.
   On Windows, `neru <command> --help` carries the same text.
3. The doc at the installed version, fetched from GitHub:

   ```bash
   tag=$(neru --version | sed -n '1s/^Neru version //p' | cut -d- -f1)
   case $tag in v*.*.*) ;; *) tag=main ;; esac
   curl -fsSL "https://raw.githubusercontent.com/y3owk1n/neru/$tag/docs/CONFIGURATION.md"
   ```

   A release build prints its tag. A dev build prints `v1.2.3-14-gabcdef`,
   which the `cut` maps to the release it was built from.

   On Windows, where `man`, `sed`, and `cut` are usually absent, the same
   fetch in PowerShell:

   ```powershell
   $tag = (((neru --version)[0] -replace '^Neru version ', '') -split '-')[0]
   if ($tag -notmatch '^v\d+\.\d+\.\d+$') { $tag = 'main' }
   Invoke-RestMethod "https://raw.githubusercontent.com/y3owk1n/neru/$tag/docs/CONFIGURATION.md"
   ```

   `neru docs config` opens the same page in a browser on every platform,
   which is enough when the user rather than the agent will read it.

The file `neru config init` writes is fully commented and names every key,
so after step 2 below, the user's own file is the quickest reference.
`docs/TIPS_TRICKS.md` at the same URL has recipes to copy for the common
asks: Vimium-style click on select, auto-exit after click, restoring the
cursor after a mode, drag with any button, one key that cycles modes, and
handing all hotkeys to skhd or another daemon.

## Steps

1. **Check the install.** `neru doctor` runs without the daemon and reports
   config validity, socket health, permissions, and what this platform
   supports. `neru status` says whether the daemon runs. On macOS, hints
   and actions need Accessibility, and the `vision` and `contour` strategies
   need Screen Recording, both under System Settings, Privacy & Security.
   On Linux the user needs the `input` group and a `/dev/uinput` udev rule,
   covered in `LINUX_SETUP.md`. Windows needs nothing beyond the install.

2. **Find or create the file.** The first existing path wins:
   `--config`, `$XDG_CONFIG_HOME/neru/config.toml`,
   `%APPDATA%\neru\config.toml` on Windows, `~/.config/neru/config.toml`,
   `~/.neru.toml`, then `neru.toml` or `config.toml` in the working
   directory. When none exists, run `neru config init`. It refuses to
   overwrite without `--force`, so never pass `--force` over a file the user
   did not ask to replace.

   Look for `config.override.toml` beside it. `neru config set` writes
   there, and it wins over the base file on every start, so an edit to the
   base file that "does nothing" is often shadowed by an override.
   `neru config reset <key>` removes one, or delete the file and reload.

3. **Ask what they want, then edit only those sections.** Leave everything
   else at defaults. Read the platform column before writing a key on Linux
   or Windows, and say so when a request needs a key the platform ignores.

   - **A global key.** `[hotkeys]`, with a mode command and its flags as the
     value: `"Primary+Shift+Space" = "hints --action left_click"`. The
     table merges over the defaults. `"<key>" = "__disabled__"` drops one
     default, an empty `[hotkeys]` section drops them all, and `--toggle`
     on any mode makes the key open and close it. Overrides for one app go
     under a top-level `[[app_configs]]` entry with a `bundle_id`.
   - **A key inside a mode.** `[hints.hotkeys]`, `[grid.hotkeys]`, and the
     rest hold the keys that work while the mode is open, such as Shift+L
     for a click or Tab to cycle. Same merging rules.
   - **Hints that miss elements.** Run `neru hints --debug` in the app
     first. A role missing from the sample goes into
     `hints.clickable_roles`, checked against `neru roles --explain`. An
     app whose tree is empty gets `strategy = "vision"` or `"contour"`,
     globally or under `[[hints.app_configs]]` with its `bundle_id`. The
     per-app table also takes `additional_clickable_roles`,
     `ignore_clickable_check`, and `label_direction`, and it is the
     supported way to make one app behave, not a request for a code change.
   - **Sequences and custom modes.** `[macros]` names a list of commands to
     run with `neru macro <name>` or from a hotkey. `[modes]` declares a
     mode with its own hotkey table and indicator. A built-in mode's name
     and the word `mode` are refused as names.
   - **Look and feel.** `[hints.ui]`, `[grid.ui]`, `[theme]`,
     `[mode_indicator]`, `[smooth_cursor]`, `[held_repeat]`, and
     `[sticky_modifiers]`. Colours take `#AARRGGBB` or `#RRGGBB`.

   For a single value, prefer `neru config set <key> <value>` over editing.
   It applies immediately, persists to the override file, and needs the
   daemon. Chain interdependent keys with `--no-reload`, then
   `neru config reload` once. `neru config dump` lists every dotted key.
   The per-app tables cannot be set this way, so those go in the file.

4. **Validate.** `neru config validate` must pass before anything else. It
   runs without the daemon and parses the mode commands inside hotkeys, so a
   mistyped flag is caught here. Read its warnings: a flag the mode does not
   accept, a role the platform cannot name, or a key inert on this platform
   all load and then do nothing, which is the failure the user will report
   later.

5. **Apply it.** With the daemon running, `neru config reload` re-reads the
   file. Nearly everything reloads, including hotkeys, modes, macros, and
   theme. `[systray]` needs a full restart: `neru services restart` for the
   installed service, or quit and `neru launch` otherwise. Then
   `neru config dump | jq '.hints'` or the section in question shows what the
   daemon holds, which confirms the key took effect.

6. **Prove it.** Press the hotkey and watch the mode open. Run the macro.
   Open the app and check the hints appear. Do not declare done on a green
   validate alone.

## What the reference does not make obvious

- **Colours are alpha first.** `#AARRGGBB` or `#RRGGBB`, never `#RRGGBBAA`.
- **Hotkey mistakes split into warn and refuse.** A bad flag, an orphaned
  flag such as `--repeat` without `--action`, or an inert key warns and
  loads. An unknown mode name, a reserved mode name under `[modes]`, or a
  non-ASCII hint character refuses the whole file.
- **A bad config is rejected whole on reload.** The previous config stays in
  place and the failure is logged. Validate before reloading so the user
  never runs on a stale config without knowing.
- **There is no log file by default.** `logging.disable_file_logging` is
  `true`. To read why a reload failed or a hotkey did nothing, set it to
  `false`, restart, and read `~/Library/Logs/neru/app.log` on macOS,
  `~/.local/state/neru/log/app.log` on Linux, or
  `%LOCALAPPDATA%\neru\log\app.log` on Windows. `log_level = "debug"` shows
  per-keypress detail.
- **Linux has no default global hotkeys.** A fresh Linux config binds
  nothing on purpose, to avoid colliding with the compositor. Either write
  `[hotkeys]` or bind `neru hints` and friends in the compositor's keymap.
- **`bundle_id` works on every platform.** Per-app tables match a bundle
  identifier on macOS, a window class on Linux, and an executable name on
  Windows, under the same key. The reference names how to find each.
- **Runtime toggles are not config.** `neru toggle-scroll-invert` and its
  siblings last until restart. The lasting version is the matching key.
- **A stale Accessibility grant looks like a broken hotkey.** After an
  update on macOS, remove Neru from the Accessibility list and add it back
  before touching the config.

## Installing the service

When the user wants Neru at login, `neru services install` writes a launchd
agent on macOS, a systemd user unit on Linux, or a Task Scheduler task on
Windows, and starts it. `neru services status` confirms. It refuses when
nix-darwin or home-manager already manages Neru, and those users configure
the service in their Nix module instead.
