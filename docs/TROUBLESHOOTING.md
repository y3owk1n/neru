# Troubleshooting

Symptoms, causes, and fixes for common Neru problems.

**Related:** [CLI Reference](CLI.md) · [Configuration Reference](CONFIGURATION.md) ·
[Linux setup](LINUX_SETUP.md#troubleshooting)

> **Platform note:** the examples below use macOS paths and, in a few places,
> macOS-only features (Mission Control, Accessibility Zoom, Activity Monitor).
> The diagnosis steps themselves apply everywhere — substitute your platform's
> [log path](#log-file-locations). For Linux-specific setup problems (evdev
> permissions, portal consent, compositor support) see
> [LINUX_SETUP.md](./LINUX_SETUP.md#troubleshooting) and
> [LINUX_DESKTOPS.md](./LINUX_DESKTOPS.md).

---

## Log File Locations

`[logging].log_file` overrides this; when unset the default is:

| Platform | Path                                     |
| -------- | ---------------------------------------- |
| macOS    | `~/Library/Logs/neru/app.log`            |
| Linux    | `~/.local/state/neru/log/app.log`        |
| Windows  | `%LOCALAPPDATA%\neru\log\app.log`        |

---

## Table of Contents

- [Log File Locations](#log-file-locations)
- [Quick Diagnosis](#quick-diagnosis)
- [Installation & Setup](#installation--setup)
- [Permissions](#permissions)
- [Hints & Grids](#hints--grids)
- [Hotkeys Not Working](#hotkeys-not-working)
- [Performance Issues](#performance-issues)
- [Daemon Issues](#daemon-issues)
- [App-Specific Issues](#app-specific-issues)
- [Keyboard Layout Issues](#keyboard-layout-issues)
- [Configuration Issues](#configuration-issues)
- [Logging and Debugging](#logging-and-debugging)
- [Getting Help](#getting-help)
- [Emergency Reset](#emergency-reset)

---

## Quick Diagnosis

**Not working at all?** Check these first:

```bash
# 1. Is daemon running?
neru status

# 2. Run diagnostics (works even if daemon is down)
neru doctor

# 3. Test basic functionality
neru hints  # Should show hints

# 4. Check logs (macOS path; see Log File Locations for Linux/Windows)
tail -20 ~/Library/Logs/neru/app.log
```

**Common issues:**

- ❌ **"Failed to connect to Neru daemon"** → Daemon not running, run `neru launch`
- ❌ **"Permission denied"** → Grant accessibility permissions
- ❌ **No hints appear** → Check app exclusions, try different app

---

## Installation & Setup

**"Cannot open Neru because the developer cannot be verified"**

```bash
xattr -cr /Applications/Neru.app  # Remove quarantine
open -a Neru
```

**"Command not found: neru"**

```bash
# Add to PATH
export PATH="/usr/local/bin:$PATH"
# Add to ~/.zshrc or ~/.bashrc
```

**Homebrew fails**

```bash
brew update && brew reinstall --cask neru
```

---

## Permissions

### Accessibility Permissions

**Required for Neru to function.**

**Grant permissions:**

1. System Settings → Privacy & Security → Accessibility
2. Add Neru and ensure checkbox is enabled

**Reset if not working:**

1. Remove Neru from list
2. Re-add Neru
3. Restart: `pkill neru && neru launch`

**Check health:** `neru doctor` — look for `accessibility: ok` in the component list. If accessibility is denied, the doctor output will show the specific error.

---

## Hints & Grids

### No hints/grids appear

**Check:**

```bash
neru doctor              # Full diagnostics (works even if daemon is down)
neru status              # Daemon running?
neru hints               # CLI works?
```

**Common fixes:**

- Start daemon: `neru launch`
- Grant permissions (see Permissions section)
- Remove app from `excluded_apps` in config
- Test in different app

### No hints visible when using multiple monitors

**Focused window and cursor are on different displays.** Neru only draws hints on the display where the cursor is. If the focused window is on another display, no hints will show for that window.

**Solution:**

If you're using a window manager, you can probably set it up so that your cursor always follows the focused window.

Alternatively, you can change the shortcut that you use to activate `hints` to chain `action move_mouse --window` before activation, so the cursor moves to the focused window first:

```toml
[hotkeys]
"Primary+Shift+Space" = ["action move_mouse --window", "hints"]
```

A tiling window manager with focus-follows-mouse avoids this at the source.

### Misaligned hints/grids

**Rare issue.** Enable debug logging and check logs:

```toml
[logging]
log_level = "debug"
```

### Hints not showing in browsers (Chrome, Firefox, Safari, Brave, Edge, Electron apps)

**Browser engine detection is fully automatic.** Neru identifies the rendering engine (Chromium, Firefox, WebKit, Electron) by inspecting the app bundle — no manual configuration needed.

If hints aren't showing in a browser or Electron app, the automatic detection likely missed it. Check the logs for the detection result:

```bash
grep "Detected non empty bundle type" ~/Library/Logs/neru/app.log
```

If your browser doesn't appear in the logs at all, the detection returned empty — open an issue at [github.com/y3owk1n/neru](https://github.com/y3owk1n/neru) with the bundle ID.

Find the bundle ID:
```bash
osascript -e 'id of app "Your Browser"'
```

> [!NOTE]
> PWAs (installed web apps) are also detected automatically — Chrome PWAs as `chromium`, Safari PWAs as `webkit`.

### Some elements that should have hints don't have hints

This can happen when the element you're trying to select is small. If you encounter this, open an issue.

Alternatively, make sure all relevant roles are enabled. Run `neru roles --explain` to see
exactly which roles your config selects on this platform, and `neru roles` for the full
vocabulary. If you've customized `hints.clickable_roles`, you can remove that customization
or restore it to the original value from
[default-config.toml](https://github.com/y3owk1n/neru/blob/main/configs/default-config.toml).

Run `neru config reload` and then test.

```toml
[hints]
clickable_roles = [
    # ...
]
```

### Certain hints don't visually match any on-screen UI

Neru may be generating hints for elements that are not directly visible to you, such as `row`
and `cell`. Copy the complete `clickable_roles` list from
[default-config.toml](https://github.com/y3owk1n/neru/blob/main/configs/default-config.toml),
then remove only those roles so the remaining defaults stay enabled.

Run `neru config reload` and then test.

```toml
[hints]
clickable_roles = [
    # ...
]
```

### Menubar/Dock hints missing

```toml
[hints]
include_menubar_hints = true
include_dock_hints = true
```

### Hints or grids appear but are misaligned

Hints or grids should always be accurate. This is rare.\*\*

**Solution:**

```bash
# Enable debug logging
# Edit ~/.config/neru/config.toml:
[logging]
log_level = "debug"

# Restart and check logs
pkill neru && neru launch
tail -f ~/Library/Logs/neru/app.log

# Report issue with:
# - macOS version
# - App name and version
# - Screenshot
```

### No hints in menubar/Dock

**Disabled in config or not enabled.**

**Solution:**

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

## Hotkeys Not Working

### Hotkey does nothing

**Possible causes:**

1. Hotkey conflict with another app
2. Daemon not running
3. App is excluded
4. Incorrect hotkey syntax

**Solutions:**

```bash
# 1. Test with CLI to bypass hotkey system
neru hints

# If CLI works, it's a hotkey issue

# 2. Check daemon status
neru status

# 3. Try different hotkey combo
# Edit ~/.config/neru/config.toml:
[hotkeys]
"Ctrl+F" = "hints"  # Try this instead

# 4. Verify syntax is correct
# Modifiers: Cmd, Ctrl, Alt/Option, Shift, Primary, Super, Meta
# Format: "Mod1+Mod2+Key" = "action"
```

### Hotkey works in some apps but not others

**App is in excluded list.**

**Solution:**

```toml
[general]
excluded_apps = [
    # "com.apple.Terminal",  # Comment out to enable
]
```

Find bundle ID:

```bash
osascript -e 'id of app "AppName"'
```

### Hotkey conflicts with system shortcuts

**Solution:**

**Option 1: Change Neru hotkey**

```toml
[hotkeys]
"Primary+Shift+Space" = ""  # Disable default
"Ctrl+Alt+Space" = "hints"  # Use different combo
```

**Option 2: Disable system shortcut**

1. Open **System Settings → Keyboard → Keyboard Shortcuts**
2. Find conflicting shortcut
3. Disable or change it

**Option 3: Use external hotkey manager**

```bash
# Use skhd or similar instead of Neru hotkeys
# ~/.config/skhd/skhdrc
ctrl - f : neru hints
```

Then disable Neru hotkeys:

```toml
[hotkeys]
# Leave empty or comment out all hotkeys
```

---

## Performance Issues

### Hints appear slowly

Possible causes:\*\*

1. Too many depth levels in the accessibility tree of current activation
2. Debug logging enabled
3. System resource constraints

**Solution:**

```bash
# 1. Remove unnecessary clickable roles from your config
# 2. Disable debug logging
[logging]
log_level = "info"  # Not "debug"

# 3. Check system resources
top -o cpu
```

### High CPU usage

**Neru should not use too much CPU.**

**Solution:**

```bash
# Check Neru CPU usage
top -pid $(pgrep neru)

# Check logs for errors
tail -f ~/Library/Logs/neru/app.log | grep ERROR

# Restart daemon
pkill neru && neru launch
```

---

## Daemon Issues

### "Failed to connect to Neru daemon"

**Daemon not running.**

**Solution:**

```bash
# Run diagnostics first (works without daemon)
neru doctor

# Start daemon
neru launch

# Check status
neru status

# If still failing, check for stale socket (path is printed in logs; typically under /var/folders/.../T)
rm -f /var/folders/*/*/T/neru.sock
neru launch
```

### Daemon crashes on startup

**Configuration error or system issue.**

**Solution:**

```bash
# Check logs
cat ~/Library/Logs/neru/app.log

# Try with default config
neru launch  # Uses defaults if no config file

# Try with minimal config
mkdir -p ~/.config/neru
cat > ~/.config/neru/config.toml << EOF
[hotkeys]
"Primary+Shift+Space" = "hints"

[logging]
log_level = "debug"
EOF

neru launch
```

### Daemon stops responding

**IPC socket issue or daemon hung.**

**Solution:**

```bash
# Force quit
pkill -9 neru

# Clean up socket (path is printed in logs; typically under /var/folders/.../T)
rm -f /var/folders/*/*/T/neru.sock

# Restart
neru launch

# Monitor logs
tail -f ~/Library/Logs/neru/app.log
```

### Daemon won't quit

**Force termination needed.**

**Solution:**

```bash
# Force quit
pkill -9 neru

# Or use Activity Monitor:
# 1. Open Activity Monitor
# 2. Search "Neru"
# 3. Select and click "Force Quit"
```

---

## App-Specific Issues

### Adobe apps: Hints misaligned or missing

**Adobe apps may need custom roles.**

**Solution:**

```toml
[[hints.app_configs]]
bundle_id = "com.adobe.illustrator"
additional_clickable_roles = ["static_text", "image"]
ignore_clickable_check = true
```

Find bundle ID:

```bash
osascript -e 'id of app "Adobe Illustrator"'
```

### Mission Control: No hints

**Ensure Dock hints are enabled (Mission Control uses Dock).**

**Solution:**

```toml
[hints]
include_dock_hints = true
detect_mission_control = true
```

> [!NOTE]
> Mission Control detection uses `CGWindowListCopyWindowInfo` to check for Dock overlay windows. It works on macOS 14+ (Sonoma) and 15+ (Sequoia/Tahoe). On macOS 13 and earlier, it looks for a "Mission Control" app window instead.

### Accessibility Zoom: cursor lands in the wrong place

**Symptom:** with macOS Accessibility Zoom (System Settings → Accessibility → Zoom) zoomed in, hints, grid movement, `move_mouse`, dragging and scrolling all send the cursor somewhere other than the target, while overlays still draw correctly. Or the cursor lands correctly but the magnified view does not follow it, so it disappears off screen.

**Solution:** update to a build that posts synthetic mouse events at the session event tap and pans the zoom viewport itself. If you still see it, file an issue with your macOS version and zoom factor.

> [!NOTE]
> While zoomed in, the window server rewrites the location of pointer-motion events that enter at the HID event tap, reading the posted point as a coordinate in zoomed-viewport space (`landed = zoomOrigin + (posted - displayCenter) / zoomFactor`). Neru posts mouse events at the session tap instead, which sits above that transform, so positioning is exact whether or not zoom is engaged. Reading the cursor position was never affected.

> [!NOTE]
> Only real pointer-device movement pans the zoom viewport — no synthetic event does, at any event tap. `UAZoomChangeFocus` does not help either: it drives the keyboard-focus and text-insertion-point paths, so it is a no-op when zoom is set to follow the mouse pointer. Neru therefore pans the viewport itself before each cursor move, by the smallest amount that brings the target on screen, which reproduces the edge-panning behavior of a real mouse. It uses SkyLight's zoom SPI resolved at runtime; if a future macOS removes it, cursor positioning stays correct and only the follow behavior is lost.

---

## Keyboard Layout Issues

### Wrong characters produced when typing

Neru supports most keyboard layouts including QWERTY, AZERTY, QWERTZ, Dvorak, and Colemak. Neru automatically detects your physical keyboard layout via macOS and translates keycodes accordingly.

If you're still experiencing issues:

1. **Check your keyboard layout is properly configured in macOS:**
    - System Settings → Keyboard → Input Source
    - Ensure your desired layout is added and selected

2. **Layout not detected correctly:**
    - Some custom layouts (e.g., Colemak, Dvorak) may not be resolved automatically
    - Force the layout by setting `kb_layout_to_use` in your config to the full bundle ID:

        ```bash
        # First, switch to your desired layout in the menu bar, then:
        defaults read com.apple.HIToolbox AppleCurrentKeyboardLayoutInputSourceID
        ```

        Then use the returned value (e.g., `com.apple.keylayout.Colemak`):

        ```toml
        [general]
        kb_layout_to_use = "com.apple.keylayout.Colemak"
        ```

3. **Layout changes at runtime not picked up:**
    - Neru now automatically re-registers global hotkeys when the keyboard layout changes (e.g., switching from US to Dvorak while Neru is running)
    - If hotkeys don't work after a layout switch, try toggling Neru off and on, or restart the daemon with `pkill neru && neru launch`

### Input methods not working (CJK IME)

Neru now supports CJK input methods (Pinyin, Wubi, etc.). When using an input method:

- Hints work correctly
- Key presses are translated through your physical keyboard layout
- The input method receives keys as expected

If input methods still don't work:

- Ensure the input method is properly installed and active in macOS
- Check that Accessibility permissions are granted to Neru

---

## Configuration Issues

### Config changes not taking effect

**Daemon needs restart to reload config.**

**Solution:**

```bash
# Restart daemon
pkill neru && neru launch

# Verify config location
neru status
# Check "Config:" line
```

### "Failed to parse config"

**TOML syntax error.**

**Solution:**

```bash
# Check logs
cat ~/Library/Logs/neru/app.log | grep ERROR

# Common issues:
# - Missing quotes around keys/values
# - Incorrect section headers
# - Invalid TOML syntax

# Validate TOML syntax online:
# https://www.toml-lint.com/

# Or use default config as reference:
curl -o /tmp/default.toml \
  https://raw.githubusercontent.com/y3owk1n/neru/main/configs/default-config.toml
```

### Colors not working

**Check hex color format.**

**Solution:**

```toml
# Correct:
background_color = "#FFD700"

# Incorrect:
background_color = "FFD700"   # Missing #
background_color = "#FFFGG"   # Invalid hex
```

### Hotkeys in wrong format

**Check modifier syntax.**

**Solution:**

```toml
# Correct:
"Primary+Shift+Space" = "hints"

# Incorrect:
"Primary-Shift-Space" = "hints"  # Use +, not -
"PRIMARY+SHIFT+SPACE" = "hints"  # Use proper case
```

---

## Logging and Debugging

### Enable debug logging

```toml
[logging]
log_level = "debug"
```

Restart:

```bash
pkill neru && neru launch
```

### View logs

```bash
# Real-time monitoring
tail -f ~/Library/Logs/neru/app.log

# Last 100 lines
tail -100 ~/Library/Logs/neru/app.log

# Search for errors
grep ERROR ~/Library/Logs/neru/app.log

# Search for specific app
grep "com.apple.Safari" ~/Library/Logs/neru/app.log
```

### Common log messages

**"Found usable accessibility tree"** - Accessibility tree detected, AX support activated

**"Hints mode activated"** - Hint overlay is active; includes hint count when available

**"Clickable element collection was slow"** - Accessibility scanning completed but took longer than expected

**"Failed to get clickable elements"** - Accessibility query failed; check macOS Accessibility permission and app-specific exclusions

**"Secure input is enabled, blocking mode activation"** - macOS secure input is active, often because a password field is focused

Most key routing, overlay redraw, and hint filtering details are logged only at `debug` to keep production logs quiet.

### Clear logs

```bash
# Remove old logs
rm ~/Library/Logs/neru/app.log

# Restart daemon (creates fresh log)
pkill neru && neru launch
```

---

## Getting Help

If none of these solutions work:

1. **Gather information:** run `neru doctor` and note your macOS version
   (`sw_vers`), `neru --version`, the app where the issue occurs, the relevant
   config sections (anonymized), and logs.
2. **Search existing issues:** <https://github.com/y3owk1n/neru/issues>
3. **Open an issue** using the bug-report form — it asks for exactly the
   information above. If you would rather fix it yourself, pull requests are
   very welcome: see [CONTRIBUTING.md](../CONTRIBUTING.md).

---


## Emergency Reset

If Neru is completely broken:

```bash
pkill -9 neru
```

Then remove Neru and its state entirely — the full steps, including purging
config and logs, are in
[INSTALLATION.md](INSTALLATION.md#uninstallation) — reinstall, run
`neru launch`, and re-grant Accessibility permission (System Settings →
Privacy & Security → Accessibility).
