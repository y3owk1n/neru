# Troubleshooting Guide

Common issues and solutions for Neru.

---

## Table of Contents

- [Quick Diagnosis](#quick-diagnosis)
- [Installation & Setup](#installation--setup)
- [Permissions](#permissions)
- [Hints & Grids](#hints--grids)
- [Hotkeys](#hotkeys)
- [Performance](#performance)
- [Daemon](#daemon)
- [App-Specific](#app-specific)
- [Configuration](#configuration)
- [Debugging](#debugging)

---

## Quick Diagnosis

**Not working at all?** Check these first:

```bash
# 1. Is daemon running?
neru status

# 2. Check permissions
neru doctor

# 3. Test basic functionality
neru hints  # Should show hints

# 4. Check logs
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

**Check health:** `neru doctor`

---

## Hints & Grids

### No hints/grids appear

**Check:**

```bash
neru status              # Daemon running?
neru doctor              # Permissions OK?
neru hints               # CLI works?
```

**Common fixes:**

- Start daemon: `neru launch`
- Grant permissions (see Permissions section)
- Remove app from `excluded_apps` in config
- Test in different app

### Misaligned hints/grids

**Rare issue.** Enable debug logging and check logs:

```toml
[logging]
log_level = "debug"
```

### Electron/Chromium/Firefox issues

**Enable additional AX support:**

```toml
[hints.additional_ax_support]
enable = true
```

**⚠️ Tiling WM users:** May conflict with yabai/Amethyst - keep `enable = false` and use grid mode.

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

### Hints don't appear in Electron apps

**Electron apps need additional AX support.**

**Solution:**

Edit `~/.config/neru/config.toml`:

```toml
[hints.additional_ax_support]
enable = true

# If your app isn't auto-detected, add it:
additional_electron_bundles = [
    "com.your.electronapp",
]
```

Restart Neru:

```bash
pkill neru && neru launch
```

**Check logs for:**

```
App requires Electron support
Enabled AXManualAccessibility for: com.your.app
```

### Hints don't appear in Chrome/Firefox content

**Browser needs additional AX support.**

**⚠️ Warning for Tiling WM Users:**

Enabling accessibility support for Chrome/Firefox can interfere with tiling window managers (yabai, Amethyst, Rectangle, etc.), causing windows to resist tiling or snap incorrectly.

**If you DON'T use a tiling window manager:**

```toml
[hints.additional_ax_support]
enable = true

# For custom Chromium browsers:
additional_chromium_bundles = ["com.your.browser"]

# For custom Firefox browsers:
additional_firefox_bundles = ["org.your.firefox"]
```

**If you DO use a tiling window manager:**

Keep `enable = false` and rely on Neru's grid-based approach instead. The grid method works well in browsers without requiring accessibility modifications that conflict with tiling WMs.

**Alternative:** Use grid mode as it doesn't require accessibility tree access.
**Alternative:** Use browser extensions like Vimium or Surfingkeys for in-page navigation, and use Neru for everything else.

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
# Modifiers: Cmd, Ctrl, Alt/Option, Shift
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
"Cmd+Shift+Space" = ""  # Disable default
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
# 1. Remove unnecessary AXRoles in your config
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
"Cmd+Shift+Space" = "hints"

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

### VS Code: Hints don't appear in editor

**Electron AX support needed.**

**Solution:**

```toml
[hints.additional_ax_support]
enable = true
# VS Code is auto-detected
```

### Adobe apps: Hints misaligned or missing

**Adobe apps may need custom roles.**

**Solution:**

```toml
[[hints.app_configs]]
bundle_id = "com.adobe.illustrator"
additional_clickable_roles = ["AXStaticText", "AXImage"]
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
[general]
include_dock_hints = true
```

### Tiling window manager conflicts

**Browser windows don't tile correctly after enabling AX support.**

**Symptoms:**

- Chrome/Firefox windows resist tiling
- Windows snap to wrong positions
- yabai/Amethyst/Rectangle layouts break
- Browser windows ignore tiling rules

**Cause:**
Enabling `AXEnhancedUserInterface` for Chromium/Firefox conflicts with tiling window managers.

**Solution:**

Disable additional AX support:

```toml
[hints.additional_ax_support]
enable = false  # Keep this off if using tiling WM
```

**If you still need browser hint support:**

1. Use Neru's grid-based hints (works without AX modifications)
2. Use browser extensions for in-page navigation:
    - Vimium (Chrome)
    - Vimium-FF (Firefox)
    - Surfingkeys
3. Keep Neru for OS-level navigation (menubar, Dock, native apps)

**Restart your tiling WM after disabling:**

```bash
# yabai
yabai --restart-service

# Amethyst
# Quit and reopen via Activity Monitor

# Rectangle
# Quit and reopen via menubar
```

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
"Cmd+Shift+Space" = "hints"

# Incorrect:
"Cmd-Shift-Space" = "hints"  # Use +, not -
"CMD+SHIFT+SPACE" = "hints"  # Use proper case
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

**"App requires Electron support"** - Electron app detected, needs AX support enabled

**"Enabled AXManualAccessibility"** - Electron support activated successfully

**"No clickable elements found"** - No hints to show (app may need custom config)

**"Failed to get AX elements"** - Accessibility permission issue

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

1. **Gather information:**
    - macOS version: `sw_vers`
    - Neru version: `neru --version`
    - App name and version where issue occurs
    - Config file (anonymize if needed)
    - Relevant logs

2. **Search existing issues:**
    - <https://github.com/y3owk1n/neru/issues>

3. **Open an issue:**
    - Include all gathered information
    - Describe expected vs actual behavior
    - Steps to reproduce

4. **Consider a PR:**
    - Pull requests are more likely to be reviewed than issues
    - Fix the problem yourself and contribute back
    - See [DEVELOPMENT.md](DEVELOPMENT.md) for contribution guidelines

---

## Emergency Reset

If Neru is completely broken:

```bash
# 1. Force quit
pkill -9 neru

# 2. Remove all Neru files
rm -rf /Applications/Neru.app
rm -f /usr/local/bin/neru
rm -rf ~/.config/neru
rm -rf ~/Library/Application\ Support/neru
rm -rf ~/Library/Logs/neru
# IPC socket lives under the OS temp directory
rm -f /var/folders/*/*/T/neru.sock

# 3. Reinstall
brew reinstall --cask neru
# or build from source

# 4. Fresh start (no config)
neru launch

# 5. Grant permissions again
# System Settings → Privacy & Security → Accessibility
```
