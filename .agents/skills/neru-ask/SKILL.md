---
name: neru-ask
description: "Answer a Neru user's question about what Neru does, which command, flag, or config key does a thing, why something does not work, or what to try next, from the help, man pages, and docs of their installed version rather than from memory. Routes config work to neru-setup-config. Use when a Neru user asks what Neru can do, how to do something with it, which command to run, why hints or a hotkey do nothing, or which skill to use."
---

# Answering questions about Neru

Neru is a keyboard-driven mouse replacement. A daemon watches global hotkeys
and draws overlays, and a CLI sends it commands over a socket. Modes take the
keyboard until a target is picked: hints label clickable elements, grid and
recursive grid divide the screen, scroll moves content, monitor select jumps
between displays. Actions click, scroll, and move the cursor without a mode.
Every answer should come from the installed version, since commands, flags,
and keys change between releases.

## What every install has

Check these before anything remote. The install script, Homebrew, Nix, and a
source build all ship them.

- `neru --help`, then `neru <command> --help`. The help lists the flags and
  accepted values of the installed version.
- `man neru` on macOS and Linux, and one page per subcommand such as
  `man neru-hints`, `man neru-config-set`, and `man neru-action-left_click`.
  `apropos neru` lists them. Windows has no `man`, and `--help` carries the
  same text.
- `neru status` for whether the daemon runs and which mode is open.
- `neru doctor` for config validity, socket health, permissions, and which
  capabilities this platform has. It runs without the daemon.
- `neru hints --debug` for the elements hints would label in the focused
  window, as a count and a sample, without drawing anything. Run it
  first for any "hints show nothing in app X" report.
- `neru config dump` for the config in force, defaults filled in.
- `neru roles --explain` for the clickable roles and how each resolves here.
- `neru docs cli` and `neru docs config` open the two references in a
  browser at the installed version.

For anything the help does not cover, fetch the doc at the installed version:

```bash
tag=$(neru --version | sed -n '1s/^Neru version //p' | cut -d- -f1)
case $tag in v*.*.*) ;; *) tag=main ;; esac
curl -fsSL "https://raw.githubusercontent.com/y3owk1n/neru/$tag/docs/CLI.md"
```

A release build prints its tag. A dev build prints `v1.2.3-14-gabcdef`,
which the `cut` maps to the release it was built from. On Windows, where
`sed` and `cut` are usually absent, the same fetch in PowerShell:

```powershell
$tag = (((neru --version)[0] -replace '^Neru version ', '') -split '-')[0]
if ($tag -notmatch '^v\d+\.\d+\.\d+$') { $tag = 'main' }
Invoke-RestMethod "https://raw.githubusercontent.com/y3owk1n/neru/$tag/docs/CLI.md"
```

The docs are `CLI.md` for every command, flag, and the IPC protocol,
`CONFIGURATION.md` for every key with its default and platform column,
`TIPS_TRICKS.md` for worked recipes such as Vimium-style click on select,
drag with any button, cycling modes on one key, and driving Neru from skhd,
`TROUBLESHOOTING.md` when something does not work, `INSTALLATION.md` for
install methods and login services, `CROSS_PLATFORM.md` for what each
platform supports, and `LINUX_SETUP.md` plus `LINUX_DESKTOPS.md` for Linux
permissions and per-compositor notes.

## What Neru does

| Ask | Answer with |
| --- | --- |
| Click something on screen by keyboard | `neru hints --action left_click`, or `right_click`, `middle_click` |
| Click where hints find nothing | `neru grid`, `neru recursive_grid` or `neru bisect`, or `neru hints --strategy vision` |
| Filter hints by typing | `neru hints --search`, or `/` inside hints |
| Scroll without a mouse | `neru scroll`, or `neru action scroll_down --steps N` |
| Move the cursor to a spot, display, or grid cell | `neru action move_mouse`, `move_monitor`, `move_cell` |
| Nudge the cursor and keep it moving while held | `neru action move_mouse_relative`, glide under `[held_repeat]` |
| Drag | `left_click --state down`, move, `left_click --state up` |
| Jump between monitors | `neru monitor_select` |
| Type text or press keys | `neru action feed` |
| Click without moving the real cursor | `save_cursor_pos`, then the click, then `restore_cursor_pos` |
| Chain steps as one unit | `neru run "..." "..."`, or `[macros]` in config, see `neru-setup-config` |
| Bind a key to any of the above | `[hotkeys]` in config, see `neru-setup-config` |
| Change hint letters, colours, grid size | `neru config set <key> <value>`, see `neru-setup-config` |
| Make one app behave differently | `[[hints.app_configs]]` by bundle id, see `neru-setup-config` |
| Hide overlays while screen sharing | `neru toggle-screen-share` |
| Run Neru at login | `neru services install`, see `neru-setup-config` |
| Pause Neru without quitting | `neru stop`, then `neru start` |

Things a user often does not know:

- **Hints have three strategies.** `axtree` reads the accessibility tree
  and is the default. `vision` recognises text on a screen capture, and on
  macOS rectangles too. `contour` finds edges on every platform. An app
  with a poor tree, such as an Electron app or a game, gets `--strategy
  vision` for one call or a per-app strategy in config.
- **Grid modes need no accessibility tree.** When hints show nothing in an
  app, grid and recursive grid still work, because they divide the screen
  rather than read elements.
- **Hints only label roles in `hints.clickable_roles`.** A missing hint is
  usually a role outside that list. `neru hints --debug` shows what was
  collected, `neru roles --explain` shows what the platform can name, and
  `neru hints --role` overrides the list for one call.
- **Inside a mode the keys are bindable too.** Each mode has its own
  `[<mode>.hotkeys]` table, so Shift+L for a left click inside hints, Tab to
  cycle, and the arrow keys to nudge are defaults, not fixed.
- **Linux ships no default global hotkeys.** The modes exist but nothing is
  bound until `[hotkeys]` or the compositor binds them. That is by design.
- **One config works on every platform.** A key, flag, or action the current
  platform cannot act on loads with a warning, listed under `platform_support`
  in `neru doctor`. It is not an error.
- **Runtime toggles do not persist.** `toggle-scroll-invert`,
  `toggle-cursor-follow-selection`, and `toggle-screen-share` last until the
  daemon restarts. The lasting form is the config key.
- **The CLI needs the daemon for most things.** Modes, actions, `config set`,
  `config dump`, and `status` all talk to it. `config init`, `config validate`,
  `doctor`, and `roles` run without it.
- **There is no log file by default.** `logging.disable_file_logging` is
  `true` out of the box. Before telling a user to read the log, have them
  set it to `false` and restart, then read `~/Library/Logs/neru/app.log`
  on macOS, `~/.local/state/neru/log/app.log` on Linux, or
  `%LOCALAPPDATA%\neru\log\app.log` on Windows.

## Routing

- Config, hotkeys, macros, modes, per-app overrides, or service work goes to
  `neru-setup-config`.
- A question with a one-command answer gets the command and the help page
  that documents it.
- When something does not work, in this order: `neru status`, `neru doctor`,
  then `neru hints --debug` for a hints report or `neru config validate` for
  a hotkey report, then the Troubleshooting doc as above, before guessing at
  a cause. Its Permissions and Hotkeys Not Working sections cover most
  reports. On macOS, a hotkey that never fires after an update is usually a
  stale Accessibility grant: remove Neru from the list and add it again.
- A bug or a missing feature goes to a GitHub issue on `y3owk1n/neru`.
  Blank issues are disabled, so use the issue forms, and include
  `neru --version` and `neru doctor` output. Questions and config help
  belong in Discussions.

Do not answer flags or key names from memory. Run `--help` or read the man
page first.
