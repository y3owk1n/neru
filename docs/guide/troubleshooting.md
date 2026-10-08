# Troubleshooting

Fixes for common Neru problems, listed by symptom. Each entry gives the cause
and the fix. Linux host problems are in
[Linux setup](linux.md#troubleshooting), and desktop-specific ones in
[Linux desktops](linux-desktops.md).

## Quick diagnosis

Run these three first. Most problems show up in one of them.

```bash
neru status   # is the daemon running?
neru doctor   # per-component health, works even if the daemon is down
neru hints    # does a mode open from the CLI?
```

If the CLI cannot reach the daemon, start it with `neru launch`. For more
detail, see [Collecting diagnostics](#collecting-diagnostics).

## Installing

### "Cannot open Neru because the developer cannot be verified"

macOS quarantined the download. Remove the flag, then open Neru:

```bash
xattr -cr /Applications/Neru.app
open -a Neru
```

### "command not found: neru"

The install directory is not on your `PATH`. The install script prints which
directory it used. Add it in your shell's rc file, for example
`export PATH="$HOME/.local/bin:$PATH"`.

### Homebrew install or upgrade fails

Run `brew update && brew reinstall --cask neru`.

### "profile.ps1 cannot be loaded because running scripts is disabled"

Windows blocks the tab completion the installer added to your PowerShell
profile. Allow signed scripts for your user, then open a new window:

```powershell
Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned
```

To drop the completion instead, delete the two lines under
`# neru shell completion (managed by install.ps1)` in the profile.

### "error while loading shared libraries"

On Linux a runtime library is missing, or Fedora names tesseract differently.
See [Linux setup](linux.md#error-while-loading-shared-libraries-libtesseractso5).

## Permissions

### Neru does nothing on macOS

Neru needs Accessibility permission, see
[Grant permissions](getting-started.md#macos). If `neru doctor` still reports
an error, which often happens after an upgrade, remove Neru from the
Accessibility list, add it again, and [restart the daemon](#restart-the-daemon).

### Modes do not open while a password field is focused

macOS secure input is on, and it blocks every app from reading keys,
including Neru. Click out of the password field and try again.

## Hints and grids

### No hints or grids appear

1. Run the [quick diagnosis](#quick-diagnosis).
2. Check the [permissions](#neru-does-nothing-on-macos).
3. Check the app is not in `excluded_apps`.
4. Try another app, to tell an app problem from a Neru problem.

### No hints on my other monitor

Neru draws hints only on the display the cursor is on. Move the cursor to the
focused window first:

```toml
[hotkeys]
"Primary+Shift+Space" = ["action move_mouse --window", "hints"]
```

Or use a window manager that makes the cursor follow focus.

### No hints in a browser or Electron app

- **macOS:** Neru detects Chromium, Firefox, WebKit and Electron from the app
  bundle. With [file logging](#log-file-locations) on, run
  `grep "Detected non empty bundle type" ~/Library/Logs/neru/app.log`. If your
  browser is missing, open an issue with its
  [bundle ID](../reference/configuration.md#app-identity-across-platforms-bundle_id).
- **Linux:** start Chromium and Electron apps with
  `--force-renderer-accessibility`.

### An element I can see has no hint

Run `neru roles --explain` to see which roles your config hints. If you changed
`hints.clickable_roles`, restore it from the
[default config](https://github.com/y3owk1n/neru/blob/main/configs/default-config.toml)
and run `neru config reload`. If the default misses it too, open an issue.

### Hints appear on things I cannot see

Neru may be hinting invisible elements such as `row` and `cell`. Copy the
`clickable_roles` list from the
[default config](https://github.com/y3owk1n/neru/blob/main/configs/default-config.toml),
remove those roles, and run `neru config reload`.

### Hints or grids are misaligned

On Wayland, hints in native apps depend on the compositor reporting window
positions, and some do not. See
[Linux desktops](linux-desktops.md#wlroots-compositors).

Anywhere else this is a bug. Turn on [file logging](#log-file-locations) and
[debug logging](#enable-debug-logging), reproduce it, and open an issue with
the log, a screenshot, your OS version, and the app name and version.

### No hints in the menu bar or Dock

Both are off by default on macOS. Under `[hints]`, set
`include_menubar_hints = true` and `include_dock_hints = true`. Add menu bar
apps by bundle ID to `additional_menubar_hints_targets`, such as
`"com.apple.controlcenter"`.

## Hotkeys

### A hotkey does nothing

1. Run the mode from the CLI, such as `neru hints`. If it opens, the hotkey is
   at fault.
2. Check the daemon is running with `neru status`.
3. Check the app is not in `excluded_apps`.
4. Run `neru config validate`, and check the binding against the
   [hotkey syntax](../reference/configuration.md#global-hotkeys).
5. Try another chord, in case another app owns this one.

On Linux, see [Bind your first hotkey](getting-started.md#bind-your-first-hotkey)
first.

### A hotkey works in some apps but not others

The app is in `[general].excluded_apps`, so remove it. Entries name apps by
[app identity](../reference/configuration.md#app-identity-across-platforms-bundle_id).

### A hotkey clashes with a system shortcut

Change the system shortcut, in **System Settings > Keyboard > Keyboard
Shortcuts** on macOS, or move Neru's binding:

```toml
[hotkeys]
"Primary+Shift+Space" = "__disabled__"
"Ctrl+Alt+Space" = "hints"
```

To drive Neru from another hotkey tool such as skhd, see
[Recipes](recipes.md#leave-global-hotkeys-to-another-tool).

### Keys type the wrong characters

Neru reads keys through your OS keyboard layout. Check the layout is selected
in your OS. Some custom layouts are not resolved automatically, so set
[`kb_layout_to_use`](../reference/configuration.md#general) to the one you want,
copied from the `keyboard_layouts` row of `neru doctor`.

Neru re-registers global hotkeys after a layout switch. On Windows this can
take up to a second. If hotkeys still fail,
[restart the daemon](#restart-the-daemon).

### A CJK input method misbehaves

Neru works with input methods such as Pinyin and Wubi. Check the method is
installed and active in your OS, and on macOS that Neru has
[Accessibility permission](#neru-does-nothing-on-macos).

## Performance

### Hints are slow to appear

The usual causes, most likely first:

1. Too many entries in `hints.clickable_roles`.
2. [Debug logging](#enable-debug-logging) left on.
3. A system under load.

### Neru uses a lot of CPU

Check with `top -pid $(pgrep neru)` on macOS, `top -p $(pgrep neru)` on Linux,
or Task Manager on Windows. Look for errors in the [log](#log-file-locations)
and [restart the daemon](#restart-the-daemon). If it happens again, open an
issue with the log.

## The daemon

### The CLI cannot reach the daemon

Run `neru doctor`, then `neru launch`, then `neru status`. If it still fails on
macOS or Linux, a stale socket file is in the way. Delete the file at the
[endpoint](../reference/ipc.md#endpoint) path, which the daemon also prints at
startup, then run `neru launch` again.

### The daemon exits on startup

Usually a config error, and `neru config validate` names the key. To rule the
config out, move `config.toml` aside and run `neru launch` in a terminal. It
runs on built-in defaults and prints why it exits.

### "version mismatch" after an upgrade

The running daemon is still the old binary.
[Restart the daemon](#restart-the-daemon).

### The daemon stops responding or will not quit

Force it to quit and start it again:

```bash
pkill -9 neru && neru launch                  # macOS, Linux
taskkill /IM neru.exe /F; neru launch         # Windows PowerShell
```

If `neru launch` still fails, clear the stale socket as in
[The CLI cannot reach the daemon](#the-cli-cannot-reach-the-daemon).

## Config

### My config changes do nothing

Neru does not watch the file, see
[Apply your changes](configuring.md#apply-your-changes). Run
`neru config validate`, then `neru config reload`. If the file has one error,
Neru rejects the whole reload and keeps the previous config. To check which
file the daemon read, see
[Where Neru looks for the file](configuring.md#where-neru-looks-for-the-file).

A value set with `neru config set` wins over your file, see
[How the layers combine](configuring.md#how-the-layers-combine).
`neru config reset <key>` removes it.

### "Failed to parse config"

A TOML syntax error or a refused value. `neru config validate` prints the line
or key and the reason. Common causes:

- a key containing `+` without quotes
- a typo in a section header
- a color without its leading `#`
- a hotkey bound to an empty string, where `__disabled__` was meant

## Specific apps

### Hints are misaligned or missing in Adobe apps (macOS)

Add roles for the app, named by its
[bundle ID](../reference/configuration.md#app-identity-across-platforms-bundle_id).

```toml
[[hints.app_configs]]
bundle_id = "com.adobe.illustrator"
additional_clickable_roles = ["static_text", "image"]
ignore_clickable_check = true
```

### No hints in Mission Control (macOS)

Dock hints include Mission Control. Set `include_dock_hints = true` and
`detect_mission_control = true` under `[hints]` to get hints on the current
desktop's windows. The desktops in the Spaces bar get hints once the bar is
expanded, so move the pointer to the top edge before showing hints.

### The cursor lands in the wrong place under Accessibility Zoom (macOS)

With **System Settings > Accessibility > Zoom** zoomed in, Neru places the
cursor exactly and pans the zoomed view to the target. If the cursor misses or
the view does not follow, open an issue with your macOS version and zoom
factor.

## Collecting diagnostics

How to restart Neru, where its log is, and how to make the log say more. Use
these for any problem above, or before filing a bug.

### Restart the daemon

`neru stop` only pauses Neru and leaves the process running. To restart it:

```bash
neru services restart                # if you installed the login service
pkill neru && neru launch            # macOS, Linux
taskkill /IM neru.exe; neru launch   # Windows PowerShell
```

### Log file locations

By default the daemon logs only to the terminal that started it. To write a
log file, set `disable_file_logging = false` under `[logging]` and
[restart the daemon](#restart-the-daemon).

| Platform | Log file                              |
| -------- | ------------------------------------- |
| macOS    | `~/Library/Logs/neru/app.log`         |
| Linux    | `~/.local/state/neru/log/app.log`     |
| Windows  | `%LOCALAPPDATA%\neru\log\app.log`     |

`[logging].log_file` overrides the path. Each line is JSON, so
`grep ERROR ~/Library/Logs/neru/app.log` finds failures Neru could not recover
from, and `grep WARN` finds ones it worked around. Rotation is in the
[logging reference](../reference/configuration.md#logging). To start a fresh
log, delete the file and restart the daemon.

### Enable debug logging

Neru logs key routing, overlay redraws and hint filtering only at `debug`.
Set `log_level = "debug"` under `[logging]` and
[restart the daemon](#restart-the-daemon). Set it back to `"info"` afterwards,
because debug logging slows hint activation.

| Log message                             | Meaning                                                                                                                  |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `Found usable accessibility tree`       | The app's accessibility tree was found (macOS)                                                                           |
| `Hints mode activated`                  | The hint overlay is up, with the hint count when available                                                               |
| `Clickable element collection was slow` | Reading the accessibility tree took longer than expected                                                                 |
| `Failed to show hints`                  | Collecting elements or drawing hints failed. Check permissions and `excluded_apps`                                       |
| `Mode activation refused`               | A mode did not start. The `error` field says why: secure input, an excluded app, the mode disabled, or Neru stopped |

## Getting help

Search the [issues](https://github.com/y3owk1n/neru/issues), then open one
with the bug-report form. It asks for `neru doctor` output, your OS version,
`neru --version`, the app, the relevant config, and the
[log](#log-file-locations).

## Emergency reset

1. Force Neru to quit, as in
   [The daemon stops responding](#the-daemon-stops-responding-or-will-not-quit).
2. Remove it, see [Uninstallation](installation.md#uninstallation).
3. Reinstall and run `neru launch`.
4. On macOS, grant Accessibility permission again.
