# Configuration

Neru uses TOML for configuration. No config file is required — sensible defaults apply. Only define the options you want to change.

---

## Quick Start

```toml
[hints.hotkeys]
"Shift+L" = ["action left_click", "idle"]

[scroll]
scroll_step = 50
```

Generate a fully-commented starter file:

```bash
neru config init                  # Creates ~/.config/neru/config.toml
neru config init --force          # Overwrite existing
neru config init -c /path/to/config.toml
```

---

## Config File Location

```
~/.config/neru/config.toml  (recommended)
```

Loaded in priority order:

1. `$XDG_CONFIG_HOME/neru/config.toml`
2. `~/.config/neru/config.toml`
3. `~/.neru.toml` (legacy)
4. `neru.toml` (current directory)
5. `config.toml` (current directory)

Override at launch: `neru launch -c /path/to/config.toml`

### CLI Management

```bash
neru config validate    # Check syntax (no daemon needed)
neru config reload      # Apply changes to running daemon
neru config dump        # Print active config as JSON (daemon required)
neru config init        # Create default config file
```

---

## Color Format

Colors use hex notation with optional alpha transparency.

| Format      | Example     | Alpha | Notes        |
| :---------- | :---------- | :---: | :----------- |
| `#AARRGGBB` | `#FF000000` |  Yes  | Recommended  |
| `#RRGGBB`   | `#FF0000`   |  No   | Fully opaque |
| `#RGB`      | `#F00`      |  No   | Shorthand    |

### Light/Dark Mode

Colors can be a single string or a dictionary with `light`/`dark` keys:

```toml
# Same for both themes
background_color = "#FF0000AA"

# Per-theme
background_color = { light = "#FF0000AA", dark = "#00FF00AA" }
```

Omitted colors inherit theme-derived defaults and update in real time when you switch system themes.

---

## Hotkeys

### Global Hotkeys

```toml
[hotkeys]
"Primary+Shift+Space" = "hints"
```

**Syntax:** `"Mod1+Mod2+Key" = "action"`

| Modifier  | Aliases                                 |
| :-------- | :-------------------------------------- |
| `Cmd`     | `Command`, `Super`, `Meta`              |
| `Ctrl`    | `Control`                               |
| `Alt`     | `Option`                                |
| `Shift`   |                                         |
| `Primary` | `Cmd` on macOS, `Ctrl` on Linux/Windows |

**Available keys:** `a`–`z`, `A`–`Z`, `0`–`9`, symbols (`` ` ``, `-`, `=`, `[`, `]`, `\`, `;`, `'`, `,`, `.`, `/`), `Space`, `Return`, `Enter`, `Escape`, `Tab`, `Delete`, `Backspace`, navigation keys, `F1`–`F20`. Multi-key sequences (e.g. `gg`) supported with 500ms timeout.

**Action values** can be a single string or array:

```toml
[hotkeys]
"Primary+Shift+D" = ["hints", "exec echo 'hints activated'"]
"PageUp"          = ["action go_top", "action page_down"]
```

**Shell commands** use `exec` prefix: `"Primary+T" = "exec open -a Terminal"`

#### Merging Behavior

| Config                 | Result                    |
| :--------------------- | :------------------------ |
| Section absent         | All defaults used         |
| Section present, empty | All hotkeys disabled      |
| Section has entries    | Merged on top of defaults |

Use `__disabled__` to remove individual defaults. Append `--toggle` for toggle behavior.

### Per-Mode Hotkeys

Active only while that mode is running. Same merging rules as global hotkeys.

```toml
[hints.hotkeys]
"Escape"    = "idle"
"Shift+L"   = ["action left_click", "idle"]

[scroll.hotkeys]
"gg"              = "action go_top"
"Primary+Shift+T" = "exec open -a Terminal"
```

### Per-App Hotkey Overrides

```toml
[[hints.app_configs]]
bundle_id = "com.brave.Browser"
hotkeys = { "Return" = "action left_click", "Shift+L" = "__disabled__" }
```

**Priority order:** Modifier toggle → `<mode>.hotkeys` + app overrides → Mode built-in keys.

### Action Reference

| Category    | Actions                                                                       |
| :---------- | :---------------------------------------------------------------------------- |
| Click       | `left_click`, `right_click`, `middle_click`                                   |
| Mouse       | `mouse_down`, `mouse_up`, `move_mouse`, `move_mouse_relative`                 |
| Scroll      | `scroll_up`, `scroll_down`, `scroll_left`, `scroll_right`                     |
| Page        | `page_up`, `page_down`, `go_top`, `go_bottom`                                 |
| Keyboard    | `feed`                                                                        |
| Hints       | `search_hints`, `cycle_hint`, `cycle_hint --backward`                         |
| Delay       | `sleep <duration>` — `0.5`, `500ms`, `1s`                                     |
| Mode        | `reset`, `backspace`                                                          |
| Composition | `wait_for_mode_exit` (with `--bail`), `save_cursor_pos`, `restore_cursor_pos` |

- `--bare` targets cursor position instead of mode selection
- `--steps` overrides `scroll_step` for scroll actions

---

## Configuration Sections

### `[general]`

| Option                                 | Type   | Default       | Description                                       |
| :------------------------------------- | :----- | :------------ | :------------------------------------------------ |
| `excluded_apps`                        | array  | `[]`          | Bundle IDs where Neru won't activate              |
| `kb_layout_to_use`                     | string | `""`          | Force keyboard layout InputSourceID               |
| `hide_overlay_in_screen_share`         | bool   | `false`       | Hide overlay in screen sharing apps               |
| `passthrough_unbounded_keys`           | bool   | `false`       | Let unbound Cmd/Ctrl/Alt shortcuts pass through   |
| `should_exit_after_passthrough`        | bool   | `false`       | Exit mode after a passthrough shortcut            |
| `passthrough_unbounded_keys_blacklist` | array  | `[]`          | Shortcuts to keep consumed when passthrough is on |
| `exec_shell`                           | string | `"/bin/bash"` | Shell binary for `exec` hotkey commands           |
| `exec_shell_args`                      | array  | `["-lc"]`     | Shell arguments                                   |

### `[theme]`

Base colors used to derive all component defaults. Use solid `#RRGGBB`.

| Key             | Default | Role                                                |
| :-------------- | :------ | :-------------------------------------------------- |
| `surface`       | Auto    | Translucent fills, badges, indicator backgrounds    |
| `accent`        | Auto    | Borders, lines, primary chrome                      |
| `accent_alt`    | Auto    | Active/emphasis states, highlights, virtual pointer |
| `on_accent_alt` | Auto    | Foreground text on `accent_alt` surfaces            |
| `text`          | Auto    | Foreground text on `surface` backgrounds            |

> Theme defaults are derived from the active system appearance (light/dark) and update in real time. Omit a key to use the derived default. Run `neru config init` to see the computed values for your current theme.

### `[hints]`

Labels clickable UI elements. Uses macOS Accessibility API (`axtree`) or Vision Framework (`vision`).

| Option                             | Type   | Default       | Description                                   |
| :--------------------------------- | :----- | :------------ | :-------------------------------------------- |
| `enabled`                          | bool   | `true`        | Enable/disable hints mode                     |
| `strategy`                         | string | `"axtree"`    | Detection: `"axtree"` or `"vision"`           |
| `hint_characters`                  | string | `"asdfghjkl"` | Characters for labels                         |
| `label_direction`                  | string | `"normal"`    | `"normal"` or `"reverse"`                     |
| `max_depth`                        | int    | `50`          | Max AX tree depth (0 = unlimited)             |
| `include_menubar_hints`            | bool   | `false`       | Show hints on menubar                         |
| `additional_menubar_hints_targets` | array  | `[]`          | Additional menu bar apps to hint              |
| `include_dock_hints`               | bool   | `false`       | Show hints on Dock                            |
| `include_nc_hints`                 | bool   | `false`       | Show hints in Notification Center             |
| `include_stage_manager_hints`      | bool   | `false`       | Show hints in Stage Manager                   |
| `include_pip_hints`                | bool   | `false`       | Show hints in Picture-in-Picture windows      |
| `include_screen_capture_hints`     | bool   | `false`       | Show hints during screen capture              |
| `detect_mission_control`           | bool   | `false`       | Enable Mission Control detection              |
| `ignore_clickable_check`           | bool   | `false`       | Skip clickability heuristic                   |
| `visible_check_enabled`            | bool   | `false`       | Enable visibility hit-test check for elements |

**UI options:** `font_size`, `font_family`, `border_radius`, `padding_x/y`, `border_width`, `placement` (`top`/`center`/`bottom`), colors (`background`, `text`, `matched_text`, `border`).

**Boundary highlight:** Optional element outlines. Off by default.

**Additional AX support:** Framework-specific improvements for Electron, Chromium, Firefox, WebKit.

**Vision settings:** Only used when `strategy = "vision"`. Options for text/rectangle detection, confidence thresholds, and element classification.

**Per-app config:**

```toml
[[hints.app_configs]]
bundle_id = "com.apple.Safari"
strategy = "vision"
label_direction = "reverse"
additional_clickable_roles = ["AXLink"]
```

Full option list available in `neru config init` output.

### `[grid]`

| Option              | Type   | Default                       | Description                                              |
| :------------------ | :----- | :---------------------------- | :------------------------------------------------------- |
| `enabled`           | bool   | `true`                        | Enable/disable                                           |
| `characters`        | string | `"abcdefghijklmnpqrstuvwxyz"` | Grid labels                                              |
| `sublayer_keys`     | string | Same as `characters`          | Subgrid labels                                           |
| `row_labels`        | string | `""`                          | Custom row labels (inferred from characters if empty)    |
| `col_labels`        | string | `""`                          | Custom column labels (inferred from characters if empty) |
| `live_match_update` | bool   | `true`                        | Highlight as you type                                    |
| `hide_unmatched`    | bool   | `true`                        | Hide non-matching cells                                  |
| `prewarm_enabled`   | bool   | `true`                        | Pre-calculate subgrid for faster navigation              |
| `enable_gc`         | bool   | `false`                       | Enable garbage collection for unused subgrids            |

**UI:** `font_size`, `font_family`, `border_width`, colors.

### `[recursive_grid]`

| Option                             | Type   | Default       | Description                |
| :--------------------------------- | :----- | :------------ | :------------------------- |
| `enabled`                          | bool   | `true`        | Enable/disable             |
| `grid_cols`/`grid_rows`            | int    | `3`/`3`       | Grid dimensions            |
| `keys`                             | string | `"rtyfghvbn"` | Cell selection keys        |
| `min_size_width`/`min_size_height` | int    | `1`/`1`       | Minimum cell size          |
| `max_depth`                        | int    | `10`          | Max recursion depth (1–20) |
| `layers`                           | array  | `[]`          | Per-depth layout overrides |

**Animation:**

| Option                  | Type | Default | Description                        |
| :---------------------- | :--- | :------ | :--------------------------------- |
| `animation.enabled`     | bool | `true`  | Enable depth transition animations |
| `animation.duration_ms` | int  | `50`    | Animation duration in milliseconds |

**UI:** `font_size`, `line_width`, `label_background`, `sub_key_preview`, colors.

### `[scroll]`

| Option             | Type | Default   | Description       |
| :----------------- | :--- | :-------- | :---------------- |
| `scroll_step`      | int  | `50`      | Pixels per scroll |
| `scroll_step_half` | int  | `500`     | Half-page pixels  |
| `scroll_step_full` | int  | `1000000` | Top/bottom jump   |
| `invert_scroll`    | bool | `false`   | Invert direction  |

Default hotkeys: `j`/`k` up/down, `h`/`l` left/right, `gg`/`G` top/bottom, `u`/`d` page up/down.

### Other Sections

#### Animation & Motion

**`[smooth_cursor]`** — Animated mouse movement. `move_mouse_enabled`, `steps`, `max_duration`.

**`[smooth_scroll]`** — macOS only. Chunked ease-out scroll events. `enabled`, `steps`, `max_duration`.

**`[held_repeat]`** — Repeat scroll/page/move while held. `enabled`, `initial_delay_ms`, `interval_ms`.

#### Visual Indicators

**`[mode_indicator]`** — Floating label showing current mode. Per-mode text, colors, and per-mode toggle.

**`[mouse_action_indicator]`** — Transient visual marker at click locations. macOS only. `enabled`, `size`, `shape`, animation settings.

**`[virtual_pointer]`** — Small dot at cursor position in hold mode. `size`, `color`.

**`[sticky_modifiers]`** — Tap modifiers to make them sticky. `enabled`, `tap_max_duration`, indicator UI.

#### Interaction & Input

**`[monitor_select]`** — Interactive display picking. `enabled`, `characters`, `ui`.

**`[systray]`** — System tray icon. `enabled` (default `true`; requires restart).

#### Diagnostics

**`[logging]`** — `log_level`, `log_file`, `disable_file_logging`, rotation settings.

Default log paths: macOS `~/Library/Logs/neru/app.log`, Linux `~/.local/state/neru/log/app.log`, Windows `%LOCALAPPDATA%\neru\log\app.log`.

---

## Recipes

### Vimium-Style Click-on-Select

```toml
[hotkeys]
"Primary+Shift+Space" = "hints --action left_click"
```

### Homerow-Style Actions

```toml
[hints.hotkeys]
"Enter"       = "action left_click"
"Shift+Enter" = "action right_click"
```

### Auto-Exit After Click

```toml
[hints.hotkeys]
"Shift+L" = ["action left_click", "idle"]
"Shift+R" = ["action right_click", "idle"]
```

### Search Hints

Press `/` to search by title, description, or value:

```toml
[hints.hotkeys]
"/" = "action search_hints"
```

Features: `Space` for multi-word, `Backspace` removes chars, `Escape` cancels, `Enter` auto-selects.

### Start with Search Visible

```toml
[hotkeys]
"Primary+Shift+Space" = "hints --search"
```

### Restore Cursor After Mode Exit

```toml
[hotkeys]
"Primary+Shift+Space" = ["action save_cursor_pos", "hints"]

[hints.hotkeys]
"Enter" = ["action left_click", "idle", "action restore_cursor_pos"]
```

### Custom Mouse Movement Step Size

```toml
[hints.hotkeys]
"Up"    = "action move_mouse_relative --dx=0 --dy=-10"
"Down"  = "action move_mouse_relative --dx=0 --dy=10"
```

### Mode Toggle On/Off

```toml
[hotkeys]
"Ctrl+F" = "grid --toggle"
"Ctrl+G" = "recursive_grid --toggle"
```

### Target Menus Without Moving Cursor

```toml
[hotkeys]
"Primary+Shift+G" = "grid --cursor-selection-mode hold"
```

Toggle cursor-follow on the fly (backtick by default):

```toml
[grid.hotkeys]
"`" = "toggle-cursor-follow-selection"
```

### Sequencing & Timing

```toml
# Click, sleep, move (e.g. Discord)
[recursive_grid.hotkeys]
"Ctrl+J" = ["action left_click", "action sleep 0.05", "action reset"]

# Give browser content time to load
[[hints.app_configs]]
bundle_id = "com.brave.Browser"
hotkeys = { "Return" = ["action left_click", "action sleep 0.8", "hints"] }

# Compose complex sequences
[hints.hotkeys]
"Enter"  = ["action save_cursor_pos", "idle", "action wait_for_mode_exit", "action restore_cursor_pos"]
"Ctrl+Z" = ["monitor_select", "action wait_for_mode_exit --bail", "recursive_grid"]
```

### Disable All Built-In Hotkeys

```toml
[hotkeys]
# No bindings — all defaults cleared
```

Use with external hotkey managers (skhd, Hammerspoon, Raycast):

```bash
# skhdrc
ctrl - f : neru hints
ctrl - g : neru grid
```

### Accessibility & Debugging

Check the Accessibility Tree:

- **UIElementInspector** (lightweight, no Xcode): [Apple Developer](https://developer.apple.com/library/mac/samplecode/UIElementInspector/UIElementInspector.zip)
- **Accessibility Inspector** (with Xcode): **Xcode → Open Developer Tool → Accessibility Inspector**

Edit config directly:

```bash
neru status 2>&1 | grep Config | awk '{print $2}' | xargs nvim
```

---

Use `neru doctor` and runtime logs to troubleshoot configuration issues. See [Troubleshooting Guide](TROUBLESHOOTING.md).
