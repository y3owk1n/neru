# Troubleshooting

Linux host problems are in [Linux setup](./linux.md#troubleshooting), and
desktop-specific ones in [Linux desktops](./linux-desktops.md).

## Quick diagnosis

```bash
neru status   # is the daemon running?
neru doctor   # per-component health, works even if the daemon is down
neru hints    # does a mode open from the CLI?
```

If the CLI cannot reach the daemon, start it with `neru launch`. For more
detail, turn on [file logging](#log-file-locations) and
[debug logging](#enable-debug-logging).

## Restart the daemon

`neru stop` only pauses Neru and leaves the process running. To restart it:

```bash
neru services restart                # if you installed the login service
pkill neru && neru launch            # macOS, Linux
taskkill /IM neru.exe; neru launch   # Windows PowerShell
```

## Log file locations

File logging is off by default, and the daemon logs to the terminal that
started it. To write a log file, set `disable_file_logging = false` under
`[logging]` and [restart the daemon](#restart-the-daemon).
`neru config reload` does not apply logging settings.

The file goes to `[logging].log_file` when set, otherwise to
`~/Library/Logs/neru/app.log` on macOS, `~/.local/state/neru/log/app.log` on
Linux, and `%LOCALAPPDATA%\neru\log\app.log` on Windows.

File logs are JSON lines, for example `grep ERROR ~/Library/Logs/neru/app.log`.
Rotation is in the [logging reference](../reference/configuration.md#logging).
To start a fresh log, delete the file and restart the daemon.

## Enable debug logging

Key routing, overlay redraws and hint filtering are logged only at `debug`.
Set `log_level = "debug"` under `[logging]` and
[restart the daemon](#restart-the-daemon). Set it back to `"info"` afterwards,
because debug logging slows hint activation.

### Common log messages

| Message                                             | Meaning                                                               |
| --------------------------------------------------- | --------------------------------------------------------------------- |
| `Found usable accessibility tree`                   | Accessibility tree detected, AX support activated (macOS)             |
| `Hints mode activated`                              | The hint overlay is active, with the hint count when available        |
| `Clickable element collection was slow`             | Accessibility scanning finished but took longer than expected         |
| `Failed to get clickable elements`                  | The accessibility query failed. Check permissions and `excluded_apps` |
| `Secure input is enabled, blocking mode activation` | macOS secure input is on, often because a password field is focused  |

## Installation & setup

**"Cannot open Neru because the developer cannot be verified"** (macOS). Remove
the quarantine flag with `xattr -cr /Applications/Neru.app`, then run
`open -a Neru`.

**"Command not found: neru"** (macOS, Linux). The install directory is not on
your `PATH`. The install script prints which directory it used. Add it in your
shell's rc file, for example `export PATH="$HOME/.local/bin:$PATH"`.

**Homebrew fails** (macOS). Run `brew update && brew reinstall --cask neru`.

**"profile.ps1 cannot be loaded because running scripts is disabled"**
(Windows). The installer added tab completion to your PowerShell profile, and
the default `Restricted` policy blocks it. The installer offers this fix before
it writes the profile. Run
`Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned` and open
a new window. To drop the completion instead, delete the two lines under
`# neru shell completion (managed by install.ps1)` in the profile.

**"error while loading shared libraries"** (Linux). A runtime library is
missing, or Fedora names tesseract differently. See
[Linux setup](./linux.md#error-while-loading-shared-libraries-libtesseractso5).

## Permissions

### macOS Accessibility permission

Neru does nothing on macOS without it. Grant it in **System Settings >
Privacy & Security > Accessibility**. If `neru doctor` still reports an error,
which often happens after an upgrade, remove Neru from the list, add it again,
and [restart the daemon](#restart-the-daemon). Other platforms' permissions are
in [Getting started](./getting-started.md#permissions).

## Hints & grids

### No hints or grids appear

Run the [quick diagnosis](#quick-diagnosis) and check
[permissions](#permissions). Remove the app from `excluded_apps`. Try another
app to tell an app problem from a Neru problem.

### No hints visible when using multiple monitors

Neru draws hints only on the display the cursor is on. Bind
`"Primary+Shift+Space" = ["action move_mouse --window", "hints"]` under
`[hotkeys]` to move the cursor to the focused window first, or use a window
manager that makes the cursor follow focus.

### Hints not showing in browsers and Electron apps (macOS)

Neru detects Chromium, Firefox, WebKit and Electron from the app bundle,
including Chrome PWAs as `chromium` and Safari PWAs as `webkit`. With
[file logging](#log-file-locations) on, run
`grep "Detected non empty bundle type" ~/Library/Logs/neru/app.log`. If your
browser is missing, open an issue with its bundle ID from
`osascript -e 'id of app "Your Browser"'`. On Linux, Chromium and Electron apps
need `--force-renderer-accessibility`
([Known limitations](./linux.md#known-limitations)).

### Some elements that should have hints don't have hints

Neru sometimes misses small elements, so open an issue for one. Check the roles
with `neru roles --explain`, and list the full vocabulary with `neru roles`. If
you customized `hints.clickable_roles`, restore it from
[default-config.toml](https://github.com/y3owk1n/neru/blob/main/configs/default-config.toml)
and run `neru config reload`.

### Certain hints don't visually match any on-screen UI

Neru may be labelling elements you cannot see, such as `row` and `cell`. Copy
the full `clickable_roles` list from
[default-config.toml](https://github.com/y3owk1n/neru/blob/main/configs/default-config.toml),
remove those roles, and run `neru config reload`.

### Hints or grids appear but are misaligned

This is a bug. Turn on [file logging](#log-file-locations) and
[debug logging](#enable-debug-logging), reproduce it, and open an issue with
the log, a screenshot, your OS version, and the app name and version. On
Wayland, native-app hints depend on a per-compositor window-origin source. See
[Platform support](../reference/platform-support.md#accessibility-and-hints).

### No hints in menubar or Dock (macOS)

Both are off by default. Under `[hints]`, set `include_menubar_hints = true`
and `include_dock_hints = true`. Add menubar apps by bundle ID to
`additional_menubar_hints_targets`, such as `"com.apple.controlcenter"`.

## Hotkeys not working

### Hotkey does nothing

1. Run the mode from the CLI, such as `neru hints`. If it opens, the hotkey is
   at fault.
2. Check the daemon is running with `neru status`.
3. Check the app is not in `excluded_apps`.
4. Check the binding against the
   [hotkeys reference](../reference/configuration.md#global-hotkeys) and run
   `neru config validate`.
5. Try another key combination, in case another app owns this one.

On Linux, Neru ships no default global hotkeys. See
[Binding your first hotkeys](./getting-started.md#binding-your-first-hotkeys).
On Wayland, `[hotkeys]` also needs the `input` group. See
[Global hotkeys on Wayland](./linux-desktops.md#global-hotkeys-on-wayland).

### Hotkey works in some apps but not others

The app is in `[general].excluded_apps`, so remove it. Find a macOS bundle ID
with `osascript -e 'id of app "AppName"'`. On Linux the app is named by
`WM_CLASS` or `app_id`. See
[App identity](../reference/configuration.md#app-identity-across-platforms-bundle_id).

### Hotkey conflicts with system shortcuts

Change the system shortcut in **System Settings > Keyboard > Keyboard
Shortcuts** on macOS, or move Neru's binding. Under `[hotkeys]`, set
`"Primary+Shift+Space" = "__disabled__"` and bind another chord, such as
`"Ctrl+Alt+Space" = "hints"`. To drive Neru from an external hotkey manager
such as skhd, see [Disabling all built-in hotkeys](./recipes.md#disabling-all-built-in-hotkeys).

## Performance issues

### Hints appear slowly

Likely causes, in order: too many entries in `hints.clickable_roles`,
[debug logging](#enable-debug-logging) left on, or a system under load.

### High CPU usage

Check with `top -pid $(pgrep neru)` on macOS, `top -p $(pgrep neru)` on Linux,
or Task Manager on Windows. Look for errors in the [log](#log-file-locations)
and [restart the daemon](#restart-the-daemon). If it recurs, open an issue
with the log.

## Daemon issues

### CLI cannot reach the daemon

Run `neru doctor`, then `neru launch`, then `neru status`. If it still fails on
macOS or Linux, remove the stale socket. The daemon prints its endpoint at
startup, and only one of these paths exists:

```bash
rm -f "${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"/neru/neru.sock
rm -f "${TMPDIR:-/tmp}"/neru-"$(id -u)"/neru.sock
neru launch
```

### Daemon crashes on startup

Usually a config error, and `neru config validate` names the offending key. To
rule the config out, move `config.toml` aside and run `neru launch` in a
terminal. It runs on built-in defaults and prints why it exits.

### Version mismatch after an upgrade

The running daemon is from the old binary. [Restart the daemon](#restart-the-daemon).

### Daemon stops responding or won't quit

Run `pkill -9 neru && neru launch` on macOS or Linux, or
`taskkill /IM neru.exe /F; neru launch` on Windows. If `neru launch` still
fails, clear the stale socket as in
[CLI cannot reach the daemon](#cli-cannot-reach-the-daemon).

## App-specific issues

### Adobe apps: hints misaligned or missing (macOS)

Add roles for the app. Find the bundle ID with
`osascript -e 'id of app "Adobe Illustrator"'`.

```toml
[[hints.app_configs]]
bundle_id = "com.adobe.illustrator"
additional_clickable_roles = ["static_text", "image"]
ignore_clickable_check = true
```

### Mission Control: no hints (macOS)

The Dock draws Mission Control, so set `include_dock_hints = true` and
`detect_mission_control = true` under `[hints]`.

### Accessibility Zoom: cursor lands in the wrong place (macOS)

With **System Settings > Accessibility > Zoom** zoomed in, Neru places the
cursor exactly and pans the zoomed view to the target. If the cursor misses or
the view does not follow, open an issue with your macOS version and zoom
factor.

## Keyboard layout issues

### Wrong characters produced when typing

Neru translates keycodes through your OS keyboard layout. Check the layout is
selected in your OS. Some custom layouts are not resolved automatically, so
copy the one you want from the `keyboard_layouts` row of `neru doctor` into
`[general].kb_layout_to_use`. Examples are `"com.apple.keylayout.Colemak"` on
macOS, `"English (Colemak)"` on Linux, and `"00010409"` for Dvorak on Windows.
Neru re-registers global hotkeys after a layout switch. Windows checks once a
second, so a punctuation hotkey can lag a switch by up to a second. If hotkeys
still fail, [restart the daemon](#restart-the-daemon).

### Input methods not working (CJK IME)

Neru works with CJK input methods such as Pinyin and Wubi. If one misbehaves,
check it is installed and active in your OS, and on macOS that Neru has
[Accessibility permission](#permissions).

## Configuration issues

### Config changes not taking effect

The daemon does not watch the file. Run `neru config validate`, then
`neru config reload`. A refused reload rejects the whole file and keeps the
previous config. `neru status --json | jq -r .config` shows which file the
daemon reads.

`neru config set` writes `config.override.toml`, which wins over your config.
`neru config reset <key>` removes one value. See
[Config layering](./getting-started.md#config-layering).

### "Failed to parse config"

A TOML syntax error or a refused value. `neru config validate` prints the line
or key and the reason. Common causes are a key containing `+` without quotes,
a section header typo, a color without its leading `#`, and a hotkey bound to
an empty string. Use `__disabled__` to remove a binding.

## Getting help

Search <https://github.com/y3owk1n/neru/issues>, then open an issue with the
bug-report form. It asks for `neru doctor` output, your OS version,
`neru --version`, the app, the relevant config, and the [log](#log-file-locations).

## Emergency reset

Force Neru to quit as in
[Daemon stops responding](#daemon-stops-responding-or-wont-quit), remove it
with [Uninstallation](./installation.md#uninstallation), reinstall, run
`neru launch`, and on macOS grant Accessibility permission again.
