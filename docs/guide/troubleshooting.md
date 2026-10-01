# Troubleshooting

Symptoms, causes, and fixes for common Neru problems. Steps that only apply to
one platform are labelled. Linux setup problems, such as libraries, the `input` group,
`/dev/uinput` and the systemd service, are in
[Linux setup](./linux.md#troubleshooting), and problems specific to one Linux
desktop are in [Linux desktops](./linux-desktops.md).

## Quick diagnosis

Check these first:

```bash
neru status   # is the daemon running?
neru doctor   # per-component health, works even if the daemon is down
neru hints    # does a mode open from the CLI?
```

| Symptom                              | First step                                                     |
| ------------------------------------ | -------------------------------------------------------------- |
| The CLI cannot reach the daemon      | Start it with `neru launch`                                    |
| `neru doctor` reports `accessibility` denied (macOS) | Grant [Accessibility permission](#permissions) |
| No hints appear                      | See [No hints or grids appear](#no-hints-or-grids-appear)      |
| A hotkey does nothing                | See [Hotkeys not working](#hotkeys-not-working)                |

When you need more detail, [turn on file logging](#log-file-locations) and
[debug logging](#enable-debug-logging).

---

## Restart the daemon

Many fixes below end with a restart. `neru stop` only pauses Neru. The process
keeps running, so restart it one of these ways:

```bash
neru services restart            # if you installed the login service

pkill neru && neru launch        # macOS, Linux
```

```powershell
taskkill /IM neru.exe; neru launch   # Windows
```

---

## Log file locations

File logging is **off by default**. The daemon logs to the terminal it was
started from, so running `neru launch` in a terminal shows its output there. To
write a log file, turn it on in `config.toml` and
[restart the daemon](#restart-the-daemon). The daemon reads logging settings
at startup, not on `neru config reload`.

```toml
[logging]
disable_file_logging = false
```

The file goes to `[logging].log_file` when set, otherwise to:

| Platform | Path                                     |
| -------- | ---------------------------------------- |
| macOS    | `~/Library/Logs/neru/app.log`            |
| Linux    | `~/.local/state/neru/log/app.log`        |
| Windows  | `%LOCALAPPDATA%\neru\log\app.log`        |

File logs are JSON lines. Rotation and the other options are in the
[logging reference](../reference/configuration.md#logging). Reading the log,
using the macOS path as the example:

```bash
tail -f ~/Library/Logs/neru/app.log           # follow
grep ERROR ~/Library/Logs/neru/app.log        # errors only
```

To start a fresh log, delete the file and restart the daemon.

---

## Enable debug logging

The default `info` level logs lifecycle, configuration and mode activation.
Key routing, overlay redraws and hint filtering are logged only at `debug`. To
investigate one of those, set:

```toml
[logging]
log_level = "debug"
```

and [restart the daemon](#restart-the-daemon). Set it back to `"info"`
afterwards, since debug logging slows hint activation.

### Common log messages

| Message                                      | Meaning                                                                       |
| -------------------------------------------- | ----------------------------------------------------------------------------- |
| `Found usable accessibility tree`            | Accessibility tree detected, AX support activated (macOS)                     |
| `Hints mode activated`                       | The hint overlay is active, with the hint count when available               |
| `Clickable element collection was slow`      | Accessibility scanning finished but took longer than expected                 |
| `Failed to get clickable elements`           | The accessibility query failed. Check permissions and `excluded_apps`         |
| `Secure input is enabled, blocking mode activation` | macOS secure input is on, often because a password field is focused   |

---

## Installation & setup

**"Cannot open Neru because the developer cannot be verified"** (macOS)

```bash
xattr -cr /Applications/Neru.app  # Remove quarantine
open -a Neru
```

**"Command not found: neru"** (macOS, Linux)

The directory holding `neru` is not on your `PATH`. The install script prints
which directory it used. Add it in your shell's rc file, for example:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

**Homebrew fails** (macOS)

```bash
brew update && brew reinstall --cask neru
```

**"profile.ps1 cannot be loaded because running scripts is disabled"** (Windows)

The installer added tab completion to your PowerShell profile, and the default
`Restricted` execution policy blocks profiles along with every other script.
Allow local scripts for your user account, then open a new window:

```powershell
Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned
```

The installer checks the policy first and offers this before writing the
profile. To drop the completion instead, delete the two lines under
`# neru shell completion (managed by install.ps1)` in the profile.

**"error while loading shared libraries"** (Linux)

A runtime library is missing, or on Fedora tesseract has a different name. See
[Linux setup](./linux.md#error-while-loading-shared-libraries-libtesseractso5).

---

## Permissions

What each platform needs is listed in [Getting started](./getting-started.md#permissions).
Linux permission problems are in [Linux setup](./linux.md#troubleshooting).

### macOS Accessibility permission

Neru does nothing without it. Grant it in **System Settings > Privacy &
Security > Accessibility** by adding Neru and enabling its checkbox.

If it is granted but Neru still cannot use it, which often happens after an
upgrade:

1. Remove Neru from the list.
2. Add it again.
3. [Restart the daemon](#restart-the-daemon).

`neru doctor` shows `accessibility: ok` when the permission works, and the
specific error when it does not.

---

## Hints & grids

### No hints or grids appear

Run the [quick diagnosis](#quick-diagnosis), then:

- Start the daemon with `neru launch` if it is not running.
- Check [permissions](#permissions).
- Remove the app from `excluded_apps` in your config.
- Try a different app, to tell an app problem from a Neru problem.

### No hints visible when using multiple monitors

Neru draws hints only on the display the cursor is on. If the focused window is
on another display, no hints show for it.

Make the hints hotkey move the cursor to the focused window first:

```toml
[hotkeys]
"Primary+Shift+Space" = ["action move_mouse --window", "hints"]
```

A window manager that makes the cursor follow focus avoids this.

### Hints not showing in browsers and Electron apps (macOS)

Neru detects the rendering engine from the app bundle, with no configuration
needed. It recognizes Chromium, Firefox, WebKit and Electron. Installed web apps are detected
too, Chrome PWAs as `chromium` and Safari PWAs as `webkit`. If a browser shows
no hints, check what was detected with
[file logging](#log-file-locations) on:

```bash
grep "Detected non empty bundle type" ~/Library/Logs/neru/app.log
```

If your browser is not in the log, detection returned nothing. Open an issue at
[github.com/y3owk1n/neru](https://github.com/y3owk1n/neru) with the bundle ID:

```bash
osascript -e 'id of app "Your Browser"'
```

On Linux, Chromium and Electron apps need `--force-renderer-accessibility`.
See [Known limitations](./linux.md#known-limitations).

### Some elements that should have hints don't have hints

Small elements are sometimes missed. If you hit one, open an issue.

Also make sure the relevant roles are enabled. `neru roles --explain` shows
which roles your config selects on this platform, and `neru roles`
lists the full vocabulary. If you customized `hints.clickable_roles`, remove the
customization or restore it from
[default-config.toml](https://github.com/y3owk1n/neru/blob/main/configs/default-config.toml),
then run `neru config reload`.

### Certain hints don't visually match any on-screen UI

Neru may be labelling elements you cannot see directly, such as `row` and
`cell`. Copy the complete `clickable_roles` list from
[default-config.toml](https://github.com/y3owk1n/neru/blob/main/configs/default-config.toml),
remove only those roles, and run `neru config reload`.

```toml
[hints]
clickable_roles = [
    # ...
]
```

### Hints or grids appear but are misaligned

Misaligned hints or grids are a bug, so report them.
Turn on [file logging](#log-file-locations) and
[debug logging](#enable-debug-logging), reproduce it, and attach the log, a
screenshot, your OS version, and the app name and version to the issue.

On Linux Wayland, hints in native apps depend on a window-origin source for
your compositor. See the per-compositor table in
[Platform support](../reference/platform-support.md#accessibility-and-hints).

### No hints in menubar or Dock (macOS)

Both are off unless enabled:

```toml
[hints]
include_menubar_hints = true
include_dock_hints = true

# For specific menubar apps:
additional_menubar_hints_targets = [
    "com.apple.controlcenter",
    "net.kovidgoyal.kitty",  # Example
]
```

---

## Hotkeys not working

### Hotkey does nothing

1. Run the mode from the CLI, for example `neru hints`. If that works, the problem is the
   hotkey, not the mode.
2. Check the daemon is running with `neru status`.
3. Check the app is not in `excluded_apps`.
4. Check the binding syntax against the
   [hotkeys reference](../reference/configuration.md#global-hotkeys), then run
   `neru config validate`.
5. Try a different key combination, in case another app owns this one.

On Linux, Neru ships no default global hotkeys, so nothing fires until you bind
some. See [Binding your first hotkeys](./getting-started.md#binding-your-first-hotkeys). On Wayland, `[hotkeys]` also
needs the `input` group. See
[Global hotkeys on Wayland](./linux-desktops.md#global-hotkeys-on-wayland).

### Hotkey works in some apps but not others

The app is in `excluded_apps`. Remove it from the list:

```toml
[general]
excluded_apps = [
    # "com.apple.Terminal",  # Comment out to enable
]
```

Find a macOS bundle ID with `osascript -e 'id of app "AppName"'`. On Linux the
app is named by `WM_CLASS` or `app_id`. See the
[configuration reference](../reference/configuration.md#app-identity-across-platforms-bundle_id).

### Hotkey conflicts with system shortcuts

Move Neru's binding to another combination:

```toml
[hotkeys]
"Primary+Shift+Space" = "__disabled__"  # Remove the default binding
"Ctrl+Alt+Space" = "hints"              # Use a different combo
```

Or change the system shortcut instead. On macOS it is under
**System Settings > Keyboard > Keyboard Shortcuts**. To drive Neru entirely
from an external hotkey manager such as skhd, see
[Disabling all built-in hotkeys](./recipes.md#disabling-all-built-in-hotkeys).

---

## Performance issues

### Hints appear slowly

Likely causes, in order:

1. Too many clickable roles. Remove the ones you do not need from
   `hints.clickable_roles`.
2. Debug logging is on. Set `log_level` back to `"info"`. See
   [Enable debug logging](#enable-debug-logging).
3. The system itself is under load.

### High CPU usage

Check Neru's CPU use with `top -pid $(pgrep neru)` on macOS,
`top -p $(pgrep neru)` on Linux, or Task Manager on Windows. Look for errors in the
[log](#log-file-locations), then [restart the daemon](#restart-the-daemon). If
it comes back, open an issue with the log.

---

## Daemon issues

### CLI cannot reach the daemon

Run `neru doctor`, which works without the daemon, then `neru launch`, then
`neru status`.

If it still fails on macOS or Linux, a stale socket may be in the way. The
daemon prints its endpoint at startup. Both of these are places it can be, and
only one will exist:

```bash
rm -f "${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"/neru/neru.sock
rm -f "${TMPDIR:-/tmp}"/neru-"$(id -u)"/neru.sock
neru launch
```

### Daemon crashes on startup

Usually a configuration error. Run `neru config validate`, which names the
offending key. To rule the config out, move `config.toml` aside and run
`neru launch`. With no config file the daemon runs on built-in defaults. Run
`neru launch` in a terminal to see why it exits.

### Version mismatch after an upgrade

The daemon still running is the one the old binary started, and the new CLI
reports a version mismatch. [Restart the daemon](#restart-the-daemon).

### Daemon stops responding or won't quit

Force it to quit, then start it again:

```bash
pkill -9 neru          # macOS, Linux
neru launch
```

```powershell
taskkill /IM neru.exe /F   # Windows
neru launch
```

On macOS or Linux, clear a stale socket as in
[CLI cannot reach the daemon](#cli-cannot-reach-the-daemon) if `neru launch`
still fails.

---

## App-specific issues

### Adobe apps: hints misaligned or missing (macOS)

Adobe apps may need custom roles:

```toml
[[hints.app_configs]]
bundle_id = "com.adobe.illustrator"
additional_clickable_roles = ["static_text", "image"]
ignore_clickable_check = true
```

Find the bundle ID with `osascript -e 'id of app "Adobe Illustrator"'`.

### Mission Control: no hints (macOS)

Mission Control is drawn by the Dock, so Dock hints must be on:

```toml
[hints]
include_dock_hints = true
detect_mission_control = true
```

### Accessibility Zoom: cursor lands in the wrong place (macOS)

Accessibility Zoom is under **System Settings > Accessibility > Zoom**. When it
is zoomed in, Neru positions the cursor exactly and pans the zoomed view to
keep the target on screen. If the cursor lands somewhere other than the target, or the zoomed
view does not follow it, open an issue with your macOS version and zoom
factor.

---

## Keyboard layout issues

### Wrong characters produced when typing

Neru detects your keyboard layout, such as QWERTY, AZERTY, QWERTZ, Dvorak or
Colemak, and translates keycodes to match it. If keys still come out wrong:

1. **Check the layout is selected in your OS.** On macOS it is under
   System Settings > Keyboard > Input Sources.
2. **Force the layout.** Some custom layouts are not resolved automatically.
   Run `neru doctor`, copy the layout you want from the `keyboard_layouts` row,
   and set it:

   ```toml
   [general]
   kb_layout_to_use = "com.apple.keylayout.Colemak"  # macOS; "English (Colemak)" on Linux, "00010409" for Dvorak on Windows
   ```

3. **A layout switch was not picked up.** Neru re-registers global hotkeys when
   the layout changes. On Windows it checks once a second, so a punctuation
   hotkey can take up to a second to follow a switch. If hotkeys still fail
   after a switch, [restart the daemon](#restart-the-daemon).

### Input methods not working (CJK IME)

Neru works with CJK input methods such as Pinyin and Wubi. Hints work, key
presses are translated through your physical layout, and the input method
receives keys as usual. If an input method still misbehaves, check it is
installed and active in your OS, and on macOS that Neru has
[Accessibility permission](#permissions).

---

## Configuration issues

### Config changes not taking effect

The daemon does not watch the file. Apply an edit with `neru config reload`. If
the reload is refused, the whole file is rejected and the daemon keeps the
previous configuration, so validate first:

```bash
neru config validate     # names the offending key
neru config reload

# Confirm which file the daemon is reading
neru status --json | jq -r .config
```

Values set with `neru config set` live in `config.override.toml` beside your
config and win over it. `neru config reset <key>` removes one. See
[Config layering](./getting-started.md#config-layering).

### "Failed to parse config"

A TOML syntax error, or a value a validator refuses. `neru config validate`
prints the line or key and why it was refused.

Common causes are missing quotes around a key that contains `+`, a section
header typo, a color without its leading `#`, and a hotkey bound to an empty
string. Use `__disabled__` to remove a binding. Compare against
[default-config.toml](https://github.com/y3owk1n/neru/blob/main/configs/default-config.toml).

---

## Getting help

If none of these work:

1. **Gather information.** Run `neru doctor`, and note your OS version,
   `neru --version`, the app where the issue occurs, the relevant config
   sections (anonymized), and the [log](#log-file-locations).
2. **Search existing issues** at <https://github.com/y3owk1n/neru/issues>.
3. **Open an issue** with the bug-report form, which asks for exactly the
   information above. Pull requests are welcome too. See
   [CONTRIBUTING.md](../../CONTRIBUTING.md).

---

## Emergency reset

If Neru is completely broken, force it to quit as in
[Daemon stops responding](#daemon-stops-responding-or-wont-quit), then remove
Neru and its state with the steps in
[Uninstallation](./installation.md#uninstallation), reinstall, run
`neru launch`, and on macOS grant Accessibility permission again.
