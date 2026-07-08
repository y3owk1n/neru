# Troubleshooting

Common issues and their solutions.

---

## Quick Diagnosis

**Not working at all?** Run these first:

```bash
# 1. Is daemon running?
neru status

# 2. Run diagnostics (works even if daemon is down)
neru doctor

# 3. Check logs
tail -20 ~/Library/Logs/neru/app.log
```

**Common issues at a glance:**

| Symptom                             | Likely Cause                         |
| :---------------------------------- | :----------------------------------- |
| "Failed to connect to Neru daemon"  | Daemon not running → `neru launch`   |
| No hints appear                     | Missing accessibility permissions    |
| Hotkey does nothing                 | Conflict with another app            |
| Hints don't appear in Electron apps | Need `additional_ax_support` enabled |

---

## Installation & Setup

| Problem                                                     | Solution                                    |
| :---------------------------------------------------------- | :------------------------------------------ |
| "Cannot open Neru because the developer cannot be verified" | `xattr -cr /Applications/Neru.app`          |
| "Command not found: neru"                                   | Add `/usr/local/bin` to your PATH           |
| Homebrew fails                                              | `brew update && brew reinstall --cask neru` |
| App won't open (quarantine)                                 | `xattr -cr /Applications/Neru.app`          |

---

## Permissions

### Neru Needs Accessibility Access

This is required for Neru to read UI elements and simulate input.

**To grant:**

1. Open **System Settings → Privacy & Security → Accessibility**
2. Click **+** and add `/Applications/Neru.app`
3. Ensure the checkbox is enabled

**To reset:**

1. Remove Neru from the list
2. Re-add Neru
3. Restart: `pkill neru && neru launch`

**Verify:** `neru doctor` — look for `accessibility: ok`.

---

## Hints & Grids

### No Hints or Grids Appear

1. Check daemon: `neru status`
2. Run diagnostics: `neru doctor`
3. Test CLI: `neru hints`
4. Check app exclusions in config
5. Try a different app

### Hints Don't Appear in Electron Apps

```toml
[hints.additional_ax_support]
enable = true
```

### Hints Don't Appear in Chrome/Firefox

Same as Electron — enable `additional_ax_support`.

### Menubar or Dock Hints Missing

```toml
[hints]
include_menubar_hints = true
include_dock_hints = true
```

### Hints Misaligned

This is rare. Enable debug logging and check:

```toml
[logging]
log_level = "debug"
```

```bash
pkill neru && neru launch
tail -f ~/Library/Logs/neru/app.log
```

Report issues with: macOS version, app name/version, screenshot.

### Mission Control: No Hints

```toml
[hints]
include_dock_hints = true
detect_mission_control = true
```

---

## Hotkeys

### Hotkey Does Nothing

```bash
# Test with CLI to bypass hotkeys
neru hints

# If CLI works, it's a hotkey issue
# Check daemon status
neru status

# Try a different combo in config
```

### Hotkey Works in Some Apps but Not Others

App is in the exclusion list:

```toml
[general]
excluded_apps = [
    # "com.apple.Terminal",  # Comment out to enable
]
```

Find bundle IDs: `osascript -e 'id of app "AppName"'`

### Hotkey Conflicts with System Shortcuts

**Option 1: Change Neru hotkey**

```toml
[hotkeys]
"Primary+Shift+Space" = ""  # Disable default
"Ctrl+Alt+Space" = "hints"
```

**Option 2: Disable system shortcut** in System Settings → Keyboard → Keyboard Shortcuts

**Option 3: Use external hotkey manager** (skhd, Hammerspoon) and disable Neru's:

```toml
[hotkeys]
# Empty = all defaults cleared
```

---

## Performance

| Problem             | Solution                                            |
| :------------------ | :-------------------------------------------------- |
| Hints appear slowly | Reduce AX tree depth, disable debug logging         |
| High CPU usage      | Check with `top -pid $(pgrep neru)`, restart daemon |

```bash
top -pid $(pgrep neru)
tail -f ~/Library/Logs/neru/app.log | grep ERROR
pkill neru && neru launch
```

---

## Daemon

| Problem                            | Solution                                                             |
| :--------------------------------- | :------------------------------------------------------------------- |
| "Failed to connect to Neru daemon" | `neru launch`                                                        |
| Daemon crashes on startup          | Check logs, try with default config                                  |
| Daemon stops responding            | `pkill -9 neru && rm -f /var/folders/*/*/T/neru.sock && neru launch` |
| Daemon won't quit                  | `pkill -9 neru`                                                      |

---

## App-Specific

### Adobe Apps

```toml
[[hints.app_configs]]
bundle_id = "com.adobe.illustrator"
additional_clickable_roles = ["AXStaticText", "AXImage"]
ignore_clickable_check = true
```

### VS Code

```toml
[hints.additional_ax_support]
enable = true
# VS Code is auto-detected
```

---

## Keyboard Layout

| Problem                      | Solution                                                            |
| :--------------------------- | :------------------------------------------------------------------ |
| Wrong characters produced    | Neru auto-detects layout; try `kb_layout_to_use`                    |
| Layout changes not picked up | Toggle Neru or restart: `pkill neru && neru launch`                 |
| CJK IME not working          | Ensure input method is active and Accessibility permissions granted |

Find your layout ID:

```bash
defaults read com.apple.HIToolbox AppleCurrentKeyboardLayoutInputSourceID
```

Then set it:

```toml
[general]
kb_layout_to_use = "com.apple.keylayout.Colemak"
```

---

## Configuration

| Problem                   | Solution                                            |
| :------------------------ | :-------------------------------------------------- |
| Changes not taking effect | Daemon needs restart: `pkill neru && neru launch`   |
| "Failed to parse config"  | Check TOML syntax, missing quotes, invalid sections |
| Colors not working        | Use correct hex format: `"#FFD700"` not `"FFD700"`  |
| Hotkeys wrong format      | Use `+` not `-`: `"Primary+Shift+Space"`            |

Validate syntax:

```bash
neru config validate
```

---

## Logging & Debugging

### Enable Debug Logging

```toml
[logging]
log_level = "debug"
```

Restart: `pkill neru && neru launch`

### View Logs

```bash
tail -f ~/Library/Logs/neru/app.log          # Real-time
tail -100 ~/Library/Logs/neru/app.log        # Last 100 lines
grep ERROR ~/Library/Logs/neru/app.log       # Errors only
grep "com.apple.Safari" ~/Library/Logs/neru/app.log  # Specific app
```

### Common Log Messages

| Message                                             | Meaning                                               |
| :-------------------------------------------------- | :---------------------------------------------------- |
| "App requires Electron support"                     | Electron app detected; enable `additional_ax_support` |
| "Hints mode activated"                              | Hint overlay is active                                |
| "Secure input is enabled, blocking mode activation" | Password field focused — Neru pauses                  |
| "Clickable element collection was slow"             | AX scanning took longer than expected                 |

### Clear Logs

```bash
rm ~/Library/Logs/neru/app.log
pkill neru && neru launch
```

---

## Getting Help

If none of these solutions work:

1. **Gather information:**
    - `neru doctor` full output
    - `sw_vers` (macOS version)
    - `neru --version`
    - App name/version where issue occurs
    - Config file (anonymized)
    - Relevant logs

2. **Search existing issues:** [github.com/y3owk1n/neru/issues](https://github.com/y3owk1n/neru/issues)

3. **Open an issue** with all gathered information.

---

## Emergency Reset

```bash
# Force quit
pkill -9 neru

# Remove all Neru files
rm -rf /Applications/Neru.app
rm -f /usr/local/bin/neru
rm -rf ~/.config/neru
rm -rf "$HOME/Library/Application Support/neru"
# or: rm -rf ~/Library/Application\ Support/neru
rm -rf ~/Library/Logs/neru
rm -f /var/folders/*/*/T/neru.sock

# Reinstall
brew reinstall --cask neru
# or build from source

# Fresh start
neru launch

# Grant permissions again
# System Settings → Privacy & Security → Accessibility
```
